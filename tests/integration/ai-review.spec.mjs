import { test, expect } from '../helpers/p1-session.mjs';
import { upload, extract, snapshot, save } from '../helpers/api.mjs';
import { validateSchema } from '../helpers/contract.mjs';
import { inspectExport } from '../helpers/zip.mjs';
import { randomUUID } from 'node:crypto';

test('P1-AI/GATE/REVIEW-001 true Go to Python run cannot bypass human completion gates', async ({ request }) => {
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  const frame = frames[0];
  const before = await snapshot(request, frame.id);
  const response = await request.post(`/api/v1/frames/${frame.id}/preannotations`, {
    headers: { 'Idempotency-Key': randomUUID() }, data: { expectedRevision: before.revision } });
  expect(response.status()).toBe(202);
  let review;
  await expect.poll(async () => {
    const current = await request.get(`/api/v1/frames/${frame.id}/review`);
    expect(current.status()).toBe(200);
    review = validateSchema('ReviewSnapshot', await current.json());
    return review.currentRun?.status;
  }, { timeout: 40_000, intervals: [100, 250, 500] }).toBe('completed');
  expect(review.currentRun.modelSHA256).toBe('a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc');
  expect(review.candidates.every(candidate => candidate.reviewState === 'pending')).toBe(true);
  const pending = await request.put(`/api/v1/frames/${frame.id}/annotations`, { data: {
    expectedRevision: review.revision, status: 'completed', annotations: [] } });
  expect(pending.status()).toBe(422);
  const manual = { id: randomUUID(), bbox: [60, 40, 30, 30], source: 'manual' };
  const saved = await save(request, frame, [manual], 'in_progress');
  review = validateSchema('ReviewSnapshot', await (await request.get(`/api/v1/frames/${frame.id}/review`)).json());
  expect(review.revision).toBe(saved.revision);
  const resolved = await request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: review.revision, decisions: review.candidates.map(candidate => ({ candidateId: candidate.id, decision: 'rejected' })),
    issueResolutions: review.issues.filter(issue => !issue.resolved).map(issue => ({ id: issue.id, resolution: 'manual_checked' })) } });
  expect(resolved.status()).toBe(200);
  await save(request, frame, [manual], 'completed');
  const exported = await request.post('/api/v1/exports', { data: { frameIds: [frame.id] } });
  expect(exported.status()).toBe(200);
  const image = await request.get(frame.imageUrl);
  inspectExport(await exported.body(), [{ ...frame, annotations: [manual], imageBytes: await image.body() }]);
});
