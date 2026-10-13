# P0.1 独立测试交接

更新时间：2026-10-13T18:16:00+08:00。状态：执行中，完整 API/浏览器/PG 门槛尚待最终实际验证。

角色：Test / Acceptance。请求配置 `gpt-6.1-sol/high`，独立上下文；实际 provider/model/reasoning 无独立可核验记录，均记 `null`，不以请求配置冒充实际配置。继承正式 Test 修复计数 `1/2`；本轮 Vitest 启动超时经 Leader 明确归为基础设施问题，不重置或增加产品修复额度。

产品二进制源码基线：`a88d7f1`；含 Source `7bca25c`、Media `49d6b58`、导入修复 `c7206f2`。测试增量 `b33fad7`。本次只修改 `tests/**`、统一检查脚本与 Verify workflow；最终产品修复待 Leader 同步后再验。

已交付统一入口：缺少 DSN 直接失败；验证五个独立 PostgreSQL 17、非管理员目标；构建外部二进制；运行真实 PG Go 测试、导入/回滚、CAS/跨帧竞争/100 次快照导出、断线恢复、后端重启、取消发布竞争、原生 Agent，以及原有 API/浏览器闭环。Linux CI 使用固定 PostgreSQL 17.11 镜像摘要、Go 1.25.5、Node 24.16.0 与既有 action pins；管理员只负责建立临时非管理员账户与独立数据库，凭证文件位于 runner.temp。

| 需求/检查 | 本轮实际证据 | 状态 |
| --- | --- | --- |
| PostgreSQL 17 目标、账户权限与隔离 | `node scripts/verify-databases.mjs`，五个外部配置文件，exit 0 | 通过 |
| 测试脚本语法与单测 | `npm run check`；最新 `npm test`，9/9，exit 0；公共出处 ZIP 严格检查仅允许且强制 credits/LICENSES，私有导出保持原严格规则 | 通过 |
| 前端 typecheck/lint | 首轮统一入口完成这两步，未输出错误 | 通过 |
| 前端 43 项单测 | `npm test -- --run --pool=threads --maxWorkers=1 --no-file-parallelism`，10 文件/43 项，exit 0 | 通过 |
| 前端生产构建 | `npm run build`，exit 0 | 通过 |
| Bash/PowerShell 语法、Git 空白 | `bash -n scripts/verify.sh`、PowerShell AST parse、`git diff --check`，exit 0 | 通过 |
| 完整统一入口 | 首轮 `verify.ps1 -E2E`，Vitest fork worker 启动超时，exit 1；未进入 Go/DB/API | 未通过：基础设施 |
| 五个二进制/原生 Agent | 构建 exit 0；Agent 精确 SHA 通过 ldflags 嵌入后，独立 Native Agent macro exit 0 | 通过 |
| Go vet/test | Go vet exit 0；当前全包测试仍运行，annotations 等已通过，尚未宣称整套通过 | 执行中 |
| P01-MAC-001/003/005/006 与 DATASET-001 合成适配 | 实际原生 Agent：9 媒体(2 video/7 image)、注册幂等/变化拒绝、20 秒视频真实 10 帧、206/416、真实进程重启/ack、原 JPEG/零头 COCO ZIP、公司 Key 隔离，exit 0 | Windows 合成通过；实际 Mac 未运行 |
| P01-SRC-001、DATASET-001、REG-001 API | 独立 PG+Native Agent+v1/company HTTP：旧完整 API 与四来源路由等共 10 项通过；整体 API/浏览器基线 6 失败/1 离线阶段跳过，exit 1 | 部分通过，整体待重跑 |
| P01-SRC-002/MAC-008 私有 bytes 不落 cloud 文件树 | harness 已实现 actual HTTP 后的 metadata/provider/hash 检查，但前置 runner 失败而未执行 | 待运行 |
| P01-SAMPLE-001/002/003 | 真实公共清单的登记/原 JPEG/credits/幂等测试、默认入口浏览器测试；实际停 Agent/重启 backend/重新 seed 后离线 API 闭环 | 待运行 |
| 实际 Mac / 云端 / GitHub CI / GLM 代码审核与浏览器体验 | 本角色未执行 | `not_run`，不冒称通过 |

外部证据根：`D:/Project/cvte/runtime/p01-test-v2/`。首轮：`acceptance-integrated-a88d7f1.txt`；单 fork 诊断：`frontend-single-worker.txt`（35 项通过但 1 个 Worker 超时，整体 exit 1）；threads 验证：`frontend-threads-single.txt`；构建：`frontend-build-a88d7f1.txt`。原生/PG/API：`acceptance-20261013T1755/`。`native-agent-check.txt` 首次因构建未嵌入版本而失败；`native-agent-check-v2.txt`/`native-agent-check-v2/mac-media-evidence.json` 是精确版本实际 Green。`full-route.txt`、`full-route-report`、`full-route-results` 保留首次 API/浏览器基线。

基线失败处置：旧尾帧测试曾要求本地 FFmpeg 的 `FRAME_UNAVAILABLE`，真实远端返回冻结契约允许的 `REMOTE_CLEANUP_PENDING`；只改为合法错误枚举，保留 failed/no published batch。公共 ZIP 增加 credits/LICENSES 导致旧独立 reader 拒绝；仅显式 public attribution 模式按新契约严格允许两项，原私有 ZIP 不变。四个浏览器启动失败因 Playwright 1.64 需要 revision1248、现有缓存为1223，尚未进行浏览器操作；精确版本外部下载正在进行。CLI 自动 GC 删除旧1223缓存的副作用已告知 Leader，后续本地安装须设置 `PLAYWRIGHT_SKIP_BROWSER_GC=1`。

配置使用外部 `p01-test-config/{go,check,acceptance,import_verify,rollback_verify}.url`；后四者由 Leader 明确授权本角色独占，go 只建立/清理独立临时 schema。不得覆盖 Leader 已完成的 `api` 基线或使用公司真实媒体。本地公共清单为 `runtime/p01-public-samples/catalog.json`，清单/媒体不进入 Git。

PostgreSQL 转发故障已由 Leader 在18:06恢复：云端容器健康，重启仅限他核验的自有 SSH PID；本角色未重启云端或其他测试服务。恢复后五目标 preflight 再次 exit 0。

后续：完成 PG 套件与全包 Go，精确浏览器安装后对最终 Source/Media 修复编译并在新独立 DB 重跑 API/浏览器/公共离线闭环。对真正失败先定位，再在既有修复预算内协调所有者。原有 draw/move/resize/delete/autosave/refresh/conflict/zero-head/original-JPEG/COCO 断言保留。本地合成与公共文件验证不能替代目标 Mac、云端断链浏览器与 GLM 门槛。
