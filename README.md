# Classroom Annotator

教室人员标注平台：视频导入、抽帧、人头标注、人工审核与数据导出。

P0 已交付人工标注闭环，并完成当前本地/合成材料范围的 14 项门槛。查看 [P0 交付报告](docs/audit/reports/report-20261013T032912+0800.md)、[执行状态](docs/waves/P0/state.json) 与 [验收清单](docs/waves/P0/acceptance.json)。模型配置偏差和 GLM 在核心操作结束后超时均如实记录。真实公司 API 尚未联调；材料驱动的合成 API 导入已验证。

- [项目宪法](CONSTITUTION.md)
- [P0计划](P0-PLAN.md)
- [P0.1计划：PostgreSQL、四素材入口与Mac存储（已暂停，修订待审核）](P0.1-PLAN.md)
- [公共示例与四类素材存储边界](specs/p0.1/sources-and-samples.md)
- [后续波次前瞻：P1/P2](NEXT-WAVES.md)
- [14项确认记录](docs/decisions/0001-confirmed-choices.md)
- [Agent 角色、极高推理与负载安排](docs/decisions/0003-lean-agent-roles.md)
- [P0 本地材料验收范围](docs/decisions/0004-local-materials-acceptance.md)
- [交付后真实样本云端许可与密码门禁](docs/decisions/0006-authorized-cloud-demo.md)
- [追加密码保护的公网演示入口](docs/decisions/0007-public-demo-entry.md)
- [最新排查与真实课堂云端展示报告](docs/audit/reports/report-20261013T090951+0800.md)
- [Agent 执行规则](AGENTS.md)
- [时间戳报告](docs/audit/reports/)

单仓库保留三模块边界：P0 实现 React/TypeScript 前端与 Go 业务后端，Python 推理模块由后续波次实现。实现状态以报告和验证证据为准。

现有功能包括本地上传、固定间隔抽帧与独立批次、画框/移动/调整/删除、撤销重做、串行自动保存、草稿恢复与 revision 冲突、显式完成和进度、完成帧 COCO/JPEG ZIP 导出。AI 预标注属于 P1。

前后端分别见 [frontend](frontend/README.md) 和 [backend](backend/README.md)。[统一验收说明](tests/README.md)、[部署说明](deploy/README.md) 和 [云端验收/恢复步骤](deploy/ACCEPTANCE.md) 说明如何复做检查。统一入口为 `scripts/verify.ps1`（Windows）或 `bash scripts/verify.sh`（Linux）；源码外配置 FFmpeg、缓存和测试数据目录。

演示应用原通过授权 SSH 转发访问；用户追加公网入口要求后，当前云端已新增带密码的 HTTP 80。具体主机地址与密码在仓库外，不在源码硬编码；普通 Chrome/Edge 使用账号 demo 登录。Windows SSH 入口 `http://127.0.0.1:8180` 仍保留，Prometheus/Grafana 仍绑定 loopback，转发端口为 `:9090`、`:3000`。Grafana 密码独立配置。P0 原验收云端使用合成视频；交付后新增 mentor 许可，当前云端已导入 `channel4-20s-redacted.mp4` 真实脱敏样本。外部 GLM 仍只获准代码/独立合成数据审核，不能访问真实媒体实例。见 [展示与门禁规格](specs/cloud-demo-access.md)。
