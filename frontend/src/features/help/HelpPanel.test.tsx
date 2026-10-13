import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { HelpPanel } from './HelpPanel';
import { AuthProvider } from '../auth/AuthProvider';
import { authSession } from '../auth/session';
const document = {
  documentVersion: '0.2.0',
  documentSHA256: 'a'.repeat(64),
  content:
    '# 使用手册\n\n## 1：注册 {#register}\n\n使用邀请码注册。\n\n## 2: 风险 {#risk}\n\n高风险框需要逐个确认。',
  sections: [
    { id: 'register', title: '注册', anchor: '#register' },
    { id: 'risk', title: '风险', anchor: '#risk' },
  ],
};
afterEach(() => {
  cleanup();
  authSession.clear();
  vi.unstubAllGlobals();
});
it('lets anonymous users read the whole manual and navigate a section', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async (url: string) =>
        new Response(
          JSON.stringify(url.endsWith('/document') ? document : { code: 'UNAUTHENTICATED' }),
          {
            status: url.endsWith('/document') ? 200 : 401,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
    ),
  );
  render(
    <AuthProvider>
      <HelpPanel close={() => {}} />
    </AuthProvider>,
  );
  await screen.findByText('使用邀请码注册。');
  expect(screen.getByText('高风险框需要逐个确认。')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '风险' }));
  expect(screen.getByRole('heading', { name: '风险' }).id).toBe('risk');
  expect((screen.getByRole('button', { name: '依据手册回答' }) as HTMLButtonElement).disabled).toBe(
    true,
  );
});
it('renders an answer as text and only links references from the controlled manual', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const value = url.endsWith('/document')
        ? document
        : url.endsWith('/me')
          ? {
              user: {
                id: 'u',
                username: 'u',
                displayName: 'u',
                role: 'annotator',
                workspaceId: 'w',
              },
              csrfToken: 'csrf',
            }
          : {
              answer: '<script>unsafe()</script>高风险框需逐个确认。',
              references: [
                { sectionId: 'risk', title: '风险', anchor: '#risk' },
                { sectionId: 'risk', title: '恶意外链', anchor: 'https://untrusted.example' },
              ],
              documentVersion: document.documentVersion,
              documentSHA256: document.documentSHA256,
            };
      return new Response(JSON.stringify(value), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  render(
    <AuthProvider>
      <HelpPanel close={() => {}} />
    </AuthProvider>,
  );
  await screen.findByText('使用邀请码注册。');
  fireEvent.change(screen.getByLabelText('操作问题'), { target: { value: '高风险框怎么办？' } });
  fireEvent.click(screen.getByRole('button', { name: '依据手册回答' }));
  await screen.findByText('<script>unsafe()</script>高风险框需逐个确认。');
  expect(screen.queryByRole('button', { name: '参见：恶意外链' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '参见：风险' }));
  expect(screen.getByRole('heading', { name: '风险' }).id).toBe('risk');
});

it('discards a late answer when the account changes', async () => {
  const user = {
    id: 'alice',
    username: 'alice',
    displayName: 'alice',
    role: 'annotator' as const,
    workspaceId: 'w',
  };
  let finish!: (response: Response) => void;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/questions'))
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      return new Response(
        JSON.stringify(url.endsWith('/document') ? document : { user, csrfToken: 'csrf' }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }),
  );
  render(
    <AuthProvider>
      <HelpPanel close={() => {}} />
    </AuthProvider>,
  );
  await screen.findByText('使用邀请码注册。');
  fireEvent.change(screen.getByLabelText('操作问题'), { target: { value: '如何注册？' } });
  fireEvent.click(screen.getByRole('button', { name: '依据手册回答' }));
  act(() => authSession.activate({ user: { ...user, id: 'bob' }, csrfToken: 'new-csrf' }));
  await act(async () =>
    finish(
      new Response(
        JSON.stringify({
          answer: 'Alice old answer',
          references: [],
          documentVersion: document.documentVersion,
          documentSHA256: document.documentSHA256,
        }),
        { headers: { 'Content-Type': 'application/json' } },
      ),
    ),
  );
  expect(screen.queryByText('Alice old answer')).toBeNull();
});
it('keeps the manual readable when the question API fails', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const value = url.endsWith('/document')
        ? document
        : url.endsWith('/me')
          ? {
              user: {
                id: 'u',
                username: 'u',
                displayName: 'u',
                role: 'annotator',
                workspaceId: 'w',
              },
              csrfToken: 'csrf',
            }
          : { code: 'HELP_UNAVAILABLE', message: '问答暂不可用，请查阅手册。' };
      return new Response(JSON.stringify(value), {
        status: url.endsWith('/questions') ? 503 : 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  render(
    <AuthProvider>
      <HelpPanel close={() => {}} />
    </AuthProvider>,
  );
  await screen.findByText('使用邀请码注册。');
  fireEvent.change(screen.getByLabelText('操作问题'), { target: { value: '如何注册？' } });
  fireEvent.click(screen.getByRole('button', { name: '依据手册回答' }));
  expect((await screen.findByRole('alert')).textContent).toContain('问答暂不可用');
  expect(screen.getByText('高风险框需要逐个确认。')).toBeTruthy();
});
