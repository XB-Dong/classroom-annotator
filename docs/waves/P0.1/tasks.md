# P0.1 任务拆分与所有权

执行说明：用户已暂停P0.1；所有Worker已停止，0.2.0四入口修订等待审核与明确恢复。保留R0工作树与历史记录，不派发新实施任务。


版本：0.2.0｜状态：暂停后的修订稿；R0只读任务已中断，实施任务未派发｜日期：2026-10-13

本稿是执行顺序和路径边界。已有三个外部P0.1工作树保留，无产品修改；恢复后更新到冻结规格SHA再派发英文task packet。文档采用中文。

## 1. 任务与文件地图

路径均相对于 `code/`。下列新文件是计划创建的位置，不代表已实现。Leader 统一管理依赖、契约、CI/Compose，worker 提交准备工件，经交接才更改所有权。

| 任务 | 责任与允许路径 | 依赖 / 独立验收 |
| --- | --- | --- |
| P01-SPEC-001 | Leader：本计划、specs/p0.1、契约增量、state；Astra只读审核 | 先核对Mac现状与兼容性，冻结规格SHA；审查SQL/媒体/回滚 |
| P01-DB-IMPL-001 | Backend：`backend/internal/platform/store/{store,frames,jobs}.go`，新增`postgres.go`/测试；`annotations/repository.go`、`exports/repository.go` | DB001–005；实际PG读写、CAS、快照、任务锁 |
| P01-DB-MIG-001 | Backend：新增`backend/migrations/{embed.go,00001_initial.sql,00002_media_refs.sql}`、`backend/cmd/migrate/main.go`、`backend/cmd/import-sqlite/main.go`、`backend/internal/migration/`及测试 | schema/依赖由Leader合入；DB006–007；只读来源、非空目标拒绝、恢复 |
| P01-SAMPLE-IMPL-001 | Backend：新增`backend/internal/samples/`、`backend/cmd/seed-samples/main.go`及测试；Leader单写版本化`deploy/samples/catalog.json`/公开出处文档 | source/provider契约冻结；公开样例两条、幂等seed/import、真实原图与credits；媒体下载/运行数据在仓库外 |
| P01-MEDIA-AGENT-001 | Backend/Mac执行端：新增`backend/cmd/media-agent/main.go`、`backend/internal/mediaagent/`及测试 | 桥接契约冻结；Mac001–006/009；禁止依赖业务store |
| P01-MEDIA-CLOUD-001 | Backend：新增`backend/internal/mediabridge/`及测试；media/extraction/exports/httpapi适配；dataset接口通过Mac调用 | 按资源provider路由；Mac源ID映射、公司适配合成链路/Key隔离；无非公开payload云端文件 |
| P01-FRONT-001 | Frontend：ImportPanel/JobsPanel，新增PublicSamplePanel.tsx与MacSourcePanel.tsx；shared/api、FramePicker、ExportButton及对应测试 | 四入口同时可见；默认公共示例；来源与离线反馈，不重写Canvas |
| P01-TEST-001 | Test：新增`tests/scripts/{postgres-check,mac-media-check,migration-check}.mjs`、`tests/e2e/`相关用例，调整已有storage/restart/dataset验证 | 规格冻结即设计测试；保留P0断言；独立PG与合成Agent，后补真实Mac/E2E |
| P01-OPS-001 | Leader单写：`docker-compose.yml`、新增`deploy/compose.postgres.yml`/`compose.mac-bridge.yml`、`deploy/mac-bridge-nginx.conf.tpl`、`deploy/nginx.conf`、`.env.example`、`scripts/verify.{sh,ps1}`、`.github/workflows/verify.yml`、monitoring与部署文档 | 共享配置不得双写；保留门禁；真实容量/恢复/端口检查 |
| P01-REVIEW-001 | Astra核心只读审查，OpenCode GLM代码/体验审查；Leader处理 | 不发真实媒体/凭证给GLM，逐条复现；问题修复不重置预算 |
| P01-DELIVERY-001 | Test证据索引/报告草稿；Leader核验签发report/state | 验收矩阵与版本记录齐全；到门槛/截止停止 |

Backend 阶段任务大部分有顺序依赖，默认一个 Backend owner 逐包完成。只有 Agent 与 cloud client 的接口冻结、改动路径独立且席位允许时并行；`main.go/config.go/domain/models.go/go.mod/go.sum` 由 Leader 分配单写 owner，交接前禁止抢写。

## 2. 每个实施任务的操作顺序

所有 checkbox 当前未执行；发布准备不绕过测试。

- [ ] 阅读任务引用的规格与测试期望，确认允许路径、基线 SHA、依赖与修复预算。
- [ ] 为该任务新增可观察行为失败用例；记录命令、时间与真实失败原因，排除工具/网络缺失造成的伪 Red。
- [ ] 在隔离工作树实现最小行为，运行对应测试得到 Green。
- [ ] 只整理涉及的能力边界，运行相关回归；不要借迁移重写整个后端或前端。
- [ ] 提交英文 message，交接需求ID、文件、提交、退出码/证据及未运行项目。
- [ ] Test独立验收；Leader处置审核意见，在剩余预算内决定修复或结束本轮。

## 3. 计划中的检查入口

以下为批准后新增/调整的命令，不在本次文档阶段执行；目前不能宣称已存在全部入口。

```text
backend/: go test ./internal/platform/store ./internal/annotations ./internal/exports
backend/: go test ./internal/migration ./internal/mediaagent ./internal/mediabridge
backend/: go vet ./... ; go test ./... ; go build ./...
frontend/: npm run typecheck ; npm run lint ; npm test -- --run ; npm run build
tests/: npm run postgres ; npm run migration ; npm run mac-media
tests/: npm run contract ; npm run e2e ; npm run monitoring
Windows: ./scripts/verify.ps1 -Integration -E2E
Linux: bash scripts/verify.sh --integration --e2e
```

上述 `;` 是各命令的独立检查清单，执行器必须逐条判断退出码，不以前一个失败后的后一个成功掩盖结果。Test 为新增三个 npm 命令补充真实外部配置、有限等待、隔离输出及清理；CI启动真实PG服务，禁止缺DSN时跳过门槛当成功。任务级 Red/Green 的精确测试名在英文 task packet 中按已冻结用例登记。

## 4. 模型、上下文、审核和审计

沿用决策0003：Leader Sol/xhigh；实现/Test Sol/high；Astra/xhigh按需；OpenCode GLM固定准确ID。记录 requested 与 actual 设置，无法核验的 actual 写 null。原生最多三个 Worker；Mac 独立执行端不计入虚构的原生槽位。

R0/R1主要Backend+Test+Astra；R3/R4主要Backend+Frontend+Test。Astra用完即释放席位。角色 MEMORY 只写有证据的事实和版本，不能把“拟采用PG”写成“PG迁移完成”。

每轮审计至少包括：timestamp（+08:00）、wave/round/task、actor、scope/paths、requested/actual模型、specVersion/specSHA、sourceSHA、动作、命令退出码、evidenceRef、repairCount、blocker、Leader决定。日志记录可观察行为，不收集模型内部思维。

每任务最多2轮修复，全波次最多6轮；外部GLM同一阻塞最多2次，每次600秒。任务换人/续跑继承计数。恢复预算沿用原8小时扣除已记录648秒R0，暂停不计开发；具体截止在明确恢复时写state，不自动重置预算。

Mac 任务由用户转交 [交接稿](MAC-HANDOFF.md)；Leader 不在未获直接授权时跨聊天发送消息。共享代码由 Leader 合入GitHub；Mac只拉明确SHA，不在内网运行来源不清的随意脚本。
