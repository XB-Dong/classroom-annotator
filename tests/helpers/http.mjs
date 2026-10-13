import assert from 'node:assert/strict';
import { waitFor } from './process.mjs';

export function api(origin) {
  let identity;
  async function authenticate() {
    if (identity) return identity;
    identity = (async () => {
      const headers = { Origin: origin, 'Content-Type': 'application/json' };
      const account = { username: 'p1_legacy_fixture', password: 'synthetic-ci-password-2026' };
      const registered = await fetch(`${origin}/api/v1/auth/register`, { method: 'POST', headers,
        body: JSON.stringify({ ...account, inviteCode: 'synthetic-ci-invite' }), signal: AbortSignal.timeout(30_000) });
      assert.ok([201, 400].includes(registered.status), 'Synthetic account creation failed');
      const login = await fetch(`${origin}/api/v1/auth/login`, { method: 'POST', headers,
        body: JSON.stringify(account), signal: AbortSignal.timeout(30_000) });
      assert.equal(login.status, 200, 'Synthetic account login failed');
      const value = await login.json();
      const cookie = login.headers.get('set-cookie')?.split(';')[0];
      assert.match(cookie || '', /^cvte_session=/);
      return { Cookie: cookie, Origin: origin, 'X-CSRF-Token': value.csrfToken };
    })();
    return identity;
  }
  async function request(route, options = {}) {
    const headers = route.startsWith('/health/') ? {} : await authenticate();
    return fetch(`${origin}${route}`, { ...options, headers: { ...headers, ...options.headers }, signal: AbortSignal.timeout(30_000) });
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
