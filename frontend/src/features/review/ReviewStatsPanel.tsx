import { useEffect, useState } from 'react';
import { idPath, request } from '../../shared/api/client';
import type { ReviewStats } from '../../shared/api/types';
export function ReviewStatsPanel({
  videoId,
  batchId,
  refresh,
}: {
  videoId: string;
  batchId: string;
  refresh: number;
}) {
  const [stats, setStats] = useState<ReviewStats>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    let active = true;
    void request<ReviewStats>(`/videos/${idPath(videoId)}/review-stats?batchId=${idPath(batchId)}`)
      .then((result) => {
        if (active) {
          setStats(result);
          setError(undefined);
        }
      })
      .catch((cause) => {
        if (active) {
          setStats(undefined);
          setError((cause as Error).message);
        }
      });
    return () => {
      active = false;
    };
  }, [videoId, batchId, refresh]);
  const rate = (value: number | null | undefined) =>
    value == null ? '无样本' : `${(value * 100).toFixed(1)}%`;
  return (
    <details className="review-stats">
      <summary>
        本批次复核统计
        {stats
          ? ` · 已决策 ${stats.accepted + stats.adjusted + stats.rejected}/${stats.candidates}`
          : ''}
      </summary>
      {error && (
        <p role="alert" className="notice error">
          复核统计暂不可用：{error}
        </p>
      )}
      {stats?.rates && (
        <>
          <progress
            max={stats.candidates || 1}
            value={stats.accepted + stats.adjusted + stats.rejected}
          />
          <div className="review-stat-grid">
            <span>待审 {stats.pending}</span>
            <span>未解风险框 {stats.unresolvedRiskCandidates}</span>
            <span>未解图片风险 {stats.unresolvedRiskImages}</span>
            <span>人工补框 {stats.manualAdded}</span>
            <span>接受 {stats.accepted}</span>
            <span>调整 {stats.adjusted}</span>
            <span>拒绝 {stats.rejected}</span>
            <span>失败图片 {stats.failedImages}</span>
          </div>
          <p>
            无需调整通过率 {rate(stats.rates.acceptedUnchanged)} · 修改率{' '}
            {rate(stats.rates.adjusted)} · 拒绝率 {rate(stats.rates.rejected)}
          </p>
          <p>
            框可疑率 {rate(stats.rates.suspiciousCandidates)} · 图片可疑率{' '}
            {rate(stats.rates.suspiciousImages)} · 复核进度 {rate(stats.rates.reviewed)}
          </p>
          <p className="muted">
            通过、修改、拒绝率分母为已决策候选；框可疑率与复核进度分母为当前候选总数，图片可疑率分母为有效推理图片。统计只计当前运行；这是人工审核结果，检测准确率未评估。
          </p>
        </>
      )}
    </details>
  );
}
