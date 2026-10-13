import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, terminalJob, save } from '../helpers/api.mjs';
import { control, review, stats, expectStats, assertTerminalAIJob } from '../helpers/review.mjs';
import { validateSchema } from '../helpers/contract.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { randomUUID } from 'node:crypto';
import pg from 'pg';

test('P1-DEGRADE/AIRACE/STATS-001 fixture batch retains one published result and one explicit failure across idempotent retries', async ({ request }) => {
  expect(process.env.P1_AI_FIXTURE_PHASE).toBe('fixture_not_real_model');
  const video = await upload(request); const { frames } = await extract(request, video.id);
  const selected = frames.slice(0, 2); const key = randomUUID();
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 1 });
  try {
    await control({ sequence: [{ mode: 'boxes' }, { mode: 'failure' }] });
    const options = { headers: { 'Idempotency-Key': key }, data: { frameIds: selected.map(f => f.id) } };
    const response = await request.post(`/api/v1/batches/${selected[0].batchId}/preannotations`, options);
    expect(response.status()).toBe(202);
    const job = validateSchema('Job', await response.json());
    expect(job.kind).toBe('preannotation');
    const terminal = await terminalJob(request, job.id);
    expect(terminal.status).toBe('failed');
    const values = await Promise.all(selected.map(frame => review(request, frame)));
    expect(values.map(value => value.currentRun.status).sort()).toEqual(['completed', 'failed']);
    assertTerminalAIJob(terminal, values);
    const success = values.find(value => value.currentRun.status === 'completed');
    const failure = values.find(value => value.currentRun.status === 'failed');
    expect(success.candidates).toHaveLength(3); expect(success.issues).toEqual([]);
    expect(failure.candidates).toEqual([]);
    expect(failure.issues).toHaveLength(1); expect(failure.issues[0].reasonCode).toBe('AI_FAILED');
    expectStats(await stats(request, video), { validImages: 1, suspiciousImages: 1, failedImages: 1,
      candidates: 3, pending: 3, accepted: 0, adjusted: 0, rejected: 0, riskyCandidates: 2,
      unresolvedRiskCandidates: 2, unresolvedRiskImages: 2, manualAdded: 0 });
    const retry = await request.post(`/api/v1/batches/${selected[0].batchId}/preannotations`, options);
    expect(retry.status()).toBe(202); expect((await retry.json()).id).toBe(job.id);
    const conflict = await request.post(`/api/v1/batches/${selected[0].batchId}/preannotations`, {
      headers: options.headers, data: { frameIds: [selected[0].id] } });
    expect(conflict.status()).toBe(409);
    expect(await Promise.all(selected.map(frame => review(request, frame)))).toEqual(values);
    expect(Number((await pool.query('SELECT count(*) FROM ai_runs WHERE job_id=$1', [job.id])).rows[0].count)).toBe(2);
    expect(Number((await pool.query('SELECT count(*) FROM ai_requests WHERE job_id=$1', [job.id])).rows[0].count)).toBe(1);
    // A partial inference failure leaves manual saving available on both frames.
    for (const frame of selected) await save(request, frame, [{ id: randomUUID(), bbox: [180, 120, 20, 20], source: 'manual' }]);
  } finally { await pool.end(); }
});
