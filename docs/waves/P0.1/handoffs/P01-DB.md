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
| `migrate.exe` | `7D64C6239FA09F90BACC0E95899D2735713698D48E84F6A43E83C6A33ECA3351` |
| `import-sqlite.exe` | `B9B3878FA26864A0D1F1015979490951C41FBE616B2228CEFB52F38CBDC4C22D` |

后续：Leader 合入依赖 manifest 后，Source Worker 在同包新增 `public_samples.go`、`media_operations.go`，不要改此提交的既有 store 文件或已执行 SQL 迁移。独立 Test 角色须演练目标非空拒绝、备份/恢复到新库、版本 4 与服务二进制一致性、失联恢复、取消/发布竞争及完整 API/ZIP。实际生产 SQLite 备份导入和演示切换尚未执行。

模型记录：用户同意当前不可用角色临时请求 `gpt-6-sol` / `high`；实际 provider/model/reasoning effort 无可核实遥测，均记 `null`，`modelVerificationStatus=unverifiable`，不得把请求值当作实际值。
