import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot, save } from '../helpers/api.mjs';
import { review, stats, enqueue, finished, decide, decideIndividually, control, held, expectStats } from '../helpers/review.mjs';
import { inspectExport } from '../helpers/zip.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { waitFor } from '../helpers/process.mjs';
import { randomUUID } from 'node:crypto';
import pg from 'pg';

test.beforeEach(async () => { expect(process.env.P1_AI_FIXTURE_PHASE).toBe('fixture_not_real_model'); });

const emptyCounts = { validImages: 0, suspiciousImages: 0, failedImages: 0, candidates: 0, pending: 0,
  accepted: 0, adjusted: 0, rejected: 0, riskyCandidates: 0, unresolvedRiskCandidates: 0,
  unresolvedRiskImages: 0, manualAdded: 0 };
const boxCounts = { ...emptyCounts, validImages: 1, suspiciousImages: 1, candidates: 3, pending: 3,
  riskyCandidates: 2, unresolvedRiskCandidates: 2, unresolvedRiskImages: 1 };
const byX = value => [...value.candidates].sort((a, b) => a.bbox[0] - b.bbox[0]);
async function fresh(request) {
  const video = await upload(request); const { frames } = await extract(request, video.id);
  return { video, frame: frames[0], frames };
}
async function rejectedUnchanged(request, frame, operation, statuses = [422]) {
  const before = await review(request, frame);
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 1 });
  const rows = async () => {
    const values = {};
    for (const [table, key] of [['annotations', 'id'], ['ai_candidates', 'id'], ['annotation_provenance', 'annotation_id'],
      ['annotation_events', 'id'], ['frame_review_issues', 'id']]) values[table] = (await pool.query(
      `SELECT row_to_json(t) AS value FROM ${table} t WHERE frame_id IN (SELECT id FROM frames WHERE video_id=$1) ORDER BY ${key}`,
      [frame.videoId])).rows;
    values.frames = (await pool.query('SELECT row_to_json(t) AS value FROM frames t WHERE video_id=$1 ORDER BY id', [frame.videoId])).rows;
    return values;
  };
  try {
    const databaseBefore = await rows();
    const response = await operation(before);
    expect(statuses).toContain(response.status());
    expect(await review(request, frame)).toEqual(before);
    expect(await rows()).toEqual(databaseBefore);
  } finally { await pool.end(); }
}

