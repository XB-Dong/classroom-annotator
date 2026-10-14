import { useEffect, useRef, useState } from 'react';
import { idPath, json, request } from '../../shared/api/client';
import type { Frame, Job } from '../../shared/api/types';
export function BatchReviewPanel({
  batchId,
  frames,
  blocked,
  beforeStart,
  onJob,
}: {
  batchId: string;
  frames: Frame[];
  blocked: boolean;
  beforeStart: () => Promise<boolean>;
  onJob: (job: Job) => void;
}) {
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const alive = useRef(true);
  const locked = useRef(false);
  const retryKey = useRef<{ fingerprint: string; key: string } | undefined>(undefined);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const eligible = frames.filter((f) => f.status !== 'completed');
  const chosen = selected.filter((id) => eligible.some((f) => f.id === id));
  async function start() {
    if (locked.current || blocked || chosen.length === 0 || chosen.length > 20) return;
    locked.current = true;
    setBusy(true);
    setError(undefined);
    try {
      if (!(await beforeStart())) throw new Error('请先保存或处理当前帧草稿，再开始批量 AI。');
      if (!alive.current) return;
      const fingerprint = JSON.stringify(chosen);
      if (retryKey.current?.fingerprint !== fingerprint)
        retryKey.current = { fingerprint, key: crypto.randomUUID() };
      const job = await request<Job>(`/batches/${idPath(batchId)}/preannotations`, {
        ...json({ frameIds: chosen }),
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': retryKey.current.key },
      });
      if (alive.current) {
        onJob(job);
        setSelected([]);
        retryKey.current = undefined;
      }
    } catch (cause) {
      if (alive.current) setError((cause as Error).message);
    } finally {
      locked.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="batch-review">
      <details>
        <summary>批量 AI 预标注</summary>
        <div className="batch-review-options">
          <p>最多 20 张；只生成候选，逐项人工审核后才能完成或导出。已完成图片不参与。</p>
          <button
            disabled={busy || blocked}
            onClick={() => setSelected(eligible.slice(0, 20).map((f) => f.id))}
          >
            选择前 20 张未完成图片
          </button>
          <div className="batch-frame-options">
            {eligible.map((f) => (
              <label key={f.id}>
                <input
                  type="checkbox"
                  checked={chosen.includes(f.id)}
                  disabled={busy || blocked || (!chosen.includes(f.id) && chosen.length >= 20)}
                  onChange={(e) =>
                    setSelected((previous) =>
                      e.target.checked ? [...previous, f.id] : previous.filter((id) => id !== f.id),
                    )
                  }
                />
                {(f.timestampMs / 1000).toFixed(3)} 秒
              </label>
            ))}
          </div>
          <button
            className="primary"
            disabled={busy || blocked || chosen.length === 0}
            onClick={() => void start()}
          >
            {busy ? '正在提交批量 AI…' : `开始批量 AI（${chosen.length} 张）`}
          </button>
        </div>
      </details>
      {error && (
        <p className="notice error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
