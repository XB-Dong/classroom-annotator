import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import jpeg from 'jpeg-js';
import { fileURLToPath } from 'node:url';

// Read-only verification after the dedicated acceptance process has stopped.
const repository = path.resolve(fileURLToPath(new URL('../..', import.meta.url)));
const root = path.resolve(process.env.P0_STORAGE_DATA_DIR || '');
assert.ok(process.env.P0_STORAGE_DATA_DIR, 'Set dedicated external P0_STORAGE_DATA_DIR');
const relative = path.relative(repository, root);
assert.ok(relative.startsWith('..') || path.isAbsolute(relative), 'Storage must be outside repository');
const db = new DatabaseSync(path.join(root, 'app.sqlite'), { readOnly: true });
try {
  assert.equal(db.prepare('PRAGMA integrity_check').get().integrity_check, 'ok');
  assert.equal(db.prepare('PRAGMA foreign_key_check').all().length, 0);
  const videos = db.prepare('SELECT storage_key FROM videos').all();
  const frames = db.prepare('SELECT storage_key,width,height FROM frames').all();
  const batches = db.prepare('SELECT batches.id,jobs.status FROM batches JOIN jobs ON batches.job_id=jobs.id').all();
  assert.ok(batches.every(batch => batch.status === 'completed'), 'only completed jobs publish batches');
  function mediaPath(key) {
    const resolved = path.resolve(root, ...key.split(/[\\/]/));
    const within = path.relative(root, resolved);
    assert.ok(!path.isAbsolute(within) && within.split(path.sep)[0] !== '..', 'storage key escapes root');
    return resolved;
  }
  for (const video of videos) assert.ok(fs.statSync(mediaPath(video.storage_key)).isFile());
  for (const frame of frames) {
    const image = jpeg.decode(fs.readFileSync(mediaPath(frame.storage_key)), { useTArray: true, maxMemoryUsageInMB: 64 });
    assert.deepEqual([image.width, image.height], [frame.width, frame.height]);
  }
  const files = directory => fs.existsSync(directory) ? fs.readdirSync(directory, { recursive: true, withFileTypes: true }).filter(entry => entry.isFile()).map(entry => path.resolve(entry.parentPath, entry.name)) : [];
  const expected = [...videos, ...frames].map(item => mediaPath(item.storage_key)).sort();
  const actual = [...files(path.join(root, 'videos')), ...files(path.join(root, 'frames'))].sort();
  assert.deepEqual(actual, expected, 'unmanaged or missing immutable media');
  const staging = path.join(root, 'staging');
  assert.equal(fs.readdirSync(staging).length, 0, 'staging remains after terminal jobs/process stop');
  console.log(`Read-only SQLite integrity/media reconciliation passed: ${videos.length} videos, ${batches.length} batches, ${frames.length} decoded JPEGs, no staging/orphans.`);
} finally { db.close(); }