test('P1-REVIEW/STATS/GATE-001 fixture decisions, AI edits, tombstone undo and reruns preserve exact provenance and denominators', async ({ request }) => {
  const { video, frame, frames } = await fresh(request);
  const prior = { id: randomUUID(), bbox: [180, 100, 25, 25], source: 'manual' };
  await save(request, frame, [prior]);
  expectStats(await stats(request, video), emptyCounts);
  const start = await review(request, frame);
  const queued = await enqueue(request, frame, start.revision);
  let value = await finished(request, frame);
  expect(value.annotations).toEqual([prior]);
  const [good, low, duplicate] = byX(value);
  expect(byX(value).map(c => c.bbox)).toEqual([[10, 10, 20, 20], [70, 20, 10, 10], [72, 22, 10, 10]]);
  expect(good.riskReasons).toEqual([]);
  expect(low.riskReasons.sort()).toEqual(['LOW_CONFIDENCE', 'POSSIBLE_DUPLICATE', 'SMALL_HEAD']);
  expect(duplicate.riskReasons.sort()).toEqual(['POSSIBLE_DUPLICATE', 'SMALL_HEAD']);
  expectStats(await stats(request, video), boxCounts);
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'completed', annotations: before.annotations } }));
  expect((await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } })).status()).toBe(422);
  expect((await request.post(`/api/v1/frames/${frame.id}/preannotations`, {
    headers: { 'Idempotency-Key': randomUUID() }, data: { expectedRevision: value.revision } })).status()).toBe(409);
  const added = { id: randomUUID(), bbox: [230, 100, 20, 20], source: 'manual' };
  await save(request, frame, [prior, added]);
  value = await review(request, frame);
  expectStats(await stats(request, video), { ...boxCounts, manualAdded: 1 });
  value = await decide(request, frame, value, [{ candidateId: good.id, decision: 'accepted' }]);
  expectStats(await stats(request, video), { ...boxCounts, pending: 2, accepted: 1, manualAdded: 1 });
  value = await decide(request, frame, value, [{ candidateId: low.id, decision: 'adjusted', bbox: [100, 20, 14, 14] }]);
  expectStats(await stats(request, video), { ...boxCounts, pending: 1, accepted: 1, adjusted: 1,
    unresolvedRiskCandidates: 1, manualAdded: 1 });
  value = await decide(request, frame, value, [{ candidateId: duplicate.id, decision: 'rejected' }]);
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, accepted: 1, adjusted: 1, rejected: 1,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0, manualAdded: 1 });
  const accepted = value.annotations.find(a => a.id === value.candidates.find(c => c.id === good.id).annotationId);
  expect(accepted).toEqual({ id: accepted.id, bbox: good.bbox, source: 'ai', candidateId: good.id, runId: good.runId });
  const originalId = accepted.id;
  const moved = value.annotations.map(a => a.id === originalId ? { ...a, bbox: [12, 10, 20, 20], source: 'ai_corrected' } : a);
  await save(request, frame, moved);
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, adjusted: 2, rejected: 1,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0, manualAdded: 1 });
  await save(request, frame, moved.filter(a => a.id !== originalId));
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, adjusted: 1, rejected: 2,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0, manualAdded: 1 });
  // An unchanged original ID is the only legal undo of the deleted AI mapping.
  await save(request, frame, [...moved.filter(a => a.id !== originalId), accepted]);
  value = await review(request, frame);
  expect(value.candidates.find(c => c.id === good.id).reviewState).toBe('accepted');
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, accepted: 1, adjusted: 1, rejected: 1,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0, manualAdded: 1 });
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'in_progress', annotations: before.annotations.map(a =>
      a.id === originalId ? { id: a.id, bbox: a.bbox, source: 'manual' } : a) } }));
  await rejectedUnchanged(request, frames[1], before => request.put(`/api/v1/frames/${frames[1].id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'in_progress', annotations: [accepted] } }));
  await rejectedUnchanged(request, frames[1], before => request.put(`/api/v1/frames/${frames[1].id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'in_progress', annotations: [{ id: accepted.id, bbox: accepted.bbox, source: 'manual' }] } }));
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'in_progress', annotations: before.annotations.map(a =>
      a.id === originalId ? { ...a, candidateId: duplicate.id } : a) } }));
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: before.revision, status: 'in_progress', annotations: [...before.annotations,
      { id: randomUUID(), bbox: [1, 1, 20, 20], source: 'ai', candidateId: good.id, runId: good.runId }] } }), [400, 422]);
  const replay = await enqueue(request, frame, queued.revision, queued.key);
  expect(replay.job.id).toBe(queued.job.id);
  const conflicting = await request.post(`/api/v1/frames/${frame.id}/preannotations`, {
    headers: { 'Idempotency-Key': queued.key }, data: { expectedRevision: value.revision } });
  expect(conflicting.status()).toBe(409);
  const labels = value.annotations;
  await enqueue(request, frame, value.revision);
  value = await finished(request, frame);
  expect(value.currentRun.id).not.toBe(good.runId);
  expect(value.annotations).toEqual(labels);
  expectStats(await stats(request, video), { ...boxCounts, riskyCandidates: 3, unresolvedRiskCandidates: 3, manualAdded: 0 });
  const overlap = byX(value)[0];
  expect(overlap.riskReasons).toContain('OVERLAP_EXISTING');
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/review`, {
    data: { expectedRevision: before.revision, decisions: [{ candidateId: overlap.id, decision: 'accepted' }], issueResolutions: [] } }));
  value = await decideIndividually(request, frame, value, value.candidates.map(c => ({ candidateId: c.id, decision: 'rejected',
    ...(c.riskReasons.includes('OVERLAP_EXISTING') ? { reasonCode: 'ALREADY_LABELED' } : {}) })));
  await save(request, frame, labels, 'completed');
  const archive = await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } });
  expect(archive.status()).toBe(200);
  inspectExport(await archive.body(), [{ ...frame, annotations: labels,
    imageBytes: await (await request.get(frame.imageUrl)).body() }]);
});

test('P1-AIRACE/STATS-001 latest stale attempt remains visible while old active-run evidence and statistics survive', async ({ request }) => {
  const { video, frame } = await fresh(request);
  await enqueue(request, frame, (await review(request, frame)).revision);
  let value = await finished(request, frame);
  value = await decideIndividually(request, frame, value, value.candidates.map(c => ({ candidateId: c.id, decision: 'rejected' })));
  const active = value.currentRun.id;
  await control({ mode: 'boxes', hold: true });
  await enqueue(request, frame, value.revision);
  const id = await held();
  const manual = { id: randomUUID(), bbox: [140, 50, 25, 25], source: 'manual' };
  const saved = await save(request, frame, [manual], 'completed');
  await control({ release: id });
  value = await finished(request, frame, 'stale');
  expect(value.currentRun.id).not.toBe(active); expect(value.activeRunId).toBe(active);
  expect(value.revision).toBe(saved.revision); expect(value.annotations).toEqual([manual]);
  expect(value.candidates).toHaveLength(3);
  expect(value.candidates.every(c => c.runId === active && c.reviewState === 'rejected')).toBe(true);
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, rejected: 3,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0, manualAdded: 1 });
});

test('P1-REVIEW-001 raw risky fixture candidates block bulk accept but permit an explicit individual acceptance', async ({ request }) => {
  const { video, frame } = await fresh(request);
  await enqueue(request, frame, (await review(request, frame)).revision);
  let value = await finished(request, frame);
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: before.revision, decisions: [{ candidateId: before.candidates[0].id, decision: 'accepted', riskReasons: [] }],
    issueResolutions: [] } }), [400, 422]);
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: before.revision, decisions: [{ candidateId: before.candidates[0].id, decision: 'adjusted', bbox: [null, 2, 15, 15] }],
    issueResolutions: [] } }), [400, 422]);
  await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: before.revision, decisions: before.candidates.map(c => ({ candidateId: c.id, decision: 'accepted' })),
    issueResolutions: [] } }));
  const raw = byX(value)[2];
  value = await decide(request, frame, value, [{ candidateId: raw.id, decision: 'accepted' }]);
  const accepted = value.candidates.find(c => c.id === raw.id);
  expect(accepted.riskReasons).toEqual(raw.riskReasons); expect(accepted.reviewState).toBe('accepted');
  expect(value.annotations).toEqual([{ id: accepted.annotationId, bbox: raw.bbox, source: 'ai', candidateId: raw.id, runId: raw.runId }]);
  value = await decideIndividually(request, frame, value, value.candidates.filter(c => c.reviewState === 'pending')
    .map(c => ({ candidateId: c.id, decision: 'rejected' })));
  expectStats(await stats(request, video), { ...boxCounts, pending: 0, accepted: 1, rejected: 2,
    unresolvedRiskCandidates: 0, unresolvedRiskImages: 0 });
});

for (const mode of ['empty', 'failure', 'null-bbox']) {
  test(`P1-GATE/STATS-001 ${mode} fixture requires explicit image confirmation and has nullable zero denominators`, async ({ request }) => {
    const { video, frame } = await fresh(request);
    await control({ mode });
    await enqueue(request, frame, (await review(request, frame)).revision);
    let value = await finished(request, frame, mode === 'empty' ? 'completed' : 'failed');
    expect(value.candidates).toEqual([]);
    expect(value.annotations).toEqual([]);
    expect(value.issues).toHaveLength(1);
    expect(value.issues[0]).toMatchObject({ reasonCode: mode === 'empty' ? 'NO_DETECTIONS' : 'AI_FAILED', resolved: false });
    expectStats(await stats(request, video), { ...emptyCounts, validImages: mode === 'empty' ? 1 : 0,
      suspiciousImages: mode === 'empty' ? 1 : 0, failedImages: mode === 'empty' ? 0 : 1, unresolvedRiskImages: 1 });
    await rejectedUnchanged(request, frame, before => request.put(`/api/v1/frames/${frame.id}/annotations`, {
      data: { expectedRevision: before.revision, status: 'completed', annotations: [] } }));
    expect((await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } })).status()).toBe(422);
    const box = { id: randomUUID(), bbox: [140, 50, 25, 25], source: 'manual' };
    await save(request, frame, mode === 'empty' ? [] : [box]);
    value = await review(request, frame);
    await decide(request, frame, value, [], value.issues.map(issue => ({ id: issue.id, resolution: 'manual_checked' })));
    const labels = mode === 'empty' ? [] : [box];
    await save(request, frame, labels, 'completed');
    const archive = await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } });
    expect(archive.status()).toBe(200);
    inspectExport(await archive.body(), [{ ...frame, annotations: labels,
      imageBytes: await (await request.get(frame.imageUrl)).body() }]);
  });
}

for (const mode of ['boxes', 'failure']) {
  test(`P1-AIRACE-001 held ${mode} fixture cannot overwrite a newer manual save`, async ({ request }) => {
    const { video, frame } = await fresh(request);
    await control({ mode, hold: true });
    await enqueue(request, frame, (await review(request, frame)).revision);
    const id = await held();
    const manual = { id: randomUUID(), bbox: [140, 50, 25, 25], source: 'manual' };
    const saved = await save(request, frame, [manual], 'completed');
    await control({ release: id });
    const value = await finished(request, frame, mode === 'boxes' ? 'stale' : 'failed');
    expect(value.revision).toBe(saved.revision); expect(value.status).toBe('completed');
    expect(value.annotations).toEqual([manual]); expect(value.candidates).toEqual([]); expect(value.issues).toEqual([]);
    expectStats(await stats(request, video), emptyCounts);
  });
}

test('P1-AIRACE-001 cancellation of an actual held HTTP request creates no candidates or failure issue', async ({ request }) => {
  const { frame } = await fresh(request);
  await control({ mode: 'boxes', hold: true });
  const queued = await enqueue(request, frame, (await review(request, frame)).revision);
  await held();
  const cancelled = await request.post(`/api/v1/jobs/${queued.job.id}/cancel`);
  expect(cancelled.status()).toBe(200);
  const value = await finished(request, frame, 'cancelled');
  expect(value.candidates).toEqual([]); expect(value.issues).toEqual([]);
  await expect.poll(async () => (await control()).held.length, { timeout: 5000 }).toBe(0);
});

test('P1-AIRACE-001 concurrent identical keys wait at a real PG barrier and create exactly one job and run', async ({ request }) => {
  const { frame } = await fresh(request);
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 3 });
  const locker = await pool.connect(); let pending;
  try {
    const before = await snapshot(request, frame.id); const key = randomUUID();
    await locker.query('BEGIN');
    const pid = Number((await locker.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
    await locker.query('SELECT pg_advisory_xact_lock(8941202)');
    pending = [enqueue(request, frame, before.revision, key), enqueue(request, frame, before.revision, key)];
    pending.forEach(promise => { void promise.catch(() => {}); });
    await waitFor(async () => Number((await pool.query(`SELECT count(*) FROM pg_stat_activity
      WHERE datname=current_database() AND pid<>$1 AND state='active' AND wait_event_type='Lock'
      AND $1::integer=ANY(pg_blocking_pids(pid))`, [pid])).rows[0].count) >= 2, 5000, 'Two actual HTTP enqueue transactions reached the PostgreSQL barrier');
    await locker.query('COMMIT');
    const results = await Promise.all(pending); expect(results[0].job.id).toBe(results[1].job.id);
    await finished(request, frame);
    expect(Number((await pool.query('SELECT count(*) FROM ai_runs WHERE frame_id=$1', [frame.id])).rows[0].count)).toBe(1);
    expect(Number((await pool.query('SELECT count(*) FROM ai_candidates WHERE frame_id=$1', [frame.id])).rows[0].count)).toBe(3);
  } finally {
    await locker.query('ROLLBACK'); locker.release();
    if (pending) await Promise.allSettled(pending); await pool.end();
  }
});
