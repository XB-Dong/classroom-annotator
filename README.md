# Classroom Annotator

教室人员标注平台：视频导入、抽帧、人头标注、人工审核与数据导出。

P0 已实现人工标注闭环，当前完成云端候选版本验收并进行交付核验。实际完成状态见 [P0 执行状态](docs/waves/P0/state.json) 和 [时间戳报告](docs/audit/reports/)。真实公司 API 尚未联调；材料驱动的合成 API 导入已验证。

- [项目宪法](CONSTITUTION.md)
- [P0计划](P0-PLAN.md)
- [后续波次前瞻：P1/P2](NEXT-WAVES.md)
- [14项确认记录](docs/decisions/0001-confirmed-choices.md)
- [Agent 角色、极高推理与负载安排](docs/decisions/0003-lean-agent-roles.md)
- [P0 本地材料验收范围](docs/decisions/0004-local-materials-acceptance.md)
- [Agent 执行规则](AGENTS.md)
- [时间戳报告](docs/audit/reports/)

单仓库保留三模块边界：P0 实现 React/TypeScript 前端与 Go 业务后端，Python 推理模块由后续波次实现。实现状态以报告和验证证据为准。

现有功能包括本地上传、固定间隔抽帧与独立批次、画框/移动/调整/删除、撤销重做、串行自动保存、草稿恢复与 revision 冲突、显式完成和进度、完成帧 COCO/JPEG ZIP 导出。AI 预标注属于 P1。

前后端分别见 [frontend](frontend/README.md) 和 [backend](backend/README.md)。[统一验收说明](tests/README.md)、[部署说明](deploy/README.md) 和 [云端验收/恢复步骤](deploy/ACCEPTANCE.md) 说明如何复做检查。统一入口为 `scripts/verify.ps1`（Windows）或 `bash scripts/verify.sh`（Linux）；源码外配置 FFmpeg、缓存和测试数据目录。

演示应用、Prometheus 和 Grafana 只绑定云端 loopback，通过授权 SSH 转发访问。本次 Windows 转发入口分别为 `http://127.0.0.1:8180`、`:9090`、`:3000`，并不是公开互联网地址。Grafana 密码只在仓库外配置，不提交 Git。真实课堂样本仅在授权 Windows 环境检查，云端及外部审核使用合成视频。
