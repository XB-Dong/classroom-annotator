import { expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { validateSchema } from './contract.mjs';

export function fixture(name = 'classroom.mp4') {
  if (!process.env.P0_FIXTURE_DIR) throw new Error('Set P0_FIXTURE_DIR to external synthetic fixture directory; run npm run fixtures first.');
  return path.join(process.env.P0_FIXTURE_DIR, name);
}

export async function upload(request, name = 'classroom.mp4') {
  const response = await request.post('/api/v1/videos', {
    multipart: { file: { name, mimeType: 'video/mp4', buffer: fs.readFileSync(fixture(name)) } },
  });
  expect(response.status()).toBe(201);
  return validateSchema('Video', await response.json());
}

export async function terminalJob(request, id) {
  let job;
  await expect.poll(async () => {
    const response = await request.get(`/api/v1/jobs/${id}`);
    expect(response.status()).toBe(200);
    job = validateSchema('Job', await response.json());
    return ['completed', 'failed', 'cancelled'].includes(job.status);
  }, { timeout: 90_000, intervals: [100, 250, 500] }).toBe(true);
  return job;
}

export async function extract(request, videoId, options = {}) {
  const response = await request.post(`/api/v1/videos/${videoId}/extractions`, {
    data: { intervalSeconds: 1, startSeconds: 0, endSeconds: 3, ...options },
  });
  expect(response.status()).toBe(202);
  const job = validateSchema('Job', await response.json());
  expect((await terminalJob(request, job.id)).status).toBe('completed');
  const framesResponse = await request.get(`/api/v1/videos/${videoId}/frames?batchId=${job.batchId}`);
  expect(framesResponse.status()).toBe(200);
  const { frames } = await framesResponse.json();
  frames.forEach(frame => validateSchema('Frame', frame));
  return { job, frames };
}

export async function snapshot(request, id) {
  const response = await request.get(`/api/v1/frames/${id}/annotations`);
  expect(response.status()).toBe(200);
  return validateSchema('Snapshot', await response.json());
}

export async function save(request, frame, annotations, status = 'in_progress') {
  const current = await snapshot(request, frame.id);
  const response = await request.put(`/api/v1/frames/${frame.id}/annotations`, {
    data: { expectedRevision: current.revision, status, annotations },
  });
  expect(response.status()).toBe(200);
  return validateSchema('Snapshot', await response.json());
}

export async function safeError(response, status) {
  const body = validateSchema('Error', await response.json());
  expect(JSON.stringify(body)).not.toMatch(/(?:[A-Z]:\\|\/var\/|\/tmp\/|SQLITE_|Authorization|api[_-]?key\s*[:=])/i);
  expect(response.status(), `error code=${body.code}`).toBe(status);
  return body;
}
