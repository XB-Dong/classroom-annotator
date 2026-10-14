import { useLayoutEffect, useRef, useState } from 'react';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ReviewStatsPanel } from './ReviewStatsPanel';
import { FramePicker } from '../annotation/FramePicker';
import type { Frame } from '../../shared/api/types';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const response = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const validStats = {
  candidates: 2,
  accepted: 1,
  adjusted: 0,
  rejected: 0,
  pending: 1,
  unresolvedRiskFrameIds: ['risky'],
  rates: { reviewed: 0.5 },
};

it('keeps same-batch statistics, expanded content and the active risk filter while refreshing', async () => {
  let finish!: (value: Response) => void;
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(response(validStats))
    .mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
    );
  vi.stubGlobal('fetch', fetch);
  const frames: Frame[] = ['risky', 'clean'].map((id) => ({
    id,
    videoId: 'v',
    batchId: 'b',
    timestampMs: 0,
    width: 100,
    height: 100,
    imageUrl: '/media/synthetic.jpg',
    status: 'in_progress',
    revision: 0,
  }));
  function Harness({ refresh }: { refresh: number }) {
    const [riskIds, setRiskIds] = useState<string[] | undefined>();
    return (
      <>
        <ReviewStatsPanel videoId="v" batchId="b" refresh={refresh} onRiskFrames={setRiskIds} />
        <FramePicker
          frames={frames}
          frameId="risky"
          busy={false}
          select={() => {}}
          unresolvedRiskFrameIds={riskIds}
        />
      </>
    );
  }
  const view = render(<Harness refresh={0} />);
  const summary = await screen.findByText('本批次复核统计 · 已决策 1/2');
  fireEvent.click(summary);
  const progress = screen.getByRole('progressbar');
  fireEvent.click(screen.getByRole('checkbox', { name: '仅看需复核图片' }));
  expect(screen.queryByTestId('frame-item-clean')).toBeNull();
  const riskFrame = screen.getByTestId('frame-item-risky');
  view.rerender(<Harness refresh={1} />);
  expect(summary.textContent).toBe('本批次复核统计 · 已决策 1/2');
  expect(screen.getByRole('progressbar')).toBe(progress);
  expect(screen.getByTestId('frame-item-risky')).toBe(riskFrame);
  expect(screen.queryByTestId('frame-item-clean')).toBeNull();
  expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);
  expect(summary.closest('details')?.open).toBe(true);
  await act(async () => finish(response({ ...validStats, accepted: 2, pending: 0 })));
  expect(summary.textContent).toBe('本批次复核统计 · 已决策 2/2');
  expect(screen.getByRole('progressbar')).toBe(progress);
  expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);
});

it('clears invalid statistics on failure and retains its error through retries until recovery', async () => {
  let retry!: (value: Response) => void;
  let recover!: (value: Response) => void;
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(response(validStats))
    .mockResolvedValueOnce(response({ message: 'synthetic outage' }, 503))
    .mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          retry = resolve;
        }),
    )
    .mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          recover = resolve;
        }),
    );
  vi.stubGlobal('fetch', fetch);
  const onRiskFrames = vi.fn();
  const props = { videoId: 'v', batchId: 'b', onRiskFrames };
  const view = render(<ReviewStatsPanel {...props} refresh={0} />);
  await screen.findByText('本批次复核统计 · 已决策 1/2');
  view.rerender(<ReviewStatsPanel {...props} refresh={1} />);
  const error = await screen.findByRole('alert');
  expect(error.textContent).toContain('synthetic outage');
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  expect(screen.queryByText(/已决策/)).toBeNull();
  view.rerender(<ReviewStatsPanel {...props} refresh={2} />);
  expect(screen.getByRole('alert')).toBe(error);
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  await act(async () => retry(response({ message: 'synthetic outage' }, 503)));
  expect(screen.getByRole('alert')).toBe(error);
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  view.rerender(<ReviewStatsPanel {...props} refresh={3} />);
  expect(screen.getByRole('alert')).toBe(error);
  await act(async () => recover(response(validStats)));
  expect(screen.queryByRole('alert')).toBeNull();
  expect(onRiskFrames).toHaveBeenLastCalledWith(['risky']);
  expect(screen.getByText('本批次复核统计 · 已决策 1/2')).toBeTruthy();
});

it('removes the previous video or batch before layout effects and clears published risk IDs', async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(response(validStats))
    .mockResolvedValueOnce(response({ ...validStats, accepted: 2 }))
    .mockImplementation(() => new Promise<Response>(() => {}));
  vi.stubGlobal('fetch', fetch);
  const onRiskFrames = vi.fn();
  const painted: string[] = [];
  function Harness({ videoId, batchId }: { videoId: string; batchId: string }) {
    const node = useRef<HTMLDivElement>(null);
    useLayoutEffect(() => {
      painted.push(node.current!.textContent!);
    }, [videoId, batchId]);
    return (
      <div ref={node}>
        <ReviewStatsPanel
          videoId={videoId}
          batchId={batchId}
          refresh={0}
          onRiskFrames={onRiskFrames}
        />
      </div>
    );
  }
  const view = render(<Harness videoId="v" batchId="b" />);
  await screen.findByText('本批次复核统计 · 已决策 1/2');
  view.rerender(<Harness videoId="v" batchId="next" />);
  expect(painted.at(-1)).not.toContain('已决策');
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  await screen.findByText('本批次复核统计 · 已决策 2/2');
  view.rerender(<Harness videoId="next-video" batchId="next" />);
  expect(painted.at(-1)).not.toContain('已决策');
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
});

