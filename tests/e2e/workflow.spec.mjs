import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import { createHash } from 'node:crypto';
import { fixture, snapshot, upload, extract, save, terminalJob } from '../helpers/api.mjs';
import { inspectExport } from '../helpers/zip.mjs';

async function drag(page, start, end) {
  await page.mouse.move(start.x, start.y);
  await page.mouse.down();
  await page.mouse.move(end.x, end.y, { steps: 10 });
  await page.mouse.up();
}

async function persisted(request, id, predicate) {
  let result;
  await expect.poll(async () => {
    result = await snapshot(request, id);
    return predicate(result);
  }).toBe(true);
  return result;
}

test('P0-E2E-001 browser upload/extract/draw/move/resize/delete/autosave/refresh/complete/export', async ({ page, request }, testInfo) => {
  await page.goto('/');
  await page.getByRole('tab', { name: '本地上传', exact: true }).click();
  await page.getByTestId('upload-file').setInputFiles(fixture());
  const uploaded = page.waitForResponse(response => response.url().endsWith('/api/v1/videos') && response.request().method() === 'POST');
  await page.getByTestId('upload-submit').click();
  const videoResponse = await uploaded;
  expect(videoResponse.status()).toBe(201);
  const video = await videoResponse.json();
  await page.getByTestId('video-select').selectOption(video.id);
  await page.getByTestId('interval-seconds').fill('1');
  await page.getByTestId('start-seconds').fill('0');
  await page.getByTestId('end-seconds').fill('3');
  const extracted = page.waitForResponse(response => response.url().endsWith('/extractions') && response.request().method() === 'POST');
  await page.getByTestId('extract-submit').click();
  const extractionResponse = await extracted;
  expect(extractionResponse.status()).toBe(202);
  const job = await extractionResponse.json();
  expect((await terminalJob(request, job.id)).status).toBe('completed');
  let frames;
  await expect.poll(async () => {
    const response = await request.get(`/api/v1/videos/${video.id}/frames?batchId=${job.batchId}`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    frames = body.frames;
    return frames.length;
  }, { timeout: 90_000 }).toBe(3);
  const frame = frames[0];
  await page.getByTestId(`frame-item-${frame.id}`).click();
  const canvas = page.getByTestId('annotation-canvas').locator('.konvajs-content');
  await expect(canvas).toBeVisible();
  const bounds = await canvas.boundingBox();
  expect(bounds).toBeTruthy();
  const point = (x, y) => ({ x: bounds.x + bounds.width * x, y: bounds.y + bounds.height * y });
  await page.getByTestId('draw-mode').click();
  await drag(page, point(0.15, 0.15), point(0.35, 0.35));
  const created = await persisted(request, frame.id, state => state.annotations.length === 1);
  const original = created.annotations[0].bbox;
  await page.getByTestId('select-mode').click();
  await drag(page, point(0.25, 0.25), point(0.35, 0.30));
  const moved = await persisted(request, frame.id, state => state.revision > created.revision);
  expect(moved.annotations[0].id).toBe(created.annotations[0].id);
  expect(moved.annotations[0].bbox[0]).toBeGreaterThan(original[0]);
  expect(moved.annotations[0].bbox[2]).toBeCloseTo(original[2], 0);
  const box = moved.annotations[0].bbox;
  const corner = point((box[0] + box[2]) / frame.width, (box[1] + box[3]) / frame.height);
  await drag(page, corner, { x: corner.x + bounds.width * 0.08, y: corner.y + bounds.height * 0.08 });
  const resized = await persisted(request, frame.id, state => state.revision > moved.revision);
  expect(resized.annotations[0].bbox[2]).toBeGreaterThan(box[2]);
  await page.getByTestId('draw-mode').click();
  await drag(page, point(0.65, 0.15), point(0.80, 0.30));
  const two = await persisted(request, frame.id, state => state.annotations.length === 2);
  await page.getByTestId('select-mode').click();
  await page.mouse.click(point(0.70, 0.20).x, point(0.70, 0.20).y);
  await page.keyboard.press('Delete');
  const one = await persisted(request, frame.id, state => state.annotations.length === 1 && state.revision > two.revision);
  expect(one.annotations[0].id).toBe(created.annotations[0].id);
  await page.reload();
  await page.getByTestId('video-select').selectOption(video.id);
  await page.getByTestId(`frame-item-${frame.id}`).click();
  await expect(page.getByTestId(`annotation-item-${one.annotations[0].id}`)).toBeVisible();
  expect(await snapshot(request, frame.id)).toEqual(one);
  await page.getByTestId('complete-frame').click();
  await persisted(request, frame.id, state => state.status === 'completed');
  await page.getByTestId(`frame-item-${frames[1].id}`).click();
  await page.getByTestId('complete-frame').click();
  await persisted(request, frames[1].id, state => state.status === 'completed' && state.annotations.length === 0);
  const downloadPromise = page.waitForEvent('download');
  await page.getByTestId('export-zip').click();
  const download = await downloadPromise;
  const file = await download.path();
  const expected = [];
  for (const item of frames.slice(0, 2)) {
    const state = await snapshot(request, item.id);
    const media = await request.get(item.imageUrl);
    expect(media.status()).toBe(200);
    expected.push({ ...item, annotations: state.annotations, imageBytes: await media.body() });
  }
  const exportBytes = fs.readFileSync(file);
  const coco = inspectExport(exportBytes, expected);
  const progressResponse = await request.get(`/api/v1/videos/${video.id}/progress?batchId=${job.batchId}`);
  expect(progressResponse.status()).toBe(200);
  const progress = await progressResponse.json();
  expect(progress).toEqual({ total: 3, notStarted: 1, inProgress: 0, completed: 2 });
  const baseline = {
    observedAt: new Date().toISOString(), deployedSHA: process.env.P0_EXPECTED_SHA,
    videoId: video.id, batchId: job.batchId, progress, coco,
    zipSHA256: createHash('sha256').update(exportBytes).digest('hex'),
    frames: await Promise.all(expected.map(async item => ({
      id: item.id, width: item.width, height: item.height,
      snapshot: await snapshot(request, item.id),
      imageSHA256: createHash('sha256').update(item.imageBytes).digest('hex'),
    }))),
  };
  fs.writeFileSync(testInfo.outputPath('restore-baseline.json'), JSON.stringify(baseline, null, 2));
  await page.getByTestId(`frame-item-${frame.id}`).click();
  await expect(page.getByTestId(`annotation-item-${one.annotations[0].id}`)).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('core-workflow.png'), fullPage: true });
});

