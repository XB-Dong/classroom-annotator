import assert from 'node:assert/strict';
import { waitFor } from './process.mjs';

const waitingOn = async (pool, pid) => (await pool.query(
  'SELECT pid,query FROM pg_stat_activity WHERE datname=current_database() AND pg_blocking_pids(pid) @> ARRAY[$1::integer]', [pid])).rows;

async function assertPublished(pool, job, status, count) {
  const row = (await pool.query('SELECT status FROM jobs WHERE id=$1', [job.id])).rows[0];
  assert.equal(row.status, status);
  assert.equal(Number((await pool.query('SELECT count(*) AS count FROM batches WHERE job_id=$1', [job.id])).rows[0].count), count ? 1 : 0);
  assert.equal(Number((await pool.query('SELECT count(*) AS count FROM frames WHERE batch_id=$1', [job.batchId])).rows[0].count), count);
}

export async function cancelPublishRace(http, pool, bridge, videoId) {
  const evidence = {};
  const locker = await pool.connect();
  const lockerPid = Number((await locker.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
  try {
    const job = await http.json(`/api/v1/videos/${videoId}/extractions`, http.body({ intervalSeconds: 1, startSeconds: 0, endSeconds: 3 }), 202);
    const operation = await bridge.operationForJob(job.id);
    await locker.query('BEGIN');
    await locker.query('SELECT id FROM jobs WHERE id=$1 FOR UPDATE', [job.id]);
    const cancel = http.json(`/api/v1/jobs/${job.id}/cancel`, http.body({}));
    await waitFor(async () => (await waitingOn(pool, lockerPid)).length >= 1, 3_000, 'Cancellation blocked on held PostgreSQL job row');
    bridge.complete(operation.operationId);
    await waitFor(async () => (await waitingOn(pool, lockerPid)).length >= 2, 3_000, 'Bridge completion also blocked behind cancellation');
    await locker.query('COMMIT');
    await cancel;
    const terminal = await http.terminal(job.id);
    assert.equal(terminal.status, 'cancelled', 'Cancellation queued first must prevent publication');
    await assertPublished(pool, job, 'cancelled', 0);
    evidence.cancelFirst = { jobId: job.id, terminal: terminal.status, observedBlockedClients: 2, publishedFrames: 0 };
  } finally { await locker.query('ROLLBACK'); locker.release(); }

  // A database-only trigger pauses INSERT after the product transaction locks
  // its job row. The competing cancel must wait on that product transaction.
  const advisory = await pool.connect();
  const advisoryPid = Number((await advisory.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
  const lockKey = 71420001;
  let functionCreated = false;
  let triggerCreated = false;
  try {
    await advisory.query('SELECT pg_advisory_lock($1)', [lockKey]);
    await pool.query(`CREATE FUNCTION p01_test_publish_barrier() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(${lockKey}); RETURN NEW; END $$`);
    functionCreated = true;
    await pool.query('CREATE TRIGGER p01_test_publish_barrier BEFORE INSERT ON batches FOR EACH ROW EXECUTE FUNCTION p01_test_publish_barrier()');
    triggerCreated = true;
    const job = await http.json(`/api/v1/videos/${videoId}/extractions`, http.body({ intervalSeconds: 1, startSeconds: 0, endSeconds: 3 }), 202);
    const operation = await bridge.operationForJob(job.id);
    bridge.complete(operation.operationId);
    const publication = await waitFor(async () => (await waitingOn(pool, advisoryPid)).find(row => /INSERT INTO batches/i.test(row.query)), 4_000,
      'Product publication holds job row and waits at dedicated batch trigger');
    const cancel = http.json(`/api/v1/jobs/${job.id}/cancel`, http.body({}));
    await waitFor(async () => (await waitingOn(pool, Number(publication.pid))).length >= 1, 3_000,
      'Cancellation blocked on the product publication transaction');
    await advisory.query('SELECT pg_advisory_unlock($1)', [lockKey]);
    const cancellation = await cancel;
    const terminal = await http.terminal(job.id);
    assert.equal(cancellation.status, 'completed', 'Publication queued first must remain completed');
    assert.equal(terminal.status, 'completed');
    await assertPublished(pool, job, 'completed', 3);
    evidence.publishFirst = { jobId: job.id, terminal: terminal.status, observedProductJobLock: true, publishedFrames: 3 };
  } finally {
    await advisory.query('SELECT pg_advisory_unlock($1)', [lockKey]);
    advisory.release();
    if (triggerCreated) await pool.query('DROP TRIGGER p01_test_publish_barrier ON batches');
    if (functionCreated) await pool.query('DROP FUNCTION p01_test_publish_barrier()');
  }
  evidence.fixture = 'synthetic bridge barriers and dedicated PostgreSQL locks';
  return evidence;
}
