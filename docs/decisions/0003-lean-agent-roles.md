# 决策 0003：精简角色、极高推理与负载安排

版本：0.5.0｜确认日期：2026-10-13｜状态：用户已确认，运行配置尚未启用

依据：用户同意上一轮“Leader 与专项审核用 xhigh、开发测试默认 high、五个核心角色按依赖调度”的建议，要求明确写入计划。本决定替代 [决策 0002](0002-agent-model-allocation.md) 的角色/模型分配；决策 0001 的 14 项选择不变。本轮仅修改文档，不授权启动 P0。

## 现行角色与模型唯一权威表

| 角色 | 模型 / 调用方式 | 推理强度 | 职责与边界 |
| --- | --- | --- | --- |
| Leader | `gpt-6.1-sol` / OpenAI 主会话 | `xhigh`（极高） | 兼产品澄清；契约、调度、审核处置、集成、实际发布和最终验收/报告 |
| Frontend | `gpt-6.1-sol` / 原生子 Agent | `high` | 前端实现、对应组件/交互测试及前端部署准备 |
| Backend | `gpt-6.1-sol` / 原生子 Agent | `high` | 后端实现、对应单元/集成测试及后端部署准备 |
| Test / Acceptance | `gpt-6.1-sol` / 原生子 Agent | `high` | 独立验收、集成/E2E、证据索引、报告/材料草稿；Leader 核实最终结论 |
| GLM Reviewer/Experience | R0 公司 Key 实测后固定准确智谱 ID / 外部 OpenCode | 仅实际支持的供应商设置 | 独立代码审核及合成数据实际浏览器体验，Leader 复现并处置意见 |
| Architecture Reviewer，按需 | `gpt-6-astra` / 原生子 Agent | `xhigh`（极高） | 架构/核心契约、坐标与保存一致性、复杂跨模块问题；只读实现，写审核工件 |
| AI Integration，P1 起按需 | `gpt-6.1-sol` / 原生子 Agent | `high` | 推理适配、输出/坐标映射、失败降级；P0 不启用 |

P0 是五个核心职责加一个按需专项审核，不是六个同时运行的原生 Agent。当前原生容量始终为 Leader 加最多三个 Worker；Astra 和未来 AI Integration 都轮换占用 Worker 席位，外部 GLM 单独计入时间预算。角色不等于常驻进程，也不要求每轮启动全部角色。

取消独立 Product 与 Evidence Assistant；前者由 Leader 承担，后者由 Test 承担。不设置 Luna 执行角色。更高强度用于关键决策/专项分析，不自动将开发测试提升为 `xhigh/max/ultra`，不承诺仅靠强度即可保证质量。

## 调度与负载规则

1. 规格阶段：Leader 澄清产品与契约，Test 提前设计独立验收，Astra 审核底座/核心契约。前后端只做不依赖未冻结契约的准备。
2. 开发阶段：通常 Frontend、Backend、Test 三个 Worker 并行。开发者完成自身 TDD，独立 Test 不等到最后才介入。共享契约变更由 Leader 协调。
3. 后端拆包：视频导入/抽帧与标注保存/导出分开任务包；依赖、路径和席位允许时可并行两个 Backend Worker，仍不超过三个 Worker，也不取消独立验收。
4. 核心审核：R0 架构/契约，以及 R3/R4 坐标与保存一致性使用专项审核任务；复杂阻塞再按需调用。所有审核消耗原波次预算和席位，不以额外 Agent 逃避截止/修复次数。
5. 部署准备：Frontend 准备前端镜像/Nginx，Backend 准备后端镜像/FFmpeg；共享 Compose、CI、Prometheus/Grafana 配置由 Leader 分配单写 owner。Worker 准备、Test 核验、Leader 审核/合入并实际发布。
6. 独立审核：R0 只验证智谱工具链；R1–R5 分批审查完成工件并体验可用云端流程；R6 汇总意见并补齐实际云端核心浏览器体验，不能以早期工具演示替代产品验收。
7. 交付阶段：Test 整理已验证证据/矩阵/报告草稿，Leader 核实、签发并停止。原始证据由生成者保留，整理不覆盖、不补造。
8. 每轮按依赖、队列、阻塞、剩余时间决定席位并记录调度理由，不追求人均等工时。上述负载分析是执行前预估；实际负载在运行日志中评估。

## 配置、上下文与审计

R0 在明确启动后落地英文角色定义/启动参数，并最小验证可调用性。本轮不安装客户端、切换主会话或改全局配置。主会话单独选择 Sol `xhigh`；子 Agent 配置不能替它切换。不可核实的实际设置记为 `null` 和未确认，不用模型自述作为证据。

当前工具显式覆盖子 Agent 型号/推理强度使用 `fork_turns="none"`，通过自包含任务包、规格、记忆与交接加载上下文。完整历史继承不能用于显式覆盖。角色/模型切换后重读交接，不假设共享隐藏记忆。

分别记录 `requestedModel/requestedReasoningEffort` 与实际 `provider/model/reasoningEffort`，以及 `modelVerificationStatus/modelEvidenceRef/contextMode`；GLM 不支持的推理设置标记为 `unsupported`。准确智谱型号经公司 Key 与浏览器/工具能力实测后在波次内固定，不编造、不作为 Codex 原生模型。

选定型号或强度不可用时，记录阻塞并停止依赖任务，推进独立授权任务；不静默替换模型/客户端、降低验收或重置预算。确需改变本决定时留下用户确认的新决定。此前审计保留原型号未知/旧目标配置，不回填。

P1/P2 使用重点见 [后续波次前瞻](../../NEXT-WAVES.md)，不得据此自动启动。GPT/GLM 研发模型与产品人头检测 ONNX 模型分别管理。

参考：[官方模型/推理强度说明](https://learn.chatgpt.com/docs/models)、[模型选择指南](https://learn.chatgpt.com/docs/model-selection)、[Codex 子 Agent 配置](https://learn.chatgpt.com/docs/agent-configuration/subagents)。角色/负载分工是本项目工程决定，尚未构成实际运行证明。
