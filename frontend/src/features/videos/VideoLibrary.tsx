import type { Video } from '../../shared/api/types';
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
  return (
    <section className="library">
      <div className="panel-title">
        <h2>视频库</h2>
        <button className="text-button" onClick={refresh}>
          刷新
        </button>
      </div>
      {loading ? (
        <p className="muted">正在加载视频…</p>
      ) : error ? (
        <p className="notice error" role="alert">
          {error}
        </p>
      ) : videos.length === 0 ? (
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
            {videos.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
        </label>
      )}
    </section>
  );
}
