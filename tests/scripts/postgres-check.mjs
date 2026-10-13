import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import AdmZip from 'adm-zip';
import { acceptance, externalPath, dedicatedConnection, openPostgres, assertEmptyBusiness, businessRows, mediaReferenceRows } from '../helpers/postgres.mjs';
import { command, dedicatedServer, waitFor } from '../helpers/process.mjs';
import { api } from '../helpers/http.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { inspectExport } from '../helpers/zip.mjs';
import { postgresProxy } from '../helpers/postgres-proxy.mjs';

const box = (id, x = 10) => ({ id, bbox: [x, 20, 30, 40], source: 'manual' });

async function databaseAvailability(http, pool, relay, frame) {
  await waitFor(() => relay.stats().active <= 2, 3_000, 'Default product pool idle bound');
  assert.ok(relay.stats().peak > 1 && relay.stats().peak <= 5, 'Default product pool must use concurrent bounded connections');
  const schema = (await pool.query('SELECT id,version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1')).rows[0];
  try {
    await pool.query('UPDATE goose_db_version SET version_id=0 WHERE id=$1', [schema.id]);
    const response = await http.request('/health/ready');
    assert.equal(response.status, 503, 'Unexpected schema must make readiness fail');
    const value = await response.json();
    assert.equal(value.code, 'NOT_READY');
  } finally { await pool.query('UPDATE goose_db_version SET version_id=$1 WHERE id=$2', [schema.version_id, schema.id]); }
  assert.equal((await http.request('/health/ready')).status, 200);
  const before = await http.snapshot(frame.id);
  relay.disconnect();
  const started = Date.now();
  try {
    const ready = await http.request('/health/ready');
    assert.equal(ready.status, 503);
    const request = await http.put(frame.id, { expectedRevision: before.revision, status: 'completed', annotations: [box(randomUUID())] });
    assert.ok(request.status >= 500 && request.status < 600, 'Disconnected DB must refuse business writes safely');
    assert.ok(Date.now() - started < 15_000, 'Disconnected DB request exceeded bounded timeout');
  } finally { relay.reconnect(); }
  await waitFor(async () => (await http.request('/health/ready')).status === 200, 15_000, 'DB reconnect readiness');
  assert.deepEqual(await http.snapshot(frame.id), before, 'Failed disconnected write changed saved snapshot');
  return { schemaMismatchReadiness: 503, disconnectedReadiness: 503, failedWriteUnchanged: true, requestElapsedMs: Date.now() - started, pool: relay.stats() };
}

async function assertDatabaseSnapshot(pool, frameId, snapshot) {
  const frame = (await pool.query('SELECT revision,status FROM frames WHERE id=$1', [frameId])).rows[0];
  assert.deepEqual({ revision: Number(frame.revision), status: frame.status }, { revision: snapshot.revision, status: snapshot.status });
  const annotations = (await pool.query('SELECT id,x,y,width,height,source FROM annotations WHERE frame_id=$1 ORDER BY id', [frameId])).rows;
  assert.deepEqual(annotations.map(row => ({ id: row.id, bbox: [row.x, row.y, row.width, row.height], source: row.source })), [...snapshot.annotations].sort((a, b) => a.id.localeCompare(b.id)));
}

async function sameRevision(http, pool, frame) {
  const before = await http.snapshot(frame.id);
  const payloads = Array.from({ length: 20 }, (_, index) => ({ expectedRevision: before.revision, status: 'completed', annotations: [box(randomUUID(), index + 1)] }));
  const responses = await Promise.all(payloads.map(value => http.put(frame.id, value)));
  assert.equal(responses.filter(value => value.status === 200).length, 1, 'Same-revision race did not have exactly one winner');
  assert.equal(responses.filter(value => value.status === 409).length, 19, 'Same-revision race losers must all return 409');
  const winner = responses.findIndex(value => value.status === 200);
  const expected = { frameId: frame.id, revision: before.revision + 1, status: 'completed', annotations: payloads[winner].annotations };
  assert.deepEqual(await http.snapshot(frame.id), expected, 'Losers changed the committed winning snapshot');
  await assertDatabaseSnapshot(pool, frame.id, expected);
  return { requests: 20, statuses: responses.map(value => value.status), winner, revision: expected.revision };
}

