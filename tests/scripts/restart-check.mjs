import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';

// Dedicated process only. Never restart an arbitrary existing/cloud deployment.
const root = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));
const binary = process.env.P0_BACKEND_BINARY;
const dataDir = process.env.P0_RESTART_DATA_DIR;
const fixtureDir = process.env.P0_FIXTURE_DIR;
assert.ok(binary && dataDir && fixtureDir, 'Set P0_BACKEND_BINARY, P0_RESTART_DATA_DIR and P0_FIXTURE_DIR.');
const resolved = path.resolve(dataDir);
const relative = path.relative(root, resolved);
assert.ok(relative.startsWith('..') || path.isAbsolute(relative), 'Restart data must be outside repository');
assert.ok(!fs.existsSync(resolved), 'Restart check requires a new dedicated external data directory');
const port = process.env.P0_RESTART_PORT || await unusedLoopbackPort();
const origin = `http://127.0.0.1:${port}`;
let child;
const environment = {
  ...process.env, APP_ADDR: `127.0.0.1:${port}`, APP_DATA_DIR: resolved,
  APP_VERSION: 'synthetic-restart-check', APP_MAX_FRAMES: '1000',
  APP_FFMPEG: process.env.FFMPEG_PATH || 'ffmpeg', APP_FFPROBE: process.env.FFPROBE_PATH || 'ffprobe',
};
async function waitFor(predicate, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try { const result = await predicate(); if (result) return result; } catch {}
    await delay(100);
  }
  throw new Error('Dedicated restart check timed out');
}
async function start() {
  child = spawn(binary, [], { env: environment, stdio: ['ignore', 'ignore', 'ignore'] });
  child.on('error', error => { console.error(error.message); });
  await waitFor(async () => (await fetch(`${origin}/health/ready`)).ok);
}
async function stop() {
  if (!child || child.exitCode !== null) return;
  const exited = new Promise(resolve => child.once('exit', resolve));
  child.kill('SIGKILL');
  await exited;
}
async function json(route, options) {
  const response = await fetch(`${origin}/api/v1${route}`, options);
  assert.ok(response.ok, `HTTP ${response.status} on ${route}`);
  return response.json();
}
function post(body) { return { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }; }
try {
  await start();
  const form = new FormData();
  form.set('file', new Blob([fs.readFileSync(path.join(fixtureDir, 'classroom.mp4'))], { type: 'video/mp4' }), 'classroom.mp4');
  const video = await json('/videos', { method: 'POST', body: form });
  const finished = await json(`/videos/${video.id}/extractions`, post({ intervalSeconds: 1, endSeconds: 1 }));
  await waitFor(async () => (await json(`/jobs/${finished.id}`)).status === 'completed');
  const { frames } = await json(`/videos/${video.id}/frames?batchId=${finished.batchId}`);
  const annotation = { id: crypto.randomUUID(), bbox: [1, 2, 30, 40], source: 'manual' };
  const saved = await json(`/frames/${frames[0].id}/annotations`, {
    ...post({ expectedRevision: 0, status: 'completed', annotations: [annotation] }), method: 'PUT',
  });
  const interrupted = await json(`/videos/${video.id}/extractions`, post({ intervalSeconds: 0.004, endSeconds: 3 }));
  await waitFor(async () => (await json(`/jobs/${interrupted.id}`)).status === 'running');
  await stop();
  await start();
  assert.deepEqual(await json(`/frames/${frames[0].id}/annotations`), saved, 'saved annotation survives restart');
  const recovered = await json(`/jobs/${interrupted.id}`);
  assert.equal(recovered.status, 'failed');
  assert.ok(recovered.error.includes('INTERRUPTED'));
  const batches = await json(`/videos/${video.id}/batches`);
  assert.ok(!batches.batches.some(batch => batch.id === interrupted.batchId));
  const unpublished = await json(`/videos/${video.id}/frames`);
  assert.ok(!unpublished.frames.some(frame => frame.batchId === interrupted.batchId));
  console.log('P0-SAVE/JOB restart: saved snapshot persisted, running job failed INTERRUPTED, partial batch absent.');
} finally { await stop(); }
