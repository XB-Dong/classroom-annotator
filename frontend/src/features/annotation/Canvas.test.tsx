import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { useEffect, useImperativeHandle, useRef, type Ref } from 'react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import Konva from 'konva';
import { AnnotationCanvas, HeadBox } from './Canvas';
import { applyNodeBox } from './nodeGeometry';
import { authSession } from '../auth/session';
const nodes = vi.hoisted(() => ({
  rect: undefined as Konva.Rect | undefined,
  transformer: undefined as Konva.Transformer | undefined,
}));
beforeEach(() => {
  vi.spyOn(Konva.Util, 'isCanvasFarblingActive').mockReturnValue(false);
});
vi.mock('react-konva', () => ({
  Rect: ({
    ref,
    x,
    y,
    width,
    height,
    onTransformEnd,
  }: {
    ref: Ref<Konva.Rect>;
    x: number;
    y: number;
    width: number;
    height: number;
    onTransformEnd?: () => void;
  }) => {
    const node = useRef(new Konva.Rect({ x, y, width, height }));
    nodes.rect = node.current;
    useImperativeHandle(ref, () => node.current);
    useEffect(() => {
      const current = node.current;
      if (onTransformEnd) current.on('transformend.test', onTransformEnd);
      return () => {
        current.off('transformend.test');
      };
    }, [onTransformEnd]);
    return null;
  },
  Transformer: ({ ref }: { ref: Ref<Konva.Transformer> }) => {
    const node = useRef(new Konva.Transformer());
    nodes.transformer = node.current;
    useImperativeHandle(ref, () => node.current);
    return null;
  },
  Stage: () => null,
  Layer: () => null,
  Image: () => null,
}));
afterEach(() => {
  cleanup();
  authSession.clear();
  nodes.rect?.destroy();
  nodes.transformer?.destroy();
  nodes.rect = undefined;
  nodes.transformer = undefined;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
it('ignores a late private image callback after the account generation changes', () => {
  const images: { onload?: () => void; onerror?: () => void; src?: string }[] = [];
  vi.stubGlobal(
    'Image',
    class {
      onload?: () => void;
      onerror?: () => void;
      src?: string;
      constructor() {
        images.push(this);
      }
    },
  );
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
  const frame = {
    id: 'frame',
    videoId: 'v',
    batchId: 'b',
    timestampMs: 0,
    width: 100,
    height: 100,
    imageUrl: '/media/private.jpg',
    status: 'not_started' as const,
    revision: 0,
  };
  render(
    <AnnotationCanvas
      frame={frame}
      annotations={[]}
      mode="select"
      disabled={false}
      select={() => {}}
      change={() => {}}
    />,
  );
  act(() => authSession.clear());
  act(() => images[0].onload?.());
  expect(screen.queryByTestId('annotation-image-bounds')).toBeNull();
});
it('shows a retry action when a frame image fails, then renders the real image after retry', () => {
  const images: { onload?: () => void; onerror?: () => void; src?: string }[] = [];
  class FakeImage {
    onload?: () => void;
    onerror?: () => void;
    src?: string;
    constructor() {
      images.push(this);
    }
  }
  vi.stubGlobal('Image', FakeImage);
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
  const frame = {
    id: 'frame',
    videoId: 'v',
    batchId: 'b',
    timestampMs: 0,
    width: 100,
    height: 100,
    imageUrl: '/media/frame.jpg',
    status: 'not_started' as const,
    revision: 0,
  };
  const props = {
    frame,
    annotations: [],
    mode: 'select' as const,
    disabled: false,
    select: () => {},
    change: () => {},
  };
  render(<AnnotationCanvas {...props} />);
  act(() => {
    images[0].onerror?.();
  });
  expect(screen.getByRole('alert').textContent).toContain('原图暂不可用');
  expect(screen.queryByRole('img', { name: '当前标注原图' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '重试加载原图' }));
  expect(images).toHaveLength(2);
  act(() => {
    images[1].onload?.();
  });
  expect(screen.getByTestId('annotation-image-bounds')).toBeTruthy();
  expect(screen.getByRole('img', { name: '当前标注原图' })).toBe(
    screen.getByTestId('annotation-image-bounds'),
  );
});
it('keeps an already loaded image usable if a later image request fails', () => {
  const images: { onload?: () => void; onerror?: () => void; src?: string }[] = [];
  class FakeImage {
    onload?: () => void;
    onerror?: () => void;
    src?: string;
    constructor() {
      images.push(this);
    }
  }
  vi.stubGlobal('Image', FakeImage);
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
  const frame = {
    id: 'frame',
    videoId: 'v',
    batchId: 'b',
    timestampMs: 0,
    width: 100,
    height: 100,
    imageUrl: '/media/frame.jpg',
    status: 'not_started' as const,
    revision: 0,
  };
  const props = {
    frame,
    annotations: [],
    mode: 'select' as const,
    disabled: false,
    select: () => {},
    change: () => {},
  };
  const view = render(<AnnotationCanvas {...props} />);
  act(() => {
    images[0].onload?.();
  });
  view.rerender(
    <AnnotationCanvas {...props} frame={{ ...frame, imageUrl: '/media/frame-retry.jpg' }} />,
  );
  act(() => {
    images[1].onerror?.();
  });
  expect(screen.getByTestId('annotation-image-bounds')).toBeTruthy();
  expect(screen.getByRole('status').textContent).toContain('已显示已加载的原图');
});
it('attaches installed Konva Transformer nodes when draw mode becomes select without reselection', () => {
  let model: [number, number, number, number] | null = null;
  const props = {
    annotation: {
      id: 'a',
      bbox: [0, 10, 20, 20] as [number, number, number, number],
      source: 'manual' as const,
    },
    selected: true,
    onSelect: () => {},
    onChange: (box: [number, number, number, number], node: Konva.Rect) => {
      model = applyNodeBox(node, box, 100, 100);
      return model !== null;
    },
  };
  const view = render(<HeadBox {...props} enabled={false} />);
  view.rerender(<HeadBox {...props} enabled={true} />);
  expect(nodes.transformer?.nodes()).toEqual([nodes.rect]);
  nodes.rect!.scaleX(1.5);
  nodes.rect!.fire('transformend');
  expect(model).toEqual([0, 10, 30, 20]);
  expect([nodes.rect!.x(), nodes.rect!.y(), nodes.rect!.width(), nodes.rect!.height()]).toEqual(
    model,
  );
});
it('applies every normalized edge box to installed Konva node and model together', () => {
  const rect = new Konva.Rect({ x: -5, y: 10, width: 20, height: 20 });
  const box = applyNodeBox(rect, [-5, 10, 20, 20], 100, 100);
  expect(box).toEqual([0, 10, 15, 20]);
  expect([rect.x(), rect.y(), rect.width(), rect.height()]).toEqual(box);
  rect.destroy();
});
