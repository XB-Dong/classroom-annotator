import { useState } from 'react';
import type { Candidate, ReviewDecision, ReviewSnapshot } from '../../shared/api/types';
import type { Box } from '../annotation/geometry';
export const riskLabels: Record<string, string> = {
  LOW_CONFIDENCE: '置信度较低',
  SMALL_HEAD: '人头过小',
  CLIPPED_BOX: '边缘框被裁剪',
  POSSIBLE_DUPLICATE: '候选可能重复',
  OVERLAP_EXISTING: '与已有标注重叠',
  TILE_BOUNDARY: '切片边界',
  NO_DETECTIONS: 'AI 未检出候选，请人工检查整图',
  AI_FAILED: 'AI 失败，请转人工检查整图',
};
const runLabels = {
  queued: '排队中',
  running: '推理中',
  completed: '推理完成 · 等待人工复核',
  stale: '结果已过期，请重新生成',
  failed: 'AI 失败，可继续人工标注',
  cancelled: 'AI 已取消',
};
export function ReviewPanel({
  review,
  selected,
  adjustedBox,
  busy,
  disabled,
  select,
  decide,
  resolve,
  generate,
}: {
  review?: ReviewSnapshot;
  selected?: string;
  adjustedBox?: Box;
  busy: boolean;
  disabled: boolean;
  select: (candidate: Candidate) => void;
  decide: (decisions: ReviewDecision[]) => void;
  resolve: (id: string) => void;
  generate: () => void;
}) {
  const [filter, setFilter] = useState<'pending' | 'risk' | 'all'>('pending');
  const candidates = review?.candidates ?? [];
  const pending = candidates.filter((c) => c.reviewState === 'pending');
  const safe = pending.filter((c) => c.riskReasons.length === 0);
  const current = pending.find((c) => c.id === selected);
  const overlap = current?.riskReasons.includes('OVERLAP_EXISTING');
  const locked = busy || disabled;
  const running = review?.currentRun && ['queued', 'running'].includes(review.currentRun.status);
  return (
    <section className="review-panel" aria-label="AI 复核">
      <div className="panel-title">
        <h2>AI 复核</h2>
        <span className="count">待审 {pending.length}</span>
      </div>
      <p className="muted">候选虚线，正式框实线。所有候选都需人工决定；置信度不代表准确率。</p>
      <p role="status">
        {review?.currentRun ? runLabels[review.currentRun.status] : '尚未生成 AI 候选'}
      </p>
      {review?.currentRun && (
        <details>
          <summary>模型与运行信息</summary>
          <p>
            {review.currentRun.modelId} · {review.currentRun.pipelineVersion}
          </p>
          <p className="model-hash">SHA256 {review.currentRun.modelSHA256}</p>
        </details>
      )}
      <button
        className="full"
        disabled={
          locked || !!running || pending.length > 0 || !review || review.status === 'completed'
        }
        onClick={generate}
      >
        {review?.currentRun ? '重新生成本帧 AI 候选' : '生成本帧 AI 候选'}
      </button>
      {pending.length > 0 && <p className="muted">先处理所有待审候选，再重新生成。</p>}
      {(review?.issues ?? [])
        .filter((i) => !i.resolved)
        .map((issue) => (
          <div className="notice conflict" key={issue.id}>
            <p>{riskLabels[issue.reasonCode] ?? issue.reasonCode}</p>
            <button disabled={locked || !!adjustedBox} onClick={() => resolve(issue.id)}>
              已人工检查本图
            </button>
          </div>
        ))}
      <label>
        候选筛选
        <select value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)}>
          <option value="pending">待审核</option>
          <option value="risk">待审核高风险</option>
          <option value="all">全部决策</option>
        </select>
      </label>
      <div className="candidate-list">
        {candidates
          .filter(
            (c) =>
              filter === 'all' ||
              (c.reviewState === 'pending' && (filter !== 'risk' || c.riskReasons.length > 0)),
          )
          .map((candidate, index) => (
            <button
              key={candidate.id}
              data-testid={`candidate-${candidate.id}`}
              className={`candidate-item ${candidate.id === selected ? 'selected' : ''} ${candidate.riskReasons.length ? 'risky' : ''}`}
              onClick={() => select(candidate)}
            >
              <strong>
                候选 {index + 1} · {(candidate.confidence * 100).toFixed(1)}%
              </strong>
              <span>
                {candidate.reviewState === 'pending'
                  ? '待审核'
                  : candidate.reviewState === 'accepted'
                    ? '已接受'
                    : candidate.reviewState === 'adjusted'
                      ? '已调整'
                      : '已拒绝'}
              </span>
              <span>
                {candidate.riskReasons.map((r) => riskLabels[r] ?? r).join(' · ') ||
                  '无已知风险，仍需复核'}
              </span>
            </button>
          ))}
      </div>
      <div className="review-actions">
        <button
          disabled={locked || !current || !!overlap || !!adjustedBox}
          onClick={() => current && decide([{ candidateId: current.id, decision: 'accepted' }])}
        >
          接受选中候选
        </button>
        <button
          disabled={locked || !current || !!overlap || !adjustedBox}
          onClick={() =>
            current &&
            adjustedBox &&
            decide([{ candidateId: current.id, decision: 'adjusted', bbox: adjustedBox }])
          }
        >
          调整后接受
        </button>
        <button
          className="danger"
          disabled={locked || !current}
          onClick={() =>
            current &&
            decide([
              {
                candidateId: current.id,
                decision: 'rejected',
                ...(overlap
                  ? { reasonCode: 'ALREADY_LABELED' }
                  : { reasonCode: 'NOT_STUDENT_HEAD' }),
              },
            ])
          }
        >
          拒绝选中候选
        </button>
      </div>
      {current && (
        <p className="muted">
          {overlap
            ? '已有标注重叠：请拒绝为“已标注”，避免重复。'
            : '选中候选后，可拖动或调整虚线框，再点击调整后接受。'}
        </p>
      )}
      {adjustedBox && (
        <p className="notice conflict">
          候选调整尚未提交。点击“调整后接受”保存；重新选择候选会取消调整预览。
        </p>
      )}
      <button
        className="full"
        disabled={locked || safe.length === 0 || !!adjustedBox}
        onClick={() => decide(safe.map((c) => ({ candidateId: c.id, decision: 'accepted' })))}
      >
        接受无风险待审框
      </button>
      <p className="muted">仅接受 {safe.length} 个无风险待审框；高风险必须逐项复核。</p>
    </section>
  );
}
