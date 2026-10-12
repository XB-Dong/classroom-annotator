import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

const root = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));
const binary = process.env.P0_BACKEND_BINARY;
const dataDir = process.env.P0_DATASET_DATA_DIR;
assert.ok(binary && dataDir && process.env.P0_FIXTURE_DIR, 'Set P0_BACKEND_BINARY, P0_DATASET_DATA_DIR, P0_FIXTURE_DIR');
const resolved = path.resolve(dataDir);
const relative = path.relative(root, resolved);
assert.ok(relative.startsWith('..') || path.isAbsolute(relative), 'Dataset data must be outside repository');
assert.ok(!fs.existsSync(resolved), 'Use a new dedicated external data directory');
const mediaBytes = fs.readFileSync(path.join(process.env.P0_FIXTURE_DIR, 'classroom.mp4'));
const key = 'synthetic-acceptance-key';
const apiCalls = [];
const mediaHeaders = [];
let invalid = false;
let failing = false;
let violation;
let child;
async function listen(server) { await new Promise(resolve => server.listen(0, '127.0.0.1', resolve)); return `http://127.0.0.1:${server.address().port}`; }
const media = http.createServer((request, response) => {
  mediaHeaders.push(request.headers);
  if (request.headers['x-api-key'] || request.headers.authorization) violation = 'API credentials reached media host';
  response.writeHead(200, { 'Content-Type': 'video/mp4', 'Content-Length': mediaBytes.length });
  response.end(mediaBytes);
});
const mediaOrigin = await listen(media);
const upstream = http.createServer((request, response) => {
  const url = new URL(request.url, 'http://fixture');
  apiCalls.push({ path: url.pathname, page: url.searchParams.get('page'), pageSize: url.searchParams.get('page_size') });
  if (request.headers['x-api-key'] !== key) violation = 'API request missing configured X-API-Key';
  response.setHeader('Content-Type', 'application/json');
  if (url.pathname === '/api/v1/datasets/42/lessons') {
    const video = channel => ({ id: channel, lesson_id: 7, catalog_file_id: channel, channel, part_index: 0, tags: {} });
    response.end(JSON.stringify({ type: 'lessons', items: [{ id: invalid ? 9007199254740992 : 7, course_name: 'Synthetic', organization_name: 'Fixture', teacher: 'Synthetic teacher', date: '2026-01-01', section_index: 1, videos: [video(1), video(4)] }], total: 2, stats: {} }));
  } else if (url.pathname === '/api/v1/catalog-files/4/play-url') {
    if (failing) { response.statusCode = 502; response.end(JSON.stringify({ error: 'synthetic upstream failure' })); }
    else response.end(JSON.stringify({ url: `${mediaOrigin}/synthetic.mp4` }));
  } else { response.statusCode = 404; response.end('{}'); }
});
const apiOrigin = await listen(upstream);
const origin = `http://127.0.0.1:${process.env.P0_DATASET_PORT || '18182'}`;
async function waitFor(predicate, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try { const result = await predicate(); if (result) return result; } catch {}
    await delay(100);
  }
  throw new Error('Dataset integration timed out');
}
async function get(route) {
  const response = await fetch(`${origin}/api/v1${route}`);
  return { response, body: await response.json() };
}
async function importJob() {
  const response = await fetch(`${origin}/api/v1/dataset/imports`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ catalogFileId: '4', name: 'Synthetic API import' }) });
  assert.equal(response.status, 202);
  const job = await response.json();
  assert.equal(job.kind, 'import');
  assert.equal(job.batchId, undefined);
  return waitFor(async () => {
    const current = (await get(`/jobs/${job.id}`)).body;
    return ['failed', 'completed'].includes(current.status) && current;
  }, 60_000);
}
try {
  child = spawn(binary, [], {
    env: { ...process.env, APP_ADDR: origin.replace('http://', ''), APP_DATA_DIR: resolved, APP_VERSION: 'synthetic-dataset-check', APP_FFMPEG: process.env.FFMPEG_PATH || 'ffmpeg', APP_FFPROBE: process.env.FFPROBE_PATH || 'ffprobe', APP_DATASET_API_URL: apiOrigin, APP_DATASET_ID: '42', APP_DATASET_API_KEY: key, APP_ALLOWED_PRIVATE_ORIGINS: `${apiOrigin},${mediaOrigin}`, APP_ALLOWED_MEDIA_ORIGINS: mediaOrigin },
    stdio: ['ignore', 'ignore', 'ignore'],
  });
  child.on('error', error => { console.error(error.message); });
  await waitFor(async () => (await fetch(`${origin}/health/ready`)).ok);
  const list = await get('/dataset/lessons?page=2&pageSize=1');
  assert.equal(list.response.status, 200);
  assert.deepEqual(list.body.items[0].videos.map(video => video.channel), [1, 4]);
  assert.ok(apiCalls.some(call => call.path === '/api/v1/datasets/42/lessons' && call.page === '2' && call.pageSize === '1'));
  const completed = await importJob();
  assert.equal(completed.status, 'completed');
  assert.equal(typeof completed.videoId, 'string');
  const videos = (await get('/videos')).body.videos;
  const imported = videos.find(video => video.id === completed.videoId);
  assert.deepEqual([imported.width, imported.height], [320, 240]);
  assert.ok(mediaHeaders.length > 0);
  assert.equal(violation, undefined);
  invalid = true;
  const invalidList = await get('/dataset/lessons');
  assert.equal(invalidList.response.status, 502);
  assert.equal(invalidList.body.code, 'DATASET_INVALID_RESPONSE');
  invalid = false;
  failing = true;
  const failed = await importJob();
  assert.equal(failed.status, 'failed');
  assert.equal(failed.videoId, undefined);
  assert.ok(!JSON.stringify(failed).includes(key));
  console.log('P0-VID-002A: observed pagination/path/schema, import/play URL, media key isolation, unsafe ID rejection, upstream failure passed.');
} finally {
  if (child && child.exitCode === null) {
    const exited = new Promise(resolve => child.once('exit', resolve));
    child.kill('SIGKILL');
    await exited;
  }
  upstream.close();
  media.close();
}
