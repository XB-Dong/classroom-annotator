import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot, save } from '../helpers/api.mjs';
import { control, enqueue, held, finished, review } from '../helpers/review.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { waitFor } from '../helpers/process.mjs';
import { randomUUID } from 'node:crypto';
import pg from 'pg';
import fs from 'node:fs';
import path from 'node:path';

test.beforeEach(async () => { expect(process.env.P1_AI_FIXTURE_PHASE).toBe('fixture_not_real_model'); });

test('P1-AIRACE/CAS-001 second actual API preserves the first active job and both instances share transactional CAS', async ({ request, playwright }) => {
  const video = await upload(request); const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 3 });
  let follower, barrier;
  const evidence = [];
  try {
    await control({ mode: 'boxes', hold: true });
    const queued = await enqueue(request, frame, (await review(request, frame)).revision);
    const heldId = await held();
    expect((await review(request, frame)).currentRun.status).toBe('running');
    await control({ action: 'start-follower' });
    const login = await playwright.request.newContext({ baseURL: process.env.P1_FOLLOWER_ORIGIN });
    try {
      const result = await login.post('/api/v1/auth/login', { headers: { Origin: process.env.P0_BASE_URL },
        data: { username: 'p1_account_two', password: 'synthetic-browser-password-2026' } });
      expect(result.status()).toBe(200); const session = await result.json();
      follower = await playwright.request.newContext({ baseURL: process.env.P1_FOLLOWER_ORIGIN,
        storageState: await login.storageState(), extraHTTPHeaders: {
          Origin: process.env.P0_BASE_URL, 'X-CSRF-Token': session.csrfToken } });
    } finally { await login.dispose(); }
    const observed = await follower.get(`/api/v1/jobs/${queued.job.id}`);
    expect(observed.status()).toBe(200); expect((await observed.json()).status).toBe('running');
    expect((await review(follower, frame)).currentRun.status).toBe('running');
    const followerTask = await follower.post(`/api/v1/frames/${frames[2].id}/preannotations`, { headers: {
      'Idempotency-Key': randomUUID() }, data: { expectedRevision: (await review(follower, frames[2])).revision } });
    expect(followerTask.status()).toBe(503);
    expect((await control()).held).toEqual([heldId]);
    await control({ release: heldId });
    const value = await finished(request, frame);
    expect(value.candidates).toHaveLength(3);
    evidence.push({ followerStartedDuringActualAIRequest: true, firstJobAfterFollowerStart: 'running',
      oneHeldRequest: true, firstJobPublishedOnce: true });
    // The two actual processes contend on the same PostgreSQL frame row.
    barrier = await pool.connect();
    for (let round = 0; round < 20; round++) {
      const target = round === 0 ? frame : frames[1];
      const before = await review(request, target);
      const auditBefore = Number((await pool.query('SELECT count(*) FROM annotation_events WHERE frame_id=$1', [target.id])).rows[0].count);
      const values = [10, 150].map(x => ({ expectedRevision: before.revision, status: 'in_progress',
        annotations: [{ id: randomUUID(), bbox: [x, 150, 20, 20], source: 'manual' }] }));
      await barrier.query('BEGIN');
      const pid = Number((await barrier.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
      await barrier.query('SELECT id FROM frames WHERE id=$1 FOR UPDATE', [target.id]);
      const primaryWrite = round === 0 ? request.put(`/api/v1/frames/${target.id}/review`, { data: {
        expectedRevision: before.revision, decisions: [{ candidateId: before.candidates.find(c => c.bbox[0] === 10).id,
          decision: 'accepted' }], issueResolutions: [] } }) : request.put(`/api/v1/frames/${target.id}/annotations`, { data: values[0] });
      const pending = [primaryWrite, follower.put(`/api/v1/frames/${target.id}/annotations`, { data: values[1] })];
      pending.forEach(promise => { void promise.catch(() => {}); });
      try {
        await waitFor(async () => Number((await pool.query(`WITH RECURSIVE blocked(pid) AS (
          SELECT $1::integer
          UNION
          SELECT a.pid FROM pg_stat_activity a JOIN blocked b ON b.pid=ANY(pg_blocking_pids(a.pid))
          WHERE a.datname=current_database() AND a.state='active' AND a.wait_event_type='Lock'
        ) SELECT count(*) FROM blocked WHERE pid<>$1`, [pid])).rows[0].count) >= 2,
        5000, 'Both separate API processes reached the PostgreSQL row barrier');
        await barrier.query('COMMIT');
      } finally { await barrier.query('ROLLBACK'); }
      const replies = await Promise.all(pending);
      expect(replies.map(r => r.status()).sort()).toEqual([200, 409]);
      const winner = replies.findIndex(r => r.status() === 200);
      const after = await review(request, target);
      expect(after.revision).toBe(before.revision + 1);
      expect(Number((await pool.query('SELECT count(*) FROM annotation_events WHERE frame_id=$1', [target.id])).rows[0].count)).toBe(auditBefore + 1);
      if (round === 0) {
        const good = after.candidates.find(c => c.bbox[0] === 10);
        expect(good.reviewState).toBe(winner === 0 ? 'accepted' : 'pending');
        expect(Number((await pool.query('SELECT count(*) FROM annotation_provenance WHERE frame_id=$1', [target.id])).rows[0].count)).toBe(winner === 0 ? 1 : 0);
        expect(after.annotations).toHaveLength(1);
        expect(after.annotations[0].source).toBe(winner === 0 ? 'ai' : 'manual');
      } else expect(after.annotations).toEqual(values[winner].annotations);
      expect(await review(follower, target)).toEqual(after);
      evidence.push({ round: round + 1, separateProductProcesses: true, databaseBarrier: true,
        statuses: replies.map(r => r.status()), auditDelta: 1, revision: after.revision });
    }
  } finally {
    if (barrier) { await barrier.query('ROLLBACK'); barrier.release(); }
    if (follower) await follower.dispose(); await pool.end();
    await control({ action: 'stop-follower' });
    fs.writeFileSync(path.join(process.env.P1_STACK_EVIDENCE, 'follower-cas-evidence.json'), JSON.stringify({
      sourceSHA: process.env.P0_EXPECTED_SHA, dataType: 'fixture_not_real_model', evidence }, null, 2));
  }
});

