import { test, expect } from '@playwright/test';
import AdmZip from 'adm-zip';
import { extract, save, snapshot } from '../helpers/api.mjs';
import { inspectExport } from '../helpers/zip.mjs';

async function firstSample(request) {
  const response = await request.get('/api/v1/samples');
  expect(response.status()).toBe(200);
  const samples = (await response.json()).items;
  expect(samples.filter(item => item.availability === 'ready')).toHaveLength(2);
  for (const item of samples) {
    expect(item.creator).toBeTruthy(); expect(item.licenseName).toBeTruthy();
    expect(item.sourcePageUrl).toMatch(/^https:\/\//); expect(item.licenseUrl).toMatch(/^https:\/\//);
    expect(item).not.toHaveProperty('downloadUrl'); expect(item).not.toHaveProperty('sourceFile');
  }
  return samples.find(item => item.availability === 'ready');
}

async function register(request, sample) {
  const response = await request.post('/api/v1/samples/imports', { data: { sampleId: sample.sampleId } });
  expect([200, 201]).toContain(response.status());
  const video = await response.json();
  expect([video.sourceKind, video.storageProvider]).toEqual(['public_sample', 'local']);
  return video;
}

async function closure(request, sample, video) {
  const { job, frames } = await extract(request, video.id);
  expect(frames).toHaveLength(3);
  const annotation = { id: crypto.randomUUID(), bbox: [10, 10, 20, 20], source: 'manual' };
  const saved = await save(request, frames[0], [annotation], 'completed');
  await save(request, frames[1], [], 'completed');
  expect(await snapshot(request, frames[0].id)).toEqual(saved);
  const replay = await register(request, sample);
  expect(replay.id).toBe(video.id);
  expect(await snapshot(request, frames[0].id)).toEqual(saved);
  const expected = [];
  for (const frame of frames.slice(0, 2)) {
    const media = await request.get(frame.imageUrl); expect(media.status()).toBe(200);
    expected.push({ ...frame, annotations: (await snapshot(request, frame.id)).annotations, imageBytes: await media.body() });
  }
  const response = await request.post('/api/v1/exports', { data: { videoId: video.id, batchId: job.batchId } });
  expect(response.status()).toBe(200);
  const bytes = await response.body(); inspectExport(bytes, expected);
  const zip = new AdmZip(bytes);
  const credits = JSON.parse(zip.readAsText('credits.json'));
  expect(credits.schemaVersion).toBe('1.0.0'); expect(credits.items).toHaveLength(1);
  expect(credits.items[0]).toMatchObject({ sampleId: sample.sampleId, videoId: video.id,
    creator: sample.creator, sourcePageUrl: sample.sourcePageUrl, licenseName: sample.licenseName, licenseUrl: sample.licenseUrl });
  const licenses = zip.readAsText('LICENSES.txt');
  expect(licenses).toContain(sample.creator); expect(licenses).toContain(sample.licenseName);
}

test('P01-SAMPLE-001 cached public samples preserve identity/annotations and credited original JPEG export', async ({ request }) => {
  test.skip(process.env.P01_HAS_PUBLIC_SAMPLES !== '1' || process.env.P01_OFFLINE_PHASE === '1', 'Requires prepared real public media catalog in the online phase');
  const sample = await firstSample(request);
  await closure(request, sample, await register(request, sample));
});

test('P01-SAMPLE-002 public extraction/save/export survives backend restart with Mac agent stopped', async ({ request }) => {
  test.skip(process.env.P01_OFFLINE_PHASE !== '1', 'Runs only after the owned native agent has actually stopped');
  const storage = await (await request.get('/api/v1/media-storage/status')).json();
  expect(storage.sources.find(item => item.sourceKind === 'public_sample').available).toBe(true);
  for (const kind of ['mac', 'upload', 'dataset']) expect(storage.sources.find(item => item.sourceKind === kind).available).toBe(false);
  const sample = await firstSample(request);
  const video = await register(request, sample);
  const prior = (await (await request.get(`/api/v1/videos/${video.id}/frames`)).json()).frames;
  expect(prior.some(frame => frame.status === 'completed')).toBe(true);
  await closure(request, sample, video);
});
