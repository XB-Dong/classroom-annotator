import { useState } from 'react';
import { json, request } from '../../shared/api/client';
import { ExportQualityCheck } from './ExportQualityCheck';
import './quality.css';
export function ExportButton({
  videoId,
  batchId,
  completed,
  blocked = false,
  revisionKey = '',
  onLocateFrame,
}: {
  videoId: string;
  batchId: string;
  completed: number;
  blocked?: boolean;
  revisionKey?: string;
  onLocateFrame?: (frameId: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  async function download() {
    if (blocked) return;
    setBusy(true);
    setError(undefined);
    try {
      const blob = await request<Blob>('/exports', json({ videoId, batchId }));
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `annotations-${batchId}.zip`;
      anchor.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="export-control">
      <button
        data-testid="export-zip"
        className="primary"
        disabled={busy || blocked || completed === 0}
        onClick={() => void download()}
      >
        {busy ? '正在打包…' : '↓ 导出当前批次 COCO ZIP'}
      </button>
      {error && (
        <span className="error-text" role="alert">
          {error}
        </span>
      )}
      <ExportQualityCheck
        key={`${videoId}:${batchId}:${revisionKey}`}
        videoId={videoId}
        batchId={batchId}
        blocked={busy || blocked || completed === 0}
        onLocateFrame={onLocateFrame}
      />
    </div>
  );
}
