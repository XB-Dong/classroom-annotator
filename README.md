# Classroom Annotator

课堂视频的人头标注平台。三模块：React/TypeScript/Konva 前端、Go/Gin/PostgreSQL17 业务后端、Python/FastAPI/ONNX CPU 推理服务。同机媒体卷，人工标注可在 AI 故障时继续。

当前 **P2 已获用户授权启动**，重点是窗口伸缩布局、扩展前端测试、按需展开的进阶操作、导出前可疑框提醒、文档维护和约20分钟答辩材料。具体能力状态以 [P2状态](docs/waves/P2/state.json) 和时间戳报告为准，不把计划当实测。智能抽帧与跨帧传播本轮只做设计评估。

P1 最终状态仍为 `partial_closed_fast_path`，详见 [快速收束报告](docs/audit/reports/report-20261014T151947+0800.md)。公开素材 QA、Mac 本机验证和生产发布分别记录。未进行生产数据切换，准确率/召回率/节时比例没有真值测量。

## 阅读入口

- [P2计划](docs/plans/P2-PLAN.md) · [规格与验收](specs/p2/acceptance.md) · [任务包](docs/waves/P2/TASK-PACKET.md)
- [全部文档导航](docs/README.md) · [项目宪法](CONSTITUTION.md) · [Agent规则](AGENTS.md)
- [用户手册](docs/product/ANNOTATOR-GUIDE.zh.md) · [当前架构](specs/architecture.md) · [API](specs/contracts/api.md)
- [部署运行手册](docs/deployment/P1-RUNBOOK.zh.md) · [统一测试](tests/README.md) · [审计报告](docs/audit/reports)
- [P0历史计划](docs/plans/archive/P0-PLAN.md) · [P0.1历史计划](docs/plans/archive/P0.1-PLAN.md) · [P1历史计划](docs/plans/archive/P1-PLAN.md)

## 功能与运行边界

已有视频上传/公开示例/同机来源/可配置公司API、固定间隔抽帧与独立批次、人头框编辑/撤销重做、revision保存与冲突、注册登录、ONNX候选/风险复核、完成帧COCO/JPEG ZIP导出及整文档问答。具体环境配置与实际验证见报告；功能存在不等于公司真实API已联调或生产验收完成。

部署使用 Docker Compose、Nginx、FFmpeg、PostgreSQL17。配置、权重、素材、数据库、日志和构建缓存均置于仓库外。问答密钥只给 Go，浏览器/Python不接触。Mac的容器/代理/arm64实际前提见对应部署记录，不能照抄历史Windows代理或amd64工件。

统一验证入口：`bash scripts/verify.sh` 或 Windows `scripts/verify.ps1`。完整验证必须准备独立测试数据库、FFmpeg和外部运行目录，不能在生产库执行。各模块说明见 [frontend](frontend/README.md)、[backend](backend/README.md)、[ai-service](ai-service/README.md)。

此Mac现已使用用户在GitHub CLI授权的XB-Dong身份完成真实克隆并取回完整Git历史，核对远端main仍为033de6c。当前P2分支以该真实main为基线；此前临时快照提交保留为本地审计引用，原工作树与失败证据不丢失。推送/CI结果以P2报告记录为准。
