# P0.1 独立测试交接

更新时间：2026-10-13T18:01:00+08:00。状态：执行中，完整 API/浏览器/PG 门槛尚待实际验证。

角色：Test / Acceptance。请求配置 `gpt-6.1-sol/high`，独立上下文；实际 provider/model/reasoning 无独立可核验记录，均记 `null`，不以请求配置冒充实际配置。继承正式 Test 修复计数 `1/2`；本轮 Vitest 启动超时经 Leader 明确归为基础设施问题，不重置或增加产品修复额度。

测试源码基线：`a88d7f1`；含 Source `7bca25c`、Media `49d6b58`、导入修复 `c7206f2`。本次只修改 `tests/**`、统一检查脚本与 Verify workflow。

已交付统一入口：缺少 DSN 直接失败；验证五个独立 PostgreSQL 17、非管理员目标；构建外部二进制；运行真实 PG Go 测试、导入/回滚、CAS/跨帧竞争/100 次快照导出、断线恢复、后端重启、取消发布竞争、原生 Agent，以及原有 API/浏览器闭环。Linux CI 使用固定 PostgreSQL 17.11 镜像摘要、Go 1.25.5、Node 24.16.0 与既有 action pins；管理员只负责建立临时非管理员账户与独立数据库，凭证文件位于 runner.temp。

| 需求/检查 | 本轮实际证据 | 状态 |
| --- | --- | --- |
| PostgreSQL 17 目标、账户权限与隔离 | `node scripts/verify-databases.mjs`，五个外部配置文件，exit 0 | 通过 |
| 测试脚本语法与单测 | `npm run check`；`npm test`，8/8，exit 0 | 通过 |
| 前端 typecheck/lint | 首轮统一入口完成这两步，未输出错误 | 通过 |
| 前端 43 项单测 | `npm test -- --run --pool=threads --maxWorkers=1 --no-file-parallelism`，10 文件/43 项，exit 0 | 通过 |
| 前端生产构建 | `npm run build`，exit 0 | 通过 |
| Bash/PowerShell 语法、Git 空白 | `bash -n scripts/verify.sh`、PowerShell AST parse、`git diff --check`，exit 0 | 通过 |
| 完整统一入口 | 首轮 `verify.ps1 -E2E`，Vitest fork worker 启动超时，exit 1；未进入 Go/DB/API | 未通过：基础设施 |
| Go vet/test 与五个二进制 | PostgreSQL 转发故障后按 Leader 指示停止自己的 vet，未进入测试 | 暂停 |
| P01-SRC-001/002、DATASET-001、MAC-002/006/008、REG-001 | 独立 PG + 真实 Native Agent + v1/company 合成 HTTP + 前端流式代理 harness 已实现；包含所有来源 metadata/provider 与私有 bytes 不落 cloud 文件树核验 | 待运行 |
| P01-SAMPLE-001/002/003 | 真实公共清单的登记/原 JPEG/credits/幂等测试、默认入口浏览器测试；实际停 Agent/重启 backend/重新 seed 后离线 API 闭环 | 待运行 |
| 实际 Mac / 云端 / GitHub CI / GLM 代码审核与浏览器体验 | 本角色未执行 | `not_run`，不冒称通过 |

外部证据根：`D:/Project/cvte/runtime/p01-test-v2/`。首轮：`acceptance-integrated-a88d7f1.txt`；单 fork 诊断：`frontend-single-worker.txt`（35 项通过但 1 个 Worker 超时，整体 exit 1）；threads 验证：`frontend-threads-single.txt`；构建：`frontend-build-a88d7f1.txt`。保留原始失败证据。

配置使用外部 `p01-test-config/{go,check,acceptance,import_verify,rollback_verify}.url`；后四者由 Leader 明确授权本角色独占，go 只建立/清理独立临时 schema。不得覆盖 Leader 已完成的 `api` 基线或使用公司真实媒体。本地公共清单为 `runtime/p01-public-samples/catalog.json`，清单/媒体不进入 Git。

后续：等待 PostgreSQL 协议健康通知；执行新版统一入口或逐段同等命令并保留准确 exit/源码 SHA；对真正失败先定位，再在既有修复预算内协调所有者。原有 draw/move/resize/delete/autosave/refresh/conflict/zero-head/original-JPEG/COCO 断言保留。本地合成与公共文件验证不能替代目标 Mac、云端断链浏览器与 GLM 门槛。
