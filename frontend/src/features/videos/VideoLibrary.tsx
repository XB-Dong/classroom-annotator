import { useState } from 'react';
import type { Video } from '../../shared/api/types';
const sourceLabels: Record<NonNullable<Video['sourceKind']>, string> = {
  public_sample: '公共示例',
  mac: '历史远端素材',
  upload: '本地上传',
  dataset: '公司接口',
  legacy: '历史素材',
  local_directory: '同机素材',
};
export function VideoLibrary({
  videos,
  videoId,
  loading,
  error,
  refresh,
  select,
}: {
  videos: Video[];
  videoId: string;
  loading: boolean;
  error?: string;
  refresh: () => void;
  select: (id: string) => void;
}) {
  const [showHistory, setShowHistory] = useState(false);
  const visible = videos.filter(
    (video) => showHistory || video.id === videoId || video.sourceKind !== 'legacy',
  );
  const selectedVideo = videos.find((video) => video.id === videoId);
  return (
    <section className="library">
      <div className="panel-title">
        <h2>视频库</h2>
        <button className="text-button" onClick={refresh}>
          刷新
        </button>
      </div>
      {videos.some((video) => video.sourceKind === 'legacy') && (
        <button className="text-button" onClick={() => setShowHistory((value) => !value)}>
          {showHistory ? '隐藏历史素材' : '显示历史素材'}
        </button>
      )}
      {loading ? (
        <p className="muted">正在加载视频…</p>
      ) : error ? (
        <p className="notice error" role="alert">
          {error}
        </p>
      ) : visible.length === 0 ? (
        <p className="muted small">还没有视频，先上传或导入。</p>
      ) : (
        <label>
          当前视频
          <select
            data-testid="video-select"
            value={videoId}
            onChange={(e) => select(e.target.value)}
          >
            <option value="">选择视频</option>
            {visible.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
                {v.sourceKind ? ` · ${sourceLabels[v.sourceKind]}` : ''}
              </option>
            ))}
          </select>
          {selectedVideo?.sourceKind && (
            <span className="muted small">
              来源：{sourceLabels[selectedVideo.sourceKind]} · 存储：
              {selectedVideo.storageProvider === 'mac'
                ? '历史远端（不可用）'
                : selectedVideo.storageProvider === 'local'
                  ? '当前部署主机'
                  : '未知'}
            </span>
          )}
        </label>
      )}
    </section>
  );
}
