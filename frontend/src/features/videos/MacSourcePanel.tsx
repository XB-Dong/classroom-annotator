import { useEffect, useState } from 'react';
import { json, request } from '../../shared/api/client';
import type { Job, MacSource, MacSourcePage } from '../../shared/api/types';

export function MacSourcePanel({ available, reasonCode, onJob }: {
  available?: boolean;
  reasonCode?: string | null;
  onJob: (job: Job) => void;
}) {
  const [items, setItems] = useState<MacSource[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  async function load(cursor?: string) {
    setBusy(true);
    setError(undefined);
    try {
      const page = await request<MacSourcePage>(`/media-sources/mac${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`);
      if (page.contractVersion !== '1.0.0') throw new Error('Mac 素材目录版本不兼容。');
      setItems((previous) => cursor ? [...previous, ...page.items] : page.items);
      setNextCursor(page.nextCursor);
      setLoaded(true);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => { if (available === true && !loaded) void load(); }, [available, loaded]);
  async function register(sourceId: string) {
    setBusy(true);
    setError(undefined);
    try {
      onJob(await request<Job>('/media-sources/mac/imports', json({ sourceId })));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div role="tabpanel" aria-label="Mac素材">
      {available === false && !loaded && <p className="notice error" role="status">Mac 媒体服务暂时离线{reasonCode ? `（${reasonCode}）` : ''}，可稍后重试。</p>}
      <button disabled={busy} onClick={() => void load()}>刷新素材目录</button>
      {items.map((item) => (
        <article className="source-card" key={item.sourceId}>
          <strong>{item.name}</strong>
          <p className="muted small">{item.mediaKind === 'image' ? '图片（仅供查看）' : '视频'} · {item.width > 0 && item.height > 0 ? `${item.width}×${item.height}` : '尺寸未知'}{item.mediaKind === 'video' ? ` · ${item.durationSeconds && item.durationSeconds > 0 ? `${item.durationSeconds} 秒` : '时长未知'}` : ''}</p>
          {item.mediaKind === 'video' && <button disabled={busy || (available === false && !loaded)} onClick={() => void register(item.sourceId)}>登记素材</button>}
        </article>
      ))}
      {nextCursor && <button disabled={busy} onClick={() => void load(nextCursor)}>加载更多素材</button>}
      {loaded && items.length === 0 && <p className="muted small">没有可用素材。</p>}
      {error && <p className="notice error" role="alert">{error}</p>}
    </div>
  );
}
