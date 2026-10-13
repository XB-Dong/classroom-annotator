# P0.1 数据库实现交接

- 主体实现提交：`5ca26eb938f263ce5edc0f7fa687b45563da6734`，分支 `p01/db`，基线 `2414176`。本交接文档与末次收敛修复另见本分支下一提交。原工作树中前任未提交修改已保留并纳入主体提交；本次接手未丢弃其改动。
- 范围：`P01-DB-001` 至 `P01-DB-006` 的本地实现与合成数据验证；`P01-DB-007` 的实际 `pg_dump/restore`、生产备份导入、演示环境切换由后续独立验收/Leader 执行，当前未通过。
- 数据库：PostgreSQL 17 真实独立测试库、逐用例隔离 schema；goose 显式迁移至版本 4。`00003` 补齐公开样例名称/描述/许可 URL/本地 key，`00004` 保存媒体 manifest SHA。旧迁移文件未改。server 校验版本，不自动建表，ready 在数据库断开或 schema 过期时返回 503。
- 持久化：pgx 连接池默认 5/2，查询超时默认 10 秒；任务发布与取消通过 job 行锁协调；标注用 revision CAS、全局 ID 冲突返回 400；读与导出采用只读 repeatable-read 快照。`publishBatchTx` 与 `publishImportTx` 可供同包的媒体操作存储层在一个事务内复用，调用方负责提交。`jobs.ErrRemoteCleanupPending` 的包装错误在取消竞态后仍记录 `failed/REMOTE_CLEANUP_PENDING`，不伪装为 `cancelled`。
- 迁移：`cmd/import-sqlite` 只读打开停止写入后的 SQLite 备份，拒绝附带 WAL、损坏/未知 schema/外键错误；目标七张业务表加锁且必须全空；单事务按五张旧表导入、逐表按主键排序规范化 SHA-256 比对、备份前后 SHA 比对，失败回滚。CLI 仅输出摘要报告，不输出 DSN 或原始数据。SQLite 驱动未进入长期运行的 server 依赖图。
- 本地媒体恢复：保留已登记且 `cache_status=ready` 的公开样例本地文件，即使 `video_id` 尚为空；清理无引用本地媒体目录。Mac 引用不作为本地路径处理。

验证环境：Windows、Go 1.25.5、`P01_TEST_DATABASE_URL` 指向仅用于测试的 `p01_test_go` 数据库；数据库地址和凭证保存在仓库外，未写入本文。测试内建 schema 创建/清理，禁止任意数据库名。合成 SQLite 备份含五张业务表、零框完成帧、超过 32 位的时间戳/revision、亚像素坐标。已观察到进口器缺失行为的真实 RED，后续 GREEN；新增公开样例列与 manifest 列分别先出现 PostgreSQL 42703 再通过。数据库测试未使用 mock SQL。

验证记录（本地，Asia/Shanghai，最终源码 SHA 同上）：

| 检查 | 结果 |
| --- | --- |
| `go test -p 1 ./... -count=1`，真实 PostgreSQL | exit 0；末次新增七表非空拒绝后 `go test -p 1 ./internal/migration -count=1` 再次 exit 0 |
| `go vet -p 1 ./...` | exit 0（末次修改后重跑） |
| `go build -p 1 -o D:/Project/cvte/runtime/p01-db-binaries/ ./cmd/server ./cmd/migrate ./cmd/import-sqlite` | exit 0 |
| `go list -deps ./cmd/server` 检查 `modernc.org/sqlite` | 无匹配 |
| `git diff --cached --check` | exit 0 |

外部二进制 SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `server.exe` | `4E8C9773F8A16E88876295893B313612EE7077EA27310085A53041CF6A9BA083` |
| `migrate.exe`（R1 重建） | `F60DF6A7CE77CD7A921406C4281425D530E796ECAEF355823A66F19E03E670AE` |
| `import-sqlite.exe`（R1 修复后） | `FC3BDB11DA501A7C47E54C02C5D8A15371F47006536B8CE19B06B933B117BC78` |

## R1 独立审核修复（2026-10-13，数据库修复 1/2）

独立审核在主体提交上实测发现：仅 `mode=ro` 读取 WAL 模式的离线备份会新建 `-wal`/`-shm`；只核验列名会接受保留数据但丢失约束的 `CREATE TABLE AS SELECT`。本次修复使用分段 URL 转义构造 `file:` URI，指定 `mode=ro&immutable=1`，并在导入前后拒绝已有或新生的 WAL/SHM sidecar；仍要求来源是停止写入、已完成 checkpoint 的备份。来源结构现将 `sqlite_master` 的六张表和两条显式索引定义与冻结 P0 `backend/internal/platform/store/schema.sql` 的签名逐项核对，覆盖主键、外键、唯一、NOT NULL、CHECK 与索引。格式空白只在 SQL 字符串字面量之外忽略；改变 CHECK 字面量即拒绝。集成工作树的 `tests/fixtures/legacy-schema.sql` 以独立 SQLite 内存库生成的八项定义签名与 P0 schema 比对，结果一致。

审核的可选排序探针随后在目标库 `en_US.utf8` 实测发现合法 ID `B`/`b` 的来源和目标默认排序不同，导致摘要误判；本轮仅在导入 SQL 中固定 SQLite `COLLATE BINARY` 与 PostgreSQL `COLLATE "C"`，不改变 ID 或数值。测试中的超 32 位时间戳/revision、亚像素坐标继续逐字段一致。

验证：新增的 WAL sidecar、无约束表、特殊路径、预存 sidecar、不同数据库排序、CHECK 字面量六组行为测试均先在旧实现观察到预期失败，修复后 `go test -p 1 ./internal/migration -count=1` 在真实独立 PostgreSQL schema 上 exit 0；审核 overlay 的三项测试也 exit 0。`go vet -p 1 ./...` exit 0；`go build -p 1 -o D:/Project/cvte/runtime/p01-db-binaries/ ./cmd/server ./cmd/migrate ./cmd/import-sqlite` exit 0，表中为重建后的 SHA。R1 只改 `internal/migration/import.go`、`import_test.go` 与本文。Windows 实测特殊文件名为 `#/%`；`?` 文件名仅在非 Windows 分支测试。

后续：Leader 合入依赖 manifest 后，Source Worker 在同包新增 `public_samples.go`、`media_operations.go`，不要改此提交的既有 store 文件或已执行 SQL 迁移。独立 Test 角色须演练目标非空拒绝、备份/恢复到新库、版本 4 与服务二进制一致性、失联恢复、取消/发布竞争及完整 API/ZIP。实际生产 SQLite 备份导入和演示切换尚未执行。

模型记录：用户同意当前不可用角色临时请求 `gpt-6-sol` / `high`；实际 provider/model/reasoning effort 无可核实遥测，均记 `null`，`modelVerificationStatus=unverifiable`，不得把请求值当作实际值。
