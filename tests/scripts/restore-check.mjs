import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { inspectExport } from '../helpers/zip.mjs';
import { acceptance, externalPath, dedicatedConnection, openPostgres, businessRows, mediaReferenceRows } from '../helpers/postgres.mjs';
import { dedicatedServer } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { api } from '../helpers/http.mjs';

// Leader executes the restricted dump/restore; this script only starts its own
// synthetic server against the separate restored target and observes semantics.
await acceptance(async () => {
  const file = externalPath(process.env.P0_RESTORE_BASELINE, 'P0_RESTORE_BASELINE');
  const baseline = JSON.parse(fs.readFileSync(file, 'utf8'));
  assert.equal(baseline.dataType, 'synthetic');
  assert.equal(baseline.deployedSHA, process.env.P0_EXPECTED_SHA);
  const root = externalPath(baseline.dataDir, 'Baseline media directory');
  const dump = externalPath(process.env.P01_RESTORE_DUMP, 'P01_RESTORE_DUMP');
  const dumpBytes = fs.readFileSync(dump);
  assert.equal(dumpBytes.subarray(0, 5).toString(), 'PGDMP', 'Require real pg_dump custom format');
  const connection = dedicatedConnection('P01_RESTORE_DATABASE_URL_FILE');
  assert.notEqual(connection.database, baseline.database, 'Restore must use a separate isolated database');
  const pool = await openPostgres('P01_RESTORE_DATABASE_URL_FILE');
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  let server;
  try {
    assert.deepEqual(await businessRows(pool), baseline.businessRows, 'Restored five-table values differ before server recovery');
    assert.deepEqual(await mediaReferenceRows(pool), baseline.mediaReferenceRows, 'Restored source/provider/media references differ');
    server = await dedicatedServer(process.env.P0_BACKEND_BINARY, { APP_DATABASE_URL: connection.connectionString,
      APP_DATA_DIR: root, APP_VERSION: baseline.deployedSHA, APP_MAX_FRAMES: '1000',
      APP_FFMPEG: process.env.FFMPEG_PATH, APP_FFPROBE: process.env.FFPROBE_PATH }, origin);
    const http = api(origin);
    assert.equal((await http.json('/health/ready')).version, baseline.deployedSHA);
    assert.deepEqual(await http.json(`/api/v1/videos/${baseline.videoId}/progress?batchId=${baseline.batchId}`), baseline.progress);
    const expected = [];
    for (const frame of baseline.frames) {
      const snapshot = await http.snapshot(frame.id);
      assert.deepEqual(snapshot, frame.snapshot, 'Restored revision/status/annotations');
      const response = await http.request(frame.imageUrl);
      assert.equal(response.status, 200);
      const bytes = Buffer.from(await response.arrayBuffer());
      assert.equal(createHash('sha256').update(bytes).digest('hex'), frame.imageSHA256, 'Restored original JPEG bytes');
      if (snapshot.status === 'completed') expected.push({ ...frame, annotations: snapshot.annotations, imageBytes: bytes });
    }
    const response = await http.request('/api/v1/exports', http.body({ videoId: baseline.videoId, batchId: baseline.batchId }));
    assert.equal(response.status, 200);
    assert.deepEqual(inspectExport(Buffer.from(await response.arrayBuffer()), expected), baseline.coco, 'Restored COCO IDs/dimensions/geometry');
    const evidence = { observedAt: new Date().toISOString(), sourceSHA: baseline.deployedSHA, dataType: 'synthetic',
      sourceDatabase: baseline.database, restoredDatabase: connection.database,
      dumpSHA256: createHash('sha256').update(dumpBytes).digest('hex'), dumpBytes: dumpBytes.length,
      businessRowsUnchanged: true, mediaReferencesUnchanged: true, snapshotsUnchanged: true, progressUnchanged: true,
      immutableJPEGsUnchanged: true, cocoUnchanged: true, completedFrames: expected.length };
    fs.writeFileSync(path.join(root, 'restore-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log('P01-DB-007 passed: separate real PostgreSQL restore preserves business rows, saved revisions, progress, immutable JPEGs and independent COCO semantics.');
  } finally { if (server) await server.stop(); await pool.end(); }
});
