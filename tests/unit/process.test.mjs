import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { dedicatedServer } from '../helpers/process.mjs';
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
