import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { BatchReviewPanel } from './BatchReviewPanel';
import type { Frame } from '../../shared/api/types';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const frames: Frame[] = Array.from({ length: 23 }, (_, i) => ({
  id: `f${i}`,
  videoId: 'v',
  batchId: 'b',
  timestampMs: i * 1000,
  width: 100,
  height: 100,
  imageUrl: '',
  status: i === 22 ? 'completed' : 'in_progress',
  revision: 0,
}));
it('submits only explicitly selected unfinished frames and limits each batch to 20', async () => {
  const beforeStart = vi.fn(async () => true);
  const onJob = vi.fn();
  const calls = vi.fn<(path: string, options?: RequestInit) => Promise<Response>>(
    async () =>
      new Response(JSON.stringify({ id: 'j' }), {
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', calls);
  render(
    <BatchReviewPanel
      batchId="b"
      frames={frames}
      blocked={false}
      beforeStart={beforeStart}
      onJob={onJob}
    />,
  );
  fireEvent.click(screen.getByText('批量 AI 预标注'));
  fireEvent.click(screen.getByText('选择前 20 张未完成图片'));
  fireEvent.click(screen.getByText('开始批量 AI（20 张）'));
  await waitFor(() => expect(onJob).toHaveBeenCalledOnce());
  expect(beforeStart).toHaveBeenCalledOnce();
  expect(JSON.parse(calls.mock.calls[0][1]?.body as string).frameIds).toEqual(
    frames.slice(0, 20).map((f) => f.id),
  );
  expect(new Headers(calls.mock.calls[0][1]?.headers).get('Idempotency-Key')).toBeTruthy();
});
it('does not queue batch inference when the active manual save cannot settle', async () => {
  const calls = vi.fn();
  vi.stubGlobal('fetch', calls);
  render(
    <BatchReviewPanel
      batchId="b"
      frames={frames}
      blocked={false}
      beforeStart={async () => false}
      onJob={() => {}}
    />,
  );
  fireEvent.click(screen.getByText('批量 AI 预标注'));
  fireEvent.click(screen.getByText('选择前 20 张未完成图片'));
  fireEvent.click(screen.getByText('开始批量 AI（20 张）'));
  await screen.findByText(/请先保存或处理当前帧草稿/);
  expect(calls).not.toHaveBeenCalled();
});
