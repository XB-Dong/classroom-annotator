# 当前系统架构

版本：2.0.0｜2026-10-14｜P1实现基线与P2迭代

浏览器 React/TypeScript/Vite/React Konva 经 Nginx 访问 Go/Gin，Go 是唯一业务API与数据入口，database/sql+pgx连接PostgreSQL17。数据库显式goose迁移，当前P1 schema为8；应用角色只有业务DML，迁移/bootstrap单独运行。SQLite仅是旧数据导入来源，不是当前业务库。P0历史架构保留在 [归档](archive/p0-architecture.md)。

Python/FastAPI/ONNX Runtime CPU是独立推理HTTP服务，不直接访问业务数据库。Go负责有界任务、候选存储、人工审核、revision CAS和导出门禁。预标注候选与正式标签隔离，过期revision结果不能覆盖人工数据。整文档使用指南由Go调用外部可配置HTTPS供应商，不发送媒体/任务/凭证，不执行业务工具。

媒体存当前部署主机的外部卷，Go/FFmpeg/PG/Python同机组成独立整栈。不依赖Mac桥接、relay或反向隧道。历史mac provider保持显式不可用，不猜测映射路径。上传经暂存/ffprobe，抽帧使用独立批次与原子发布，取消/重启清理未发布产物。

标注持久化采用事务expectedRevision CAS，冲突409且不部分写入。导出从repeatable-read快照读取已完成帧和正式框，保留显式完成零框图；媒体SHA/尺寸/候选审核/provenance仍必须验证。P2新增的只读质量提醒不替代这些硬门禁、不修改数据库。

前端按features分能力，保存控制与画布几何独立，账号/工作区/部署隔离草稿。P2优化自适应布局和进阶按需展开，不更换画布库或引入复杂UI框架。规格见 [P2](p2/acceptance.md)。

Compose包含Nginx/Go/PG/ONNX及Prometheus/Grafana/node exporter。监控仅loopback，数据库/AI仅容器内部。配置、权重、数据、日志、缓存、工作树均在代码外。arm64/amd64镜像及二进制必须匹配。生产状态与QA/本机结果只以具体版本报告为准。
