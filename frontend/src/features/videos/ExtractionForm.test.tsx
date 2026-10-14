import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ExtractionForm } from './ExtractionForm';
import type { Video } from '../../shared/api/types';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const video: Video = {
  id: 'v',
  name: '合成视频',
  width: 100,
  height: 100,
  durationSeconds: 12,
  status: 'ready',
  createdAt: '',
};

it('keeps interval and submit visible while optional range values survive disclosure toggles', async () => {
  const calls = vi.fn(
    async () =>
      new Response(JSON.stringify({ id: 'job' }), {
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', calls);
  const onJob = vi.fn();
  render(<ExtractionForm video={video} onJob={onJob} />);
  const summary = screen.getByText('指定抽帧范围（可选）');
  const disclosure = summary.closest('details')!;
  expect(disclosure.open).toBe(false);
  expect(screen.getByTestId('interval-seconds').closest('details')).toBeNull();
  expect(screen.getByTestId('extract-submit').closest('details')).toBeNull();
  fireEvent.click(summary);
  fireEvent.change(screen.getByTestId('start-seconds'), { target: { value: '2' } });
  fireEvent.change(screen.getByTestId('end-seconds'), { target: { value: '8' } });
  fireEvent.click(summary);
  fireEvent.click(screen.getByTestId('extract-submit'));
  await waitFor(() => expect(onJob).toHaveBeenCalledOnce());
  expect(
    JSON.parse((calls.mock.calls[0] as unknown as [string, RequestInit])[1].body as string),
  ).toEqual({ intervalSeconds: 5, startSeconds: 2, endSeconds: 8 });
});

it('keeps range validation errors visible with the range disclosure collapsed', async () => {
  render(<ExtractionForm video={video} onJob={() => {}} />);
  fireEvent.click(screen.getByText('指定抽帧范围（可选）'));
  fireEvent.change(screen.getByTestId('start-seconds'), { target: { value: '20' } });
  fireEvent.click(screen.getByText('指定抽帧范围（可选）'));
  fireEvent.click(screen.getByTestId('extract-submit'));
  expect(screen.getByRole('alert').closest('details')).toBeNull();
});
