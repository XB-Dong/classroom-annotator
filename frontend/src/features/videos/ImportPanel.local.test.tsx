import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ImportPanel } from './ImportPanel';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it('lists configured local sources and imports an opaque identifier with an idempotency key', async () => {
  const onJob = vi.fn();
  const calls = vi.fn(
    async (path: string, options?: RequestInit) =>
      new Response(
        JSON.stringify(
          path.endsWith('/sources/local')
            ? {
                items: [
                  {
                    sourceId: 'opaque /source',
                    title: '本机授权样本',
                    sizeBytes: 2048,
                    sha256: 'a'.repeat(64),
                    width: 320,
                    height: 240,
                    durationSeconds: 5,
                  },
                ],
              }
            : options?.method === 'POST'
              ? { id: 'j', kind: 'import', status: 'queued' }
              : { items: [] },
        ),
        { headers: { 'Content-Type': 'application/json' } },
      ),
  );
  vi.stubGlobal('fetch', calls);
  render(<ImportPanel onVideo={() => {}} onJob={onJob} />);
  fireEvent.click(screen.getByRole('tab', { name: '本机素材' }));
  await screen.findByText('本机授权样本');
  fireEvent.click(screen.getByText('导入本机素材'));
  await waitFor(() => expect(onJob).toHaveBeenCalledOnce());
  const call = calls.mock.calls.find(([path]) => path.endsWith('/imports'))!;
  expect(call[0]).toBe('/api/v1/sources/local/opaque%20%2Fsource/imports');
  expect(new Headers(call[1]?.headers).get('Idempotency-Key')).toBeTruthy();
  expect(calls.mock.calls.some(([path]) => path.includes('/mac'))).toBe(false);
});
it('shows a 503 catalog error and retries without inventing configured sources', async () => {
  let ready = false;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string) =>
      path.endsWith('/sources/local') && !ready
        ? new Response(
            JSON.stringify({ code: 'LOCAL_SOURCE_NOT_CONFIGURED', message: '本机目录尚未配置' }),
            { status: 503 },
          )
        : new Response(JSON.stringify({ items: [] }), {
            headers: { 'Content-Type': 'application/json' },
          }),
    ),
  );
  render(<ImportPanel onVideo={() => {}} onJob={() => {}} />);
  fireEvent.click(screen.getByRole('tab', { name: '本机素材' }));
  await screen.findByText('本机目录尚未配置');
  expect(screen.queryByText('导入本机素材')).toBeNull();
  ready = true;
  fireEvent.click(screen.getByText('刷新本机目录'));
  await screen.findByText('目录中暂无授权视频。');
});
