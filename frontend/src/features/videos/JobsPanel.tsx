import type { Frame, Job } from '../../shared/api/types';
const labels: Record<string, string> = {
  queued: '排队中',
  running: '执行中',
  cancelling: '正在取消',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
  stale: '已过期，需要重新生成',
};
function statusLabel(status: string) {
  return Object.hasOwn(labels, status) ? labels[status] : status;
}
function elapsedSeconds(start?: string, end?: string) {
  if (!start || !end) return undefined;
  const elapsed = Date.parse(end) - Date.parse(start);
  return Number.isFinite(elapsed) && elapsed >= 0 ? (elapsed / 1000).toFixed(1) : undefined;
}
function JobTiming({ job }: { job: Job }) {
  const queue = elapsedSeconds(job.createdAt, job.startedAt ?? job.finishedAt);
  const execution = elapsedSeconds(job.startedAt, job.finishedAt);
  return (
    <div className="small muted">
      {queue !== undefined && <p>排队等待 {queue} 秒</p>}
      {execution !== undefined && <p>执行耗时 {execution} 秒</p>}
      {job.status === 'queued' && !job.startedAt && <p>排队等待中</p>}
      {['running', 'cancelling'].includes(job.status) &&
        job.startedAt && Number.isFinite(Date.parse(job.startedAt)) && !job.finishedAt && (
          <p>执行计时中</p>
        )}
    </div>
  );
}
export function JobsPanel({
  jobs,
  frames = [],
  error,
  cancel,
}: {
  jobs: Job[];
  frames?: Frame[];
  error?: string;
  cancel: (id: string) => void;
}) {
  const currentFrames = new Map(frames.map((frame) => [frame.id, frame]));
  return (
    <section className="jobs-panel">
      <div className="panel-title">
        <h2>任务记录</h2>
        <span className="eyebrow">自动刷新</span>
      </div>
      {error && (
        <p role="alert" className="notice error">
          {error}
        </p>
      )}
      {jobs.length === 0 ? (
        <p className="muted small">暂无导入、抽帧或 AI 任务。</p>
      ) : (
        <div className="job-list">
          {jobs.map((j) => (
            <div className="job" key={j.id}>
              <div>
                <strong>
                  {j.kind === 'import'
                    ? '视频导入'
                    : j.kind === 'extraction'
                      ? '视频抽帧'
                      : 'AI 预标注'}
                </strong>
                <span className={`badge ${j.status}`}>{statusLabel(j.status)}</span>
              </div>
              <progress max={1} value={j.progress} />
              <small>
                {Math.round(j.progress * 100)}% ·{' '}
                {new Date(j.createdAt).toLocaleTimeString('zh-CN')}
              </small>
              {j.error && <p className="error-text">{j.error}</p>}
              {j.kind === 'preannotation' && (
                <>
                  <JobTiming job={j} />
                  {!!j.partialResults?.length && (
                    <details>
                      <summary>图片结果（{j.partialResults.length}）</summary>
                      <ul className="small">
                        {j.partialResults.map((result) => {
                          const frame = currentFrames.get(result.frameId);
                          return (
                            <li key={`${result.frameId}:${result.runId}`}>
                              <p title={result.frameId}>
                                {frame && Number.isFinite(frame.timestampMs)
                                  ? `采样时间 ${(frame.timestampMs / 1000).toFixed(3)} 秒`
                                  : `图片结果 ${result.frameId.slice(0, 8)}`}
                              </p>
                              <span>{statusLabel(result.status)}</span>
                              <p>
                                {result.status === 'completed'
                                  ? result.applied
                                    ? '当前候选已生成，仍需人工复核'
                                    : '结果未激活'
                                  : '未应用结果'}
                              </p>
                              {result.errorCode && <p className="error-text">{result.errorCode}</p>}
                            </li>
                          );
                        })}
                      </ul>
                    </details>
                  )}
                </>
              )}
              {['queued', 'running', 'cancelling'].includes(j.status) && (
                <button disabled={j.status === 'cancelling'} onClick={() => cancel(j.id)}>
                  {j.status === 'cancelling' ? '等待清理完成…' : '取消任务'}
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
