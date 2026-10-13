import type { Draft } from './saveController';
import { normalizeBox } from './geometry';
export interface DraftScope {
  deploymentOrigin: string;
  workspaceId: string;
  userId: string;
}
export const draftKey = (frameId: string, scope: DraftScope) =>
  `classroom-annotator:draft:v2:${[scope.deploymentOrigin, scope.workspaceId, scope.userId, frameId].map(encodeURIComponent).join(':')}`;
export function readDraft(
  frameId: string,
  width: number,
  height: number,
  scope: DraftScope,
): Draft | undefined {
  try {
    const raw = localStorage.getItem(draftKey(frameId, scope));
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
