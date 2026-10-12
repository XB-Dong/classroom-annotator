import { useState } from 'react';
import { catalogId, json, request } from '../../shared/api/client';
import type { DatasetPage, Job, Video } from '../../shared/api/types';
export function ImportPanel({
  onVideo,
  onJob,
}: {
  onVideo: (v: Video) => void;
  onJob: (j: Job) => void;
}) {
  const [tab, setTab] = useState<'local' | 'dataset'>('local');
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [data, setData] = useState<DatasetPage>();
  const [page, setPage] = useState(1);
  const [loaded, setLoaded] = useState(false);
  async function upload() {
    if (!file) return;
    setBusy(true);
    setError(undefined);
    try {
      const body = new FormData();
      body.append('file', file);
      onVideo(await request<Video>('/videos', { method: 'POST', body }));
      setFile(undefined);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function load(next = 1) {
    setBusy(true);
    setError(undefined);
    try {
      setData(await request<DatasetPage>(`/dataset/lessons?page=${next}&pageSize=20`));
      setPage(next);
      setLoaded(true);
    } catch (e) {
      setData(undefined);
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function importVideo(id: number, name: string) {
    setBusy(true);
    setError(undefined);
    try {
      onJob(await request<Job>('/dataset/imports', json({ catalogFileId: catalogId(id), name })));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="import-panel">
      <div className="panel-title">
        <h2>导入视频</h2>
        <span className="eyebrow">01 / 素材</span>
      </div>
      <div className="tabs">
        <button className={tab === 'local' ? 'active' : ''} onClick={() => setTab('local')}>
          本地上传
        </button>
        <button className={tab === 'dataset' ? 'active' : ''} onClick={() => setTab('dataset')}>
          公司数据
        </button>
      </div>
      {tab === 'local' ? (
        <div className="upload-area">
          <label className="file-label">
            选择本地视频
            <input
              data-testid="upload-file"
              type="file"
              accept="video/*"
              disabled={busy}
              onChange={(e) => setFile(e.target.files?.[0])}
            />
          </label>
          <p className="muted small">上传后可设置间隔，创建独立抽帧批次。</p>
          <button
            className="primary full"
            data-testid="upload-submit"
            disabled={!file || busy}
            onClick={() => void upload()}
          >
            {busy ? '正在上传…' : '上传视频'}
          </button>
        </div>
      ) : (
        <div>
          <p className="muted small">仅展示 channel 4 学生全景视频。接口由部署管理员配置。</p>
          <button disabled={busy} onClick={() => void load(1)}>
            {busy ? '正在加载…' : loaded ? '刷新公司课节' : '加载公司课节'}
          </button>
          {data && (
            <>
              <p className="muted small">
                共 {data.total} 节 · 第 {page} 页
              </p>
              <div className="dataset-list">
                {data.items.length === 0 ? (
                  <p>当前页没有课节。</p>
                ) : (
                  data.items.map((lesson) => (
                    <div className="dataset-lesson" key={lesson.id}>
                      <strong>{lesson.course_name || lesson.name || `课节 ${lesson.id}`}</strong>
                      <p className="muted small">
                        {lesson.organization_name} · {lesson.date} · {lesson.teacher}
                      </p>
                      {lesson.videos
                        .filter((v) => v.channel === 4)
                        .map((video) => (
                          <button
                            key={video.id}
                            disabled={busy}
                            onClick={() =>
                              void importVideo(
                                video.catalog_file_id,
                                lesson.course_name || `课节 ${lesson.id}`,
                              )
                            }
                          >
                            导入学生全景 · 分段 {video.part_index ?? 1}
                          </button>
                        ))}
                      {!lesson.videos.some((v) => v.channel === 4) && (
                        <p className="muted small">无学生全景视频</p>
                      )}
                    </div>
                  ))
                )}
              </div>
              <div className="pager">
                <button disabled={page <= 1 || busy} onClick={() => void load(page - 1)}>
                  上一页
                </button>
                <button
                  disabled={page * 20 >= data.total || busy}
                  onClick={() => void load(page + 1)}
                >
                  下一页
                </button>
              </div>
            </>
          )}
        </div>
      )}
      {error && (
        <p className="notice error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
