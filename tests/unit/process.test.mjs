import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { dedicatedServer, waitFor } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';

test('stopping an already killed dedicated server is bounded and idempotent', { timeout: 10_000 }, async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'p01-process-stop-'));
  const script = path.join(root, 'server.cjs');
  fs.writeFileSync(script, `require('node:http').createServer((request,response)=>{response.writeHead(200);response.end('{}');}).listen(Number(process.env.APP_ADDR.split(':').at(-1)),'127.0.0.1');`);
  let server;
  try {
    const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
    server = await dedicatedServer(process.execPath, { NODE_OPTIONS: `--require ${JSON.stringify(script)}` }, origin);
    await server.stop();
    assert.ok(server.child.signalCode || server.child.exitCode !== null, 'The real child did not exit');
    const stopped = await Promise.race([server.stop().then(() => true), delay(250).then(() => false)]);
    assert.equal(stopped, true, 'A second stop waited forever for an exit event already delivered');
  } finally {
    if (server && server.child.exitCode === null && !server.child.signalCode) server.child.kill('SIGKILL');
    fs.unlinkSync(script);
    fs.rmdirSync(root);
  }
});

test('dedicated server observes a split stderr marker only for its current process', { timeout: 10_000 }, async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'p1-process-marker-'));
  const script = path.join(root, 'server.cjs');
  const marker = 'task ownership lost; manual HTTP remains available';
  const split = 19;
  fs.writeFileSync(script, `
    const marker = ${JSON.stringify(marker)};
    process.stderr.write('x'.repeat(128 * 1024));
    process.stderr.write(marker.slice(0, ${split}));
    require('node:http').createServer((request, response) => {
      if (request.url === '/emit') process.stderr.write(marker.slice(${split}));
      response.writeHead(200); response.end('{}');
    }).listen(Number(process.env.APP_ADDR.split(':').at(-1)), '127.0.0.1');
  `);
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const environment = { NODE_OPTIONS: `--require ${JSON.stringify(script)}` };
  let server;
  try {
    server = await dedicatedServer(process.execPath, environment, origin, '/health/ready', [], { stderrMarker: marker });
    assert.equal(typeof server.stderrMarkerObserved, 'function', 'Dedicated process must expose bounded marker observation');
    assert.equal(server.stderrMarkerObserved(), false, 'A partial startup marker must not count as ownership loss');
    assert.equal((await fetch(`${origin}/emit`)).status, 200);
    await waitFor(() => server.stderrMarkerObserved(), 1000, 'Split stderr lifecycle marker');
    await server.stop();
    server = await dedicatedServer(process.execPath, environment, origin, '/health/ready', [], { stderrMarker: marker });
    assert.equal(server.stderrMarkerObserved(), false, 'A restarted process must not inherit an earlier ownership-loss marker');
    await assert.rejects(waitFor(() => server.stderrMarkerObserved(), 100, 'Fresh ownership loss'), /Fresh ownership loss timed out/);
  } finally {
    if (server) await server.stop();
    fs.unlinkSync(script);
    fs.rmdirSync(root);
  }
});
