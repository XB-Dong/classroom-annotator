import { describe, expect, it, vi } from 'vitest';
import { SaveController, type Draft, type SaveView } from './saveController';
import type { Snapshot } from '../../shared/api/types';
const snapshot = (revision = 0, status: Snapshot['status'] = 'not_started'): Snapshot => ({
  frameId: 'f',
  revision,
  status,
  annotations: [],
});
function setup(save: (s: Snapshot) => Promise<Snapshot>, draft?: Draft) {
  let view: SaveView | undefined;
  let stored: Draft | undefined;
  const clear = vi.fn(() => {
    stored = undefined;
  });
  const controller = new SaveController(
    snapshot(),
    {
      save,
      load: async () => snapshot(7, 'completed'),
      writeDraft: (d) => {
        stored = d;
      },
      clearDraft: clear,
      changed: (v) => {
        view = v;
      },
    },
    draft,
  );
  return {
    controller,
    clear,
    get view() {
      return view;
    },
    get stored() {
      return stored;
    },
  };
}
describe('captured snapshot saving and draft recovery', () => {
  it('keeps edit B while A saves, serializes saves and clears draft only after matching acknowledgement', async () => {
    let resolveA!: (s: Snapshot) => void;
    const requests: Snapshot[] = [];
    const state = setup((s) => {
      requests.push(structuredClone(s));
      return requests.length === 1
        ? new Promise((resolve) => {
            resolveA = resolve;
          })
        : Promise.resolve({ ...s, revision: 2 });
    });
    state.controller.edit(snapshot(0, 'in_progress'));
    const first = state.controller.flush();
    state.controller.edit(snapshot(0, 'completed'));
    await state.controller.flush();
    expect(requests).toHaveLength(1);
    resolveA(snapshot(1, 'in_progress'));
    await first;
    expect(state.controller.snapshot.status).toBe('completed');
    expect(state.clear).not.toHaveBeenCalled();
    expect(state.stored?.baseRevision).toBe(1);
    await state.controller.flush();
    expect(requests[1]).toMatchObject({ revision: 1, status: 'completed' });
    expect(state.clear).toHaveBeenCalledOnce();
    state.controller.dispose();
  });
  it('suspends autosave for a recovered stale draft and replaces with latest server revision explicitly', async () => {
    const requests: Snapshot[] = [];
    const state = setup(
      async (s) => {
        requests.push(s);
        return { ...s, revision: 8 };
      },
      { baseRevision: 3, editSequence: 2, snapshot: snapshot(3, 'in_progress') },
    );
    expect(state.view?.state).toBe('conflict');
    await state.controller.flush();
    expect(requests).toHaveLength(0);
    await state.controller.resolve(true);
    await state.controller.flush();
    expect(requests[0]).toMatchObject({ revision: 7, status: 'in_progress' });
    state.controller.dispose();
  });
  it('discarding a stale draft restores latest server and clears it without saving', async () => {
    const save = vi.fn(async (s) => s);
    const state = setup(save, {
      baseRevision: 3,
      editSequence: 2,
      snapshot: snapshot(3, 'in_progress'),
    });
    await state.controller.resolve(false);
    expect(state.controller.snapshot).toMatchObject({ revision: 7, status: 'completed' });
    expect(state.clear).toHaveBeenCalledOnce();
    expect(save).not.toHaveBeenCalled();
    state.controller.dispose();
  });
  it('exposes save failure and retains draft for retry', async () => {
    const state = setup(async () => {
      throw new Error('network failed');
    });
    state.controller.edit(snapshot(0, 'completed'));
    await state.controller.flush();
    expect(state.view?.state).toBe('error');
    expect(state.stored).toBeDefined();
    state.controller.dispose();
  });
  it('does not let an old disposed save clear a draft belonging to a reopened editor', async () => {
    let resolve!: (s: Snapshot) => void;
    const state = setup(
      () =>
        new Promise((ack) => {
          resolve = ack;
        }),
    );
    state.controller.edit(snapshot(0, 'in_progress'));
    const saving = state.controller.flush();
    state.controller.dispose();
    const reopened = new SaveController(snapshot(), state.controller.dependencies, state.stored);
    reopened.edit(snapshot(0, 'completed'));
    resolve(snapshot(1, 'in_progress'));
    await saving;
    expect(state.clear).not.toHaveBeenCalled();
    expect(state.stored?.snapshot.status).toBe('completed');
    expect(state.stored?.editSequence).toBe(2);
    reopened.dispose();
  });
  it('does not allow disposed conflict resolution to overwrite a reopened draft', async () => {
    const state = setup(async (s) => s, {
      baseRevision: 3,
      editSequence: 2,
      snapshot: snapshot(3, 'in_progress'),
    });
    let resolve!: (s: Snapshot) => void;
    state.controller.dependencies.load = () =>
      new Promise((ack) => {
        resolve = ack;
      });
    const resolving = state.controller.resolve(true);
    state.controller.dispose();
    const reopened = new SaveController(snapshot(), state.controller.dependencies);
    reopened.edit(snapshot(0, 'completed'));
    resolve(snapshot(7));
    await resolving;
    expect(state.stored?.snapshot.status).toBe('completed');
    expect(state.stored?.baseRevision).toBe(0);
    reopened.dispose();
  });
});
