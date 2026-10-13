# 独立验收

本目录是独立 npm workspace，不混入 frontend 的 Vitest。运行数据、合成视频、浏览器安装缓存和凭证必须在仓库外。Node 24、npm、FFmpeg/ffprobe 和可访问的真实部署为外部要求。

```sh
cd tests
npm ci
npm run check
npm test
export P0_FIXTURE_DIR=/tmp/classroom-annotator-fixtures
npm run fixtures
export P0_BASE_URL=http://127.0.0.1:8180
export P0_EXPECTED_SHA=<actual-deployed-full-sha>
npm run contract
npx playwright install chromium
npm run e2e
```

PowerShell 使用 `$env:P0_FIXTURE_DIR=...` 等环境变量形式。`FFMPEG_PATH` 可配置外部可执行文件；`PLAYWRIGHT_BROWSERS_PATH`、npm cache 和 `P0_TEST_OUTPUT` 可配置仓库外缓存/输出。生成脚本拒绝将媒体放进仓库。测试会新增合成视频及标注，应使用独立验收实例/数据卷。

P0.1 默认 `verify` 必须连接 PostgreSQL 17。预先配置仓库外的 `P01_GO_DATABASE_URL_FILE/P01_DATABASE_URL_FILE/P01_ACCEPTANCE_DATABASE_URL_FILE/P01_IMPORT_DATABASE_URL_FILE/P01_ROLLBACK_DATABASE_URL_FILE`，分别用于 Go 临时 schema、PG 并发/恢复、完整 API、迁移及回滚；五者数据库不同、账户以 `p01_test_` 开头且没有管理员权限。Go 数据库固定为 `p01_test_go`。缺文件、连接失败或版本不符直接失败，不跳过。创建数据库属于环境准备，测试脚本不自行操作已有部署。

默认入口执行静态检查、真实 PG Go 测试、构建、Node 单测、五表迁移核对、20 写入 CAS、跨帧 ID 竞争、100 次并发快照导出、断线恢复、实际后端重启、取消/发布竞争、原生媒体 Agent、公司 API 凭证隔离及完整 API。`-Integration` / `--integration` 保留为完整 API 的显式别名；`-E2E` / `--e2e` 增加真实浏览器。API/浏览器栈由脚本启动独立 PG 后端、原生 Agent、合成 v1 素材/公司服务和构建后前端 HTTP 流式代理，关闭时只停止自己的进程。输出目录、二进制、视频与凭证都位于临时外部目录。实际 Mac、云端、GLM 与未启用的浏览器门槛不能因此写为通过。

主闭环测试只通过 UI 完成变更；API 仅观测持久化与独立验证下载 ZIP。两个保存竞态测试使用 API 准备帧并模拟另一个写入者。要求 UI 提供 `upload-file/upload-submit/video-select/interval-seconds/start-seconds/end-seconds/extract-submit/frame-item-{id}/annotation-canvas/draw-mode/select-mode/annotation-item-{id}/complete-frame/export-zip/save-state/conflict-reload` 的 `data-testid`。画布坐标以容器中的 `.konvajs-content` 实际图像区域为准，避免外层留白影响坐标。选择器是实现协作点，缺失产生明确失败，不应删除断言。

重启/任务中断、超时、上游 API Key 隔离和重定向等需受控独立进程集成，详见后端测试及 `scripts/restart-check.mjs`；外部部署不能在没有 Leader 授权时由测试直接重启。合成 API 验证不代表真实公司环境联调。GLM 实际代码审核和产品浏览器体验是另外的必需门槛。

完整栈可单独运行：设置外部 `P0_BACKEND_BINARY/P01_MIGRATE_BINARY/P01_AGENT_BINARY/P0_FIXTURE_DIR/P01_ACCEPTANCE_DATABASE_URL_FILE`、全新不存在的 `P0_API_DATA_DIR` 与准确的二进制源码 `P0_EXPECTED_SHA`，执行 `npm run local-api` 或 `node scripts/run-api.mjs --e2e`。前端须先完成构建。`P01_PUBLIC_SAMPLE_CATALOG` 与 `P01_SEED_BINARY` 可指定已准备的真实公共示例清单及 seed 工具；缺公共清单时不宣称公共素材闭环已验证。对现有云端执行 `npm run contract/e2e` 时，单独设置该部署的 URL/SHA；不得使用本地分支 HEAD 代替实际部署版本。`dataset/storage` 是历史 P0 SQLite 辅助脚本，不属于现行默认入口。

`npm run monitoring` 的实际云端要求见 `deploy/ACCEPTANCE.md`，需要通过外部环境变量配置 Grafana 认证；脚本不打印凭证。`npm run check` 只验证这些脚本语法，不会运行监控验收。