async function openPreparedFrame(page, request) {
  const video = await upload(request);
  const { frames } = await extract(request, video.id);
  await page.goto('/');
  await page.getByTestId('video-select').selectOption(video.id);
  await page.getByTestId(`frame-item-${frames[0].id}`).click();
  const canvas = page.getByTestId('annotation-canvas').locator('.konvajs-content');
  await expect(canvas).toBeVisible();
  const bounds = await canvas.boundingBox();
  const point = (x, y) => ({ x: bounds.x + bounds.width * x, y: bounds.y + bounds.height * y });
  return { frame: frames[0], point };
}

test('P0-SAVE-001 real server acknowledgement cannot discard a newer canvas edit', async ({ page, request }) => {
  const { frame, point } = await openPreparedFrame(page, request);
  let first = true;
  let acknowledge;
  let captured;
  const sent = new Promise(resolve => { captured = resolve; });
  const release = new Promise(resolve => { acknowledge = resolve; });
  await page.route(`**/api/v1/frames/${frame.id}/annotations`, async route => {
    if (route.request().method() !== 'PUT' || !first) return route.continue();
    first = false;
    const response = await route.fetch(); // Real server commits first snapshot.
    captured();
    await release; // Delay only acknowledgement while user makes a second edit.
    await route.fulfill({ response });
  });
  await page.getByTestId('draw-mode').click();
  await drag(page, point(0.1, 0.1), point(0.3, 0.3));
  await sent;
  await drag(page, point(0.6, 0.1), point(0.8, 0.3));
  acknowledge();
  const final = await persisted(request, frame.id, state => state.annotations.length === 2 && state.revision >= 2);
  await page.reload();
  await page.getByTestId('video-select').selectOption(frame.videoId);
  await page.getByTestId(`frame-item-${frame.id}`).click();
  for (const annotation of final.annotations) await expect(page.getByTestId(`annotation-item-${annotation.id}`)).toBeVisible();
});

test('P0-SAVE-001 stale browser revision shows conflict and preserves server snapshot', async ({ page, request }) => {
  const { frame, point } = await openPreparedFrame(page, request);
  const otherWriterId = crypto.randomUUID();
  const remote = await save(request, frame, [{ id: otherWriterId, bbox: [10, 10, 20, 20], source: 'manual' }]);
  await page.getByTestId('draw-mode').click();
  await drag(page, point(0.6, 0.1), point(0.8, 0.3));
  await expect(page.getByTestId('save-state')).toContainText('冲突');
  expect(await snapshot(request, frame.id)).toEqual(remote);
  await page.getByTestId('conflict-reload').click();
  await expect(page.getByTestId(`annotation-item-${otherWriterId}`)).toBeVisible();
  expect(await snapshot(request, frame.id)).toEqual(remote);
});
