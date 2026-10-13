import assert from 'node:assert/strict';
import http from 'node:http';
import { createHash, randomUUID } from 'node:crypto';
import { waitFor } from './process.mjs';

const canonical = value => Array.isArray(value) ? value.map(canonical)
  : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])])) : value;

// Synthetic bridge barriers are controlled only by the acceptance process.
// No product-only routes or hooks are required to order cancellation/publish.
export async function bridgeFixture(token, jpeg, version) {
  const operations = new Map();
  const events = [];
  const sha256 = createHash('sha256').update(jpeg).digest('hex');
  const error = (response, status, code) => {
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ code, message: code, requestId: randomUUID() }));
  };
  const json = (response, status, value) => {
    response.writeHead(status, { 'Content-Type': 'application/json' }); response.end(JSON.stringify(value));
  };
  const server = http.createServer(async (request, response) => {
    try {
      const url = new URL(request.url, 'http://fixture');
      if (request.headers.authorization !== `Bearer ${token}`) return error(response, 401, 'UNAUTHORIZED');
      response.setHeader('Cache-Control', 'no-store');
      if (url.pathname === '/bridge/v1/capabilities') return json(response, 200, {
        contractVersion: '1.0.0', agentVersion: version, mediaRootWritable: true, ffmpegAvailable: true, ffprobeAvailable: true,
        maxUploadBytes: 1073741824, maxFrames: 1000, leaseSeconds: 120, queueSize: 8,
        maxOperationSeconds: 600, archiveTtlSeconds: 3600, maxControlBytes: 16777216, maxArchiveImageBytes: 2147483648,
      });
      if (url.pathname === '/bridge/v1/extractions' && request.method === 'POST') {
        const chunks = []; for await (const chunk of request) chunks.push(chunk);
        const bytes = Buffer.concat(chunks); const input = JSON.parse(bytes.toString());
        assert.ok(Array.isArray(input.timestampsMs) && input.timestampsMs.length > 0);
        const operation = { contractVersion: '1.0.0', operationId: input.operationId, operationType: 'extraction',
          jobId: input.jobId, resourceId: input.batchId, requestFingerprint: createHash('sha256').update(JSON.stringify(canonical(input))).digest('hex'),
          publicationStatus: 'unpublished', publicationRevision: null, status: 'running', progress: 0, errorCode: null,
          manifest: { batchId: input.batchId, frames: input.timestampsMs.map(timestampMs => ({ frameId: randomUUID(), timestampMs,
            width: 320, height: 240, mediaKey: randomUUID(), sha256, sizeBytes: jpeg.length })) } };
        operations.set(input.operationId, operation); events.push({ kind: 'submitted', operationId: input.operationId, jobId: input.jobId });
        const { manifest, ...accepted } = operation;
        return json(response, 202, accepted);
      }
      const match = /^\/bridge\/v1\/operations\/([^/]+)(?:\/(cancel|renew|ack))?$/.exec(url.pathname);
      if (match) {
        const operation = operations.get(match[1]); if (!operation) return error(response, 404, 'RESOURCE_NOT_FOUND');
        const action = match[2];
        events.push({ kind: action || 'polled', operationId: operation.operationId, status: operation.status });
        if (action === 'cancel' && operation.status === 'running') operation.status = 'cancelled';
        if (action === 'ack') {
          const chunks = []; for await (const chunk of request) chunks.push(chunk);
          const input = JSON.parse(Buffer.concat(chunks).toString());
          if (!Number.isInteger(input.publicationRevision) || input.publicationRevision < 1) return error(response, 400, 'INVALID_REQUEST');
          if (operation.publicationRevision !== null && operation.publicationRevision !== input.publicationRevision) return error(response, 409, 'OPERATION_CONFLICT');
          operation.publicationRevision = input.publicationRevision; operation.publicationStatus = 'committed';
        }
        const { manifest, ...safe } = operation;
        return json(response, 200, operation.status === 'completed' ? operation : safe);
      }
      if (/^\/bridge\/v1\/media\/[^/]+$/.test(url.pathname)) {
        response.writeHead(200, { 'Content-Type': 'image/jpeg', 'Content-Length': jpeg.length, ETag: `"${sha256}"` });
        return response.end(request.method === 'HEAD' ? undefined : jpeg);
      }
      return error(response, 404, 'RESOURCE_NOT_FOUND');
    } catch { return error(response, 400, 'INVALID_REQUEST'); }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return { origin: `http://127.0.0.1:${server.address().port}`, events,
    inspect: id => structuredClone(operations.get(id)),
    operationForJob: id => waitFor(() => [...operations.values()].find(operation => operation.jobId === id), 10_000, 'Synthetic bridge received extraction'),
    complete: id => { const operation = operations.get(id); assert.ok(operation && operation.status === 'running'); operation.status = 'completed'; operation.progress = 1; events.push({ kind: 'barrierReleased', operationId: id }); },
    stop: async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); },
  };
}
