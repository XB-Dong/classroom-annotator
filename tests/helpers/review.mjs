import { expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { validateSchema } from './contract.mjs';

export async function review(request, frame) {
  const response = await request.get(`/api/v1/frames/${frame.id}/review`);
  expect(response.status()).toBe(200);
  return validateSchema('ReviewSnapshot', await response.json());
}

export async function stats(request, video) {
  const response = await request.get(`/api/v1/videos/${video.id}/review-stats`);
  expect(response.status()).toBe(200);
  return validateSchema('ReviewStats', await response.json());
}

export async function enqueue(request, frame, revision, key = randomUUID()) {
  const response = await request.post(`/api/v1/frames/${frame.id}/preannotations`, {
    headers: { 'Idempotency-Key': key }, data: { expectedRevision: revision } });
  expect(response.status()).toBe(202);
  const job = validateSchema('Job', await response.json());
  expect(job.kind).toBe('preannotation');
  return { job, key, revision };
}

export async function finished(request, frame, status = 'completed') {
  let value;
  await expect.poll(async () => {
    value = await review(request, frame); return value.currentRun?.status;
  }, { timeout: 40_000, intervals: [50, 100, 250] }).toBe(status);
  return value;
}

export async function decide(request, frame, value, decisions = [], issueResolutions = []) {
  const response = await request.put(`/api/v1/frames/${frame.id}/review`, { data: {
    expectedRevision: value.revision, decisions, issueResolutions } });
  expect(response.status()).toBe(200);
  return validateSchema('ReviewSnapshot', await response.json());
}

export async function decideIndividually(request, frame, value, decisions) {
  for (const decision of decisions) value = await decide(request, frame, value, [decision]);
  return value;
}

export async function control(value) {
  expect(process.env.P1_AI_FIXTURE_PHASE).toBe('fixture_not_real_model');
  const response = await fetch(`${process.env.P1_AI_FIXTURE_ORIGIN}/control`, {
    method: value ? 'POST' : 'GET', headers: { Authorization: `Bearer ${process.env.P1_AI_FIXTURE_CONTROL_TOKEN}`,
      'Content-Type': 'application/json' }, body: value ? JSON.stringify(value) : undefined });
  expect(response.status).toBe(200);
  return response.json();
}

export async function held() {
  let value;
  await expect.poll(async () => { value = await control(); return value.held.length; },
    { timeout: 5000, intervals: [50, 100] }).toBe(1);
  return value.held[0];
}

export function expectStats(actual, counts) {
  for (const [key, value] of Object.entries(counts)) expect(actual[key], key).toBe(value);
  const decided = counts.accepted + counts.adjusted + counts.rejected;
  const ratio = (n, d) => d === 0 ? null : n / d;
  const rates = { acceptedUnchanged: ratio(counts.accepted, decided), adjusted: ratio(counts.adjusted, decided),
    rejected: ratio(counts.rejected, decided), suspiciousCandidates: ratio(counts.riskyCandidates, counts.candidates),
    suspiciousImages: ratio(counts.suspiciousImages, counts.validImages), reviewed: ratio(decided, counts.candidates) };
  for (const [key, value] of Object.entries(rates)) {
    if (value === null) expect(actual.rates[key], key).toBeNull();
    else expect(actual.rates[key], key).toBeCloseTo(value, 10);
  }
}
