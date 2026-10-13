import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import jpeg from 'jpeg-js';
import { acceptance, externalPath, openPostgres } from '../helpers/postgres.mjs';

// Read-only PG/file reconciliation. No media may be created or removed here.
await acceptance(async () => {
  const root = externalPath(process.env.P0_STORAGE_DATA_DIR, 'P0_STORAGE_DATA_DIR');
  const pool = await openPostgres();
  try {
    const videos = (await pool.query('SELECT storage_provider,storage_key,media_sha256,media_size_bytes FROM videos')).rows;
    const frames = (await pool.query('SELECT storage_provider,storage_key,width,height,media_sha256,media_size_bytes FROM frames')).rows;
    const batches = (await pool.query('SELECT b.id,j.status FROM batches b JOIN jobs j ON b.job_id=j.id')).rows;
    assert.ok(batches.every(batch => batch.status === 'completed'), 'Noncompleted jobs published batches');
    const inconsistent = (await pool.query('SELECT f.id FROM frames f JOIN videos v ON v.id=f.video_id WHERE f.storage_provider<>v.storage_provider')).rows;
    assert.deepEqual(inconsistent, [], 'Frame provider differs from its video');
    const mediaPath = key => {
      const resolved = path.resolve(root, ...key.split(/[\\/]/));
      const relative = path.relative(root, resolved);
      assert.ok(relative && !path.isAbsolute(relative) && relative.split(path.sep)[0] !== '..', 'Storage key escapes media root');
      return resolved;
    };
    const expected = [];
    for (const record of [...videos, ...frames]) {
      if (record.storage_provider === 'mac') continue;
      assert.equal(record.storage_provider, 'local');
      const filename = mediaPath(record.storage_key);
      const bytes = fs.readFileSync(filename);
      expected.push(filename);
      if (record.media_sha256) assert.equal(createHash('sha256').update(bytes).digest('hex'), record.media_sha256);
      if (record.media_sha256 || Number(record.media_size_bytes) > 0) assert.equal(bytes.length, Number(record.media_size_bytes));
      if (record.width !== undefined) {
        const image = jpeg.decode(bytes, { useTArray: true, maxMemoryUsageInMB: 64 });
        assert.deepEqual([image.width, image.height], [record.width, record.height]);
      }
    }
    const files = directory => fs.existsSync(directory) ? fs.readdirSync(directory, { recursive: true, withFileTypes: true })
      .filter(entry => entry.isFile()).map(entry => path.resolve(entry.parentPath, entry.name)) : [];
    const actual = [...files(path.join(root, 'videos')), ...files(path.join(root, 'frames'))].sort();
    assert.deepEqual(actual, expected.sort(), 'Local immutable media is missing or orphaned');
    assert.equal(files(path.join(root, 'staging')).length, 0, 'Terminal dedicated process retained staging media');
    console.log(`PostgreSQL read-only storage reconciliation passed: ${videos.length} videos, ${batches.length} batches, ${frames.length} JPEG references; local identities verified. Mac physical-file/no-cloud-payload gate requires mac-media evidence separately.`);
  } finally { await pool.end(); }
});
