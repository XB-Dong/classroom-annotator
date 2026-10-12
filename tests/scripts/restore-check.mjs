import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createHash } from 'node:crypto';
import { inspectExport } from '../helpers/zip.mjs';

// Observe only. Leader owns stopping, backing up and restoring the cloud volume.
assert.ok(process.env.P0_RESTORE_BASELINE, 'Set external P0_RESTORE_BASELINE written by the successful browser workflow');
const baseline = JSON.parse(fs.readFileSync(process.env.P0_RESTORE_BASELINE));
const origin = process.env.P0_BASE_URL || 'http://127.0.0.1:8180';
const expectedSHA = process.env.P0_EXPECTED_SHA || baseline.deployedSHA;
async function response(route, options) {
  const result = await fetch(`${origin}${route}`, { ...options, signal: AbortSignal.timeout(30_000) });
  assert.equal(result.status, 200, `${route} status`);
  return result;
}
const health = await (await response('/health/ready')).json();
assert.equal(health.status, 'ok');
assert.equal(health.version, expectedSHA);
const progress = await (await response(`/api/v1/videos/${baseline.videoId}/progress?batchId=${baseline.batchId}`)).json();
assert.deepEqual(progress, baseline.progress, 'restored batch progress');
const expected = [];
for (const frame of baseline.frames) {
  const snapshot = await (await response(`/api/v1/frames/${frame.id}/annotations`)).json();
  assert.deepEqual(snapshot, frame.snapshot, 'restored revision/status/annotations');
  const imageBytes = Buffer.from(await (await response(`/media/frames/${frame.id}.jpg`)).arrayBuffer());
  assert.equal(createHash('sha256').update(imageBytes).digest('hex'), frame.imageSHA256, 'restored immutable JPEG bytes');
  expected.push({ ...frame, annotations: snapshot.annotations, imageBytes });
}
const exported = await response('/api/v1/exports', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ videoId: baseline.videoId, batchId: baseline.batchId }) });
const bytes = Buffer.from(await exported.arrayBuffer());
assert.deepEqual(inspectExport(bytes, expected), baseline.coco, 'restored COCO IDs, dimensions, associations and geometry');
console.log(`P0-OPS/SAVE restore observation passed: ${expected.length} completed frames, saved revisions, progress, image hashes and independent COCO semantics unchanged; deployed SHA=${expectedSHA}.`);
