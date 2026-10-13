import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { acceptance, externalPath, dedicatedConnection, openPostgres, businessRows, mediaReferenceRows } from '../helpers/postgres.mjs';
import { dedicatedServer, waitFor } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { api } from '../helpers/http.mjs';
import { inspectExport } from '../helpers/zip.mjs';

// Own-process only: this script never restarts an existing deployment.
await acceptance(async () => {
  const file = externalPath(process.env.P0_RESTORE_BASELINE, 'P0_RESTORE_BASELINE');
  const baseline = JSON.parse(fs.readFileSync(file, 'utf8'));
  const connection = dedicatedConnection();
  assert.equal(baseline.database, connection.database, 'Restart baseline belongs to another isolated database');
  assert.equal(baseline.dataType, 'synthetic', 'Restart only synthetic acceptance data');
  assert.equal(baseline.deployedSHA, process.env.P0_EXPECTED_SHA, 'Restart binary source SHA differs from baseline');
  const root = externalPath(baseline.dataDir, 'Baseline media directory');
  assert.ok(file.startsWith(`${root}${path.sep}`), 'Baseline must belong to the synthetic media directory');
  const pool = await openPostgres();
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const environment = { APP_DATABASE_URL: connection.connectionString, APP_DATA_DIR: root,
    APP_VERSION: baseline.deployedSHA, APP_MAX_FRAMES: '1000', APP_FFMPEG: process.env.FFMPEG_PATH,
    APP_FFPROBE: process.env.FFPROBE_PATH };
  let server;
  try {
    server = await dedicatedServer(process.env.P0_BACKEND_BINARY, environment, origin);
    const http = api(origin);
    const interrupted = await http.json(`/api/v1/videos/${baseline.videoId}/extractions`, http.body({ intervalSeconds: 0.004, startSeconds: 0, endSeconds: 3 }), 202);
    await waitFor(async () => (await http.json(`/api/v1/jobs/${interrupted.id}`)).status === 'running', 30_000, 'Interrupted extraction running');
    await server.stop();
    server = await dedicatedServer(process.env.P0_BACKEND_BINARY, environment, origin);
    const recovered = await http.json(`/api/v1/jobs/${interrupted.id}`);
    assert.equal(recovered.status, 'failed');
    assert.match(recovered.error, /INTERRUPTED/);
    assert.equal(Number((await pool.query('SELECT count(*) AS count FROM batches WHERE id=$1', [interrupted.batchId])).rows[0].count), 0);
    assert.equal(Number((await pool.query('SELECT count(*) AS count FROM frames WHERE batch_id=$1', [interrupted.batchId])).rows[0].count), 0);
    assert.deepEqual(await http.json(`/api/v1/videos/${baseline.videoId}/progress?batchId=${baseline.batchId}`), baseline.progress);
    const expected = [];
    for (const frame of baseline.frames) {
      assert.deepEqual(await http.snapshot(frame.id), frame.snapshot, 'Saved revision/status/boxes changed after actual process restart');
      const response = await http.request(frame.imageUrl);
      assert.equal(response.status, 200);
      const bytes = Buffer.from(await response.arrayBuffer());
      assert.equal(createHash('sha256').update(bytes).digest('hex'), frame.imageSHA256, 'Immutable JPEG changed after process restart');
      if (frame.snapshot.status === 'completed') expected.push({ ...frame, annotations: frame.snapshot.annotations, imageBytes: bytes });
    }
    const response = await http.request('/api/v1/exports', http.body({ videoId: baseline.videoId, batchId: baseline.batchId }));
    assert.equal(response.status, 200);
    assert.deepEqual(inspectExport(Buffer.from(await response.arrayBuffer()), expected), baseline.coco, 'Export semantics changed after process restart');
    baseline.businessRows = await businessRows(pool);
    baseline.mediaReferenceRows = await mediaReferenceRows(pool);
    baseline.observedAt = new Date().toISOString();
    fs.writeFileSync(file, JSON.stringify(baseline, null, 2));
    fs.writeFileSync(path.join(root, 'restart-evidence.json'), JSON.stringify({ sourceSHA: baseline.deployedSHA, observedAt: new Date().toISOString(),
      interruptedJob: { id: interrupted.id, status: recovered.status, error: recovered.error }, frameCount: baseline.frames.length,
      savedSnapshotsUnchanged: true, immutableJPEGsUnchanged: true, progressUnchanged: true, cocoUnchanged: true }, null, 2));
    console.log('P01-DB-005 restart passed: actual PostgreSQL saved snapshots and original JPEG/COCO survive; interrupted extraction is failed with no published batch.');
  } finally { if (server) await server.stop(); await pool.end(); }
});
