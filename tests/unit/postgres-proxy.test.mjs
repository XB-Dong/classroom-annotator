import assert from 'node:assert/strict';
import net from 'node:net';
import test from 'node:test';
import { postgresProxy } from '../helpers/postgres-proxy.mjs';

test('fault relay forwards bytes to its original destination and reconnects', async () => {
  let accepted = 0;
  const original = net.createServer(socket => { accepted++; socket.pipe(socket); });
  await new Promise(resolve => original.listen(0, '127.0.0.1', resolve));
  const relay = await postgresProxy(`postgres://p01_test_owner:synthetic@127.0.0.1:${original.address().port}/p01_test_api?sslmode=disable`);
  const exchange = () => new Promise((resolve, reject) => {
    const target = new URL(relay.connectionString);
    const socket = net.connect(Number(target.port), target.hostname);
    const timer = setTimeout(() => { socket.destroy(); reject(new Error('Original destination received no forwarded bytes')); }, 500);
    socket.once('connect', () => socket.write('original-destination'));
    socket.once('data', bytes => { clearTimeout(timer); socket.destroy(); resolve(bytes.toString()); });
    socket.once('error', error => { clearTimeout(timer); reject(error); });
  });
  try {
    assert.equal(await exchange(), 'original-destination');
    relay.disconnect();
    relay.reconnect();
    assert.equal(await exchange(), 'original-destination');
    assert.equal(accepted, 2);
  } finally {
    await relay.stop();
    await new Promise(resolve => original.close(resolve));
  }
});
