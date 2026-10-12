import type Konva from 'konva';
import { normalizeBox, type Box } from './geometry';
export function applyNodeBox(
  node: Konva.Rect,
  box: Box,
  width: number,
  height: number,
): Box | null {
  const normalized = normalizeBox(box, width, height);
  if (normalized)
    node.setAttrs({
      x: normalized[0],
      y: normalized[1],
      width: normalized[2],
      height: normalized[3],
      scaleX: 1,
      scaleY: 1,
    });
  return normalized;
}
