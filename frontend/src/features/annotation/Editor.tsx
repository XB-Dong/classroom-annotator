import { useEffect, useRef, useState } from 'react';
import type {
  Annotation,
  Candidate,
  Frame,
  Job,
  ReviewDecision,
  ReviewSnapshot,
  Snapshot,
} from '../../shared/api/types';
import { idPath, json, request, saveSnapshot } from '../../shared/api/client';
import { ReviewPanel } from '../review/ReviewPanel';
import type { Box } from './geometry';
import { AnnotationCanvas } from './Canvas';
import { SaveController, type SaveView } from './saveController';
import { draftKey, readDraft, type DraftScope } from './drafts';
import { EditHistory } from './history';
interface Props {
  frame: Frame;
  draftScope: DraftScope;
  onSaved: (snapshot: Snapshot) => void;
  onController?: (controller?: SaveController) => void;
  onStateChange?: (view: SaveView) => void;
  onJob?: (job: Job) => void;
}
const statusLabels = {
  saved: '已保存',
  dirty: '待保存',
  saving: '保存中',
  error: '保存失败',
  conflict: '版本冲突',
};
export function Editor({ frame, draftScope, onSaved, onController, onStateChange, onJob }: Props) {
  const [view, setView] = useState<SaveView>();
  const [loadError, setLoadError] = useState<string>();
  const [draftWarning, setDraftWarning] = useState<string>();
  const [selected, setSelected] = useState<string>();
  const [mode, setMode] = useState<'draw' | 'select'>('draw');
  const [review, setReview] = useState<ReviewSnapshot>();
  const [reviewBusy, setReviewBusy] = useState(false);
  const [reviewError, setReviewError] = useState<string>();
  const [newVersion, setNewVersion] = useState(false);
  const [candidateId, setCandidateId] = useState<string>();
  const [adjustedBox, setAdjustedBox] = useState<Box>();
  const [interacting, setInteracting] = useState(false);
  const busyRef = useRef(false);
  const interactionRef = useRef(false);
  const candidateDraftRef = useRef(false);
  candidateDraftRef.current = !!adjustedBox;
  const alive = useRef(false);
  const reviewRequest = useRef(false);
  const generationKey = useRef<{ revision: number; key: string } | undefined>(undefined);
  const controller = useRef<SaveController>(undefined);
  const history = useRef(new EditHistory());
  const savedCallback = useRef(onSaved);
  savedCallback.current = onSaved;
  const lifecycleCallbacks = useRef({ onController, onStateChange });
  lifecycleCallbacks.current = { onController, onStateChange };
  useEffect(() => {
    let active = true;
    alive.current = true;
    setView(undefined);
    setLoadError(undefined);
    setSelected(undefined);
    history.current = new EditHistory();
    const load = async () => {
      const result = await request<ReviewSnapshot>(`/frames/${idPath(frame.id)}/review`);
      if (
        result.frameId !== frame.id ||
        !Array.isArray(result.annotations) ||
        !Array.isArray(result.candidates) ||
        !Array.isArray(result.issues)
      )
        throw new Error('复核快照格式不受支持，请联系管理员。');
      if (active) setReview(result);
      return result;
    };
    void load()
      .then((snapshot) => {
        if (!active) return;
        controller.current = new SaveController(
          snapshot,
          {
            save: saveSnapshot,
            load,
            changed: (v) => {
              if (active) {
                setView(v);
                lifecycleCallbacks.current.onStateChange?.(v);
                if (v.state === 'saved') savedCallback.current(v.snapshot);
              }
            },
            writeDraft: (d) => {
              try {
                localStorage.setItem(draftKey(frame.id, draftScope), JSON.stringify(d));
              } catch {
                setDraftWarning('浏览器草稿存储不可用，请保持页面打开直到保存成功。');
              }
            },
            clearDraft: () => {
              try {
                localStorage.removeItem(draftKey(frame.id, draftScope));
              } catch {
                setDraftWarning('无法清理浏览器草稿。');
              }
            },
          },
          readDraft(frame.id, frame.width, frame.height, draftScope),
        );
        lifecycleCallbacks.current.onController?.(controller.current);
      })
      .catch((e) => {
        if (active) setLoadError(e.message);
      });
    return () => {
      active = false;
      alive.current = false;
      controller.current?.dispose();
      controller.current = undefined;
      lifecycleCallbacks.current.onController?.(undefined);
    };
  }, [
    frame.id,
    frame.width,
    frame.height,
    draftScope.deploymentOrigin,
    draftScope.workspaceId,
    draftScope.userId,
  ]);
  useEffect(() => {
    let active = true;
    const poll = async () => {
      const current = controller.current;
      if (
        !active ||
        document.hidden ||
        busyRef.current ||
        interactionRef.current ||
        candidateDraftRef.current ||
        reviewRequest.current ||
        !current?.canRefresh
      )
        return;
      const revision = current.snapshot.revision;
      reviewRequest.current = true;
      try {
        const latest = await request<ReviewSnapshot>(`/frames/${idPath(frame.id)}/review`);
        if (!active) return;
        if (busyRef.current || interactionRef.current || candidateDraftRef.current) {
          if (latest.revision !== current.snapshot.revision) setNewVersion(true);
          return;
        }
        if (current.acceptServerSnapshot(latest, revision)) {
          setReview(latest);
          setNewVersion(false);
          setReviewError(undefined);
          if (latest.revision !== revision) {
            history.current = new EditHistory();
            setCandidateId(undefined);
            setAdjustedBox(undefined);
          }
        } else if (latest.revision !== current.snapshot.revision) setNewVersion(true);
      } catch (error) {
        if (active) setReviewError((error as Error).message);
      } finally {
        reviewRequest.current = false;
      }
    };
    const timer = setInterval(() => void poll(), 5000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [frame.id]);
  async function reviewAction(action: (revision: number) => Promise<ReviewSnapshot | Job>) {
    const current = controller.current;
    if (!current || busyRef.current || interactionRef.current) return;
    busyRef.current = true;
    setReviewBusy(true);
    setReviewError(undefined);
    try {
      if (!(await current.settle()) || !current.canRefresh)
        throw new Error('请先保存或处理当前草稿，再复核候选。');
      const revision = current.snapshot.revision;
      const result = await action(revision);
      if (!alive.current || current !== controller.current) return;
      if ('frameId' in result) {
        if (!current.acceptServerSnapshot(result, revision)) {
          setNewVersion(true);
          return;
        }
        setReview(result);
        history.current = new EditHistory();
        setAdjustedBox(undefined);
        setCandidateId(undefined);
      } else onJob?.(result);
      const capturedRevision = current.snapshot.revision;
      const latest = await request<ReviewSnapshot>(`/frames/${idPath(frame.id)}/review`);
      if (!alive.current || current !== controller.current) return;
      if (current.acceptServerSnapshot(latest, capturedRevision)) {
        setReview(latest);
        setNewVersion(false);
      } else setNewVersion(true);
    } catch (error) {
      if (alive.current && current === controller.current) setReviewError((error as Error).message);
    } finally {
      busyRef.current = false;
      if (alive.current) setReviewBusy(false);
    }
  }
  function decide(decisions: ReviewDecision[]) {
    void reviewAction((revision) =>
      request<ReviewSnapshot>(`/frames/${idPath(frame.id)}/review`, {
        ...json({ expectedRevision: revision, decisions, issueResolutions: [] }),
        method: 'PUT',
      }),
    );
  }
  function selectCandidate(candidate: Candidate) {
    setCandidateId(candidate.id);
    setAdjustedBox(undefined);
    setSelected(candidate.annotationId ?? undefined);
    setMode('select');
  }
  function generate() {
    void reviewAction(async (revision) => {
      if (generationKey.current?.revision !== revision)
        generationKey.current = { revision, key: crypto.randomUUID() };
      const job = await request<Job>(`/frames/${idPath(frame.id)}/preannotations`, {
        ...json({ expectedRevision: revision }),
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': generationKey.current.key,
        },
      });
      generationKey.current = undefined;
      return job;
    });
  }
  function edit(snapshot: Snapshot) {
    if (!controller.current || view?.state === 'conflict' || busyRef.current) return;
    history.current.push(controller.current.snapshot);
    controller.current.edit(snapshot);
  }
  function change(annotations: Annotation[]) {
    if (view)
      edit({
        ...view.snapshot,
        annotations,
        status: 'in_progress',
      });
  }
  function remove() {
    if (selected && view) {
      change(view.snapshot.annotations.filter((a) => a.id !== selected));
      setSelected(undefined);
    }
  }
  function undo() {
    if (controller.current && view?.state !== 'conflict') {
      const s = history.current.undo(controller.current.snapshot);
      if (s) controller.current.edit(s);
    }
  }
  function redo() {
    if (controller.current && view?.state !== 'conflict') {
      const s = history.current.redo(controller.current.snapshot);
      if (s) controller.current.edit(s);
    }
  }
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const el = event.target;
      if (
        (el instanceof HTMLElement &&
          el.closest('input,textarea,select,[contenteditable="true"]')) ||
        view?.state === 'conflict' ||
        reviewBusy ||
        interacting
      )
        return;
      if (event.key === 'Delete' || event.key === 'Backspace') {
        event.preventDefault();
        remove();
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
        event.preventDefault();
        if (event.shiftKey) redo();
        else undo();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  });
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (controller.current && view?.state !== 'saved') {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [view?.state]);
  if (loadError)
    return (
      <div className="empty error" role="alert">
        标注加载失败：{loadError}
        <button onClick={() => window.location.reload()}>刷新重试</button>
      </div>
    );
  if (!view) return <div className="empty">正在加载标注…</div>;
  const locked = view.state === 'conflict' || reviewBusy;
  const pending = (review?.candidates ?? []).some((c) => c.reviewState === 'pending');
  const unresolved = (review?.issues ?? []).some((i) => !i.resolved);
  return (
    <div className="editor">
      <section className="canvas-area">
        <div className="canvas-toolbar">
          <div className="segmented">
            <button
              data-testid="draw-mode"
              className={mode === 'draw' ? 'active' : ''}
              onClick={() => setMode('draw')}
              disabled={locked}
            >
              ＋ 画框
            </button>
            <button
              data-testid="select-mode"
              className={mode === 'select' ? 'active' : ''}
              onClick={() => setMode('select')}
              disabled={locked}
            >
              ↖ 选择 / 调整
            </button>
          </div>
          <button onClick={undo} disabled={locked}>
            撤销
          </button>
          <button onClick={redo} disabled={locked}>
            重做
          </button>
          <span data-testid="save-state" className={`badge ${view.state}`} role="status">
            {statusLabels[view.state]}
          </span>
        </div>
        {view.error && (
          <div className="notice error" role="alert">
            {view.error}
            {view.state === 'error' && (
              <button onClick={() => void controller.current?.flush()}>重试保存</button>
            )}
          </div>
        )}
        {draftWarning && (
          <div className="notice error" role="alert">
            {draftWarning}
          </div>
        )}
        {reviewError && (
          <div className="notice error" role="alert">
            AI 复核：{reviewError}
          </div>
        )}
        {newVersion && (
          <div className="notice conflict" role="status">
            服务器已有新版本；本地草稿已保留，请先完成保存或处理冲突。
          </div>
        )}
        {view.state === 'conflict' && (
          <div className="notice conflict">
            <strong>草稿需要处理</strong>
            <p>
              服务器版本 {view.conflict?.revision ?? '待获取'}
              。自动保存已暂停。请读取最新版本，保留草稿后在新版本上重新编辑。
            </p>
            <button
              disabled={view.resolving}
              onClick={() => void controller.current?.resolve(true)}
            >
              查看最新并保留草稿
            </button>
            <button
              data-testid="conflict-reload"
              disabled={view.resolving}
              onClick={() => void controller.current?.resolve(false)}
            >
              丢弃草稿，使用服务器内容
            </button>
          </div>
        )}
        {view.retainedDraft && (
          <div className="notice conflict" role="status">
            <strong>本账号原草稿已保留</strong>
            <p>
              旧版本 {view.retainedDraft.baseRevision} ·{' '}
              {view.retainedDraft.snapshot.annotations.length}{' '}
              个框。画布已显示最新服务器版本；请参考草稿重新编辑，保存新的修改后清理旧草稿。
            </p>
            <details>
              <summary>查看草稿坐标</summary>
              {view.retainedDraft.snapshot.annotations.map((annotation) => (
                <p key={annotation.id}>
                  {annotation.bbox.map((value) => value.toFixed(1)).join(', ')}
                </p>
              ))}
            </details>
          </div>
        )}
        <AnnotationCanvas
          frame={frame}
          annotations={view.snapshot.annotations}
          selected={selected}
          mode={mode}
          disabled={locked}
          select={(id) => {
            setSelected(id);
            setCandidateId(undefined);
            setAdjustedBox(undefined);
          }}
          change={change}
          candidates={(review?.candidates ?? []).filter((c) => c.reviewState === 'pending')}
          selectedCandidate={candidateId}
          adjustedCandidateBox={adjustedBox}
          selectCandidate={(id) => {
            const candidate = review?.candidates?.find((c) => c.id === id);
            if (candidate) selectCandidate(candidate);
          }}
          changeCandidate={setAdjustedBox}
          interactionChanged={(value) => {
            interactionRef.current = value;
            controller.current?.setInteracting(value);
            setInteracting(value);
          }}
        />
        <div className="canvas-footer">
          <span>
            原图 {frame.width} × {frame.height} · 采样时间 {(frame.timestampMs / 1000).toFixed(3)}{' '}
            秒
          </span>
          <span>框坐标以原图像素保存</span>
        </div>
      </section>
      <aside className="annotation-panel">
        <div className="panel-title">
          <h2>人头标注</h2>
          <span className="count">{view.snapshot.annotations.length}</span>
        </div>
        <p className="muted">只标学生可见人头；遮挡时框住可见部分。</p>
        <button
          data-testid="complete-frame"
          className="primary full"
          disabled={
            locked ||
            interacting ||
            (view.snapshot.status !== 'completed' && (pending || unresolved))
          }
          onClick={() =>
            edit({
              ...view.snapshot,
              status: view.snapshot.status === 'completed' ? 'in_progress' : 'completed',
            })
          }
        >
          {view.snapshot.status === 'completed'
            ? '✓ 已完成 · 点击重新编辑'
            : view.snapshot.annotations.length === 0
              ? '标记完成（零人头）'
              : '标记本帧完成'}
        </button>
        <p className="muted small">完成后才进入导出范围。保存成功前请勿关闭页面。</p>
        {(pending || unresolved) && (
          <p className="notice conflict">待审候选或图片风险尚未处理，暂不能完成或导出本帧。</p>
        )}
        <ReviewPanel
          review={review}
          selected={candidateId}
          adjustedBox={adjustedBox}
          busy={reviewBusy || interacting}
          disabled={view.state === 'conflict' || !!view.retainedDraft}
          select={selectCandidate}
          decide={decide}
          resolve={(id) =>
            void reviewAction((revision) =>
              request<ReviewSnapshot>(`/frames/${idPath(frame.id)}/review`, {
                ...json({
                  expectedRevision: revision,
                  decisions: [],
                  issueResolutions: [{ id, resolution: 'manual_checked' }],
                }),
                method: 'PUT',
              }),
            )
          }
          generate={generate}
        />
        <div className="annotation-list">
          {view.snapshot.annotations.length === 0 ? (
            <div className="empty compact">
              还没有人头框
              <br />
              在画布上拖动创建
            </div>
          ) : (
            view.snapshot.annotations.map((a, index) => (
              <button
                className={`annotation-item ${a.id === selected ? 'selected' : ''}`}
                key={a.id}
                data-testid={`annotation-item-${a.id}`}
                onClick={() => {
                  setSelected(a.id);
                  setMode('select');
                }}
              >
                <strong>学生人头 {index + 1}</strong>
                <span>{a.bbox.map((n) => n.toFixed(1)).join(', ')}</span>
              </button>
            ))
          )}
        </div>
        <button className="danger full" disabled={!selected || locked} onClick={remove}>
          删除选中框
        </button>
        <div className="shortcut-help">
          Delete 删除选中框
          <br />
          Ctrl / ⌘ + Z 撤销
          <br />
          Shift + Ctrl / ⌘ + Z 重做
        </div>
      </aside>
    </div>
  );
}
