# P0.1 数据库规格与迁移方案

执行说明：用户已暂停P0.1并要求修订四类入口；0.2.0稿等待审核与明确恢复，不能继续开发或迁移。状态见docs/waves/P0.1/state.json。


版本：0.2.0｜状态：暂停后的修订稿，待审核，未实施｜日期：2026-10-13

## 1. 基线和确定选型

基线 `d848d3517cb985e68eea513424f56f75fc2cc1e0`：`backend/internal/platform/store/store.go` 用 `database/sql` + modernc SQLite，创建目录与 schema、连接数为 1；SQL 分散于 store、annotations、exports 三个能力边界。不是 ORM 项目。

R0修订：pgx v5.11.0已取得；goose v3.29.0要求Go1.26，改锁兼容当前Go1.25.5的v3.27.0，并以GOTOOLCHAIN=local完成获取。保留Go版本，不自动升级工具链。PostgreSQL17.11-bookworm镜像仍在核验，digest通过才锁定；依赖入go.mod/go.sum。不用latest、不暗中更换主版本。[goose v3.27官方go.mod](https://raw.githubusercontent.com/pressly/goose/v3.27.0/go.mod)、[pgx stdlib](https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib)。

业务 server 仅连接 PostgreSQL。一次性 `cmd/import-sqlite` 使用现有 pure-Go SQLite 驱动读取停止写入后的备份，不把 SQLite 引入长期运行的 server。沿用显式 repository SQL，不做字符串通用替换器，不重写领域模型或标注编辑器。

## 2. Schema 与兼容

- 原表 videos/jobs/batches/frames/annotations 的 ID、关联、状态、revision、坐标和存储 key 原值保留；schema_version 的旧版本是导入来源说明，不覆盖 goose 自己的迁移版本表。
- ID 和 created_at 保留 TEXT，避免本次同时改变 ID/时间 API 的表示和旧记录值。timestamp_ms/revision 使用 BIGINT；尺寸 INTEGER；坐标/时长/进度 DOUBLE PRECISION。有限数值仍在应用边界校验。
- 保留外键、复合 batch/video 关联、唯一 frame/batch/time、全局 annotation ID 唯一及状态 CHECK。新迁移不得用 `SELECT *` 依赖列位置。
- 初始迁移 `00001_initial.sql` 表达 P0 业务结构；`00002_media_refs.sql` 添加视频/帧 `storage_provider`（local/mac，默认 local）、`media_sha256` 与 `media_size_bytes`。已有 storage_key 保留；Mac 用不透明 key，不放绝对路径或带凭证 URL。
- 既有业务返回保持坐标、revision与状态语义，新增可见sourceKind/storageProvider供前端区分来源；内部storage_key/物理路径/凭证不返回。各入口availability由状态API按提供方和来源分别返回。
- 视频新增source_kind（public_sample/upload/mac/dataset/legacy，旧记录默认legacy），nullable source_id用于稳定登记去重；以provider+source_kind+source_id+media_sha256唯一。upload可无source_id，Mac sourceId不是公司catalogFileId。视频/帧storage_key唯一性改为provider+storage_key，已有值不变。
- 新增public_samples表管理有限预置sampleId、版本、作者/许可/页面、媒体摘要/元数据、缓存状态和可空video引用；不存视频bytes。样例seed/import按sampleId+version+hash幂等，不能重置已存在标注。source/provider映射为public_sample→local、upload/mac/dataset→mac，以CHECK和业务校验约束；legacy保持原provider。帧provider从所属video继承，不能由客户端覆盖。
- 新增 `media_operations` 表保存云端 job ID、remote operation ID、provider、关联资源和安全状态；不用它承载媒体 bytes。表中不存 API Key、SSH key、Token、请求体或原始公司响应。
- media_operations明确operation_type（registration/upload/extraction/dataset_import/archive）、nullable job_id/resource_id、request_fingerprint、publication_status（unpublished/committed）和publication_revision。它是业务提交标记，不是标注revision。发布资源/终态job/committed标记在同一事务提交；上传/注册/导出无需伪造business job。
- goose SQL 迁移随源码提交。`cmd/migrate` 显式执行，使用 PostgreSQL advisory lock 防止两个迁移命令并行；已执行文件不修改。server readiness 同时验证 DB ping 与预期 schema 版本，版本不符返回 503。

## 3. SQL 与事务必须改变的地方

| 基线行为 | PostgreSQL 设计 | 需求 |
| --- | --- | --- |
| `?` 参数、`INSERT OR IGNORE`、PRAGMA | 显式 `$1…$n`，`ON CONFLICT`，独立 SQL 迁移 | P01-DB-001 |
| `sum(status='completed')`、`max(progress,?)` | FILTER/CASE 聚合、`GREATEST` | P01-DB-001 |
| 单连接掩盖读写并发 | 连接池最多 5、空闲 2；配置校验与连接超时 | P01-DB-002 |
| 同帧读 revision 后写入 | 事务内 CAS 更新成功才替换框；零行更新为 409 | P01-DB-003 |
| 多次 SELECT 的标注/导出快照 | readonly REPEATABLE READ，结束事务后操作媒体 | P01-DB-004 |
| 发布前读 job 状态 | 同一事务锁 job 行，确认 running，发布并更新 completed | P01-DB-005 |
| 启动扫描本地目录并删未引用目录 | DB 恢复与媒体恢复分开；local 只扫本地，Mac 经受限操作核对 | P01-DB-005 |

PostgreSQL 默认 READ COMMITTED 不能保证多次读取看到同一快照，不能沿用“只要 BeginTx 就一致”的假设。只读标注与导出明确采用 REPEATABLE READ。[官方事务隔离说明](https://www.postgresql.org/docs/17/transaction-iso.html)。

保存竞态：两个请求 `expectedRevision=r`，一个完成 CAS 与框替换、revision=r+1，另一个返回 409 且不删除/写入任何框。框 ID 已被其他帧占用为 400；23505/23503 等 DB 细节不回传用户，也不能把所有约束失败都当成 revision 冲突。测试包含同时发生的跨帧 ID 竞争。

任务取消与发布用同一 job 行锁协调，谁先完成事务决定终态；不能出现 cancelled job 配 completed batch。startup 只恢复当前实例负责的未终态任务，不假装恢复旧子进程；Mac 中断清理见媒体规格。

## 4. 配置与部署边界

| 配置 | 规则 |
| --- | --- |
| `APP_DATABASE_URL` | 必填，仅外部环境；禁止打印完整 DSN，无 SQLite fallback |
| `APP_DB_MAX_OPEN_CONNS` / `APP_DB_MAX_IDLE_CONNS` | 默认 5/2，idle <= open |
| `APP_DB_CONNECT_TIMEOUT_SECONDS` | 默认 5，必须正数 |
| `APP_DB_QUERY_TIMEOUT_SECONDS` | 默认 10，关键 DB 操作用 Context |
| `APP_DB_CONN_MAX_LIFETIME_SECONDS` | 默认 1800 |
| `APP_DB_CONN_MAX_IDLE_SECONDS` | 默认 300 |
| `POSTGRES_IMAGE` / 外部卷与凭证 | digest 锁定，独立外部持久卷，DB 不映射公网端口 |

迁移账号持有 DDL 权限；server 使用只具业务 DML 权限的独立账号。角色密码与初始化配置放在源码外，测试使用独立角色/数据库。容器私有网络内可明确配置非 TLS DSN；跨主机连接必须校验 TLS，不把私有网络选择扩散到公网 DB。

初始 PostgreSQL 限额 512MiB，`shared_buffers=64MB`、`max_connections=20`；应用池最多 5。限额是起点，R0/R2/R5 记录 RSS、主机余量与 OOM。沿用现有监控，不因为新增 DB 一起扩大所有内存限额。

## 5. 一次性数据迁移

1. 在隔离副本先演练：停止写入后获得一致 SQLite 备份及媒体清单，计算备份 SHA；不能复制活跃的 SQLite 主文件而忽略 WAL。
2. 新 PostgreSQL 数据库执行迁移；只读打开备份，检查 SQLite integrity/外键，拒绝非预期结构。
3. 按 videos → jobs → batches → frames → annotations 顺序，用单目标事务导入 P0 业务值。目标业务表必须为空，非空直接拒绝；失败整体回滚。命令结果写外部记录，不存原始内容于公开日志。
4. 导入后比较每表行数、按主键排序的规范化字段内容 SHA-256、外键、状态分布、revision、坐标、零框完成帧及本地文件引用。REAL/DOUBLE 用同一有限 float64 序列化规则比较，不做隐式四舍五入。
5. 导入工具第二次运行到非空目标必须拒绝且不修改。实际迁移失败不回写源 SQLite；目标丢弃重建，不靠人工零散修补。
6. server 启动的中断状态恢复单独验收、单独记录变化；不把迁移前后状态变化藏进内容摘要校验。

演示切换：进入维护状态拒绝新写入/任务，等已有任务退出；停止旧 backend 后备份数据卷，导入新 DB。新实例先通过私有只读验收核对已存在框、revision、完成进度、JPEG/ZIP；正确后再开放写入。目标维护窗口 10 分钟，超过先恢复旧服务入口。受控合成写入/撤销仅在隔离副本验证，不能用无记录写入污染切换核对。

## 6. 备份与回滚

- 使用 `pg_dump -Fc` 保存逻辑备份，`pg_restore` 恢复到新独立数据库并通过同一验收；不能只检查命令 exit 0。[官方备份说明](https://www.postgresql.org/docs/17/backup-dump.html)。
- DB 备份不包含媒体。local 旧媒体单独保留原卷；Mac 备份不可变素材/帧 manifest 和文件，引用/尺寸/摘要可核对。
- **开放新写入前**：停止新实例，保留失败现场，恢复旧 backend+SQLite+原媒体卷及现有认证 overlay。
- **开放新写入后**：保留新 PG 和备份，回退到兼容当前 schema 的已验证 P0.1 镜像；不能把旧 SQLite 直接重新作为写入库。若暂无兼容镜像，进入维护只读/停止写入并报告，先修复或恢复新 PG 副本，不默默丢新标注。
- 不实施 Pg → SQLite 自动逆向迁移，不自动执行破坏性 down，不删除旧卷或 `down -v`。初始结构 down 只供全新测试库演练，不用作演示库回滚。

## 7. 必须的实际证据

真实 PostgreSQL 上的 store/annotation/export 集成；相同 revision 并发和跨帧 ID 竞争；同时保存期间导出的一致快照；取消与发布竞态；一次性导入的逐字段摘要；重启前后数据一致；pg_dump/restore 后 revision、进度、图片引用与 ZIP 一致；DB 故障 readiness=503 和安全错误。

不允许用 SQLite 单测、mock SQL 或仅连接成功替代上述门槛。需求编号与用例见 [验收矩阵](acceptance.md)。
