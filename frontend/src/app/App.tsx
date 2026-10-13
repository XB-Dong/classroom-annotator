import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { idPath, json, request } from '../shared/api/client';
import type { Batch, Frame, Job, Progress, Snapshot, Video } from '../shared/api/types';
import { ImportPanel } from '../features/videos/ImportPanel';
import { JobsPanel } from '../features/videos/JobsPanel';
import { ExtractionForm } from '../features/videos/ExtractionForm';
import { VideoLibrary } from '../features/videos/VideoLibrary';
import { BatchBar } from '../features/videos/BatchBar';
import { FramePicker } from '../features/annotation/FramePicker';
import { ScopedRequest } from '../shared/api/scopedRequest';
import type { SaveController, SaveState, SaveView } from '../features/annotation/saveController';
import { draftKey } from '../features/annotation/drafts';
import { AuthProvider, useAuth } from '../features/auth/AuthProvider';
import { AuthPage } from '../features/auth/AuthPage';
import { NotebookMark } from '../shared/ui/NotebookMark';
import { HelpPanel } from '../features/help/HelpPanel';
import { BatchReviewPanel } from '../features/review/BatchReviewPanel';
import { ReviewStatsPanel } from '../features/review/ReviewStatsPanel';
const Editor = lazy(() =>
  import('../features/annotation/Editor').then((module) => ({ default: module.Editor })),
);
import { ExportButton } from '../features/export/ExportButton';

