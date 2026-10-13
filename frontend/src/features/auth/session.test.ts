import { afterEach, expect, it, vi } from 'vitest';
import { authSession } from './session';
import { request } from '../../shared/api/client';

afterEach(() => {
  authSession.clear();
  vi.unstubAllGlobals();
});
const user = {
  id: 'alice',
  username: 'alice',
  displayName: 'Alice',
  role: 'annotator' as const,
  workspaceId: 'shared',
};

it('rejects a late response after logout and another account logs in', async () => {
  authSession.activate({ user, csrfToken: 'first' });
  let finish!: (response: Response) => void;
  vi.stubGlobal(
    'fetch',
    vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
    ),
  );
  const pending = request('/videos');
  authSession.clear();
  authSession.activate({ user: { ...user, id: 'bob' }, csrfToken: 'second' });
  finish(
    new Response(JSON.stringify({ videos: ['alice-private'] }), {
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  await expect(pending).rejects.toMatchObject({ code: 'STALE_AUTH_RESPONSE' });
});

it('sends CSRF from memory for writes and credentials for session cookies', async () => {
  authSession.activate({ user, csrfToken: 'current-csrf' });
  const fetchMock = vi.fn<(url: string, options?: RequestInit) => Promise<Response>>(
    async () => new Response('{}', { headers: { 'Content-Type': 'application/json' } }),
  );
  vi.stubGlobal('fetch', fetchMock);
  await request('/frames/f/annotations', { method: 'PUT' });
  const options = fetchMock.mock.calls[0][1] as RequestInit;
  expect(options.credentials).toBe('same-origin');
  expect(new Headers(options.headers).get('X-CSRF-Token')).toBe('current-csrf');
  expect(localStorage.length).toBe(0);
});

it('allows logout revocation to finish after clearing browser identity', async () => {
  authSession.activate({ user, csrfToken: 'current-csrf' });
  let finish!: (response: Response) => void;
  let signal!: AbortSignal;
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, options: RequestInit) => {
      signal = options.signal!;
      return new Promise<Response>((resolve) => {
        finish = resolve;
      });
    }),
  );
  const pending = request('/auth/logout', { method: 'POST' });
  authSession.clear();
  expect(signal.aborted).toBe(false);
  finish(new Response(null, { status: 204 }));
  await expect(pending).resolves.toBeUndefined();
});
it('exposes a failed logout revocation after local identity is cleared', async () => {
  authSession.activate({ user, csrfToken: 'current-csrf' });
  let reject!: (e: Error) => void;
  vi.stubGlobal(
    'fetch',
    vi.fn(
      () =>
        new Promise<Response>((_, fail) => {
          reject = fail;
        }),
    ),
  );
  const pending = request('/auth/logout', { method: 'POST' });
  authSession.clear();
  reject(new Error('offline'));
  await expect(pending).rejects.toMatchObject({ code: 'NETWORK_ERROR' });
});

it('clears identity and aborts requests on session expiry', async () => {
  authSession.activate({ user, csrfToken: 'current-csrf' });
  const previous = authSession.generation;
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => new Response(JSON.stringify({ code: 'UNAUTHENTICATED' }), { status: 401 })),
  );
  await expect(request('/videos')).rejects.toMatchObject({ status: 401 });
  expect(authSession.current).toBeUndefined();
  expect(authSession.generation).toBeGreaterThan(previous);
});