test('P1-DEGRADE-001 scheduler lease loss is permanent until restart while recovered DB still permits manual work', async ({ request }) => {
  const video = await upload(request); const { frames } = await extract(request, video.id); const frame = frames[0];
  try {
    await control({ action: 'disconnect-primary-db' });
    await expect.poll(async () => (await request.get('/health/ready')).status(), { timeout: 10_000 }).toBe(503);
    await control({ action: 'reconnect-primary-db' });
    await expect.poll(async () => (await request.get('/api/v1/videos')).status(), { timeout: 15_000 }).toBe(200);
    const box = { id: randomUUID(), bbox: [100, 100, 20, 20], source: 'manual' };
    const saved = await save(request, frame, [box], 'completed');
    expect(await snapshot(request, frame.id)).toEqual(saved);
    const extraction = await request.post(`/api/v1/videos/${video.id}/extractions`, { data: {
      intervalSeconds: 1, startSeconds: 0, endSeconds: 3 } });
    expect(extraction.status()).toBe(503);
    const ai = await request.post(`/api/v1/frames/${frame.id}/preannotations`, { headers: {
      'Idempotency-Key': randomUUID() }, data: { expectedRevision: saved.revision } });
    expect(ai.status()).toBe(503);
    await control({ action: 'restart-primary' });
    await enqueue(request, frame, saved.revision);
    expect((await finished(request, frame)).annotations).toEqual([box]);
  } finally {
    await control({ action: 'reconnect-primary-db' });
    // Restore an owned scheduler even if an earlier assertion failed.
    await control({ action: 'restart-primary' });
  }
});
