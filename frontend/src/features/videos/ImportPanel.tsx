import { useEffect, useState } from 'react';
import { catalogId, json, request } from '../../shared/api/client';
import type {
  DatasetPage,
  Job,
  MediaStorageStatus,
  SourceKind,
  Video,
} from '../../shared/api/types';
import { PublicSamplePanel } from './PublicSamplePanel';
import { LocalSourcePanel } from './LocalSourcePanel';
export function ImportPanel({
  onVideo,
  onJob,
}: {
  onVideo: (v: Video) => void;
  onJob: (j: Job) => void;
}) {
  const [tab, setTab] = useState<'local' | Exclude<SourceKind, 'mac' | 'local_directory'>>('public_sample');
  const [status, setStatus] = useState<MediaStorageStatus>();
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [data, setData] = useState<DatasetPage>();
  const [page, setPage] = useState(1);
  const [loaded, setLoaded] = useState(false);
  useEffect(() => {
    let active = true;
    void request<MediaStorageStatus>('/media-storage/status')
      .then((result) => {
        if (active) setStatus(result);
      })
      .catch(() => {
        if (active) setStatus(undefined);
      });
    return () => {
      active = false;
    };
  }, []);
  const sourceStatus = (kind: SourceKind) =>
    status?.sources?.find((source) => source.sourceKind === kind);
  async function refreshStatus() {
    try {
      setStatus(await request<MediaStorageStatus>('/media-storage/status'));
    } catch {
      setStatus(undefined);
    }
  }
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
      <div className="tabs source-tabs" role="tablist" aria-label="素材来源">
        {(
          [
            ['public_sample', '公共示例'],
            ['local', '本机素材'],
            ['upload', '本地上传'],
            ['dataset', '公司接口'],
          ] as const
        ).map(([kind, label]) => (
          <button
            key={kind}
            role="tab"
            aria-selected={tab === kind}
            className={tab === kind ? 'active' : ''}
            onClick={() => setTab(kind)}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === 'public_sample' && <PublicSamplePanel onVideo={onVideo} />}
      {tab === 'local' && <LocalSourcePanel onJob={onJob} />}
      {tab === 'upload' && (
        <div className="upload-area" role="tabpanel" aria-label="本地上传">
          {sourceStatus('upload')?.available === false && (
            <p className="notice error" role="status">
              当前部署主机的媒体服务暂不可用，上传暂不可用。{' '}
              <button onClick={() => void refreshStatus()}>重试连接</button>
            </p>
          )}
          <label className="file-label">
            选择本地视频
            <input
              data-testid="upload-file"
              type="file"
              accept="video/*"
              disabled={busy || sourceStatus('upload')?.available === false}
              onChange={(e) => setFile(e.target.files?.[0])}
            />
          </label>
          {file && (
            <p className="muted small">
              已选择：<span>{file.name}</span>
            </p>
          )}
          <p className="muted small">上传后可设置间隔，创建独立抽帧批次。</p>
          <button
            className="primary full"
            data-testid="upload-submit"
            disabled={!file || busy || sourceStatus('upload')?.available === false}
            onClick={() => void upload()}
          >
            {busy ? '正在上传…' : '上传视频'}
          </button>
        </div>
      )}
      {tab === 'dataset' && (
        <div role="tabpanel" aria-label="公司接口">
          {sourceStatus('dataset')?.available === false && (
            <p className="notice error" role="status">
              公司接口暂不可用（{sourceStatus('dataset')?.reasonCode ?? '服务离线'}），可重试加载。
            </p>
          )}
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
