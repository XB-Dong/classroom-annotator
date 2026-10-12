import type { Snapshot } from '../../shared/api/types';
export interface Draft { baseRevision: number; editSequence: number; snapshot: Snapshot }
export type SaveState = 'saved' | 'dirty' | 'saving' | 'error' | 'conflict';
export interface SaveView { snapshot: Snapshot; state: SaveState; error?: string; conflict?: Snapshot }
export interface SaveDependencies { save: (snapshot: Snapshot) => Promise<Snapshot>; load: () => Promise<Snapshot>; writeDraft: (draft: Draft) => void; clearDraft: () => void; changed: (view: SaveView) => void }
export class SaveController {
  private sequence = 0;
  private acknowledged = 0;
  private inFlight = false;
  private suspended = false;
  private disposed = false;
  private timer?: ReturnType<typeof setTimeout>;
  private state: SaveState = 'saved';
  constructor(public snapshot: Snapshot, public dependencies: SaveDependencies, draft?: Draft) {
    if (draft) {
      this.sequence = Math.max(1,draft.editSequence);
      const stale = draft.baseRevision !== snapshot.revision;
      this.snapshot = {...structuredClone(draft.snapshot),revision:snapshot.revision};
      this.suspended = stale;
      this.state = stale ? 'conflict' : 'dirty';
      this.emit(stale ? '草稿与服务器版本不同，请明确选择恢复方式。' : undefined, stale ? snapshot : undefined);
      if (!stale) this.schedule();
    } else this.emit();
  }
  private emit(error?: string, conflict?: Snapshot) { if (!this.disposed) this.dependencies.changed({snapshot:structuredClone(this.snapshot),state:this.state,error,conflict}); }
  private persist() { this.dependencies.writeDraft({baseRevision:this.snapshot.revision,editSequence:this.sequence,snapshot:structuredClone(this.snapshot)}); }
  private schedule() { clearTimeout(this.timer); if (!this.disposed && !this.suspended) this.timer=setTimeout(() => void this.flush(),500); }
  edit(snapshot: Snapshot) {
    this.snapshot={...structuredClone(snapshot),revision:this.snapshot.revision};
    this.sequence++; this.persist();
    if (!this.suspended) this.state=this.inFlight?'saving':'dirty';
    this.emit(); this.schedule();
  }
  async flush() {
    clearTimeout(this.timer);
    if (this.disposed || this.suspended || this.inFlight || this.sequence===this.acknowledged) return;
    const captured=structuredClone(this.snapshot); const capturedSequence=this.sequence;
    this.inFlight=true; this.state='saving'; this.emit();
    try {
      const ack=await this.dependencies.save(captured);
      this.acknowledged=capturedSequence;
      this.snapshot={...this.snapshot,revision:ack.revision};
      if (this.sequence===capturedSequence) { this.snapshot=ack; this.state='saved'; this.dependencies.clearDraft(); }
      else { this.state='dirty'; this.persist(); }
      this.emit();
    } catch (error) {
      if (typeof error==='object' && error!==null && 'status' in error && error.status===409) {
        this.suspended=true; this.state='conflict';
        try { const latest=await this.dependencies.load(); this.emit('服务器内容已改变，请选择替换或丢弃草稿。',latest); }
        catch { this.emit('版本冲突；获取服务器版本失败，请重试恢复。'); }
      } else { this.state='error'; this.emit(error instanceof Error ? error.message : '保存失败'); }
    } finally { this.inFlight=false; if (this.state==='dirty') this.schedule(); }
  }
  async resolve(replace: boolean) {
    if (this.inFlight) return;
    try {
      const latest=await this.dependencies.load();
      this.suspended=false;
      if (replace) { this.snapshot={...this.snapshot,revision:latest.revision}; this.sequence++; this.state='dirty'; this.persist(); this.schedule(); }
      else { this.snapshot=latest; this.acknowledged=this.sequence; this.state='saved'; this.dependencies.clearDraft(); }
      this.emit();
    } catch (error) { this.emit(error instanceof Error ? error.message : '无法获取最新版本'); }
  }
  dispose() { this.disposed=true; clearTimeout(this.timer); }
}
