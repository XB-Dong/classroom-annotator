import type { Snapshot } from '../../shared/api/types';
export class EditHistory {
  private past: Snapshot[] = [];
  private future: Snapshot[] = [];
  push(previous: Snapshot) {
    this.past.push(structuredClone(previous));
    this.future = [];
    if (this.past.length > 100) this.past.shift();
  }
  undo(current: Snapshot) {
    const previous = this.past.pop();
    if (previous) this.future.push(structuredClone(current));
    return previous;
  }
  redo(current: Snapshot) {
    const next = this.future.pop();
    if (next) this.past.push(structuredClone(current));
    return next;
  }
}
