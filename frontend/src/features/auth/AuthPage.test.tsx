import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AuthProvider, useAuth } from './AuthProvider';
import { AuthPage } from './AuthPage';
import { authSession } from './session';
const user = {
  id: 'u',
  username: 'member',
  displayName: 'member',
  role: 'annotator',
  workspaceId: 'w',
};
function Gate() {
  const auth = useAuth();
  return auth.session ? <p>当前账号 {auth.session.user.username}</p> : <AuthPage />;
}
afterEach(() => {
  cleanup();
  authSession.clear();
  vi.unstubAllGlobals();
});

it('enforces username syntax with the modern browser Unicode Sets pattern', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => new Response('{}', { status: 401 })),
  );
  render(
    <AuthProvider>
      <Gate />
    </AuthProvider>,
  );
  const input = await screen.findByRole('textbox', { name: '用户名' });
  const pattern = input.getAttribute('pattern');
  expect(pattern).toBeTruthy();
  const browserPattern = new RegExp(`^(?:${pattern})$`, 'v');
  expect(browserPattern.test('reviewer_01')).toBe(true);
  expect(browserPattern.test('member-2')).toBe(true);
  for (const invalid of ['ab', 'member.name', 'member name', '标注员']) {
    expect(browserPattern.test(invalid)).toBe(false);
  }
});

it('registers with invite code, then separately signs in using the entered credentials', async () => {
  const calls: Array<[string, RequestInit | undefined]> = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, options?: RequestInit) => {
      calls.push([url, options]);
      if (url.endsWith('/me'))
        return new Response(JSON.stringify({ code: 'UNAUTHENTICATED' }), { status: 401 });
      return new Response(
        JSON.stringify(url.endsWith('/register') ? { user } : { user, csrfToken: 'csrf' }),
        {
          status: url.endsWith('/register') ? 201 : 200,
          headers: { 'Content-Type': 'application/json' },
        },
      );
    }),
  );
  render(
    <AuthProvider>
      <Gate />
    </AuthProvider>,
  );
  await screen.findByRole('button', { name: '注册账号' });
  fireEvent.click(screen.getByRole('button', { name: '注册账号' }));
  fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'member' } });
  fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'synthetic-password' } });
  fireEvent.change(screen.getByLabelText('邀请码'), { target: { value: 'synthetic-invite' } });
  fireEvent.click(screen.getByRole('button', { name: '创建账号' }));
  await screen.findByText('账号已创建，请登录进入工作区。');
  expect(calls.some(([url]) => url.endsWith('/login'))).toBe(false);
  expect(JSON.parse(calls.find(([url]) => url.endsWith('/register'))?.[1]?.body as string)).toEqual(
    { username: 'member', password: 'synthetic-password', inviteCode: 'synthetic-invite' },
  );
  fireEvent.click(screen.getByRole('button', { name: '登录工作区' }));
  await screen.findByText('当前账号 member');
  await waitFor(() => expect(authSession.current?.user.id).toBe('u'));
  expect(localStorage.length).toBe(0);
});

it('shows failed registration and never grants workspace access', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify({ code: 'INVALID_CREDENTIALS', message: '注册信息无效。' }), {
          status: 401,
        }),
    ),
  );
  render(
    <AuthProvider>
      <Gate />
    </AuthProvider>,
  );
  await screen.findByRole('button', { name: '注册账号' });
  fireEvent.click(screen.getByRole('button', { name: '注册账号' }));
  fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'member' } });
  fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'synthetic-password' } });
  fireEvent.change(screen.getByLabelText('邀请码'), { target: { value: 'wrong-invite' } });
  fireEvent.click(screen.getByRole('button', { name: '创建账号' }));
  expect((await screen.findByRole('alert')).textContent).toContain('注册信息无效');
  expect(authSession.current).toBeUndefined();
});
