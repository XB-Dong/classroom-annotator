import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Editor } from './Editor';
import type { Frame, Snapshot } from '../../shared/api/types';
vi.mock('./Canvas', () => ({ AnnotationCanvas: () => <div data-testid="annotation-canvas" /> }));
const frame: Frame = {
  id: 'f',
  videoId: 'v',
  batchId: 'b',
  timestampMs: 250,
  width: 100,
  height: 100,
  imageUrl: '/media/frames/f.jpg',
  status: 'not_started',
  revision: 0,
};
const initial: Snapshot = {
  frameId: 'f',
  revision: 0,
  status: 'not_started',
  annotations: [{ id: 'a', bbox: [1, 1, 5, 5], source: 'manual' }],
};
beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async (_url: string, options?: RequestInit) =>
        new Response(
          JSON.stringify(
            options?.method === 'PUT'
              ? { ...JSON.parse(options.body as string), frameId: 'f', revision: 1 }
              : initial,
          ),
          { headers: { 'Content-Type': 'application/json' } },
        ),
    ),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
describe('annotation editor interactions', () => {
  it('deletes the selected stable annotation and undo restores it', async () => {
    render(<Editor frame={frame} onSaved={() => {}} />);
    await screen.findByText('学生人头 1');
    fireEvent.click(screen.getByTestId('annotation-item-a'));
    fireEvent.keyDown(window, { key: 'Delete' });
    expect(screen.queryByText('学生人头 1')).toBeNull();
    fireEvent.keyDown(window, { key: 'z', ctrlKey: true });
    expect(screen.getByText('学生人头 1')).toBeTruthy();
  });
  it('does not delete while an input is focused', async () => {
    render(
      <>
        <input aria-label="test-input" />
        <Editor frame={frame} onSaved={() => {}} />
      </>,
    );
    await screen.findByText('学生人头 1');
    fireEvent.click(screen.getByTestId('annotation-item-a'));
    fireEvent.keyDown(screen.getByLabelText('test-input'), { key: 'Delete' });
    expect(screen.getByText('学生人头 1')).toBeTruthy();
  });
  it('explicitly completes a zero-head frame and saves the empty annotations', async () => {
    render(<Editor frame={frame} onSaved={() => {}} />);
    await screen.findByText('学生人头 1');
    fireEvent.click(screen.getByTestId('annotation-item-a'));
    fireEvent.click(screen.getByText('删除选中框'));
    fireEvent.click(screen.getByTestId('complete-frame'));
    await waitFor(() => expect(screen.getByText('已保存')).toBeTruthy(), { timeout: 1500 });
    const mock = vi.mocked(fetch);
    const write = mock.mock.calls.find(([, o]) => o?.method === 'PUT');
    expect(JSON.parse(write?.[1]?.body as string)).toMatchObject({
      expectedRevision: 0,
      status: 'completed',
      annotations: [],
    });
  });
  it('editing a completed frame reopens it until explicitly completed again', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async (_url: string, options?: RequestInit) =>
          new Response(
            JSON.stringify(
              options?.method === 'PUT'
                ? { ...JSON.parse(options.body as string), frameId: 'f', revision: 2 }
                : { ...initial, status: 'completed', revision: 1 },
            ),
            { headers: { 'Content-Type': 'application/json' } },
          ),
      ),
    );
    render(<Editor frame={frame} onSaved={() => {}} />);
    await screen.findByText('学生人头 1');
    fireEvent.click(screen.getByTestId('annotation-item-a'));
    fireEvent.click(screen.getByText('删除选中框'));
    expect(screen.getByTestId('complete-frame').textContent).toBe('标记完成（零人头）');
  });
});