async function crossFrameRace(http, pool, frames) {
  const prior = [];
  for (const frame of frames) {
    const current = await http.snapshot(frame.id);
    const seeded = await http.put(frame.id, { expectedRevision: current.revision, status: 'in_progress', annotations: [box(randomUUID(), 3)] });
    assert.equal(seeded.status, 200);
    prior.push(seeded.body);
  }
  const firstId = `p01-a-${randomUUID()}`;
  const secondId = `p01-z-${randomUUID()}`;
  const competing = [[box(firstId), box(secondId, 100)], [box(secondId, 100), box(firstId)]];
  const lock = await pool.connect();
  const lockerPid = Number((await lock.query('SELECT pg_backend_pid() AS pid')).rows[0].pid);
  await lock.query('BEGIN');
  try {
    await lock.query('SELECT id FROM frames WHERE id=ANY($1::text[]) ORDER BY id FOR UPDATE', [frames.map(frame => frame.id)]);
    const pending = frames.map((frame, index) => http.put(frame.id, { expectedRevision: prior[index].revision, status: 'completed', annotations: competing[index] }));
    await waitFor(async () => Number((await pool.query('SELECT count(*) AS waiting FROM pg_stat_activity WHERE datname=current_database() AND pg_blocking_pids(pid) @> ARRAY[$1::integer]', [lockerPid])).rows[0].waiting) >= 2,
      3_000, 'Both cross-frame writers blocked on independent PostgreSQL row locks');
    await lock.query('COMMIT');
    const responses = await Promise.all(pending);
    assert.deepEqual(responses.map(value => value.status).sort(), [200, 400], 'Cross-frame competing IDs must have one 200 and one safe 400');
    for (let index = 0; index < frames.length; index++) {
      const winner = responses[index].status === 200;
      const current = await http.snapshot(frames[index].id);
      if (winner) {
        assert.equal(current.revision, prior[index].revision + 1);
        assert.deepEqual([...current.annotations].sort((a, b) => a.id.localeCompare(b.id)), [...competing[index]].sort((a, b) => a.id.localeCompare(b.id)));
      } else assert.deepEqual(current, prior[index], 'Rejected cross-frame transaction partially changed its prior annotations/revision');
      await assertDatabaseSnapshot(pool, frames[index].id, current);
    }
    const owners = (await pool.query('SELECT DISTINCT frame_id FROM annotations WHERE id=ANY($1::text[])', [[firstId, secondId]])).rows;
    assert.equal(owners.length, 1, 'Global annotation IDs were split across frames');
    return { statuses: responses.map(value => value.status), annotationOwner: owners[0].frame_id };
  } finally { await lock.query('ROLLBACK'); lock.release(); }
}

async function snapshotRace(http, pool, frames) {
  const annotated = frames.slice(0, -2);
  const zero = frames.at(-2);
  const unfinished = frames.at(-1);
  for (const frame of frames) {
    const current = await http.snapshot(frame.id);
    const saved = await http.put(frame.id, { expectedRevision: current.revision, status: frame.id === unfinished.id ? 'in_progress' : 'completed', annotations: frame.id === zero.id || frame.id === unfinished.id ? [] : [box(randomUUID(), 10)] });
    assert.equal(saved.status, 200);
  }
  const initialSnapshot = await http.snapshot(annotated[0].id);
  const imageBytes = new Map(await Promise.all(frames.map(async frame => {
    const response = await http.request(frame.imageUrl);
    assert.equal(response.status, 200);
    return [frame.id, Buffer.from(await response.arrayBuffer())];
  })));
  let stop = false;
  let transitions = 0;
  let writerError;
  const writer = (async () => {
    const connection = await pool.connect();
    try {
      while (!stop) {
        await connection.query('BEGIN');
        try {
          await connection.query('UPDATE frames SET revision=revision+1 WHERE id=ANY($1::text[])', [annotated.map(frame => frame.id)]);
          await connection.query('UPDATE annotations SET x=$1 WHERE frame_id=ANY($2::text[])', [transitions % 2 ? 10 : 100, annotated.map(frame => frame.id)]);
          await connection.query('COMMIT');
          transitions++;
        } catch (error) { await connection.query('ROLLBACK'); throw error; }
        await delay(1);
      }
    } finally { connection.release(); }
  })().catch(error => { writerError = error; stop = true; });
  let exported = 0;
  try {
    for (; exported < 100; exported++) {
      const snapshot = await http.snapshot(annotated[0].id);
      assert.equal(snapshot.annotations.length, 1);
      assert.equal(snapshot.annotations[0].bbox[0], (snapshot.revision - initialSnapshot.revision) % 2 ? 100 : 10,
        'Annotation read mixed a revision and boxes from separately committed PostgreSQL snapshots');
      const response = await http.request('/api/v1/exports', http.body({ frameIds: frames.map(frame => frame.id) }));
      assert.equal(response.status, 200);
      const bytes = Buffer.from(await response.arrayBuffer());
      const archive = new AdmZip(bytes);
      const coco = JSON.parse(archive.readAsText('annotations.json'));
      assert.equal(coco.annotations.length, annotated.length);
      const values = new Set(coco.annotations.map(annotation => annotation.bbox[0]));
      assert.equal(values.size, 1, 'Export mixed annotations from separately committed PostgreSQL snapshots');
      const x = [...values][0];
      assert.ok([10, 100].includes(x));
      inspectExport(bytes, frames.filter(frame => frame.id !== unfinished.id).map(frame => ({ ...frame, imageBytes: imageBytes.get(frame.id), annotations: frame.id === zero.id ? [] : [box('independent-geometry-check', x)] })));
    }
  } finally { stop = true; await writer; }
  if (writerError) throw writerError;
  assert.ok(transitions >= 20, 'Export test did not overlap enough real PostgreSQL writes');
  return { exports: exported, annotationSnapshots: exported, atomicWriterCommits: transitions, annotatedFrames: annotated.length, zeroHeadFrameIncluded: true, unfinishedFrameExcluded: true };
}

