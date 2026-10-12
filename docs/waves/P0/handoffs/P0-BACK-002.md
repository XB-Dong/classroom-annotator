# P0-BACK-002 交接

状态：本能力包实现与本地验证完成；整体 P0 验收由 Leader 决定。
交接时间：2026-10-13T02:23:00.4910834+08:00。工作目录：`D:/Project/cvte/worktrees/p0-annotations`，分支：`p0/annotations`。
基准：`2c41709cc1044bb070b7e2a5eda2fc78d8857b10`。业务契约：冻结版 1.0.0。

## 提交与范围

- `fa43f43aa4b002dfd4faa9993b35f725482954b1`：标注快照、revision 条件更新、完整替换事务、字段/几何验证、行为测试及真实 Red/Green。
- `1912493ee14efc8ccae1abdb7ebcf5f926aea64c`：一致快照 COCO ZIP、main 路由接入、持久化/指标/导出测试、完整测试/vet/build 原始证据。
- 最终交接提交仅增加本交接和审计，不修改已验证代码。准确最终 SHA 由提交后消息提供。

修改范围严格限于 `backend/internal/annotations/**`、`backend/internal/exports/**`、明确交接的 `backend/cmd/server/main.go` 和本任务审计/交接。未修改共享 store、domain、router、observability、配置或依赖清单。

## 行为与需求追踪

| 需求 | 实现及行为证据 |
| --- | --- |
| P0-ANN-001 | 独立验证有限原图 xywh、正宽高和边界；允许正数亚像素框；重复 ID、跨帧 ID 冲突、非法 source/status 返回 422。`TestInvalidWritesAreAtomic`、`TestGeometryRejectsNonfiniteAndAcceptsPositiveSubpixelBoxes`。 |
| P0-SAVE-001 | GET 使用读事务返回一致快照；PUT 在同一事务校验 revision、条件 UPDATE、删除/替换标注并更新 status/revision；旧 revision 返回 409，任何失败均回滚。`TestSaveRevisionGeometryAndZeroCompletion`、`TestSQLFailureRollsBackReplacement`、`TestConcurrentSameRevisionHasOneWinner`、`TestSnapshotSurvivesDatabaseReopen`。 |
| P0-PROG-001 | 显式零人头完成合法；撤销完成回到 in_progress 后进度同步。`TestSaveRevisionGeometryAndZeroCompletion`。继续编辑 completed 的前端状态重置属于前端包，本 API 按显式请求保存状态。 |
| P0-EXP-001 | 恰选 videoId+可选 batchId 或非空唯一 frameIds；未知 ID/错误批次归属 404，没有已完成帧 422。读事务取得帧与标注快照后释放，再读取不可变 JPEG，完整写入并关闭临时 ZIP 后发送头。独立消费者核对图片字节/实际尺寸、数字唯一 ID、关联、head 类别、bbox/area、iscrowd、零头完成和未完成排除。 |
| P0-OBS-001 | annotation conflicts/errors 及 exports result 指标注册在现有应用 Registry，通过实际 `/metrics` 端点验证。`TestMetricsUseApplicationRegistryAndFiniteResultLabels`。 |

导出一致性由 `TestConcurrentExportsAndSavesNeverMixSnapshots`（并发保存替换与 30 次真实 HTTP ZIP 请求）及 `TestSnapshotReleasedBeforeZIPAndRemainsImmutable`（捕获快照后保存成功，旧快照仍独立导出）验证。图片缺失、尺寸错配均在 ZIP 头发送前安全失败并清理临时文件。

## 原始验证证据

目录：`docs/audit/evidence/P0/P0-BACK-002/`。所有文件保存真实命令、带时区时间及实际子命令退出码。

| 命令/阶段 | 退出码 | 文件 |
| --- | --- | --- |
| `go test ./internal/annotations -count=1` Red：501 占位行为导致断言失败 | 1 | `red-annotations.txt` |
| 标注相同命令 Green | 0 | `green-annotations.txt` |
| `go test ./internal/exports -count=1` Red：501 占位行为导致断言失败 | 1 | `red-exports.txt` |
| 导出首次实现：缺失图片后遗留部分 ZIP，清理断言失败 | 1 | `green-exports-attempt-1.txt` |
| 捕获实际临时文件名修复后，相同导出命令 Green | 0 | `green-exports.txt` |
| `go test ./... -count=1`（2026-10-13T02:20:40+08:00） | 0 | `full-tests.txt` |
| `go vet ./...` | 0 | `vet.txt` |
| `go build ./...` | 0 | `build.txt` |
| `git diff --check <base> HEAD -- backend` | 0 | `source-diff-check.txt` |

环境：Windows、Go 1.25.5、`GOMODCACHE=D:/Project/cvte/runtime/go-mod-cache`、`GOCACHE=D:/Project/cvte/runtime/go-cache`。测试只在 Go 临时目录生成合成 JPEG，无真实课堂图片/运行数据入库。原始 Red 日志包含测试打印产生的行尾空格，按原始证据保留；源码 diff whitespace 检查通过。

## 资源边界与限制

- 每帧最多 10,000 个标注，ID 最多 128 字节；HTTP 复用 1 MiB 严格 JSON 解码上限。必需字段缺失/格式损坏 400，几何/语义不合法 422。
- main 中导出图片上限为 `min(APP_MAX_FRAMES, 10000)`；每份导出最多 100,000 个标注、累计 2 GiB JPEG 字节。超限 422 `EXPORT_LIMIT_EXCEEDED`，可选择较小批次或帧集合。小上限测试使用两帧，无巨型数组。
- 本包未运行 GLM 独立审核、浏览器体验、CI 或云端部署，也不把本地测试视为这些门槛完成。没有 push/merge/deploy。
- 修复预算消耗 1/2：命名返回值在错误返回时覆盖路径，defer 删除空路径；现捕获不可变临时文件名。没有重置预算。
- 请求模型 `gpt-6.1-sol`、推理 `high`、上下文 `fresh`；本 Worker 没有可独立核验实际 provider/model/reasoning 的工具证据，实际值保持 null，Leader 可关联原生 dispatch 证据。

下一步：Leader 审查实际 diff、安排 GLM 独立审核与独立验收，将前后端集成并执行授权的部署门槛。本 Worker 到此停止，不进入 P1/P2。

