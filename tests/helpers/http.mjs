import assert from 'node:assert/strict';
import { waitFor } from './process.mjs';

export function api(origin) {
  async function request(route, options = {}) {
    return fetch(`${origin}${route}`, { ...options, signal: AbortSignal.timeout(30_000) });
  }
  async function json(route, options = {}, status = 200) {
    const response = await request(route, options);
    assert.equal(response.status, status, `Unexpected HTTP status on ${route}`);
    return response.json();
  }
  const body = (value, method = 'POST') => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value) });
  const snapshot = id => json(`/api/v1/frames/${id}/annotations`);
  const put = async (id, value) => {
    const response = await request(`/api/v1/frames/${id}/annotations`, body(value, 'PUT'));
    const result = await response.json();
    if (!response.ok) {
      assert.ok(typeof result.code === 'string' && typeof result.message === 'string' && typeof result.requestId === 'string');
      assert.ok(!/postgres(?:ql)?:\/\/|SQLSTATE|23505|23503|password|token|api[_-]?key/i.test(JSON.stringify(result)), 'Unsafe public error response');
    }
    return { status: response.status, body: result };
  };
  const terminal = id => waitFor(async () => {
    const job = await json(`/api/v1/jobs/${id}`);
    return ['completed', 'failed', 'cancelled'].includes(job.status) && job;
  }, 90_000, 'Synthetic extraction terminal status');
  return { request, json, body, snapshot, put, terminal };
}
