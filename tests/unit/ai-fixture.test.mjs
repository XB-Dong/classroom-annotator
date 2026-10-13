import assert from 'node:assert/strict';
import test from 'node:test';
import jpeg from 'jpeg-js';
import { aiFixture, fixtureDetections } from '../helpers/ai-fixture.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { waitFor } from '../helpers/process.mjs';

const pixels = jpeg.encode({ width: 320, height: 240, data: Buffer.alloc(320 * 240 * 4, 255) }, 80).data;
function detection(origin, token = 'synthetic-internal-token') {
  const form = new FormData();
  form.set('image', new Blob([pixels], { type: 'image/jpeg' }), 'synthetic.jpg');
  form.set('confidenceThreshold', '.25'); form.set('nmsThreshold', '.5');
  return fetch(`${origin}/v1/detections`, { method: 'POST', body: form,
    headers: { Authorization: `Bearer ${token}`, 'X-Request-ID': 'synthetic-request-1' } });
}

test('deterministic AI fixture enforces internal/control isolation and holds a real HTTP response until explicit release', async () => {
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const fixture = await aiFixture(origin, 'synthetic-internal-token');
  const control = async data => fetch(`${origin}/control`, { method: 'POST',
    headers: { Authorization: `Bearer ${fixture.controlToken}`, 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
  try {
    assert.equal((await detection(origin, 'wrong-token')).status, 401);
    assert.equal((await fetch(`${origin}/control`)).status, 401);
    assert.equal(fixture.evidence.length, 0);
    assert.equal((await control({ mode: 'boxes', hold: true })).status, 200);
    let settled = false;
    const pending = detection(origin).then(response => { settled = true; return response; });
    await waitFor(() => fixture.evidence.length === 1, 3000);
    assert.equal(settled, false, 'The actual detection HTTP response must still be waiting');
    assert.equal(fixture.evidence[0].dataType, 'fixture_not_real_model');
    assert.match(fixture.evidence[0].imageSHA256, /^[a-f0-9]{64}$/);
    assert.equal((await control({ release: fixture.evidence[0].id })).status, 200);
    const response = await pending; assert.equal(response.status, 200);
    const value = await response.json();
    assert.deepEqual(value.detections, fixtureDetections);
    assert.deepEqual(value.image, { width: 320, height: 240 });
    assert.equal(value.contractVersion, '1.0.0');
    assert.equal(value.pipelineVersion, 'classroom-head-v1-fullframe-2x2-15');
  } finally { await fixture.stop(); }
});

test('fixture invalid output and failure are explicit synthetic modes rather than successful model evidence', async () => {
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const fixture = await aiFixture(origin, 'synthetic-internal-token');
  try {
    for (const [mode, expected] of [['empty', 200], ['failure', 503], ['null-bbox', 200]]) {
      const configured = await fetch(`${origin}/control`, { method: 'POST', headers: {
        Authorization: `Bearer ${fixture.controlToken}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ mode }) });
      assert.equal(configured.status, 200);
      const response = await detection(origin); assert.equal(response.status, expected);
      const value = await response.json();
      if (mode === 'empty') assert.deepEqual(value.detections, []);
      if (mode === 'failure') assert.equal(value.code, 'INFERENCE_FAILED');
      if (mode === 'null-bbox') assert.equal(value.detections[0].bbox[0], null);
    }
    assert.deepEqual(fixture.evidence.map(row => row.mode), ['empty', 'failure', 'null-bbox']);
  } finally { await fixture.stop(); }
});
