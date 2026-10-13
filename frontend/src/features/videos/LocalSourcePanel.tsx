import { useEffect, useRef, useState } from 'react';
import { idPath, request } from '../../shared/api/client';
import type { Job, LocalSource } from '../../shared/api/types';
export function LocalSourcePanel({ onJob }: { onJob: (job: Job) => void }) {
  const [items, setItems] = useState<LocalSource[]>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const alive = useRef(true);
  const importKey = useRef<{ sourceId: string; key: string } | undefined>(undefined);
  async function load() {
    setBusy(true);
    setError(undefined);
    try {
      const result = await request<{ items: LocalSource[] }>('/sources/local');
      if (!Array.isArray(result.items)) throw new Error('本机目录暂不可用，请刷新重试。');
      if (alive.current) setItems(result.items);
    } catch (cause) {
      if (alive.current) {
        setItems(undefined);
        setError((cause as Error).message);
      }
    } finally {
      if (alive.current) setBusy(false);
    }
  }
  useEffect(() => {
    alive.current = true;
    void load();
    return () => {
      alive.current = false;
    };
  }, []);
  async function importSource(sourceId: string) {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    if (importKey.current?.sourceId !== sourceId)
      importKey.current = { sourceId, key: crypto.randomUUID() };
    try {
      const job = await request<Job>(`/sources/local/${idPath(sourceId)}/imports`, {
        method: 'POST',
        headers: { 'Idempotency-Key': importKey.current.key },
      });
      if (alive.current) {
        onJob(job);
        importKey.current = undefined;
      }
    } catch (cause) {
      if (alive.current) setError((cause as Error).message);
    } finally {
      if (alive.current) setBusy(false);
    }
  }
  return (
    <div role="tabpanel" aria-label="本机素材">
      <p className="muted">本机指平台所在的部署主机，不是浏览器电脑的目录。</p>
      <button disabled={busy} onClick={() => void load()}>
        刷新本机目录
      </button>
      {busy && <p role="status">正在读取或导入本机素材…</p>}
      {error && (
        <p role="alert" className="notice error">
          {error}
        </p>
      )}
      {items?.length === 0 && <p>目录中暂无授权视频。</p>}
      {items?.map((source) => (
        <article key={source.sourceId} className="source-card">
          <strong>{source.title}</strong>
          <p>
            {source.width > 0 && source.height > 0
              ? `${source.width} × ${source.height}`
              : '尺寸待解析'}{' '}
            ·{' '}
            {source.durationSeconds > 0 ? `${source.durationSeconds.toFixed(1)} 秒` : '时长待解析'}{' '}
            · {(source.sizeBytes / 1024 / 1024).toFixed(1)} MiB
          </p>
          <button
            className="primary full"
            disabled={busy}
            onClick={() => void importSource(source.sourceId)}
          >
            导入本机素材
          </button>
        </article>
      ))}
    </div>
  );
}
