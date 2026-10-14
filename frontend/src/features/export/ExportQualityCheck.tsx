import { useEffect, useRef, useState } from 'react';
import { json, request } from '../../shared/api/client';
import type { ExportQualityReport } from '../../shared/api/types';

const explanations: Record<string, string> = {
  DUPLICATE_BOX: '两个框高度重叠，请人工确认。',
  TINY_BOX: '框有一边小于 4 原图像素，请检查是否误画。',
  LARGE_BOX: '人头框超过图片面积的 25%，请检查可见头部边界。',
};

export function ExportQualityCheck({
  videoId,
  batchId,
  blocked,
  onLocateFrame,
}: {
  videoId: string;
  batchId: string;
  blocked: boolean;
  onLocateFrame?: (frameId: string) => void;
}) {
  const [report, setReport] = useState<ExportQualityReport>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const active = useRef(true);
  const controller = useRef<AbortController>(undefined);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      controller.current?.abort();
    };
  }, []);

  async function check() {
    if (busy || blocked) return;
    setBusy(true);
    setError(undefined);
    setReport(undefined);
    controller.current = new AbortController();
    try {
      const result = await request<ExportQualityReport>('/exports/quality-check', {
        ...json({ videoId, batchId }),
        signal: controller.current.signal,
      });
      if (!Array.isArray(result.findings) || !Array.isArray(result.frameRevisions))
        throw new Error('质量检查结果格式不受支持，请重试。');
      if (active.current) setReport(result);
    } catch (cause) {
      if (active.current) setError((cause as Error).message);
    } finally {
      if (active.current) setBusy(false);
    }
  }

  return (
    <details className="export-quality">
      <summary>导出前质量检查（可选）</summary>
      <p className="muted small">启发式提醒可能误报或漏报，请结合原图判断。修改后需重新检查。</p>
      <button disabled={busy || blocked} onClick={() => void check()}>
        {busy ? '正在检查…' : '开始质量检查'}
      </button>
      {error && (
        <p role="alert" className="notice error">
          检查失败：{error}。仍可使用上方导出，服务端会执行原有数据校验。
        </p>
      )}
      {report && (
        <div className="quality-results" role="status" aria-live="polite">
          <p>
            检查快照：{report.checkedFrames} 张已完成图片、{report.checkedAnnotations} 个框
            （{report.rulesVersion}）。已发现 {report.totalFindings} 处提醒。
          </p>
          {report.truncated ? (
            <p className="notice conflict">检查未完整覆盖：已达到检查或结果数量上限。可缩小范围后复查。</p>
          ) : report.findings.length === 0 ? (
            <p>本次未发现上述可疑情况，仍需人工确认标注质量。</p>
          ) : null}
          <ul>
            {report.findings.map((finding, index) => (
              <li key={`${finding.frameId}:${finding.code}:${index}`}>
                <span>{explanations[finding.code] ?? '发现可疑标注，请人工检查。'}</span>
                <span className="muted small">框：{finding.annotationIds.join('、')}</span>
                {onLocateFrame ? (
                  <button onClick={() => onLocateFrame(finding.frameId)}>
                    定位图片 {finding.frameId}
                  </button>
                ) : (
                  <span className="small">图片：{finding.frameId}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </details>
  );
}
