import { useEffect, useState } from 'react';
import type { Frame } from '../../shared/api/types';
const labels = { not_started: '未开始', in_progress: '标注中', completed: '已完成' };
function FrameThumbnail({ frame, index }: { frame: Frame; index: number }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [frame.imageUrl]);
  return failed ? (
    <span className="thumbnail-unavailable">缩略图暂不可用</span>
  ) : (
    <img
      src={frame.imageUrl}
      alt={`第 ${index + 1} 帧`}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}
export function FramePicker({
  frames,
  frameId,
  busy,
  select,
  unresolvedRiskFrameIds,
}: {
  frames: Frame[];
  frameId: string;
  busy: boolean;
  select: (id: string) => void;
  unresolvedRiskFrameIds?: string[];
}) {
  const [onlyRisk, setOnlyRisk] = useState(false);
  const riskIds = new Set(unresolvedRiskFrameIds);
  const riskAvailable = unresolvedRiskFrameIds !== undefined;
  const filtering = onlyRisk && riskAvailable;
  const riskCount = frames.filter((frame) => riskIds.has(frame.id)).length;
  const visibleFrames = frames
    .map((frame, index) => ({ frame, index }))
    .filter(({ frame }) => !filtering || riskIds.has(frame.id));
  return (
    <aside className="frame-sidebar">
      <div className="panel-title">
        <h2>图片帧</h2>
        <span className="muted small">{frames.length} 张</span>
      </div>
      <p className="muted small">
        {riskAvailable ? `需复核图片 ${riskCount} 张` : '图片风险数据暂不可用'}
      </p>
      <label>
        <input
          type="checkbox"
          checked={filtering}
          disabled={!riskAvailable}
          onChange={(event) => setOnlyRisk(event.target.checked)}
        />
        仅看需复核图片
      </label>
      {filtering && frameId && !riskIds.has(frameId) && (
        <p className="muted small">当前帧不在筛选结果中，画布继续保留。</p>
      )}
      {filtering && visibleFrames.length === 0 && (
        <p className="muted small">当前批次没有未解决风险图片。</p>
      )}
      <div className="frame-list">
        {visibleFrames.map(({ frame, index }) => (
          <button
            data-testid={`frame-item-${frame.id}`}
            key={frame.id}
            className={`frame-item ${frame.id === frameId ? 'selected' : ''}`}
            disabled={busy}
            onClick={() => select(frame.id)}
          >
            <FrameThumbnail frame={frame} index={index} />
            <span>
              <strong>帧 {String(index + 1).padStart(3, '0')}</strong>
              <small>采样时间 {(frame.timestampMs / 1000).toFixed(3)} 秒</small>
              <em className={frame.status}>{labels[frame.status]}</em>
              {riskIds.has(frame.id) && <em className="notice conflict">需复核</em>}
            </span>
          </button>
        ))}
      </div>
    </aside>
  );
}
