import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ExportButton } from './ExportButton';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const report = {
  rulesVersion: 'P2-QC-1',
  checkedFrames: 1,
  checkedAnnotations: 2,
  totalFindings: 1,
  truncated: false,
  frameRevisions: [{ frameId: 'f', revision: 2 }],
  findings: [{ code: 'DUPLICATE_BOX', severity: 'warning', frameId: 'f', annotationIds: ['a', 'b'], message: '两个框高度重叠，请人工确认。' }],
};

it('reveals optional quality checking without making a request or downloading', () => {
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  render(<ExportButton videoId="v" batchId="b" completed={1} />);
  const summary = screen.getByText('导出前质量检查（可选）');
  expect(summary.closest('details')?.open).toBe(false);
  fireEvent.click(summary);
  expect(summary.closest('details')?.open).toBe(true);
  expect(fetch).not.toHaveBeenCalled();
  expect(screen.getByTestId('export-zip').hasAttribute('disabled')).toBe(false);
});

it('locates a suspicious frame and clearly identifies advisory results', async () => {
  const locate = vi.fn();
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(report), { headers: { 'Content-Type': 'application/json' } })));
  render(<ExportButton videoId="v" batchId="b" completed={1} onLocateFrame={locate} />);
  fireEvent.click(screen.getByText('导出前质量检查（可选）'));
  fireEvent.click(screen.getByRole('button', { name: '开始质量检查' }));
  await screen.findByText('两个框高度重叠，请人工确认。');
  fireEvent.click(screen.getByRole('button', { name: '定位图片 f' }));
  expect(locate).toHaveBeenCalledWith('f');
  expect(screen.getByText(/启发式提醒/)).toBeTruthy();
});

it('reports incomplete coverage instead of declaring a truncated result clean', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ ...report, findings: [], totalFindings: 0, truncated: true }), { headers: { 'Content-Type': 'application/json' } })));
  render(<ExportButton videoId="v" batchId="b" completed={1} />);
  fireEvent.click(screen.getByText('导出前质量检查（可选）'));
  fireEvent.click(screen.getByRole('button', { name: '开始质量检查' }));
  await screen.findByText(/检查未完整覆盖/);
  expect(screen.queryByText(/未发现上述可疑情况/)).toBeNull();
});

it('keeps the original export available after advisory checking fails', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ code: 'EXPORT_FAILED', message: 'synthetic precheck outage' }), { status: 503, headers: { 'Content-Type': 'application/json' } })));
  render(<ExportButton videoId="v" batchId="b" completed={1} />);
  fireEvent.click(screen.getByText('导出前质量检查（可选）'));
  fireEvent.click(screen.getByRole('button', { name: '开始质量检查' }));
  await screen.findByText(/synthetic precheck outage/);
  await waitFor(() => expect(screen.getByTestId('export-zip').hasAttribute('disabled')).toBe(false));
  expect(screen.getByText(/仍可使用上方导出/)).toBeTruthy();
});

it('ignores a late check after switching batches and invalidates results on a saved revision change', async () => {
  let release!: () => void;
  vi.stubGlobal('fetch', vi.fn(async () => {
    await new Promise<void>((resolve) => { release = resolve; });
    return new Response(JSON.stringify(report), { headers: { 'Content-Type': 'application/json' } });
  }));
  const view = render(<ExportButton videoId="v" batchId="b" completed={1} revisionKey="f:2" />);
  fireEvent.click(screen.getByText('导出前质量检查（可选）'));
  fireEvent.click(screen.getByRole('button', { name: '开始质量检查' }));
  view.rerender(<ExportButton videoId="v" batchId="other" completed={1} revisionKey="g:1" />);
  await act(async () => release());
  expect(screen.queryByText('两个框高度重叠，请人工确认。')).toBeNull();
});
