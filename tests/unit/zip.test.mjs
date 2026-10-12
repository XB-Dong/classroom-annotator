import test from 'node:test';
import assert from 'node:assert/strict';
import AdmZip from 'adm-zip';
import jpeg from 'jpeg-js';
import { inspectExport } from '../helpers/zip.mjs';
import { validateSchema } from '../helpers/contract.mjs';

function sample(mutate = () => {}) {
  const image = jpeg.encode({ width: 4, height: 3, data: Buffer.alloc(4 * 3 * 4, 255) }, 80).data;
  const coco = { images: [{ id: 1, file_name: 'images/f.jpg', width: 4, height: 3 }], annotations: [{ id: 1, image_id: 1, category_id: 1, bbox: [1, 1, 2, 1], area: 2, iscrowd: 0 }], categories: [{ id: 1, name: 'head' }] };
  mutate(coco);
  const zip = new AdmZip();
  zip.addFile('images/f.jpg', image);
  zip.addFile('annotations.json', Buffer.from(JSON.stringify(coco)));
  return { buffer: zip.toBuffer(), expected: [{ id: 'f', width: 4, height: 3, annotations: [{ bbox: [1, 1, 2, 1] }], imageBytes: image }] };
}

test('independent reader accepts consistent JPEG/COCO and zero-head frames', () => {
  const { buffer, expected } = sample();
  inspectExport(buffer, expected);
  const zero = sample(coco => { coco.annotations = []; });
  zero.expected[0].annotations = [];
  inspectExport(zero.buffer, zero.expected);
});
for (const [name, mutate] of [
  ['false dimensions', coco => { coco.images[0].width = 5; }],
  ['dangling annotation', coco => { coco.annotations[0].image_id = 2; }],
  ['wrong area', coco => { coco.annotations[0].area = 3; }],
  ['out of bounds box', coco => { coco.annotations[0].bbox = [3, 1, 2, 1]; }],
]) test(`independent reader rejects ${name}`, () => {
  const { buffer, expected } = sample(mutate);
  assert.throws(() => inspectExport(buffer, expected));
});
test('frozen contract rejects malformed annotation and mixed export selectors', () => {
  assert.throws(() => validateSchema('Annotation', { id: 'a', bbox: [0, 0, 2, 2] }));
  assert.throws(() => validateSchema('ExportRequest', { videoId: 'v', frameIds: ['f'] }));
  validateSchema('ExportRequest', { frameIds: ['f'] });
});
