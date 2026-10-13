import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot, save } from '../helpers/api.mjs';
import { inspectExport } from '../helpers/zip.mjs';
import { randomUUID } from 'node:crypto';

test('P1-DEGRADE-001 stopped actual Python service leaves the complete manual workflow usable', async ({ request }) => {
  expect(process.env.P1_AI_STOPPED).toBe('1');
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const before = await snapshot(request, frame.id);
  const queued = await request.post(`/api/v1/frames/${frame.id}/preannotations`, { headers: {
    'Idempotency-Key': randomUUID() }, data: { expectedRevision: before.revision } });
  expect(queued.status()).toBe(202);
  let review;
  await expect.poll(async () => {
    const current = await request.get(`/api/v1/frames/${frame.id}/review`);
    expect(current.status()).toBe(200); review = await current.json();
    return review.currentRun?.status;
  }, { timeout: 40_000 }).toBe('failed');
  const annotation = { id: randomUUID(), bbox: [60, 40, 30, 30], source: 'manual' };
  await save(request, frame, [annotation], 'in_progress');
  review = await (await request.get(`/api/v1/frames/${frame.id}/review`)).json();
  expect(review.issues.some(issue => issue.reasonCode === 'AI_FAILED')).toBe(true);
  const resolved = await request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: review.revision, decisions: [], issueResolutions: review.issues.filter(issue => !issue.resolved)
      .map(issue => ({ id: issue.id, resolution: 'manual_checked' })) } });
  expect(resolved.status()).toBe(200);
  const completed = await save(request, frame, [annotation], 'completed');
  expect(await snapshot(request, frame.id)).toEqual(completed);
  const archive = await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } });
  expect(archive.status()).toBe(200);
  inspectExport(await archive.body(), [{ ...frame, annotations: [annotation], imageBytes: await (await request.get(frame.imageUrl)).body() }]);
});
