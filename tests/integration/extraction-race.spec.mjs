import { test, expect } from '../helpers/p1-session.mjs';
import { upload, terminalJob } from '../helpers/api.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import { waitFor } from '../helpers/process.mjs';
import pg from 'pg';

test('P1-LOCAL-001 actual FFmpeg publication locks the job before a competing cancellation', async ({ request }) => {
  const video = await upload(request);
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 3 });
  const barrier = await pool.connect();
  const key = 71420002;
  let created = false, trigger = false, cancellation;
  try {
    const barrierPid = Number((await barrier.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
    await barrier.query('SELECT pg_advisory_lock($1)', [key]);
    await pool.query(`CREATE FUNCTION p1_test_publish_barrier() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN PERFORM pg_advisory_xact_lock(${key}); RETURN NEW; END $$`);
    created = true;
    await pool.query('CREATE TRIGGER p1_test_publish_barrier BEFORE INSERT ON batches FOR EACH ROW EXECUTE FUNCTION p1_test_publish_barrier()');
    trigger = true;
    const queued = await request.post(`/api/v1/videos/${video.id}/extractions`, { data: {
      intervalSeconds: 1, startSeconds: 0, endSeconds: 3 } });
    expect(queued.status()).toBe(202);
    const job = await queued.json();
    const publicationPid = await waitFor(async () => (await pool.query(`SELECT pid FROM pg_stat_activity
      WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)) AND query ILIKE '%INSERT INTO batches%'`,
    [barrierPid])).rows[0]?.pid, 15_000, 'Real FFmpeg output reached the publication transaction');
    cancellation = request.post(`/api/v1/jobs/${job.id}/cancel`);
    void cancellation.catch(() => {});
    await waitFor(async () => (await pool.query(`SELECT count(*) FROM pg_stat_activity
      WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid))`, [Number(publicationPid)])).rows[0].count > 0,
    5000, 'Actual cancellation waits behind the product publication job lock');
    await barrier.query('SELECT pg_advisory_unlock($1)', [key]);
    const cancelled = await cancellation;
    expect(cancelled.status()).toBe(200);
    expect((await cancelled.json()).status).toBe('completed');
    expect((await terminalJob(request, job.id)).status).toBe('completed');
    expect(Number((await pool.query('SELECT count(*) FROM batches WHERE job_id=$1', [job.id])).rows[0].count)).toBe(1);
    expect(Number((await pool.query('SELECT count(*) FROM frames WHERE batch_id=$1', [job.batchId])).rows[0].count)).toBe(3);
  } finally {
    await barrier.query('SELECT pg_advisory_unlock($1)', [key]);
    if (cancellation) await cancellation.catch(() => {});
    if (trigger) await pool.query('DROP TRIGGER p1_test_publish_barrier ON batches');
    if (created) await pool.query('DROP FUNCTION p1_test_publish_barrier()');
    barrier.release(); await pool.end();
  }
});
