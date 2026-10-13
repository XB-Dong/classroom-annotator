import { test, expect } from '@playwright/test';
import { randomUUID, createHash } from 'node:crypto';
import { dedicatedConnection } from '../helpers/postgres.mjs';
import pg from 'pg';

const origin = () => ({ Origin: process.env.P0_BASE_URL });
test('P1-AUTH-001 registration is real, role-safe, unique, bounded and does not log in', async ({ request }) => {
  const username = `qa_${randomUUID().slice(0, 8)}`;
  const account = { username, password: 'synthetic-auth-password-2026', inviteCode: 'synthetic-ci-invite' };
  expect((await request.post('/api/v1/auth/register', { headers: origin(), data: { ...account, inviteCode: 'wrong' } })).status()).toBe(400);
  for (const password of ['short', 'x'.repeat(129)])
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
