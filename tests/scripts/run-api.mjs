import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';

const root = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));
const binary = process.env.P0_BACKEND_BINARY;
const dataDir = process.env.P0_API_DATA_DIR;
assert.ok(binary && dataDir && process.env.P0_FIXTURE_DIR, 'Set P0_BACKEND_BINARY, P0_API_DATA_DIR and P0_FIXTURE_DIR');
const resolved = path.resolve(dataDir);
const relative = path.relative(root, resolved);
assert.ok(relative.startsWith('..') || path.isAbsolute(relative), 'API acceptance data must be outside repository');
assert.ok(!fs.existsSync(resolved), 'Use a new dedicated external API data directory');
const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
const version = process.env.P0_EXPECTED_SHA || 'synthetic-local-api-check';
const child = spawn(binary, [], {
  env: { ...process.env, APP_ADDR: origin.replace('http://', ''), APP_DATA_DIR: resolved, APP_VERSION: version, APP_FFMPEG: process.env.FFMPEG_PATH || 'ffmpeg', APP_FFPROBE: process.env.FFPROBE_PATH || 'ffprobe' },
  stdio: ['ignore', 'ignore', 'ignore'],
});
let startupError;
child.on('error', error => { startupError = error; });
try {
  const deadline = Date.now() + 30_000;
  let ready = false;
  while (Date.now() < deadline) {
    if (startupError) throw startupError;
    if (child.exitCode !== null) throw new Error(`Dedicated backend exited ${child.exitCode}`);
    try { ready = (await fetch(`${origin}/health/ready`)).ok; } catch {}
    if (ready) break;
    await delay(100);
  }
  assert.ok(ready, 'Dedicated backend readiness timed out');
  const cli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url));
  const runner = spawn(process.execPath, [cli, 'test', '--project=api', ...process.argv.slice(2)], {
    env: { ...process.env, P0_BASE_URL: origin, P0_EXPECTED_SHA: version }, stdio: 'inherit',
  });
  const code = await new Promise((resolve, reject) => { runner.once('error', reject); runner.once('exit', resolve); });
  process.exitCode = code ?? 1;
} finally {
  if (child.exitCode === null) {
    const stopped = new Promise(resolve => child.once('exit', resolve));
    child.kill('SIGKILL');
    await stopped;
  }
}
