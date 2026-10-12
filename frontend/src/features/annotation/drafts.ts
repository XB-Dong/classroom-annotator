import type { Draft } from './saveController';
import { normalizeBox } from './geometry';
export const draftKey = (frameId: string) => `classroom-annotator:draft:v1:${frameId}`;
export function readDraft(frameId: string, width: number, height: number): Draft | undefined {
  try {
    const raw = localStorage.getItem(draftKey(frameId));
    if (!raw) return;
    const d = JSON.parse(raw) as Draft;
    if (
      !Number.isInteger(d.baseRevision) ||
      d.baseRevision < 0 ||
      !Number.isInteger(d.editSequence) ||
      d.snapshot.frameId !== frameId ||
      !['not_started', 'in_progress', 'completed'].includes(d.snapshot.status) ||
      !Array.isArray(d.snapshot.annotations)
    )
      return;
    if (
      d.snapshot.annotations.some(
        (a) =>
          typeof a.id !== 'string' ||
          a.source !== 'manual' ||
          !Array.isArray(a.bbox) ||
          a.bbox.length !== 4 ||
          !normalizeBox(a.bbox, width, height),
      )
    )
      return;
    return d;
  } catch {
    return;
  }
}
