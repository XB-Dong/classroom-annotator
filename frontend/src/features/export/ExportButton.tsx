import { useState } from 'react';
import { json, request } from '../../shared/api/client';
export function ExportButton({
  videoId,
  batchId,
  completed,
}: {
  videoId: string;
  batchId: string;
  completed: number;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  async function download() {
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
        disabled={busy || completed === 0}
        onClick={() => void download()}
      >
        {busy ? '正在打包…' : '↓ 导出当前批次 COCO ZIP'}
      </button>
      {error && (
        <span className="error-text" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}