export function App() {
  return (
    <AuthProvider>
      <AccountWorkspace />
    </AuthProvider>
  );
}
function AccountWorkspace() {
  const auth = useAuth();
  const [helpOpen, setHelpOpen] = useState(false);
  return (
    <div className={`app ${helpOpen ? 'help-open' : ''}`}>
      <header className="app-header">
        <div className="brand-mark">
          <NotebookMark />
        </div>
        <div>
          <h1>课堂人头标注</h1>
          <p>认真标注 · 清楚复核</p>
        </div>
        <div className="account-actions">
          {auth.session && (
            <div className="workspace-identity">
              <strong>{auth.session.user.displayName}</strong>
              <span>共享工作区 · {auth.session.user.role === 'admin' ? '管理员' : '标注成员'}</span>
            </div>
          )}
          <button
            aria-expanded={helpOpen}
            aria-controls="help-panel"
            onClick={() => setHelpOpen((value) => !value)}
          >
            使用指南
          </button>
          {auth.session && <button onClick={auth.logout}>退出登录</button>}
        </div>
      </header>
      <div className="app-content">
        {helpOpen && <HelpPanel close={() => setHelpOpen(false)} />}
        {auth.loading ? (
          <main className="empty" role="status">
            正在确认登录状态…
          </main>
        ) : auth.session ? (
          <Workspace key={`${auth.session.user.workspaceId}:${auth.session.user.id}`} />
        ) : (
          <AuthPage />
        )}
      </div>
    </div>
  );
}
function Workspace() {
  const { session } = useAuth();
  const draftScope = {
    deploymentOrigin: window.location.origin,
    workspaceId: session!.user.workspaceId,
    userId: session!.user.id,
  };
  const [videos, setVideos] = useState<Video[]>([]);
  const [videoId, setVideoId] = useState('');
  const [jobs, setJobs] = useState<Job[]>([]);
  const [libraryError, setLibraryError] = useState<string>();
  const [jobsError, setJobsError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [batches, setBatches] = useState<Batch[]>([]);
  const [batchId, setBatchId] = useState('');
  const [frames, setFrames] = useState<Frame[]>([]);
  const [frameId, setFrameId] = useState('');
  const [progress, setProgress] = useState<Progress>();
  const [batchError, setBatchError] = useState<string>();
  const [framesLoading, setFramesLoading] = useState(false);
  const [batchLoading, setBatchLoading] = useState(false);
  const [batchRefresh, setBatchRefresh] = useState(0);
  const [reviewRefresh, setReviewRefresh] = useState(0);
  const batchesVideo = useRef('');
  const controller = useRef<SaveController>(undefined);
  const [editorReady, setEditorReady] = useState(false);
  const [editorState, setEditorState] = useState<SaveState>('dirty');
  const [navigationBusy, setNavigationBusy] = useState(false);
  const navigationLock = useRef(false);
  const [navigationError, setNavigationError] = useState<string>();
  const progressRequests = useRef(new ScopedRequest());
  const scope = `${videoId}:${batchId}`;
  progressRequests.current.setScope(scope);
  const unresolvedDrafts = frames.filter((f) => {
    try {
      return localStorage.getItem(draftKey(f.id, draftScope)) !== null;
    } catch {
      return true;
    }
  }).length;
  const video = videos.find((v) => v.id === videoId);
  const frame = frames.find((f) => f.id === frameId);
  const activeFrame = useRef<Frame>(undefined);
  activeFrame.current = frame;
  const controllerChanged = useCallback((next?: SaveController) => {
    controller.current = next;
    setEditorReady(!!next);
  }, []);
  const editorChanged = useCallback((view: SaveView) => {
    setEditorState(view.state);
  }, []);
  async function navigate(action: () => void) {
    if (navigationLock.current) return;
    navigationLock.current = true;
    setNavigationBusy(true);
    try {
      if (activeFrame.current && (!controller.current || !(await controller.current.settle()))) {
        setNavigationError('当前帧尚未保存。请先重试保存或处理草稿冲突，再切换。');
        return;
      }
      setNavigationError(undefined);
      setEditorReady(false);
      action();
    } finally {
      navigationLock.current = false;
      setNavigationBusy(false);
    }
  }
  const loadVideos = useCallback(async () => {
    try {
      const result = await request<{ videos: Video[] }>('/videos');
      setVideos(result.videos);
      setLibraryError(undefined);
    } catch (e) {
      setLibraryError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void loadVideos();
    let active = true;
    async function poll() {
      try {
        const result = await request<{ jobs: Job[] }>('/jobs');
        if (active) {
          setJobs((previous) => {
            if (
              result.jobs.some(
                (j) =>
                  j.status === 'completed' &&
                  previous.find((p) => p.id === j.id)?.status !== 'completed',
              )
            ) {
              void loadVideos();
              setBatchRefresh((n) => n + 1);
            }
            return result.jobs;
          });
          setJobsError(undefined);
        }
      } catch (e) {
        if (active) setJobsError((e as Error).message);
      }
    }
    void poll();
    const timer = setInterval(() => void poll(), 2000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [loadVideos]);
  useEffect(() => {
    let active = true;
    if (batchesVideo.current !== videoId) {
      batchesVideo.current = videoId;
      setBatches([]);
      setBatchId('');
      setFrames([]);
      setFrameId('');
      setProgress(undefined);
    }
    setBatchError(undefined);
    if (!videoId) return;
    setBatchLoading(true);
    void request<{ batches: Batch[] }>(`/videos/${idPath(videoId)}/batches`)
      .then((r) => {
        if (active) {
          setBatches(r.batches);
          setBatchId((previous) =>
            r.batches.some((b) => b.id === previous) ? previous : (r.batches[0]?.id ?? ''),
          );
        }
      })
      .catch((e) => {
        if (active) setBatchError(e.message);
      })
      .finally(() => {
        if (active) setBatchLoading(false);
      });
    return () => {
      active = false;
    };
  }, [videoId, batchRefresh]);
  useEffect(() => {
    let active = true;
    setFrames([]);
    setFrameId('');
    setProgress(undefined);
    setBatchError(undefined);
    if (!videoId || !batchId) return;
    setFramesLoading(true);
    const query = `?batchId=${idPath(batchId)}`;
    void progressRequests.current
      .run(
        scope,
        () =>
          Promise.all([
            request<{ frames: Frame[] }>(`/videos/${idPath(videoId)}/frames${query}`),
            request<Progress>(`/videos/${idPath(videoId)}/progress${query}`),
          ]),
        ([r, p]) => {
          if (active) {
            setFrames(r.frames);
            setFrameId(r.frames[0]?.id ?? '');
            setProgress(p);
          }
        },
        (e) => {
          if (active) setBatchError((e as Error).message);
        },
      )
      .finally(() => {
        if (active) setFramesLoading(false);
      });
    return () => {
      active = false;
    };
  }, [videoId, batchId]);
  function addJob(job: Job) {
    setJobs((previous) => [job, ...previous.filter((j) => j.id !== job.id)]);
  }
  async function cancel(id: string) {
    try {
      addJob(await request<Job>(`/jobs/${idPath(id)}/cancel`, json({})));
      setJobsError(undefined);
    } catch (e) {
      setJobsError((e as Error).message);
    }
  }
  function saved(snapshot: Snapshot) {
    setReviewRefresh((value) => value + 1);
    setFrames((previous) => {
      const target = previous.find((f) => f.id === snapshot.frameId);
      if (!target || (target.revision === snapshot.revision && target.status === snapshot.status))
        return previous;
      return previous.map((f) =>
        f.id === snapshot.frameId
          ? { ...f, status: snapshot.status, revision: snapshot.revision }
          : f,
      );
    });
    if (videoId && batchId)
      void progressRequests.current.run(
        scope,
        () => request<Progress>(`/videos/${idPath(videoId)}/progress?batchId=${idPath(batchId)}`),
        setProgress,
        (e) => setBatchError((e as Error).message),
      );
  }
  return (
    <div className="app-layout">
      <aside className="video-sidebar">
        <ImportPanel
          onVideo={(v) => {
            setVideos((previous) => [v, ...previous.filter((x) => x.id !== v.id)]);
            void navigate(() => setVideoId(v.id));
          }}
          onJob={addJob}
        />
        <VideoLibrary
          videos={videos}
          videoId={videoId}
          loading={loading}
          error={libraryError}
          refresh={() => void loadVideos()}
          select={(id) => {
            if (id !== videoId) void navigate(() => setVideoId(id));
          }}
        />
        {video && <ExtractionForm key={video.id} video={video} onJob={addJob} />}
        <JobsPanel jobs={jobs} error={jobsError} cancel={(id) => void cancel(id)} />
      </aside>
      <main className="workspace">
        <div className="workspace-header">
          <div>
            <span className="eyebrow">
              标注工作台{' '}
              {frame ? ` / 帧 ${frames.findIndex((item) => item.id === frame.id) + 1}` : ''}
            </span>
            <h2>{video?.name || '准备开始标注'}</h2>
          </div>
          {videoId && batchId && progress && (
            <ExportButton
              videoId={videoId}
              batchId={batchId}
              completed={progress.completed}
              blocked={
                navigationBusy || !editorReady || editorState !== 'saved' || unresolvedDrafts > 0
              }
            />
          )}
        </div>
        {video ? (
          <>
            <BatchBar
              batches={batches}
              batchId={batchId}
              loading={batchLoading}
              busy={navigationBusy}
              progress={progress}
              select={(id) => {
                if (id !== batchId) void navigate(() => setBatchId(id));
              }}
            />
            {batchId && frames.length > 0 && (
              <div className="review-batch-bar">
                <BatchReviewPanel
                  key={batchId}
                  batchId={batchId}
                  frames={frames}
                  blocked={navigationBusy || !editorReady || editorState === 'conflict'}
                  beforeStart={async () =>
                    !!controller.current && (await controller.current.settle())
                  }
                  onJob={addJob}
                />
                <ReviewStatsPanel
                  key={`${videoId}:${batchId}`}
                  videoId={videoId}
                  batchId={batchId}
                  refresh={reviewRefresh}
                />
              </div>
            )}
            {batchError && (
              <div className="notice error" role="alert">
                {batchError}
              </div>
            )}
            {navigationError && (
              <div role="alert" className="notice error">
                {navigationError}
              </div>
            )}
            {unresolvedDrafts > 0 && (
              <div role="status" className="notice conflict">
                当前批次有 {unresolvedDrafts}{' '}
                帧草稿尚未保存或处理，导出已暂停。请打开这些帧完成保存或丢弃草稿。
              </div>
            )}
            {framesLoading ? (
              <div className="empty">正在加载批次图片…</div>
            ) : frames.length === 0 ? (
              <div className="empty">
                <div className="empty-icon">▧</div>
                <h3>还没有可标注的帧</h3>
                <p>创建抽帧批次，任务完成后会在这里显示。</p>
              </div>
            ) : (
              <div className="workbench">
                <FramePicker
                  frames={frames}
                  frameId={frameId}
                  busy={navigationBusy}
                  select={(id) => {
                    if (id !== frameId) void navigate(() => setFrameId(id));
                  }}
                />
                {frame && (
                  <Suspense fallback={<div className="empty">正在加载标注工具…</div>}>
                    <Editor
                      key={frame.id}
                      frame={frame}
                      draftScope={draftScope}
                      onSaved={saved}
                      onController={controllerChanged}
                      onStateChange={editorChanged}
                      onJob={addJob}
                    />
                  </Suspense>
                )}
              </div>
            )}
          </>
        ) : (
          <div className="empty intro">
            <div className="empty-icon">▧</div>
            <h3>从公共示例或自己的视频开始</h3>
            <p>选择素材 → 按间隔抽帧 → 标注学生人头 → 导出训练数据</p>
            <p className="muted">仅已完成帧会进入 COCO 图片包，零人头帧也需显式完成。</p>
          </div>
        )}
      </main>
    </div>
  );
}
