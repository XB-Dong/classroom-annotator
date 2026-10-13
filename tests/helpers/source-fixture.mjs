import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { createHash } from 'node:crypto';
import { externalPath } from './postgres.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve));

// The observed existing service has id/url/size_bytes/content_type/sha256;
// it is deliberately different from the project's sourceId bridge contract.
export async function sourceFixture(directory, token, datasetKey) {
  const root = externalPath(directory, 'Synthetic fixture directory');
  const video = fs.readFileSync(path.join(root, 'classroom20.mp4'));
  const image = fs.readFileSync(path.join(root, 'source-image.jpg'));
  const records = new Map(Array.from({ length: 9 }, (_, index) => {
    const isVideo = index < 2;
    return [`opaque-object-${index + 1}.${isVideo ? 'mp4' : 'jpg'}`, { bytes: isVideo ? video : image, contentType: isVideo ? 'video/mp4' : 'image/jpeg' }];
  }));
  const evidence = { sourceRequests: [], apiRequests: [], mediaRequests: [], violations: [] };
  let changed = false;
  let apiFailure = false;
  let unsafeId = false;
  let origin;
  const media = http.createServer((request, response) => {
    evidence.mediaRequests.push({ path: request.url, hasAuthorization: !!request.headers.authorization, hasAPIKey: !!request.headers['x-api-key'] });
    if (request.headers.authorization || request.headers['x-api-key']) evidence.violations.push('Company credentials reached media origin');
    response.writeHead(200, { 'Content-Type': 'video/mp4', 'Content-Length': video.length }); response.end(video);
  });
  await listen(media);
  const mediaOrigin = `http://127.0.0.1:${media.address().port}`;
  const api = http.createServer((request, response) => {
    const url = new URL(request.url, 'http://fixture');
    evidence.apiRequests.push({ path: url.pathname, page: url.searchParams.get('page'), pageSize: url.searchParams.get('page_size') });
    if (request.headers['x-api-key'] !== datasetKey) evidence.violations.push('Company API key missing');
    response.setHeader('Content-Type', 'application/json');
    if (apiFailure) { response.writeHead(502); response.end('{"error":"synthetic failure"}'); return; }
    if (url.pathname === '/api/v1/datasets/42/lessons') {
      response.end(JSON.stringify({ type: 'lessons', items: [{ id: unsafeId ? 9007199254740992 : 7, course_name: 'Synthetic class', organization_name: 'Fixture', teacher: 'Fixture teacher', date: '2026-01-01', section_index: 1,
        videos: [1, 4].map(channel => ({ id: channel, lesson_id: 7, catalog_file_id: channel, channel, part_index: 0, tags: {} })) }], total: 2, stats: {} }));
    } else if (url.pathname === '/api/v1/catalog-files/4/play-url') response.end(JSON.stringify({ url: `${mediaOrigin}/synthetic.mp4` }));
    else { response.writeHead(404); response.end('{}'); }
  });
  await listen(api);
  const server = http.createServer((request, response) => {
    const url = new URL(request.url, origin);
    evidence.sourceRequests.push({ path: url.pathname, method: request.method, range: request.headers.range || null });
    if (request.headers.authorization !== `Bearer ${token}`) { response.writeHead(401); response.end(); return; }
    response.setHeader('Cache-Control', 'no-store');
    if (url.pathname === '/v1/media' && request.method === 'GET') {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ items: [...records].map(([id, record]) => ({ id, url: `/v1/media/${id}`, size_bytes: record.bytes.length, content_type: record.contentType, sha256: changed && id.endsWith('1.mp4') ? hash(Buffer.from('changed identity')) : hash(record.bytes) })) })); return;
    }
    const record = records.get(decodeURIComponent(url.pathname.replace(/^\/v1\/media\//, '')));
    if (!record || !['GET', 'HEAD'].includes(request.method)) { response.writeHead(404); response.end(); return; }
    let start = 0; let end = record.bytes.length - 1;
    if (request.headers.range) {
      const match = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range);
      if (!match || Number(match[1]) >= record.bytes.length || (match[2] && Number(match[2]) < Number(match[1]))) {
        response.writeHead(416, { 'Content-Range': `bytes */${record.bytes.length}` }); response.end(); return;
      }
      start = Number(match[1]); if (match[2]) end = Math.min(Number(match[2]), end);
      response.statusCode = 206; response.setHeader('Content-Range', `bytes ${start}-${end}/${record.bytes.length}`);
    }
    response.setHeader('Content-Type', record.contentType); response.setHeader('Content-Length', end - start + 1);
    response.setHeader('ETag', `"${hash(record.bytes)}"`); response.setHeader('Accept-Ranges', 'bytes');
    response.end(request.method === 'HEAD' ? undefined : record.bytes.subarray(start, end + 1));
  });
  await listen(server); origin = `http://127.0.0.1:${server.address().port}`;
  return { origin, apiOrigin: `http://127.0.0.1:${api.address().port}`, mediaOrigin, records, video, image, evidence,
    setChanged: value => { changed = value; }, setAPIFailure: value => { apiFailure = value; }, setUnsafeId: value => { unsafeId = value; },
    assertClean: () => assert.deepEqual(evidence.violations, []),
    stop: async () => { for (const service of [server, api, media]) { service.closeAllConnections(); await new Promise(resolve => service.close(resolve)); } },
  };
}
