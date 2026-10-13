import assert from 'node:assert/strict';
import http from 'node:http';
import { createHash, randomUUID } from 'node:crypto';
import jpeg from 'jpeg-js';

export const fixtureDetections = Object.freeze([
  { bbox: [10, 10, 20, 20], confidence: .9, classId: 7, clipped: false, tileBoundary: false },
  { bbox: [70, 20, 10, 10], confidence: .4, classId: 7, clipped: false, tileBoundary: false },
  { bbox: [72, 22, 10, 10], confidence: .6, classId: 7, clipped: false, tileBoundary: false },
]);

// This is a deterministic HTTP test provider, never model/accuracy evidence.
// It binds only after the runner has stopped its owned actual Python process.
export async function aiFixture(origin, internalToken, action = async () => {}) {
  const controlToken = randomUUID();
  const evidence = [];
  const holds = new Map();
  let plans = [];
  async function body(request, limit) {
    const chunks = []; let size = 0;
    for await (const chunk of request) {
      size += chunk.length;
      assert.ok(size <= limit, 'Fixture request exceeded its bound');
      chunks.push(chunk);
    }
    return Buffer.concat(chunks);
  }
  const json = (response, status, value) => {
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(value));
  };
  const server = http.createServer(async (request, response) => {
    try {
      if (request.url.startsWith('/control')) {
        if (request.headers.authorization !== `Bearer ${controlToken}`) return json(response, 401, { code: 'UNAUTHORIZED' });
        if (request.method === 'GET') return json(response, 200, { evidence, held: [...holds.keys()] });
        const value = JSON.parse((await body(request, 4096)).toString());
        if (value.release) {
          const release = holds.get(value.release);
          assert.ok(release, 'Unknown held fixture request');
          release(); holds.delete(value.release);
        } else if (value.action) await action(value.action);
        else {
          const sequence = value.sequence || [value];
          assert.ok(Array.isArray(sequence) && sequence.length > 0 && sequence.length <= 20);
          plans = sequence.map(plan => {
            assert.ok(['boxes', 'empty', 'failure', 'null-bbox'].includes(plan.mode));
            return { mode: plan.mode, hold: plan.hold === true };
          });
        }
        return json(response, 200, { ok: true });
      }
      if (request.url !== '/v1/detections' || request.method !== 'POST') return json(response, 404, { code: 'NOT_FOUND' });
      if (request.headers.authorization !== `Bearer ${internalToken}`) return json(response, 401, { code: 'UNAUTHORIZED' });
      const bytes = await body(request, 16 * 1024 * 1024);
      const contentType = request.headers['content-type'];
      assert.match(contentType, /^multipart\/form-data; boundary=/);
      const form = await new Response(bytes, { headers: { 'Content-Type': contentType } }).formData();
      const imageBytes = Buffer.from(await form.get('image').arrayBuffer());
      const image = jpeg.decode(imageBytes, { maxResolutionInMP: 1, maxMemoryUsageInMB: 32 });
      assert.equal(image.width, 320); assert.equal(image.height, 240);
      const plan = plans.shift() || { mode: 'boxes', hold: false };
      const id = randomUUID();
      const record = { id, dataType: 'fixture_not_real_model', mode: plan.mode,
        imageSHA256: createHash('sha256').update(imageBytes).digest('hex'), width: image.width, height: image.height,
        requestId: request.headers['x-request-id'], receivedAt: new Date().toISOString(), held: plan.hold };
      assert.match(record.requestId, /^[A-Za-z0-9._:-]{1,128}$/);
      evidence.push(record);
      if (plan.hold) await new Promise(resolve => {
        const timer = setTimeout(resolve, 20_000);
        holds.set(id, () => { clearTimeout(timer); resolve(); });
        response.once('close', () => { clearTimeout(timer); holds.delete(id); resolve(); });
      });
      holds.delete(id);
      record.releasedAt = new Date().toISOString();
      if (response.destroyed) { record.clientDisconnected = true; return; }
      if (plan.mode === 'failure') return json(response, 503, { code: 'INFERENCE_FAILED', message: 'Synthetic fixture failure', requestId: record.requestId });
      const detections = plan.mode === 'empty' ? [] : structuredClone(fixtureDetections);
      if (plan.mode === 'null-bbox') detections[0].bbox[0] = null;
      json(response, 200, { contractVersion: '1.0.0', modelId: 'yolov9_n_wholebody34_0100_1x3x640x640',
        modelSHA256: 'a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc',
        pipelineVersion: 'classroom-head-v1-fullframe-2x2-15', mode: 'fullframe',
        image: { width: image.width, height: image.height }, detections, timingMs: 1 });
    } catch (error) { json(response, 500, { code: 'FIXTURE_ERROR', message: error.message }); }
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject); server.listen(Number(new URL(origin).port), '127.0.0.1', resolve);
  });
  return { origin, controlToken, evidence, stop: async () => {
    for (const release of holds.values()) release(); holds.clear();
    server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
  } };
}
