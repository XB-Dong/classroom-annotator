import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { JobsPanel } from './JobsPanel';
import type { Frame, Job } from '../../shared/api/types';

afterEach(cleanup);

const aiJob: Job = {
  id: 'ai-job',
  kind: 'preannotation',
  status: 'failed',
  progress: 0.5,
  createdAt: '2026-10-13T19:00:00Z',
};
const frame: Frame = {
  id: 'frame-completed',
  videoId: 'video',
  batchId: 'batch',
  timestampMs: 1234,
  width: 100,
  height: 100,
  imageUrl: '/media/frame.jpg',
  status: 'not_started',
  revision: 0,
};

it('shows mixed AI image outcomes without treating generated candidates as reviewed labels', () => {
  const job: Job = {
    ...aiJob,
    error: 'BATCH_TIMEOUT',
    startedAt: '2026-10-13T19:00:10Z',
    finishedAt: '2026-10-13T19:00:30Z',
    partialResults: [
      { frameId: frame.id, runId: 'run-a', status: 'completed', applied: true },
      { frameId: 'frame-failed', runId: 'run-b', status: 'failed', applied: false, errorCode: 'AI_TIMEOUT' },
      { frameId: 'frame-cancelled', runId: 'run-c', status: 'cancelled', applied: false },
    ],
  };
  render(<JobsPanel jobs={[job]} frames={[frame]} cancel={vi.fn()} />);
  expect(screen.getByText('排队等待 10.0 秒')).toBeTruthy();
  expect(screen.getByText('执行耗时 20.0 秒')).toBeTruthy();
  expect(screen.getByText('图片结果（3）')).toBeTruthy();
  const results = screen.getAllByRole('listitem', { hidden: true });
  expect(results).toHaveLength(3);
  expect(within(results[0]).getByText('采样时间 1.234 秒')).toBeTruthy();
  expect(within(results[0]).getByText('当前候选已生成，仍需人工复核')).toBeTruthy();
  expect(within(results[1]).getByText('失败')).toBeTruthy();
  expect(within(results[1]).getByText('AI_TIMEOUT')).toBeTruthy();
  expect(within(results[1]).getByText('未应用结果')).toBeTruthy();
  expect(within(results[2]).getByText('已取消')).toBeTruthy();
  expect(within(results[2]).getByText('未应用结果')).toBeTruthy();
  expect(screen.queryByText(/已接受|全部成功|标注已完成/)).toBeNull();
  expect(screen.queryByRole('button', { name: '取消任务' })).toBeNull();
});

it('uses stable image identities for results outside the current frame list', () => {
  const job: Job = {
    ...aiJob,
    partialResults: [
      { frameId: 'missing-123456789', runId: 'run-a', status: 'stale', applied: false },
      { frameId: 'oldframe-987654321', runId: 'run-b', status: 'completed', applied: false },
    ],
  };
  render(<JobsPanel jobs={[job]} frames={[frame]} cancel={vi.fn()} />);
  expect(screen.getByText('图片结果 missing-')).toBeTruthy();
  expect(screen.getByText('图片结果 oldframe')).toBeTruthy();
  expect(screen.getByText('已过期，需要重新生成')).toBeTruthy();
  expect(screen.getByText('结果未激活')).toBeTruthy();
  expect(screen.queryByText(/采样时间/)).toBeNull();
  expect(screen.queryByText('当前候选已生成，仍需人工复核')).toBeNull();
});

it('keeps jobs with no optional metadata safe and preserves existing import and extraction cancellation', () => {
  const cancel = vi.fn();
  render(
    <JobsPanel
      jobs={[
        { ...aiJob, status: 'queued' },
        { ...aiJob, id: 'import', kind: 'import', status: 'running' },
        { ...aiJob, id: 'extract', kind: 'extraction', status: 'cancelling' },
      ]}
      cancel={cancel}
    />,
  );
  expect(screen.getByText('AI 预标注')).toBeTruthy();
  expect(screen.getByText('视频导入')).toBeTruthy();
  expect(screen.getByText('视频抽帧')).toBeTruthy();
  expect(screen.queryByText(/图片结果（|NaN|Invalid Date|执行耗时|排队等待 \d/)).toBeNull();
  fireEvent.click(screen.getAllByRole('button', { name: '取消任务' })[1]);
  expect(cancel).toHaveBeenCalledWith('import');
  expect((screen.getByRole('button', { name: '等待清理完成…' }) as HTMLButtonElement).disabled).toBe(true);
});

it('labels ongoing execution separately from recorded queue wait', () => {
  render(
    <JobsPanel
      jobs={[{ ...aiJob, status: 'running', startedAt: '2026-10-13T19:00:05Z' }]}
      cancel={vi.fn()}
    />,
  );
  expect(screen.getByText('排队等待 5.0 秒')).toBeTruthy();
  expect(screen.getByText('执行计时中')).toBeTruthy();
  expect(screen.queryByText(/执行耗时/)).toBeNull();
});

it('records a queue timeout without inventing execution time', () => {
  render(
    <JobsPanel
      jobs={[{ ...aiJob, finishedAt: '2026-10-13T19:02:00Z', error: 'QUEUE_TIMEOUT' }]}
      cancel={vi.fn()}
    />,
  );
  expect(screen.getByText('排队等待 120.0 秒')).toBeTruthy();
  expect(screen.queryByText(/执行耗时|执行计时中/)).toBeNull();
});

it('renders unknown statuses and error codes as plain text and rejects invalid durations', () => {
  const job: Job = {
    ...aiJob,
    status: 'future_status' as Job['status'],
    startedAt: 'invalid',
    finishedAt: '2026-10-13T18:00:00Z',
    partialResults: [
      { frameId: 'future-frame', runId: 'run-a', status: 'constructor' as NonNullable<Job['partialResults']>[number]['status'], applied: true, errorCode: '<b>FUTURE_ERROR</b>' },
    ],
  };
  render(<JobsPanel jobs={[job]} cancel={vi.fn()} />);
  expect(screen.getByText('future_status')).toBeTruthy();
  expect(screen.getByText('constructor')).toBeTruthy();
  expect(screen.getByText('<b>FUTURE_ERROR</b>').querySelector('b')).toBeNull();
  expect(screen.queryByText('当前候选已生成，仍需人工复核')).toBeNull();
  expect(screen.queryByText(/NaN|Invalid Date|排队等待 \d|执行耗时/)).toBeNull();
});
