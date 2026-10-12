import type { Frame } from '../../shared/api/types';
const labels = { not_started: '未开始', in_progress: '标注中', completed: '已完成' };
export function FramePicker({
  frames,
  frameId,
  busy,
  select,
}: {
  frames: Frame[];
  frameId: string;
  busy: boolean;
  select: (id: string) => void;
}) {
  return (
    <aside className="frame-sidebar">
      <div className="panel-title">
        <h2>图片帧</h2>
        <span className="muted small">{frames.length} 张</span>
      </div>
      <div className="frame-list">
        {frames.map((frame, index) => (
          <button
            data-testid={`frame-item-${frame.id}`}
            key={frame.id}
            className={`frame-item ${frame.id === frameId ? 'selected' : ''}`}
            disabled={busy}
            onClick={() => select(frame.id)}
          >
            <img src={frame.imageUrl} alt={`第 ${index + 1} 帧`} loading="lazy" />
            <span>
              <strong>帧 {String(index + 1).padStart(3, '0')}</strong>
              <small>采样时间 {(frame.timestampMs / 1000).toFixed(3)} 秒</small>
              <em className={frame.status}>{labels[frame.status]}</em>
            </span>
          </button>
        ))}
      </div>
    </aside>
  );
}