it('ignores superseded same-batch responses and responses received after unmount', async () => {
  let old!: (value: Response) => void;
  let current!: (value: Response) => void;
  let afterUnmount!: (value: Response) => void;
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValueOnce(response(validStats))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            old = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            current = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            afterUnmount = resolve;
          }),
      ),
  );
  const onRiskFrames = vi.fn();
  const props = { videoId: 'v', batchId: 'b', onRiskFrames };
  const view = render(<ReviewStatsPanel {...props} refresh={0} />);
  await screen.findByText('本批次复核统计 · 已决策 1/2');
  view.rerender(<ReviewStatsPanel {...props} refresh={1} />);
  view.rerender(<ReviewStatsPanel {...props} refresh={2} />);
  await act(async () =>
    current(response({ ...validStats, accepted: 2, unresolvedRiskFrameIds: ['current'] })),
  );
  await act(async () =>
    old(response({ ...validStats, accepted: 0, unresolvedRiskFrameIds: ['old'] })),
  );
  expect(screen.getByText('本批次复核统计 · 已决策 2/2')).toBeTruthy();
  expect(onRiskFrames).toHaveBeenLastCalledWith(['current']);
  view.rerender(<ReviewStatsPanel {...props} refresh={3} />);
  const calls = onRiskFrames.mock.calls.length;
  view.unmount();
  await act(async () => afterUnmount(response(validStats)));
  expect(onRiskFrames).toHaveBeenCalledTimes(calls);
});
it('shows a statistics failure outside the collapsed statistics disclosure', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      throw new Error('synthetic outage');
    }),
  );
  render(<ReviewStatsPanel videoId="v" batchId="b" refresh={0} />);
  const error = await screen.findByRole('alert');
  expect(error.closest('details')).toBeNull();
  expect(screen.getByText(/本批次复核统计/).closest('details')?.open).toBe(false);
});
it('renders fraction rates as percentages with explicit denominators and null as no samples', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            validImages: 3,
            suspiciousImages: 1,
            failedImages: 1,
            candidates: 10,
            pending: 2,
            accepted: 4,
            adjusted: 2,
            rejected: 2,
            riskyCandidates: 3,
            unresolvedRiskCandidates: 1,
            unresolvedRiskImages: 1,
            manualAdded: 2,
            rates: {
              acceptedUnchanged: 0.5,
              adjusted: 0.25,
              rejected: 0.25,
              suspiciousCandidates: 0.3,
              suspiciousImages: 1 / 3,
              reviewed: 0.8,
            },
          }),
          { headers: { 'Content-Type': 'application/json' } },
        ),
    ),
  );
  render(<ReviewStatsPanel videoId="v" batchId="b" refresh={0} />);
  await screen.findByText(/无需调整通过率 50.0%/);
  expect(screen.getByText(/检测准确率未评估/)).toBeTruthy();
  expect(screen.getByText(/图片可疑率 33.3%/)).toBeTruthy();
});
it('does not claim 100 percent when no candidates have been reviewed', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            candidates: 0,
            accepted: 0,
            adjusted: 0,
            rejected: 0,
            rates: {
              acceptedUnchanged: null,
              adjusted: null,
              rejected: null,
              suspiciousCandidates: null,
              suspiciousImages: null,
              reviewed: null,
            },
          }),
          { headers: { 'Content-Type': 'application/json' } },
        ),
    ),
  );
  render(<ReviewStatsPanel videoId="v" batchId="b" refresh={0} />);
  await screen.findByText(/无需调整通过率 无样本/);
  expect(screen.queryByText(/100.0%/)).toBeNull();
});

it('publishes risk IDs, clears them for a new scope or failure, and ignores old-scope replies', async () => {
  let finishOld!: (value: Response) => void;
  const onRiskFrames = vi.fn();
  const response = (value: unknown) =>
    new Response(JSON.stringify(value), { headers: { 'Content-Type': 'application/json' } });
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.includes('batchId=old'))
        return new Promise<Response>((resolve) => {
          finishOld = resolve;
        });
      if (url.includes('batchId=error')) throw new Error('synthetic outage');
      if (url.includes('batchId=missing'))
        return response({ candidates: 0, accepted: 0, adjusted: 0, rejected: 0 });
      return response({
        candidates: 0,
        accepted: 0,
        adjusted: 0,
        rejected: 0,
        unresolvedRiskFrameIds: ['current'],
      });
    }),
  );
  const { rerender } = render(
    <ReviewStatsPanel {...{ videoId: 'v', batchId: 'old', refresh: 0, onRiskFrames }} />,
  );
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  rerender(<ReviewStatsPanel {...{ videoId: 'v', batchId: 'new', refresh: 0, onRiskFrames }} />);
  await waitFor(() => expect(onRiskFrames).toHaveBeenLastCalledWith(['current']));
  await act(async () => {
    finishOld(
      response({
        candidates: 0,
        accepted: 0,
        adjusted: 0,
        rejected: 0,
        unresolvedRiskFrameIds: ['old'],
      }),
    );
  });
  expect(onRiskFrames).toHaveBeenLastCalledWith(['current']);
  rerender(<ReviewStatsPanel {...{ videoId: 'v', batchId: 'error', refresh: 0, onRiskFrames }} />);
  await screen.findByText(/无法连接服务/);
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
  rerender(
    <ReviewStatsPanel {...{ videoId: 'v', batchId: 'missing', refresh: 0, onRiskFrames }} />,
  );
  await waitFor(() => expect(screen.queryByText(/无法连接服务/)).toBeNull());
  expect(onRiskFrames).toHaveBeenLastCalledWith(undefined);
});
