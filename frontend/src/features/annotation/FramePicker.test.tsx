import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { FramePicker } from './FramePicker';

afterEach(cleanup);
it('shows an unavailable thumbnail without covering the frame selection action', () => {
  const select = vi.fn();
  render(<FramePicker frames={[{ id: 'f', videoId: 'v', batchId: 'b', timestampMs: 0, width: 100, height: 100, imageUrl: '/media/f.jpg', status: 'not_started', revision: 0 }]} frameId="" busy={false} select={select} />);
  fireEvent.error(screen.getByAltText('第 1 帧'));
  expect(screen.getByText('缩略图暂不可用')).toBeTruthy();
  fireEvent.click(screen.getByTestId('frame-item-f'));
  expect(select).toHaveBeenCalledWith('f');
});
