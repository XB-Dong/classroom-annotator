import { test, expect } from '@playwright/test';
import { upload, extract, snapshot, save, safeError, terminalJob } from '../helpers/api.mjs';
import { inspectExport } from '../helpers/zip.mjs';

test('P0-BASE-001 readiness and safe missing resources', async ({ request }) => {
  for (const route of ['/health/live', '/health/ready']) {
    const response = await request.get(route);
    expect(response.status()).toBe(200);
    const health = await response.json();
    expect(health.status).toBe('ok');
    expect(typeof health.version).toBe('string');
    if (process.env.P0_EXPECTED_SHA) expect(health.version).toBe(process.env.P0_EXPECTED_SHA);
  }
  await safeError(await request.get('/api/v1/frames/missing/annotations'), 404);
});

test('P0-VID/FRM/SAVE/PROG/EXP independent full API acceptance', async ({ request }) => {
  const video = await upload(request);
  expect([video.width, video.height]).toEqual([320, 240]);
  expect(video.durationSeconds).toBeCloseTo(4, 1);
  const first = await extract(request, video.id);
  expect(first.frames.map(frame => frame.timestampMs)).toEqual([0, 1000, 2000]);
  expect(first.frames.every(frame => frame.width === 320 && frame.height === 240)).toBe(true);
  const frame = first.frames[0];
  const annotation = { id: 'independent-head-1', bbox: [60, 40, 30, 30], source: 'manual' };
  const saved = await save(request, frame, [annotation]);
  expect(saved.revision).toBe(1);
  await safeError(await request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: 0, status: 'completed', annotations: [] },
  }), 409);
  expect(await snapshot(request, frame.id)).toEqual(saved);
  for (const annotations of [
    [{ ...annotation, bbox: [-1, 40, 30, 30] }],
    [{ ...annotation, bbox: [300, 40, 30, 30] }],
    [{ ...annotation, bbox: [60, 40, 0, 30] }],
    [annotation, annotation],
  ]) {
    await safeError(await request.put(`/api/v1/frames/${frame.id}/annotations`, {
      data: { expectedRevision: saved.revision, status: 'in_progress', annotations },
    }), 422);
    expect(await snapshot(request, frame.id)).toEqual(saved);
  }
  await safeError(await request.put(`/api/v1/frames/${frame.id}/annotations`, { data: { annotations: [] } }), 400);
  await safeError(await request.post('/api/v1/exports', { data: { videoId: video.id } }), 422);
  const complete = await save(request, frame, [annotation], 'completed');
  await save(request, first.frames[1], [], 'completed');
  const second = await extract(request, video.id, { intervalSeconds: 2 });
  expect(second.job.batchId).not.toBe(first.job.batchId);
  expect(second.frames.map(item => item.timestampMs)).toEqual([0, 2000]);
  expect(await snapshot(request, frame.id)).toEqual(complete);
  const batches = await (await request.get(`/api/v1/videos/${video.id}/batches`)).json();
  expect(batches.batches.map(item => item.id)).toEqual([second.job.batchId, first.job.batchId]);
  const progress = await (await request.get(`/api/v1/videos/${video.id}/progress?batchId=${first.job.batchId}`)).json();
  expect(progress).toEqual({ total: 3, notStarted: 1, inProgress: 0, completed: 2 });
  for (const selectors of [{}, { frameIds: [] }, { videoId: video.id, frameIds: [frame.id] }, { batchId: first.job.batchId }, { frameIds: [frame.id], batchId: first.job.batchId }]) {
    await safeError(await request.post('/api/v1/exports', { data: selectors }), 400);
  }
  await safeError(await request.post('/api/v1/exports', { data: { frameIds: ['missing'] } }), 404);
  await safeError(await request.post('/api/v1/exports', { data: { videoId: video.id, batchId: 'missing' } }), 404);
  const response = await request.post('/api/v1/exports', { data: { videoId: video.id, batchId: first.job.batchId } });
  expect(response.status()).toBe(200);
  expect(response.headers()['content-type']).toMatch(/application\/zip/);
  const expected = [];
  for (const [index, annotations] of [[0, [annotation]], [1, []]]) {
    const item = first.frames[index];
    const media = await request.get(item.imageUrl);
    expect(media.status()).toBe(200);
    expected.push({ ...item, annotations, imageBytes: await media.body() });
  }
  inspectExport(await response.body(), expected);
  await save(request, frame, [annotation], 'in_progress');
  const revoked = await (await request.get(`/api/v1/videos/${video.id}/progress?batchId=${first.job.batchId}`)).json();
  expect(revoked.completed).toBe(1);
});

test('P0-VID-001 corrupt upload and invalid extraction do not publish', async ({ request }) => {
  const before = await (await request.get('/api/v1/videos')).json();
  await expect(upload(request, 'corrupt.mp4')).rejects.toThrow();
  const after = await (await request.get('/api/v1/videos')).json();
  expect(after).toEqual(before);
  const video = await upload(request);
  for (const options of [{ intervalSeconds: 0 }, { intervalSeconds: 0.0001 }, { intervalSeconds: 1, startSeconds: 3, endSeconds: 2 }, { intervalSeconds: 1, endSeconds: 5 }]) {
    const response = await request.post(`/api/v1/videos/${video.id}/extractions`, { data: options });
    expect([400, 422]).toContain(response.status());
  }
});

test('P0-JOB-001 cancellation leaves no published batch; terminal cancellation is stable', async ({ request }) => {
  const video = await upload(request);
  const response = await request.post(`/api/v1/videos/${video.id}/extractions`, { data: { intervalSeconds: 0.01, endSeconds: 3 } });
  expect(response.status()).toBe(202);
  const job = await response.json();
  const cancelled = await request.post(`/api/v1/jobs/${job.id}/cancel`);
  expect(cancelled.status()).toBe(200);
  const final = await terminalJob(request, job.id);
  expect(final.status).toBe('cancelled');
  const batches = await (await request.get(`/api/v1/videos/${video.id}/batches`)).json();
  expect(batches.batches.some(batch => batch.id === job.batchId)).toBe(false);
  expect(await (await request.post(`/api/v1/jobs/${job.id}/cancel`)).json()).toEqual(final);
});

test('P0-FRM-001 unavailable tail fails rather than silently dropping targets', async ({ request }) => {
  const video = await upload(request);
  const response = await request.post(`/api/v1/videos/${video.id}/extractions`, { data: { intervalSeconds: 0.001, startSeconds: 3.999, endSeconds: 4 } });
  expect(response.status()).toBe(202);
  const job = await response.json();
  const final = await terminalJob(request, job.id);
  expect(final.status).toBe('failed');
  expect(final.error).toContain('FRAME_UNAVAILABLE');
  const batches = await (await request.get(`/api/v1/videos/${video.id}/batches`)).json();
  expect(batches.batches.some(batch => batch.id === job.batchId)).toBe(false);
});
