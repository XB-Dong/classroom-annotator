export type Box = [number, number, number, number];
export function normalizeBox(
  box: Box,
  width: number,
  height: number,
  creating = false,
): Box | null {
  if (![...box, width, height].every(Number.isFinite) || width <= 0 || height <= 0) return null;
  const [x, y, w, h] = box;
  const left = Math.max(0, Math.min(width, Math.min(x, x + w)));
  const top = Math.max(0, Math.min(height, Math.min(y, y + h)));
  const right = Math.max(0, Math.min(width, Math.max(x, x + w)));
  const bottom = Math.max(0, Math.min(height, Math.max(y, y + h)));
  const result: Box = [left, top, right - left, bottom - top];
  return result[2] > 0 && result[3] > 0 && (!creating || (result[2] >= 2 && result[3] >= 2))
    ? result
    : null;
}
export function originalPoint(point: { x: number; y: number }, scale: number) {
  return { x: point.x / scale, y: point.y / scale };
}
