import type { Batch, Progress } from '../../shared/api/types';
export function BatchBar({
  batches,
  batchId,
  loading,
  busy,
  progress,
  select,
}: {
  batches: Batch[];
  batchId: string;
  loading: boolean;
  busy: boolean;
  progress?: Progress;
  select: (id: string) => void;
}) {
  return (
    <div className="batch-bar">
      <label>
        抽帧批次
        <select
          aria-label="抽帧批次"
          data-testid="batch-select"
          value={batchId}
          onChange={(e) => select(e.target.value)}
          disabled={busy || loading || batches.length === 0}
        >
          <option value="">
            {loading ? '加载中…' : batches.length ? '选择批次' : '暂无已完成批次'}
          </option>
          {batches.map((batch, index) => (
            <option key={batch.id} value={batch.id}>
              批次 {index + 1} · {new Date(batch.createdAt).toLocaleString('zh-CN')} ·{' '}
              {batch.frameCount} 帧
            </option>
          ))}
        </select>
      </label>
      {progress && (
        <div className="batch-progress">
          <span>
            完成{' '}
            <strong>
              {progress.completed} / {progress.total}
            </strong>
          </span>
          <progress value={progress.completed} max={progress.total || 1} />
          <small>
            未开始 {progress.notStarted} · 标注中 {progress.inProgress}
          </small>
        </div>
      )}
    </div>
  );
}
