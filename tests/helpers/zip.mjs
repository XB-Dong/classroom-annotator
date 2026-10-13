import assert from 'node:assert/strict';
import AdmZip from 'adm-zip';
import jpeg from 'jpeg-js';

// Independent reader: never calls product export code or trusts COCO dimensions.
export function inspectExport(buffer, expected, { attributed = false } = {}) {
  const zip = new AdmZip(buffer);
  const entries = zip.getEntries();
  const names = entries.map(entry => entry.entryName);
  assert.equal(new Set(names).size, names.length, 'duplicate ZIP entry');
  const metadata = attributed ? ['credits.json', 'LICENSES.txt'] : [];
  assert.ok(names.every(name => name === 'annotations.json' || metadata.includes(name) || /^images\/[^/\\]+\.jpg$/.test(name)), 'unexpected ZIP path');
  for (const name of metadata) assert.ok(names.includes(name) && zip.readFile(name)?.length > 0, 'missing required public attribution');
  const coco = JSON.parse(zip.readAsText('annotations.json'));
  assert.equal(coco.categories.length, 1);
  assert.equal(coco.categories[0].id, 1);
  assert.equal(coco.categories[0].name, 'head');
  assert.equal(coco.images.length, expected.length, 'completed image count');
  const ids = new Set();
  const expectedByName = new Map(expected.map(frame => [`images/${frame.id}.jpg`, frame]));
  for (const image of coco.images) {
    assert.ok(Number.isInteger(image.id) && !ids.has(image.id), 'unique numeric image ID');
    ids.add(image.id);
    const frame = expectedByName.get(image.file_name);
    assert.ok(frame, `unexpected image ${image.file_name}`);
    const bytes = zip.readFile(image.file_name);
    assert.ok(bytes?.length > 0, 'missing JPEG bytes');
    const decoded = jpeg.decode(bytes, { useTArray: true, maxMemoryUsageInMB: 64 });
    assert.deepEqual([image.width, image.height], [decoded.width, decoded.height]);
    assert.deepEqual([decoded.width, decoded.height], [frame.width, frame.height]);
    if (frame.imageBytes) assert.deepEqual(bytes, frame.imageBytes, 'export changed immutable image');
    expectedByName.delete(image.file_name);
  }
  assert.equal(expectedByName.size, 0);
  assert.equal(names.length, expected.length + 1 + metadata.length, 'extra/missing ZIP entries');
  const annotationIds = new Set();
  for (const annotation of coco.annotations) {
    assert.ok(Number.isInteger(annotation.id) && !annotationIds.has(annotation.id));
    annotationIds.add(annotation.id);
    assert.ok(ids.has(annotation.image_id), 'dangling annotation');
    assert.equal(annotation.category_id, 1);
    assert.equal(annotation.iscrowd, 0);
    const image = coco.images.find(item => item.id === annotation.image_id);
    const [x, y, width, height] = annotation.bbox;
    assert.ok(annotation.bbox.length === 4 && annotation.bbox.every(Number.isFinite));
    assert.ok(x >= 0 && y >= 0 && width > 0 && height > 0 && x + width <= image.width && y + height <= image.height);
    assert.equal(annotation.area, width * height);
  }
  for (const frame of expected) {
    const image = coco.images.find(item => item.file_name === `images/${frame.id}.jpg`);
    const boxes = coco.annotations.filter(item => item.image_id === image.id).map(item => item.bbox);
    assert.deepEqual(boxes, frame.annotations.map(item => item.bbox), 'saved annotation geometry');
  }
  return coco;
}