await acceptance(async () => {
  assert.match(process.env.P0_EXPECTED_SHA || '', /^[0-9a-f]{40}$/, 'Set exact binary source SHA');
  const root = externalPath(process.env.P01_API_OUTPUT, 'P01_API_OUTPUT');
  assert.ok(!fs.existsSync(root), 'Use a fresh external API acceptance directory');
  const fixture = externalPath(path.join(process.env.P0_FIXTURE_DIR || '', 'classroom.mp4'), 'Synthetic video fixture');
  const bytes = fs.readFileSync(fixture);
  const connection = dedicatedConnection();
  const pool = await openPostgres();
  const relay = await postgresProxy(connection.connectionString);
  const environment = { APP_DATABASE_URL: relay.connectionString, APP_DATA_DIR: root,
    APP_VERSION: process.env.P0_EXPECTED_SHA, APP_FFMPEG: process.env.FFMPEG_PATH,
    APP_FFPROBE: process.env.FFPROBE_PATH, APP_MAX_FRAMES: '1000' };
  let server;
  try {
    await command(process.env.P01_MIGRATE_BINARY, ['up'], environment);
    await assertEmptyBusiness(pool);
    fs.mkdirSync(path.join(root, 'videos'), { recursive: true });
    fs.writeFileSync(path.join(root, 'videos', 'p01-test.mp4'), bytes);
    const videoId = randomUUID();
    await pool.query('INSERT INTO videos (id,name,width,height,duration_seconds,status,created_at,storage_key,source_kind,storage_provider,media_sha256,media_size_bytes) VALUES ($1,$2,320,240,4,\'ready\',$3,$4,\'legacy\',\'local\',$5,$6)', [videoId, 'Synthetic PG acceptance', new Date().toISOString(), path.join('videos', 'p01-test.mp4'), createHash('sha256').update(bytes).digest('hex'), bytes.length]);
    const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
    server = await dedicatedServer(process.env.P0_BACKEND_BINARY, environment, origin);
    const http = api(origin);
    const health = await http.json('/health/ready');
    assert.equal(health.version, process.env.P0_EXPECTED_SHA);
    const job = await http.json(`/api/v1/videos/${videoId}/extractions`, http.body({ intervalSeconds: 0.5, startSeconds: 0, endSeconds: 3 }), 202);
    assert.equal((await http.terminal(job.id)).status, 'completed');
    const { frames } = await http.json(`/api/v1/videos/${videoId}/frames?batchId=${job.batchId}`);
    assert.equal(frames.length, 6, 'Real FFmpeg extraction did not publish all planned frames');
    const evidence = { sourceSHA: process.env.P0_EXPECTED_SHA, observedAt: new Date().toISOString(), dataType: 'synthetic',
      sameRevision: await sameRevision(http, pool, frames[0]),
      crossFrame: await crossFrameRace(http, pool, frames.slice(1, 3)),
      exportSnapshot: await snapshotRace(http, pool, frames),
      databaseAvailability: await databaseAvailability(http, pool, relay, frames[0]) };
    const snapshots = await Promise.all(frames.map(async frame => ({ ...frame, snapshot: await http.snapshot(frame.id),
      imageSHA256: createHash('sha256').update(Buffer.from(await (await http.request(frame.imageUrl)).arrayBuffer())).digest('hex') })));
    const exportResponse = await http.request('/api/v1/exports', http.body({ videoId, batchId: job.batchId }));
    assert.equal(exportResponse.status, 200);
    const exportBytes = Buffer.from(await exportResponse.arrayBuffer());
    const coco = JSON.parse(new AdmZip(exportBytes).readAsText('annotations.json'));
    const baseline = { observedAt: new Date().toISOString(), deployedSHA: process.env.P0_EXPECTED_SHA, dataType: 'synthetic',
      dataDir: root, database: connection.database, videoId, batchId: job.batchId, coco, businessRows: await businessRows(pool),
      mediaReferenceRows: await mediaReferenceRows(pool),
      zipSHA256: createHash('sha256').update(exportBytes).digest('hex'), frames: snapshots,
      progress: await http.json(`/api/v1/videos/${videoId}/progress?batchId=${job.batchId}`) };
    fs.writeFileSync(path.join(root, 'restore-baseline.json'), JSON.stringify(baseline, null, 2));
    fs.writeFileSync(path.join(root, 'postgres-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log('P01-DB-003/004 passed: real PostgreSQL 20-writer CAS, opposite-ID cross-frame race, concurrent atomic-snapshot exports and original JPEG verification.');
  } finally { if (server) await server.stop(); await relay.stop(); await pool.end(); }
});
