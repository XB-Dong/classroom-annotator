import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import { externalPath, sanitizedFailure } from './postgres.mjs';

export async function command(binary, args, environment, expected = 0) {
  const executable = externalPath(binary, 'Acceptance binary');
  const child = spawn(executable, args, { env: { ...process.env, ...environment }, stdio: ['ignore', 'pipe', 'pipe'] });
  let output = '';
  for (const stream of [child.stdout, child.stderr]) stream.on('data', chunk => {
    output += chunk.toString();
    if (output.length > 2 * 1024 * 1024) child.kill('SIGKILL');
  });
  const timer = setTimeout(() => child.kill('SIGKILL'), 60_000);
  try {
    const code = await new Promise((resolve, reject) => {
      child.once('error', reject); child.once('exit', resolve);
    });
    const safeOutput = sanitizedFailure(output);
    assert.ok(code !== null, 'Acceptance command exceeded its time/output limit');
    if (expected === 'failure') assert.notEqual(code, 0, 'Command unexpectedly accepted the refused operation');
    else assert.equal(code, expected, safeOutput.slice(-2000));
    return { code, output: safeOutput };
  } finally { clearTimeout(timer); }
}

export async function waitFor(predicate, timeout = 30_000, label = 'Acceptance condition') {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const result = await predicate();
    if (result) return result;
    await delay(50);
  }
  throw new Error(`${label} timed out`);
}

export async function dedicatedServer(binary, environment, origin, readinessRoute = '/health/ready') {
  const child = spawn(externalPath(binary, 'P0_BACKEND_BINARY'), [], {
    env: { ...process.env, ...environment, APP_ADDR: origin.replace('http://', '') },
    stdio: ['ignore', 'ignore', 'ignore'],
  });
  let startupError;
  child.once('error', error => { startupError = error; });
  let stopping;
  const stop = async () => {
    if (stopping) return stopping;
    if (child.exitCode !== null || child.signalCode !== null || startupError) return;
    stopping = new Promise((resolve, reject) => {
      const exited = () => { clearTimeout(timer); resolve(); };
      const timer = setTimeout(() => {
        child.removeListener('exit', exited);
        reject(new Error('Dedicated server did not stop within 5 seconds'));
      }, 5000);
      child.once('exit', exited);
      child.kill('SIGKILL');
    });
    return stopping;
  };
  try {
    await waitFor(async () => {
      if (startupError) throw startupError;
      assert.equal(child.exitCode, null, 'Dedicated server exited during startup');
      try { return (await fetch(`${origin}${readinessRoute}`, { signal: AbortSignal.timeout(1000) })).ok; } catch { return false; }
    }, 30_000, 'Dedicated acceptance process readiness');
    return { stop, child };
  } catch (error) { await stop(); throw error; }
}
