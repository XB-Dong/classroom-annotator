import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, terminalJob } from '../helpers/api.mjs';
import { enqueue, review, assertTerminalAIJob } from '../helpers/review.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { waitFor } from '../helpers/process.mjs';
import pg from 'pg';
import fs from 'node:fs';
import path from 'node:path';

test('P1-AIRACE-001 actual AI publication holds the job lock before a competing cancellation and preserves committed results', async ({ request }) => {
  expect(process.env.P1_AI_FIXTURE_PHASE).toBe('fixture_not_real_model');
  const video = await upload(request); const { frames } = await extract(request, video.id); const frame = frames[0];
  expect(frame.id).toMatch(/^[a-f0-9-]{36}$/);
  const before = await review(request, frame);
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 3 });
  const barrier = await pool.connect(); const key = 71420003;
  const evidence = { dataType: 'fixture_not_real_model', sourceSHA: process.env.P0_EXPECTED_SHA, frameId: frame.id,
    publicationBlocked: false, cancellationBlockedOnPublisher: false, status: 'running' };
  let functionCreated = false, triggerCreated = false, cancellation;
  try {
    const pid = Number((await barrier.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
    await barrier.query('SELECT pg_advisory_lock($1)', [key]);
    await pool.query(`CREATE FUNCTION p1_test_ai_publish_barrier() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN IF NEW.frame_id=TG_ARGV[0] THEN PERFORM pg_advisory_xact_lock(${key}); END IF; RETURN NEW; END $$`);
    functionCreated = true;
    await pool.query(`CREATE TRIGGER p1_test_ai_publish_barrier BEFORE INSERT ON ai_candidates
      FOR EACH ROW EXECUTE FUNCTION p1_test_ai_publish_barrier('${frame.id}')`);
    triggerCreated = true;
    const queued = await enqueue(request, frame, before.revision);
    evidence.jobId = queued.job.id;
    const publisher = await waitFor(async () => (await pool.query(`SELECT pid FROM pg_stat_activity
      WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid))
        AND query ILIKE '%INSERT INTO ai_candidates%'`, [pid])).rows[0]?.pid,
    15_000, 'Actual fixture output reached the PostgreSQL candidate publication transaction');
    evidence.publicationBlocked = true;
    cancellation = request.post(`/api/v1/jobs/${queued.job.id}/cancel`);
    void cancellation.catch(() => {});
    await waitFor(async () => Number((await pool.query(`SELECT count(*) FROM pg_stat_activity
      WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid))
        AND wait_event_type='Lock' AND query ILIKE '%jobs%'`, [Number(publisher)])).rows[0].count) > 0,
    5000, 'Actual cancellation waits behind the publication job lock');
    evidence.cancellationBlockedOnPublisher = true;
    await barrier.query('SELECT pg_advisory_unlock($1)', [key]);
    const reply = await cancellation; expect(reply.status()).toBe(200);
    const terminal = await terminalJob(request, queued.job.id);
    // Published per-frame results survive cancellation; cancellation may win
    // the following job-finalization transaction, which is a separate outcome.
    expect(['completed', 'cancelled']).toContain(terminal.status);
    const after = await review(request, frame);
    evidence.timingAndResults = assertTerminalAIJob(terminal, [after]);
    expect(after.currentRun.status).toBe('completed'); expect(after.revision).toBe(before.revision + 1);
    expect(after.activeRunId).toBe(after.currentRun.id); expect(after.annotations).toEqual([]);
    expect(after.candidates).toHaveLength(3); expect(after.candidates.every(c => c.reviewState === 'pending')).toBe(true);
    expect(after.issues).toEqual([]);
    expect(Number((await pool.query('SELECT count(*) FROM ai_candidates WHERE run_id=$1', [after.currentRun.id])).rows[0].count)).toBe(3);
    expect(Number((await pool.query('SELECT count(*) FROM annotation_events WHERE frame_id=$1 AND action=$2', [frame.id, 'ai_publish'])).rows[0].count)).toBe(1);
    evidence.status = 'passed'; evidence.terminalJob = terminal.status; evidence.publishedCandidateCount = 3;
  } finally {
    await barrier.query('SELECT pg_advisory_unlock($1)', [key]);
    if (cancellation) await cancellation.catch(() => {});
    if (triggerCreated) await pool.query('DROP TRIGGER p1_test_ai_publish_barrier ON ai_candidates');
    if (functionCreated) await pool.query('DROP FUNCTION p1_test_ai_publish_barrier()');
    barrier.release(); await pool.end();
    if (evidence.status !== 'passed') evidence.status = 'failed';
    fs.writeFileSync(path.join(process.env.P1_STACK_EVIDENCE, 'ai-publication-cancel-evidence.json'), JSON.stringify(evidence, null, 2));
  }
});
