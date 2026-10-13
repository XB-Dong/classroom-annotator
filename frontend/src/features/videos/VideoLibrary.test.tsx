import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { VideoLibrary } from './VideoLibrary';
import type { Video } from '../../shared/api/types';

afterEach(cleanup);
it('keeps legacy cloud media behind an explicit history toggle while new sources stay selectable', () => {
  const videos: Video[] = [
    { id: 'legacy', name: '旧素材', width: 100, height: 100, durationSeconds: 10, status: 'ready', createdAt: '', sourceKind: 'legacy', storageProvider: 'local' },
    { id: 'public', name: '公开示例', width: 100, height: 100, durationSeconds: 10, status: 'ready', createdAt: '', sourceKind: 'public_sample', storageProvider: 'local' },
    { id: 'mac', name: 'Mac 视频', width: 100, height: 100, durationSeconds: 10, status: 'ready', createdAt: '', sourceKind: 'mac', storageProvider: 'mac' },
  ];
  const select = vi.fn();
  render(<VideoLibrary videos={videos} videoId="" loading={false} refresh={() => {}} select={select} />);
  expect(screen.queryByRole('option', { name: /旧素材/ })).toBeNull();
  expect(screen.getByRole('option', { name: /公开示例/ })).toBeTruthy();
  expect(screen.getByRole('option', { name: /Mac 视频/ })).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '显示历史素材' }));
  expect(screen.getByRole('option', { name: /旧素材/ })).toBeTruthy();
  fireEvent.change(screen.getByTestId('video-select'), { target: { value: 'legacy' } });
  expect(select).toHaveBeenCalledWith('legacy');
});
