import { afterEach, expect, it } from 'vitest';
import { draftKey, readDraft } from './drafts';
const scope = {
  deploymentOrigin: 'https://annotation.example',
  workspaceId: 'shared',
  userId: 'alice',
};
const draft = {
  baseRevision: 2,
  editSequence: 1,
  snapshot: { frameId: 'f', revision: 2, status: 'in_progress', annotations: [] },
};
afterEach(() => localStorage.clear());

it('isolates draft keys by origin, workspace, user and frame', () => {
  const key = draftKey('f', scope);
  for (const other of [
    { ...scope, deploymentOrigin: 'https://other.example' },
    { ...scope, workspaceId: 'other' },
    { ...scope, userId: 'bob' },
  ]) {
    expect(draftKey('f', other)).not.toBe(key);
  }
  expect(draftKey('g', scope)).not.toBe(key);
});

it('restores only this account draft and ignores old unscoped data', () => {
  localStorage.setItem('classroom-annotator:draft:v1:f', JSON.stringify(draft));
  expect(readDraft('f', 100, 100, scope)).toBeUndefined();
  localStorage.setItem(draftKey('f', scope), JSON.stringify(draft));
  expect(readDraft('f', 100, 100, scope)?.baseRevision).toBe(2);
  expect(readDraft('f', 100, 100, { ...scope, userId: 'bob' })).toBeUndefined();
});
it('restores mixed manual and server-owned AI annotations without losing pending manual edits', () => {
  const mixed = {
    ...draft,
    snapshot: {
      ...draft.snapshot,
      annotations: [
        { id: 'ai-a', bbox: [1, 1, 8, 8], source: 'ai_corrected', candidateId: 'c', runId: 'r' },
        { id: 'manual-a', bbox: [20, 20, 8, 8], source: 'manual' },
      ],
    },
  };
  localStorage.setItem(draftKey('f', scope), JSON.stringify(mixed));
  expect(readDraft('f', 100, 100, scope)?.snapshot.annotations).toEqual(mixed.snapshot.annotations);
});
