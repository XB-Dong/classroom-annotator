import { test, expect } from '@playwright/test';
import { randomUUID, createHash } from 'node:crypto';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import pg from 'pg';
import { test as authenticatedTest } from '../helpers/p1-session.mjs';
import { upload, extract } from '../helpers/api.mjs';

const origin = () => ({ Origin: process.env.P0_BASE_URL });
test('P1-AUTH-001 registration is real, role-safe, unique, bounded and does not log in', async ({ request }) => {
  const username = `qa_${randomUUID().slice(0, 8)}`;
  const account = { username, password: 'synthetic-auth-password-2026', inviteCode: 'synthetic-ci-invite' };
  expect((await request.post('/api/v1/auth/register', { headers: origin(), data: { ...account, inviteCode: 'wrong' } })).status()).toBe(400);
  for (const password of ['short', 'x'.repeat(129), '码'.repeat(43)])
    expect((await request.post('/api/v1/auth/register', { headers: origin(), data: { ...account, password } })).status()).toBe(400);
  expect((await request.post('/api/v1/auth/register', { headers: origin(), data: { ...account, role: 'admin', userId: 'forged' } })).status()).toBe(400);
  const registered = await request.post('/api/v1/auth/register', { headers: origin(), data: account });
  expect(registered.status()).toBe(201);
  expect(registered.headers()['set-cookie']).toBeUndefined();
  const value = await registered.json();
  expect(value.user.role).toBe('annotator');
  expect(value.user.username).toBe(username);
  expect(JSON.stringify(value)).not.toMatch(/password|token|hash/i);
  expect((await request.get('/api/v1/auth/me')).status()).toBe(401);
  expect((await request.post('/api/v1/auth/register', { headers: origin(), data: account })).status()).toBe(400);
  expect((await request.post('/api/v1/auth/register', { headers: origin(), data: { ...account, username: username.toUpperCase() } })).status()).toBe(400);
  expect((await request.post('/api/v1/auth/login', { headers: { Origin: 'https://untrusted.invalid' }, data: account })).status()).toBe(403);
  const logged = await request.post('/api/v1/auth/login', { headers: origin(), data: { username, password: account.password } });
  expect(logged.status()).toBe(200);
  const cookie = logged.headers()['set-cookie'];
  expect(cookie).toMatch(/^cvte_session=/);
  expect(cookie).toMatch(/HttpOnly/i);
  expect(cookie).toMatch(/SameSite=Lax/i);
  const session = await logged.json();
  expect(session.csrfToken).toMatch(/^[A-Za-z0-9_-]{43}$/);
  expect((await request.post('/api/v1/auth/logout', { headers: origin() })).status()).toBe(403);
  expect((await request.post('/api/v1/auth/logout', { headers: { ...origin(), 'X-CSRF-Token': session.csrfToken } })).status()).toBe(204);
  expect((await request.get('/api/v1/auth/me')).status()).toBe(401);
  let limited = false;
  for (let attempt = 0; attempt < 12 && !limited; attempt++) {
    const duplicate = await request.post('/api/v1/auth/register', { headers: origin(), data: account });
    expect([400, 429]).toContain(duplicate.status());
    limited = duplicate.status() === 429;
    if (limited) expect(duplicate.headers()['retry-after']).toBe('60');
  }
  expect(limited).toBe(true);
});

test('P1-AUTH-001 anonymous business, media, HEAD, Range and export are denied', async ({ request }) => {
  for (const route of ['/api/v1/videos', '/api/v1/jobs', '/api/v1/dataset/lessons', '/api/v1/frames/missing/review',
    '/media/frames/missing.jpg']) {
    const response = await request.get(route);
    expect(response.status(), route).toBe(401);
    expect(response.headers()['cache-control']).toContain('no-store');
  }
  expect((await request.head('/media/frames/missing.jpg')).status()).toBe(401);
  expect((await request.get('/media/frames/missing.jpg', { headers: { Range: 'bytes=0-20' } })).status()).toBe(401);
  for (const route of ['/api/v1/exports', '/api/v1/help/questions', '/api/v1/frames/missing/preannotations'])
    expect((await request.post(route, { headers: origin(), data: {} })).status(), route).toBe(401);
});

test('P1-AUTH-001 server expiration revokes actual sessions and media access', async ({ playwright }) => {
  const context = await playwright.request.newContext({ baseURL: process.env.P0_BASE_URL });
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 1 });
  try {
    const response = await context.post('/api/v1/auth/login', { headers: origin(), data: {
      username: 'p1_account_two', password: 'synthetic-browser-password-2026' } });
    expect(response.status()).toBe(200);
    const cookie = (await context.storageState()).cookies.find(value => value.name === 'cvte_session');
    const digest = createHash('sha256').update(cookie.value).digest();
    expect((await context.get('/api/v1/auth/me')).status()).toBe(200);
    await pool.query("UPDATE sessions SET expires_at=now()-interval '1 second' WHERE token_digest=$1", [digest]);
    expect((await context.get('/api/v1/auth/me')).status()).toBe(401);
    expect((await context.head('/media/frames/missing.jpg')).status()).toBe(401);
  } finally { await context.dispose(); await pool.end(); }
});

authenticatedTest('P1-AUTH-001 resource identity, list, batch, media and ZIP remain scoped to the workspace', async ({ request }) => {
  const pool = new pg.Pool({ connectionString: dedicatedConnection('P1_QA_DATABASE_URL_FILE').connectionString, max: 1 });
  const video = await upload(request);
  const { job, frames } = await extract(request, video.id);
  const foreignWorkspace = randomUUID();
  let originalWorkspace;
  try {
    originalWorkspace = (await pool.query('SELECT workspace_id FROM videos WHERE id=$1', [video.id])).rows[0].workspace_id;
    await pool.query('INSERT INTO workspaces(id,name) VALUES($1,$2)', [foreignWorkspace, 'Synthetic foreign workspace']);
    await pool.query('UPDATE videos SET workspace_id=$1 WHERE id=$2', [foreignWorkspace, video.id]);
    expect((await (await request.get('/api/v1/videos')).json()).videos.some(value => value.id === video.id)).toBe(false);
    expect((await (await request.get('/api/v1/jobs')).json()).jobs.some(value => value.id === job.id)).toBe(false);
    for (const route of [`/api/v1/videos/${video.id}/frames`, `/api/v1/frames/${frames[0].id}/annotations`,
      `/api/v1/frames/${frames[0].id}/review`, `/api/v1/jobs/${job.id}`, frames[0].imageUrl])
      expect((await request.get(route)).status(), route).toBe(403);
    expect((await request.head(frames[0].imageUrl)).status()).toBe(403);
    expect((await request.get(frames[0].imageUrl, { headers: { Range: 'bytes=0-10' } })).status()).toBe(403);
    expect((await request.post(`/api/v1/batches/${job.batchId}/preannotations`, {
      headers: { 'Idempotency-Key': randomUUID() }, data: { frameIds: [frames[0].id] } })).status()).toBe(403);
    expect((await request.post('/api/v1/exports', { data: { frameIds: [frames[0].id] } })).status()).toBe(403);
  } finally {
    if (originalWorkspace) await pool.query('UPDATE videos SET workspace_id=$1 WHERE id=$2', [originalWorkspace, video.id]);
    await pool.query('DELETE FROM workspaces WHERE id=$1', [foreignWorkspace]);
    await pool.end();
  }
});
