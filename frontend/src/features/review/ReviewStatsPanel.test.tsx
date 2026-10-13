import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ReviewStatsPanel } from './ReviewStatsPanel';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
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
