# 决策 0002：Agent 模型与推理强度分配

版本：0.4.0｜确认日期：2026-10-13｜状态：用户已确认，运行配置尚未启用

依据：用户要求“按照你的推荐思路帮我明确”，接受上一轮按角色设定默认模型、按波次区分任务重点的建议。本决定补齐决策 0001，不改变其 14 项选择，也不授权启动 P0。

## 固定角色配置

本表是模型配置的唯一权威来源。计划中的摘要必须与本表一致；更改型号或推理强度需记录新决定，不静默替换。

| 角色 | Provider / 调用方式 | 目标模型 ID | 推理强度 | 责任与启用范围 |
| --- | --- | --- | --- | --- |
| Leader | OpenAI / 主会话 | `gpt-6.1-sol` | `high` | 所有波次的协调、审核处置、集成与验收 |
| Architecture Reviewer | OpenAI / Codex 原生子 Agent | `gpt-6-astra` | `high` | 架构/核心契约审核、复杂跨模块问题；按任务启用 |
| Product | OpenAI / Codex 原生子 Agent | `gpt-6.1-sol` | `medium` | 流程、边界与验收例子 |
| Frontend | OpenAI / Codex 原生子 Agent | `gpt-6.1-sol` | `high` | 标注交互、状态与保存恢复 |
| Backend | OpenAI / Codex 原生子 Agent | `gpt-6.1-sol` | `high` | 视频/抽帧、持久化、导出与业务集成 |
| Test | OpenAI / Codex 原生子 Agent | `gpt-6.1-sol` | `high` | 测试设计、独立验证与缺陷复现 |
| AI Integration | OpenAI / Codex 原生子 Agent | `gpt-6.1-sol` | `high` | P1 起的推理适配、坐标转换与降级；P0 不启用 |
| Evidence Assistant | OpenAI / Codex 原生子 Agent | `gpt-6-luna` | `high` | 已验证证据索引/材料/报告初稿；不能判定验收或补造证据 |
| GLM Reviewer/Experience | Zhipu / 外部 OpenCode | R0 公司 Key 实测后固定准确 ID | 按实际支持设置，未支持则明确记录 | 每个交付波次独立代码审核及合成数据浏览器体验 |

GLM 的准确型号仍是可用性事实检查，不凭额度充足推定权限或工具兼容。R0 记录可用型号、选择依据、实际调用成功与浏览器证据；选定后在本波次固定。没有实测就不能声称审核已完成。

## 波次与资源原则

- P0 重点是工程底座与人工闭环；Sol 承担实现/测试，Astra 审核架构、坐标和保存一致性。
- P1 才增加 AI Integration，Astra 审核输出映射、效果评估方法和失败降级。P2 由 Sol 扩展回归/修复，Luna 整理材料，Leader 核实最终结论。具体未来任务见 [后续波次前瞻](../../NEXT-WAVES.md)。
- 智谱提供跨供应商独立意见；Leader 必须复现、核对规格再决定，不能用不同模型的投票代替证据。
- 所有原生角色共用 Leader 加最多三个 Worker 的容量，专项角色轮换占位。外部智谱执行也消耗波次时间，不虚报并发。
- 不为展示模型差异而频繁切换，不自动提升至 `max/ultra`，不因换模型重置任务预算。

## 执行与审计

1. R0 在明确启动后落地项目专用英文角色定义/启动参数，并验证选定模型是否实际可调用。本轮不安装客户端或修改全局配置。
2. Leader 主会话单独选择 `gpt-6.1-sol` / `high`；子 Agent 设置不能改变 Leader。若运行接口不能核实主会话配置，实际字段记为未确认，启动依赖检查仍未满足。
3. 当前子 Agent 工具指定不同模型/推理强度必须用 `fork_turns="none"`；以自包含任务包、规格、记忆与交接构造上下文。完整历史继承模式不能用于显式覆盖。
4. 任务和事件分别记录 `requestedModel/requestedReasoningEffort` 与实际 `provider/model/reasoningEffort`，以及 `modelVerificationStatus/modelEvidenceRef/contextMode`。实际值无法核实用 `null` 并说明原因；供应商不支持的推理控制标记为 `unsupported`。不记录内部推理。
5. 型号不可用时记录阻塞、停止依赖任务，继续独立任务；不静默改型号、客户端或验收。确需改变本决定时，由用户确认并留下新决策。
6. 角色/模型变更后重读任务包、记忆与交接；上下文不会因角色名称自动共享。过往继承配置、未确认型号的审计保持原样，不回填新选择。

已确认的目标配置不构成实际执行证明。GPT/GLM 是研发模型；P1 人头检测 ONNX 权重是产品推理模型，两者分别管理。

参考：[Codex 子 Agent 配置](https://learn.chatgpt.com/docs/agent-configuration/subagents)、[官方模型选择说明](https://learn.chatgpt.com/docs/models)。本项目的角色分配是工程决定，不是模型效果保证。
