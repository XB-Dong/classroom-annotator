import { test, expect } from '../helpers/p1-session.mjs';
import { extract, terminalJob, upload } from '../helpers/api.mjs';
import { randomUUID, createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { validateSchema } from '../helpers/contract.mjs';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import pg from 'pg';

test('P1-LOCAL/DATASET-001 upload and observed-schema channel4 company import use current-host media', async ({ request }) => {
  const uploaded = await upload(request);
  expect([uploaded.sourceKind, uploaded.storageProvider]).toEqual(['upload', 'local']);
  const { frames } = await extract(request, uploaded.id);
  expect(frames.every(frame => frame.storageProvider === 'local')).toBe(true);
  const lessons = await request.get('/api/v1/dataset/lessons?page=2&pageSize=1');
  expect(lessons.status()).toBe(200);
  const page = await lessons.json();
  expect(page.items[0].videos.map(item => item.channel)).toEqual([1, 4]);
  const response = await request.post('/api/v1/dataset/imports', { data: { catalogFileId: '4', name: 'Synthetic local company import' } });
  expect(response.status()).toBe(202);
  const job = await terminalJob(request, (await response.json()).id);
  expect(job.status).toBe('completed');
  const videos = (await (await request.get('/api/v1/videos')).json()).videos;
  const imported = videos.find(video => video.id === job.videoId);
  expect([imported.sourceKind, imported.storageProvider]).toEqual(['dataset', 'local']);
  expect((await extract(request, imported.id)).frames).toHaveLength(3);
});

test('P1-LOCAL-001 directory import preserves content identity and never exposes filesystem paths', async ({ request }) => {
  const response = await request.get('/api/v1/sources/local');
  expect(response.status()).toBe(200);
  const sources = (await response.json()).items;
  expect(sources.length).toBeGreaterThan(0);
  sources.forEach(source => validateSchema('LocalSource', source));
  expect(JSON.stringify(sources)).not.toContain(process.env.P0_FIXTURE_DIR);
  const digest = createHash('sha256').update(fs.readFileSync(path.join(process.env.P0_FIXTURE_DIR, 'classroom.mp4'))).digest('hex');
  const source = sources.find(source => source.sha256 === digest);
  expect(source).toBeTruthy();
  const key = randomUUID();
  const imported = await request.post(`/api/v1/sources/local/${encodeURIComponent(source.sourceId)}/imports`, {
    headers: { 'Idempotency-Key': key } });
  expect(imported.status()).toBe(202);
  const job = await imported.json();
  const retry = await request.post(`/api/v1/sources/local/${encodeURIComponent(source.sourceId)}/imports`, {
    headers: { 'Idempotency-Key': key } });
  expect(retry.status()).toBe(202);
  expect((await retry.json()).id).toBe(job.id);
  expect((await terminalJob(request, job.id)).status).toBe('completed');
  const video = (await (await request.get('/api/v1/videos')).json()).videos.find(video => video.id === job.videoId);
  expect(video.storageProvider).toBe('local');
  expect((await extract(request, video.id)).frames).toHaveLength(3);
});

test('P1-LOCAL-001 retired Mac identities never read matching local paths or produce a ZIP', async ({ request }) => {
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const originalImage = await request.get(frame.imageUrl);
  expect(originalImage.status()).toBe(200);
  const digest = createHash('sha256').update(await originalImage.body()).digest('hex');
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 1 });
  const retired = { video: randomUUID(), job: randomUUID(), batch: randomUUID(), frame: randomUUID() };
  try {
    await pool.query(`INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key,
      source_kind,storage_provider,media_sha256,media_size_bytes,workspace_id)
      SELECT $1,'Synthetic retired Mac identity',width,height,duration_seconds,status,created_at,storage_key,
      'mac','mac',media_sha256,media_size_bytes,workspace_id FROM videos WHERE id=$2`, [retired.video, video.id]);
    await pool.query(`INSERT INTO jobs(id,kind,video_id,batch_id,status,created_at) VALUES($1,'extraction',$2,$3,'completed',$4)`,
      [retired.job, retired.video, retired.batch, new Date().toISOString()]);
    await pool.query('INSERT INTO batches(id,video_id,job_id,created_at) VALUES($1,$2,$3,$4)',
      [retired.batch, retired.video, retired.job, new Date().toISOString()]);
    await pool.query(`INSERT INTO frames(id,video_id,batch_id,timestamp_ms,width,height,status,revision,storage_key,
      storage_provider,media_sha256,media_size_bytes)
      SELECT $1,$2,$3,timestamp_ms,width,height,'completed',0,storage_key,'mac',media_sha256,media_size_bytes FROM frames WHERE id=$4`,
    [retired.frame, retired.video, retired.batch, frame.id]);
    const image = await request.get(`/media/frames/${retired.frame}.jpg`);
    expect(image.status()).toBe(410);
    expect((await image.json()).code).toBe('REMOTE_STORAGE_RETIRED');
    const archived = await request.post('/api/v1/exports', { data: { frameIds: [retired.frame] } });
    expect(archived.status()).toBe(410);
    expect((await archived.json()).code).toBe('REMOTE_STORAGE_RETIRED');
    expect((await request.post(`/api/v1/videos/${retired.video}/extractions`, { data: {
      intervalSeconds: 1, startSeconds: 0, endSeconds: 3 } })).status()).toBe(410);
    const retained = await request.get(frame.imageUrl);
    expect(retained.status()).toBe(200);
    expect(createHash('sha256').update(await retained.body()).digest('hex')).toBe(digest);
  } finally {
    await pool.query('DELETE FROM frames WHERE id=$1', [retired.frame]);
    await pool.query('DELETE FROM batches WHERE id=$1', [retired.batch]);
    await pool.query('DELETE FROM jobs WHERE id=$1', [retired.job]);
    await pool.query('DELETE FROM videos WHERE id=$1', [retired.video]);
    await pool.end();
  }
});
