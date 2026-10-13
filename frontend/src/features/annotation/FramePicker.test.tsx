import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { FramePicker } from './FramePicker';

afterEach(cleanup);
it('shows an unavailable thumbnail without covering the frame selection action', () => {
  const select = vi.fn();
  render(
    <FramePicker
      frames={[
        {
          id: 'f',
          videoId: 'v',
          batchId: 'b',
          timestampMs: 0,
          width: 100,
          height: 100,
          imageUrl: '/media/f.jpg',
          status: 'not_started',
          revision: 0,
        },
      ]}
      frameId=""
      busy={false}
      select={select}
    />,
  );
  fireEvent.error(screen.getByAltText('第 1 帧'));
  expect(screen.getByText('缩略图暂不可用')).toBeTruthy();
  fireEvent.click(screen.getByTestId('frame-item-f'));
  expect(select).toHaveBeenCalledWith('f');
});

it('marks only unresolved risk images and filters without navigating away from the current frame', () => {
  const select = vi.fn();
  const frames = ['safe', 'risky', 'resolved'].map((id, index) => ({
    id,
    videoId: 'v',
    batchId: 'b',
    timestampMs: index * 1000,
    width: 100,
    height: 100,
    imageUrl: `/media/${id}.jpg`,
    status: 'not_started' as const,
    revision: 0,
  }));
  render(
    <FramePicker
      {...{
        frames,
        frameId: 'safe',
        busy: false,
        select,
        unresolvedRiskFrameIds: ['risky', 'outside-batch'],
      }}
    />,
  );
  expect(screen.getByText('需复核图片 1 张')).toBeTruthy();
  expect(screen.getByTestId('frame-item-risky').textContent).toContain('需复核');
  const filter = screen.getByRole('checkbox', { name: '仅看需复核图片' });
  fireEvent.click(filter);
  expect(screen.queryByTestId('frame-item-safe')).toBeNull();
  expect(screen.queryByTestId('frame-item-resolved')).toBeNull();
  expect(screen.getByAltText('第 2 帧')).toBeTruthy();
  expect(screen.getByText('当前帧不在筛选结果中，画布继续保留。')).toBeTruthy();
  expect(select).not.toHaveBeenCalled();
  fireEvent.click(screen.getByTestId('frame-item-risky'));
  expect(select).toHaveBeenCalledWith('risky');
});

it('reports unknown risk data and an empty known risk filter honestly', () => {
  const props = {
    frames: [
      {
        id: 'f',
        videoId: 'v',
        batchId: 'b',
        timestampMs: 0,
        width: 100,
        height: 100,
        imageUrl: '/media/f.jpg',
        status: 'not_started' as const,
        revision: 0,
      },
    ],
    frameId: 'f',
    busy: false,
    select: vi.fn(),
  };
  const { rerender } = render(<FramePicker {...props} />);
  expect(screen.getByText('图片风险数据暂不可用')).toBeTruthy();
  expect(
    (screen.getByRole('checkbox', { name: '仅看需复核图片' }) as HTMLInputElement).disabled,
  ).toBe(true);
  rerender(<FramePicker {...{ ...props, unresolvedRiskFrameIds: [] }} />);
  expect(screen.getByText('需复核图片 0 张')).toBeTruthy();
  fireEvent.click(screen.getByRole('checkbox', { name: '仅看需复核图片' }));
  expect(screen.getByText('当前批次没有未解决风险图片。')).toBeTruthy();
  expect(props.select).not.toHaveBeenCalled();
  rerender(<FramePicker {...props} />);
  expect(screen.getByTestId('frame-item-f')).toBeTruthy();
});
