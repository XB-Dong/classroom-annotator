import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import { acceptance, externalPath } from '../helpers/postgres.mjs';
import { dedicatedServer, waitFor } from '../helpers/process.mjs';
import { unusedLoopbackPort } from '../helpers/runtime.mjs';
import { sourceFixture } from '../helpers/source-fixture.mjs';
import { inspectExport } from '../helpers/zip.mjs';

const hash = bytes => createHash('sha256').update(bytes).digest('hex');

await acceptance(async () => {
  assert.match(process.env.P01_EXPECTED_AGENT_SHA || '', /^[0-9a-f]{40}$/, 'Set exact agent binary source SHA');
  const root = externalPath(process.env.P01_MAC_OUTPUT, 'P01_MAC_OUTPUT');
  assert.ok(!fs.existsSync(root), 'Use a fresh external agent acceptance directory');
  fs.mkdirSync(root, { recursive: true });
  const fixtures = externalPath(process.env.P0_FIXTURE_DIR, 'P0_FIXTURE_DIR');
  const token = randomUUID(); const sourceToken = randomUUID(); const datasetKey = randomUUID();
  const tokenFile = path.join(root, 'agent-token'); const sourceTokenFile = path.join(root, 'source-token');
  const datasetKeyFile = path.join(root, 'dataset-key');
  fs.writeFileSync(tokenFile, token, { mode: 0o600 }); fs.writeFileSync(sourceTokenFile, sourceToken, { mode: 0o600 });
  fs.writeFileSync(datasetKeyFile, datasetKey, { mode: 0o600 });
  const fixture = await sourceFixture(fixtures, sourceToken, datasetKey);
  const origin = `http://127.0.0.1:${await unusedLoopbackPort()}`;
  const mediaRoot = path.join(root, 'agent-media');
  const environment = { MAC_AGENT_ADDR: origin.replace('http://', ''), MAC_MEDIA_ROOT: mediaRoot, MAC_TOKEN_FILE: tokenFile,
    MAC_SOURCE_SERVICE_URL: fixture.origin, MAC_SOURCE_TOKEN_FILE: sourceTokenFile,
    MAC_FFMPEG: process.env.FFMPEG_PATH, MAC_FFPROBE: process.env.FFPROBE_PATH,
    MAC_DATASET_API_URL: fixture.apiOrigin, MAC_DATASET_ID: '42', MAC_DATASET_API_KEY_FILE: datasetKeyFile,
    MAC_DATASET_ALLOWED_PRIVATE_ORIGINS: `${fixture.origin},${fixture.apiOrigin},${fixture.mediaOrigin}`,
    MAC_DATASET_ALLOWED_MEDIA_ORIGINS: fixture.mediaOrigin,
    APP_ALLOWED_PRIVATE_ORIGINS: `${fixture.origin},${fixture.apiOrigin},${fixture.mediaOrigin}`,
    APP_ALLOWED_MEDIA_ORIGINS: fixture.mediaOrigin,
    APP_VERSION: process.env.P01_EXPECTED_AGENT_SHA, MAC_AGENT_VERSION: process.env.P01_EXPECTED_AGENT_SHA };
  let processHandle;
  const request = (route, options = {}) => fetch(`${origin}/bridge/v1${route}`, { ...options,
    headers: { Authorization: `Bearer ${token}`, ...options.headers }, signal: AbortSignal.timeout(30_000) });
  const body = value => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value) });
  const json = async (route, options = {}, status = 200) => {
    const response = await request(route, options); assert.equal(response.status, status, `Bridge status on ${route}`);
    return response.json();
  };
  const terminal = id => waitFor(async () => {
    const operation = await json(`/operations/${id}`);
    return ['completed', 'failed', 'cancelled'].includes(operation.status) && operation;
  }, 90_000, 'Actual agent operation terminal state');
  const evidence = { observedAt: new Date().toISOString(), sourceSHA: process.env.P01_EXPECTED_AGENT_SHA,
    environment: 'Windows native Go media agent with synthetic existing v1 service/company API', dataType: 'synthetic', actualMacVerificationStatus: 'not_run' };
  try {
    processHandle = await dedicatedServer(process.env.P01_AGENT_BINARY, environment, origin, '/health/live');
    const unauthorized = await fetch(`${origin}/bridge/v1/capabilities`);
    assert.equal(unauthorized.status, 401);
    const capabilities = await json('/capabilities');
    assert.equal(capabilities.contractVersion, '1.0.0'); assert.equal(capabilities.agentVersion, process.env.P01_EXPECTED_AGENT_SHA);
    assert.equal(capabilities.ffmpegAvailable, true); assert.equal(capabilities.ffprobeAvailable, true);
    assert.equal(capabilities.mediaRootWritable, true); assert.equal(capabilities.maxFrames, 1000);
    evidence.capabilities = capabilities;
    const sources = await json('/sources');
    assert.equal(sources.contractVersion, '1.0.0'); assert.equal(sources.items.length, 9); assert.equal(sources.nextCursor, null);
    const videos = sources.items.filter(item => item.mediaKind === 'video');
    assert.equal(videos.length, 2); assert.equal(sources.items.filter(item => item.mediaKind === 'image').length, 7);
    assert.equal(new Set(sources.items.map(item => item.sourceId)).size, 9);
    assert.ok(sources.items.every(item => !fixture.records.has(item.sourceId)), 'Existing id was confused with generated sourceId');
    assert.ok(!JSON.stringify(sources).includes(mediaRoot) && !JSON.stringify(sources).includes(fixture.origin));
    const registrationRequest = { operationId: randomUUID(), videoId: randomUUID(), sourceId: videos[0].sourceId };
    const registered = await json('/registrations', body(registrationRequest));
    assert.equal(registered.operationId, registrationRequest.operationId); assert.equal(registered.videoId, registrationRequest.videoId);
    assert.equal(registered.sha256, hash(fixture.video)); assert.equal(registered.sizeBytes, fixture.video.length);
    assert.deepEqual([registered.width, registered.height], [320, 240]); assert.equal(registered.durationSeconds, 20);
    assert.deepEqual(await json('/registrations', body(registrationRequest)), registered, 'Identical registration changed media identity');
    assert.equal((await request('/registrations', body({ ...registrationRequest, videoId: randomUUID() }))).status, 409);
    const ack = { publicationRevision: 1 };
    await json(`/operations/${registrationRequest.operationId}/ack`, body(ack));
    await json(`/operations/${registrationRequest.operationId}/ack`, body(ack));
    assert.equal((await request(`/operations/${registrationRequest.operationId}/ack`, body({ publicationRevision: 2 }))).status, 409);
    const original = await request(`/media/${registered.mediaKey}`); assert.equal(original.status, 200);
    assert.deepEqual(Buffer.from(await original.arrayBuffer()), fixture.video);
    const head = await request(`/media/${registered.mediaKey}`, { method: 'HEAD' }); assert.equal(head.status, 200);
    assert.equal(Number(head.headers.get('content-length')), fixture.video.length);
    const range = await request(`/media/${registered.mediaKey}`, { headers: { Range: 'bytes=7-22' } }); assert.equal(range.status, 206);
    assert.deepEqual(Buffer.from(await range.arrayBuffer()), fixture.video.subarray(7, 23));
    assert.equal((await request(`/media/${registered.mediaKey}`, { headers: { Range: `bytes=${fixture.video.length}-` } })).status, 416);
    assert.ok([400, 404].includes((await request('/media/%2e%2e%2fmanifest.json')).status));
    const extraction = { operationId: randomUUID(), jobId: randomUUID(), batchId: randomUUID(), videoId: registered.videoId,
      mediaKey: registered.mediaKey, timestampsMs: Array.from({ length: 10 }, (_, index) => index * 2000), deadlineSeconds: 600 };
    await json('/extractions', body(extraction), 202);
    const extracted = await terminal(extraction.operationId); assert.equal(extracted.status, 'completed');
    assert.equal(extracted.resourceId, extraction.batchId); assert.equal(extracted.jobId, extraction.jobId);
    const frames = extracted.manifest.frames;
    assert.equal(extracted.manifest.batchId, extraction.batchId);
    assert.equal(frames.length, 10); assert.deepEqual(frames.map(frame => frame.timestampMs), extraction.timestampsMs);
    assert.equal(new Set(frames.map(frame => frame.frameId)).size, 10); assert.equal(new Set(frames.map(frame => frame.mediaKey)).size, 10);
    for (const frame of frames) {
      assert.deepEqual([frame.width, frame.height], [320, 240]);
      const media = await request(`/media/${frame.mediaKey}`); assert.equal(media.status, 200);
      frame.imageBytes = Buffer.from(await media.arrayBuffer());
      assert.equal(hash(frame.imageBytes), frame.sha256); assert.equal(frame.imageBytes.length, frame.sizeBytes);
    }
    const replay = await request('/extractions', body(extraction)); assert.ok([200, 202].includes(replay.status));
    assert.deepEqual((await json(`/operations/${extraction.operationId}`)).manifest.frames,
      frames.map(({ imageBytes, ...frame }) => frame), 'Extraction replay changed original frame identities');
    assert.equal((await request('/extractions', body({ ...extraction, timestampsMs: [0] }))).status, 409);
    // Prepared resources survive an actual process restart before acknowledgement.
    await processHandle.stop();
    processHandle = await dedicatedServer(process.env.P01_AGENT_BINARY, environment, origin, '/health/live');
    assert.deepEqual((await json('/sources')).items.map(item => item.sourceId).sort(), sources.items.map(item => item.sourceId).sort());
    assert.deepEqual((await json(`/operations/${extraction.operationId}`)).manifest.frames, frames.map(({ imageBytes, ...frame }) => frame));
    for (const frame of frames) assert.deepEqual(Buffer.from(await (await request(`/media/${frame.mediaKey}`)).arrayBuffer()), frame.imageBytes);
    await json(`/operations/${extraction.operationId}/ack`, body(ack));
    const selected = frames.slice(0, 2);
    selected[0].annotations = [{ id: randomUUID(), bbox: [10, 20, 30, 40], source: 'manual' }]; selected[1].annotations = [];
    const coco = { images: selected.map((frame, index) => ({ id: index + 1, file_name: `images/${frame.frameId}.jpg`, width: frame.width, height: frame.height })),
      annotations: [{ id: 1, image_id: 1, category_id: 1, bbox: [10, 20, 30, 40], area: 1200, iscrowd: 0 }], categories: [{ id: 1, name: 'head' }] };
    const archiveRequest = { operationId: randomUUID(), images: selected.map(frame => ({ frameId: frame.frameId, mediaKey: frame.mediaKey,
      sha256: frame.sha256, sizeBytes: frame.sizeBytes, width: frame.width, height: frame.height })), coco };
    await json('/archives', body(archiveRequest), 202);
    const archive = await terminal(archiveRequest.operationId); assert.equal(archive.status, 'completed');
    assert.equal(archive.jobId, null, 'Archive must not borrow a business job');
    const archiveResponse = await request(`/media/${archive.manifest.mediaKey}`); assert.equal(archiveResponse.status, 200);
    const zipBytes = Buffer.from(await archiveResponse.arrayBuffer());
    assert.equal(hash(zipBytes), archive.manifest.sha256); assert.equal(zipBytes.length, archive.manifest.sizeBytes);
    inspectExport(zipBytes, selected.map(frame => ({ ...frame, id: frame.frameId })));
    const lessons = await json('/dataset/lessons?page=2&page_size=1');
    assert.deepEqual(lessons.items[0].videos.map(item => item.channel), [1, 4]);
    assert.ok(fixture.evidence.apiRequests.some(call => call.page === '2' && call.pageSize === '1'));
    const imported = { operationId: randomUUID(), jobId: randomUUID(), videoId: randomUUID(), catalogFileId: '4', name: 'Synthetic company import' };
    await json('/dataset/imports', body(imported), 202);
    const dataset = await terminal(imported.operationId); assert.equal(dataset.status, 'completed');
    assert.equal(dataset.resourceId, imported.videoId); assert.equal(dataset.jobId, imported.jobId);
    assert.ok(fixture.evidence.apiRequests.some(call => call.path === '/api/v1/catalog-files/4/play-url'));
    assert.ok(fixture.evidence.mediaRequests.length > 0); fixture.assertClean();
    evidence.sourceCount = 9; evidence.videoSources = 2; evidence.imageSources = 7;
    evidence.registrationIdempotent = true; evidence.actualFFmpegFrameCount = 10;
    evidence.intervalMs = 2000; evidence.rangeStatus = 206; evidence.unsatisfiableRangeStatus = 416;
    evidence.ackReplayAndConflictChecked = true; evidence.restartBeforeAckPreservedPreparedResources = true;
    evidence.originalFrameBytesInArchive = true; evidence.zeroHeadFrameInArchive = true; evidence.syntheticDatasetAdapter = fixture.evidence;
    fs.writeFileSync(path.join(root, 'mac-media-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log('Windows synthetic media-agent acceptance passed: observed v1 source mapping/types, registration identity/idempotence, real FFmpeg 10-frame extraction, Range, restart/ack, original JPEG/COCO archive and company-key isolation. Actual Mac and cloud full-route gates remain separate.');
  } finally { if (processHandle) await processHandle.stop(); await fixture.stop(); }
});
