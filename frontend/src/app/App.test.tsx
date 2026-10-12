import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { App } from './App';
import type { Snapshot } from '../shared/api/types';
vi.mock('../features/annotation/Canvas', () => ({
  AnnotationCanvas: () => <div data-testid="annotation-canvas" />,
}));
afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.unstubAllGlobals();
});
async function setup(status: Snapshot['status']) {
  let ack!: () => void;
  const writes: Snapshot[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, options?: RequestInit) => {
      let value: unknown;
      if (url === '/api/v1/videos')
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
      else if (url.includes('/annotations')) {
        const frameId = url.split('/')[4];
        if (options?.method === 'PUT') {
          const body = JSON.parse(options.body as string);
          writes.push({ ...body, frameId });
          await new Promise<void>((resolve) => {
            ack = resolve;
          });
          value = { ...body, frameId, revision: 1 };
        } else
          value = {
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
  fireEvent.change(screen.getByTestId('video-select'), { target: { value: 'v' } });
  await screen.findByTestId('annotation-item-head');
  return { writes, ack: () => ack() };
}
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
    'classroom-annotator:draft:v1:b',
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
