import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { App } from './App';
import type { Snapshot } from '../shared/api/types';
import { authSession } from '../features/auth/session';
import { draftKey } from '../features/annotation/drafts';
import { act } from '@testing-library/react';
const draftScope = {
  deploymentOrigin: window.location.origin,
  workspaceId: 'shared',
  userId: 'test-user',
};
vi.mock('../features/annotation/Canvas', () => ({
  AnnotationCanvas: () => <div data-testid="annotation-canvas" />,
}));
afterEach(() => {
  cleanup();
  authSession.clear();
  localStorage.clear();
  vi.unstubAllGlobals();
});
async function setup(
  status: Snapshot['status'],
  options: { delayedUpload?: boolean; saveError?: boolean } = {},
) {
  let ack!: () => void;
  let uploadAck!: () => void;
  const writes: Snapshot[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, requestOptions?: RequestInit) => {
      let value: unknown;
      if (url === '/api/v1/auth/me')
        value = {
          user: {
            id: 'test-user',
            username: 'tester',
            displayName: 'tester',
            role: 'annotator',
            workspaceId: 'shared',
          },
          csrfToken: 'test-csrf',
        };
      else if (url === '/api/v1/videos' && requestOptions?.method === 'POST') {
        await new Promise<void>((resolve) => {
          uploadAck = resolve;
        });
        value = {
          id: 'new',
          name: '延迟上传视频',
          width: 100,
          height: 100,
          durationSeconds: 10,
          status: 'ready',
          createdAt: '2026-10-13T00:00:00Z',
        };
      } else if (url === '/api/v1/videos')
        value = {
          videos: [
            {
              id: 'v',
              name: '合成视频',
              width: 100,
              height: 100,
              durationSeconds: 10,
              status: 'ready',
              createdAt: '2026-10-13T00:00:00Z',
            },
          ],
        };
      else if (url === '/api/v1/jobs') value = { jobs: [] };
      else if (url === '/api/v1/videos/new/batches') value = { batches: [] };
      else if (url.endsWith('/batches'))
        value = {
          batches: [
            { id: 'b', videoId: 'v', jobId: 'j', createdAt: '2026-10-13T00:00:00Z', frameCount: 2 },
          ],
        };
      else if (url.includes('/frames?'))
        value = {
          frames: ['a', 'b'].map((id) => ({
            id,
            videoId: 'v',
            batchId: 'b',
            timestampMs: 0,
            width: 100,
            height: 100,
            imageUrl: `/media/${id}.jpg`,
            status: 'completed',
            revision: 0,
          })),
        };
      else if (url.includes('/progress'))
        value = { total: 2, notStarted: 0, inProgress: 0, completed: 2 };
      else if (url.includes('/annotations') || url.endsWith('/review')) {
        const frameId = url.split('/')[4];
        if (requestOptions?.method === 'PUT') {
          const body = JSON.parse(requestOptions.body as string);
          writes.push({ ...body, frameId });
          await new Promise<void>((resolve) => {
            ack = resolve;
          });
          if (options.saveError)
            return new Response(
              JSON.stringify({
                code: 'SAVE_FAILED',
                message: 'synthetic save failed',
                requestId: 'test',
              }),
              { status: 503, headers: { 'Content-Type': 'application/json' } },
            );
          value = { ...body, frameId, revision: 1 };
        } else
          value = {
            candidates: [],
            currentRun: null,
            issues: [],
            frameId,
            revision: 0,
            status,
            annotations: [{ id: 'head', bbox: [1, 1, 5, 5], source: 'manual' }],
          };
      }
      return new Response(JSON.stringify(value), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  render(<App />);
  await screen.findByTestId('video-select');
  if (options.delayedUpload) {
    fireEvent.click(screen.getByRole('tab', { name: '本地上传' }));
    fireEvent.change(screen.getByTestId('upload-file'), {
      target: { files: [new File(['synthetic'], 'new.mp4', { type: 'video/mp4' })] },
    });
    fireEvent.click(screen.getByTestId('upload-submit'));
  }
  fireEvent.change(screen.getByTestId('video-select'), { target: { value: 'v' } });
  await screen.findByTestId('annotation-item-head', {}, { timeout: 10000 });
  return { writes, ack: () => ack(), finishUpload: () => uploadAck() };
}
it('does not load private workspace queries for an anonymous session', async () => {
  const fetchMock = vi.fn<(url: string) => Promise<Response>>(
    async () => new Response(JSON.stringify({ code: 'UNAUTHENTICATED' }), { status: 401 }),
  );
  vi.stubGlobal('fetch', fetchMock);
  render(<App />);
  await screen.findByRole('button', { name: '登录工作区' });
  expect(fetchMock.mock.calls.every(([url]) => String(url).endsWith('/auth/me'))).toBe(true);
});

it('unmounts the old workspace on logout and does not display a late private video response', async () => {
  let finish!: (response: Response) => void;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/auth/me'))
        return new Response(
          JSON.stringify({
            user: {
              id: 'alice',
              username: 'alice',
              displayName: 'Alice',
              role: 'annotator',
              workspaceId: 'shared',
            },
            csrfToken: 'csrf',
          }),
          { headers: { 'Content-Type': 'application/json' } },
        );
      if (url.endsWith('/videos'))
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      if (url.endsWith('/auth/logout')) return new Response(null, { status: 204 });
      return new Response(JSON.stringify({ jobs: [], items: [], sources: [] }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  render(<App />);
  await screen.findByText('Alice');
  fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
  await screen.findByRole('button', { name: '登录工作区' });
  await act(async () =>
    finish(
      new Response(JSON.stringify({ videos: [{ id: 'private', name: 'Alice private video' }] }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    ),
  );
  expect(screen.queryByText('Alice private video')).toBeNull();
  expect(screen.queryByTestId('video-select')).toBeNull();
});
it('flushes editing a completed frame before a fast frame switch and blocks stale export', async () => {
  const state = await setup('completed');
  fireEvent.click(screen.getByTestId('annotation-item-head'));
  fireEvent.click(screen.getByText('删除选中框'));
  fireEvent.click(screen.getByTestId('frame-item-b'));
  expect(screen.getByTestId('frame-item-a').className).toContain('selected');
  expect((screen.getByTestId('export-zip') as HTMLButtonElement).disabled).toBe(true);
  await waitFor(() => expect(state.writes).toHaveLength(1));
  expect(state.writes[0].status).toBe('in_progress');
  state.ack();
  await waitFor(() => expect(screen.getByTestId('frame-item-b').className).toContain('selected'));
});
it('uses the latest active editor when an upload begun without a frame completes later', async () => {
  const state = await setup('in_progress', { delayedUpload: true });
  fireEvent.click(screen.getByTestId('complete-frame'));
  state.finishUpload();
  await waitFor(() => expect(state.writes).toHaveLength(1));
  expect((screen.getByTestId('video-select') as HTMLSelectElement).value).toBe('v');
  expect(state.writes[0].status).toBe('completed');
  state.ack();
  await waitFor(() =>
    expect((screen.getByTestId('video-select') as HTMLSelectElement).value).toBe('new'),
  );
  expect(localStorage.getItem(draftKey('a', draftScope))).toBeNull();
});
it('blocks delayed upload navigation when the latest selected editor has a conflict', async () => {
  localStorage.setItem(
    draftKey('a', draftScope),
    JSON.stringify({
      baseRevision: 3,
      editSequence: 1,
      snapshot: {
        frameId: 'a',
        revision: 3,
        status: 'in_progress',
        annotations: [{ id: 'head', bbox: [1, 1, 5, 5], source: 'manual' }],
      },
    }),
  );
  const state = await setup('in_progress', { delayedUpload: true });
  state.finishUpload();
  await screen.findByText('当前帧尚未保存。请先重试保存或处理草稿冲突，再切换。');
  expect((screen.getByTestId('video-select') as HTMLSelectElement).value).toBe('v');
  expect(state.writes).toHaveLength(0);
  expect(localStorage.getItem(draftKey('a', draftScope))).not.toBeNull();
});
it('blocks delayed upload navigation when saving the latest selected editor fails', async () => {
  const state = await setup('in_progress', { delayedUpload: true, saveError: true });
  fireEvent.click(screen.getByTestId('complete-frame'));
  state.finishUpload();
  await waitFor(() => expect(state.writes).toHaveLength(1));
  state.ack();
  await screen.findByText('当前帧尚未保存。请先重试保存或处理草稿冲突，再切换。');
  expect((screen.getByTestId('video-select') as HTMLSelectElement).value).toBe('v');
  expect(screen.getByTestId('save-state').textContent).toBe('保存失败');
  expect(localStorage.getItem(draftKey('a', draftScope))).not.toBeNull();
});
it('saves an explicit completion before immediately switching away', async () => {
  const state = await setup('in_progress');
  fireEvent.click(screen.getByTestId('complete-frame'));
  fireEvent.click(screen.getByTestId('frame-item-b'));
  expect(screen.getByTestId('frame-item-a').className).toContain('selected');
  await waitFor(() => expect(state.writes).toHaveLength(1));
  expect(state.writes[0].status).toBe('completed');
  state.ack();
  await waitFor(() => expect(screen.getByTestId('frame-item-b').className).toContain('selected'));
});
it.each(['video-select', 'batch-select'])(
  'settles completion before %s navigation',
  async (selector) => {
    const state = await setup('in_progress');
    fireEvent.click(screen.getByTestId('complete-frame'));
    fireEvent.change(screen.getByTestId(selector), { target: { value: '' } });
    expect((screen.getByTestId(selector) as HTMLSelectElement).value).toBe(
      selector === 'video-select' ? 'v' : 'b',
    );
    await waitFor(() => expect(state.writes).toHaveLength(1));
    state.ack();
    await waitFor(() => expect((screen.getByTestId(selector) as HTMLSelectElement).value).toBe(''));
  },
);
it('blocks export for a persisted draft in another frame of the selected batch', async () => {
  localStorage.setItem(
    draftKey('b', draftScope),
    JSON.stringify({
      baseRevision: 0,
      editSequence: 1,
      snapshot: { frameId: 'b', revision: 0, status: 'in_progress', annotations: [] },
    }),
  );
  await setup('completed');
  expect((screen.getByTestId('export-zip') as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/当前批次有 1 帧草稿/)).toBeTruthy();
});
