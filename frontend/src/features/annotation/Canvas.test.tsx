import { cleanup, render } from '@testing-library/react';
import { useEffect, useImperativeHandle, useRef, type Ref } from 'react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import Konva from 'konva';
import { HeadBox } from './Canvas';
import { applyNodeBox } from './nodeGeometry';
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
  nodes.rect?.destroy();
  nodes.transformer?.destroy();
  nodes.rect = undefined;
  nodes.transformer = undefined;
  vi.restoreAllMocks();
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
