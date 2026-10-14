import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ReviewStatsPanel } from './ReviewStatsPanel';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
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

it('publishes risk IDs, clears them while loading or unavailable, and ignores old-scope replies', async () => {
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
