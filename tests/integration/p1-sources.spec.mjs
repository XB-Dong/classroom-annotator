import { test, expect } from '../helpers/p1-session.mjs';
import { extract, terminalJob, upload } from '../helpers/api.mjs';
import { randomUUID, createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { validateSchema } from '../helpers/contract.mjs';

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
