import { useEffect, useState } from 'react';
import { json, request } from '../../shared/api/client';
import type { PublicSample, Video } from '../../shared/api/types';

export function PublicSamplePanel({ onVideo }: { onVideo: (video: Video) => void }) {
  const [samples, setSamples] = useState<PublicSample[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string>();
  const [error, setError] = useState<string>();
  async function load() {
    setLoading(true);
    setError(undefined);
    try {
      const result = await request<{ items: PublicSample[] }>('/samples');
      setSamples(result.items);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);
  async function register(sampleId: string) {
    setBusyId(sampleId);
    setError(undefined);
    try {
      onVideo(await request<Video>('/samples/imports', json({ sampleId })));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusyId(undefined);
    }
  }
  return (
    <div role="tabpanel" aria-label="公共示例">
      <p className="muted small">选择已缓存的公开视频，登记后可创建抽帧批次。</p>
      {loading && <p className="muted small">正在加载示例…</p>}
      {!loading && samples.length === 0 && <p className="muted small">暂无已登记的公共示例。</p>}
      {samples.map((sample) => (
        <article className="source-card" key={sample.sampleId}>
          <strong>{sample.name}</strong>
          <p className="muted small">{sample.description}</p>
          <p className="muted small">{sample.width}×{sample.height} · {sample.durationSeconds} 秒 · {sample.creator}</p>
          <p className="source-links">
            <a href={sample.sourcePageUrl} target="_blank" rel="noreferrer">素材出处</a>
            <a href={sample.licenseUrl} target="_blank" rel="noreferrer">{sample.licenseName}</a>
          </p>
          {sample.availability === 'unavailable' && <p className="notice error">示例尚未缓存或暂不可用。</p>}
          <button className="primary full" disabled={sample.availability !== 'ready' || !!busyId} onClick={() => void register(sample.sampleId)}>
            {busyId === sample.sampleId ? '正在登记…' : '开始标注'}
          </button>
        </article>
      ))}
      {error && <p className="notice error" role="alert">{error} <button onClick={() => void load()}>重试</button></p>}
    </div>
  );
}
