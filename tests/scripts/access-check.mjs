import assert from 'node:assert/strict';

const base = process.env.P0_BASE_URL || 'http://127.0.0.1:8180';
const request = (path, authorization) => fetch(new URL(path, base), {
  headers: authorization ? { authorization } : {},
  signal: AbortSignal.timeout(15000),
});

for (const path of ['/', '/api/v1/videos', '/media/nonexistent.jpg', '/health/ready']) {
  const response = await request(path);
  assert.equal(response.status, 401, `Unauthenticated ${path} must be protected`);
  assert.match(response.headers.get('www-authenticate') || '', /^Basic /i);
}
assert.equal((await request('/health/live')).status, 200, 'Container liveness must remain available');

if (process.env.P0_DEMO_USER && process.env.P0_DEMO_PASSWORD) {
  const encode = (value) => `Basic ${Buffer.from(value).toString('base64')}`;
  const valid = encode(`${process.env.P0_DEMO_USER}:${process.env.P0_DEMO_PASSWORD}`);
  const invalid = encode(`${process.env.P0_DEMO_USER}:invalid-password-for-acceptance`);
  for (const path of ['/', '/api/v1/videos', '/media/nonexistent.jpg', '/health/ready']) {
    assert.equal((await request(path, invalid)).status, 401, `Incorrect credentials must not read ${path}`);
  }
  assert.equal((await request('/', valid)).status, 200);
  assert.equal((await request('/api/v1/videos', valid)).status, 200);
  if (process.env.P0_DEMO_MEDIA_PATH) {
    assert.equal((await request(process.env.P0_DEMO_MEDIA_PATH)).status, 401, 'Real frame must reject missing credentials');
    assert.equal((await request(process.env.P0_DEMO_MEDIA_PATH, invalid)).status, 401, 'Real frame must reject incorrect credentials');
    const response = await request(process.env.P0_DEMO_MEDIA_PATH, valid);
    assert.equal(response.status, 200, 'Authorized frame must be readable');
    assert.match(response.headers.get('content-type') || '', /^image\/jpeg/);
  }
  console.log('PASS: missing/wrong/valid credentials, liveness, and requested media checks.');
} else {
  console.log('PASS: unauthenticated protection and liveness; authenticated checks NOT RUN.');
}
