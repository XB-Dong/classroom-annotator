# 独立验收

P1当前正式入口是Linux CI的`bash scripts/verify.sh --e2e`；五份外部PG连接文件沿用`P01_*_DATABASE_URL_FILE`名字，分别为Go单测、事务快照/重启、全栈、SQLite导入与导入失败回滚专属库。Linux GHA创建真实PG17非超级用户数据库。Windows不启动数据库、Docker或隧道；可在`Local\CVTE_P1_BUILD`互斥锁内做`npm run check/npm test/npm run list`，最多等待120秒。其余真实PG/API/浏览器门槛在Linux执行。

P1同机全栈单独入口`node tests/scripts/p1-run-stack.mjs --e2e`（从`tests`目录执行时为`node scripts/p1-run-stack.mjs --e2e`）要求前端已构建，且外部`P1_RUNTIME/P1_MODEL_PATH/P1_PYTHON_BINARY/P0_BACKEND_BINARY/P01_MIGRATE_BINARY/P01_SEED_BINARY/P0_FIXTURE_DIR/P01_ACCEPTANCE_DATABASE_URL_FILE`已配置。此入口只使用隔离测试命名的数据库和合成媒体，启动自己拥有的Go/Python及同源HTTP代理，并在finally中回收。不会操作已部署业务。先执行`node scripts/sync-help-document.mjs`再Go构建；它逐字复制唯一手册供embed。

新增独立API测试覆盖注册/邀请码/密码字节范围/非自动登录、Cookie/CSRF/Origin/过期/匿名媒体、20轮PG行锁屏障CAS与逐轮数据库/审计、Go→真实Python模型→PG复核门禁、完整手册SHA与无供应商降级、本机目录内容身份、公司schema/channel4凭证隔离、实际FFmpeg发布/取消锁顺序。浏览器覆盖旧人工闭环、公共样例归属和原图下载、三尺寸截图、两账户冲突/草稿保留及5独立会话/3账号固定10分钟读写/完成/导出。API与浏览器之间等待61秒以遵守实际登录IP限流，不绕过限流器。

锁定Python单测、合成JPEG真实ONNX计算和课堂图质量是三种不同证据。CI只从作者单文件release下载3221705字节、固定SHA的真实Nano；`tests/p1-model-check.py`会加载真实ONNX，但不把合成图结果描述成准确率/课堂8图验证。独立AI原8私有图证据不进Git/CI/GLM。

全栈入口在真实Go→Python API阶段之后，停止自己拥有的Python进程，在原AI loopback端口运行`fixture_not_real_model`确定性HTTP服务；Go API地址与真实PG不变。三个固定原图框用于独立检查接受/调整/拒绝、原始风险、空分母、删除撤销与不可变provenance、重跑保留旧正式框、失败/取消/stale/批次部分结果。fixture读取实际320×240合成JPEG并保存输入SHA、请求ID与屏障时间，不把固定框当模型效果。恢复真实Python后才执行浏览器/10分钟负载，最后单独停实际Python做人工降级闭环。

fixture专属控制令牌只传入本次测试子进程，不写证据。它只绑定loopback，控制自己启动的第二个Go API与首实例的loopback数据库故障代理。第二实例启动时首实例AI HTTP请求仍被屏障挂起；随后20轮双进程PG行锁竞态必须每轮恰好一个200、一个409且只有一个审计事件。首实例失锁并恢复DB后，人工保存仍可用，任务入口保持503直到重启。`fixture-review-evidence.json`、`follower-cas-evidence.json`及独立fixture Playwright目录即使失败也保留；收集用例或Node fixture单测通过不代表这些PG/API断言已执行。

完整checks通过后才导出`p1-linux-amd64-SHA`的精确二进制/前端；后置job交叉编译arm64 Go工具并打包同一已验证前端，ARM64运行仍需另验。另一个双架构job构建真实AI Docker镜像、通过容器readiness与真实模型/JPEG推理后保存离线`p1-ai-{amd64,arm64}-SHA`。arm64在Linux QEMU运行，不等于Mac实机验证。镜像、源SHA、平台、模型许可证、检查结果与SHA256SUMS一并保存；任一步失败就不上传“verified”镜像工件。

下文P0/P0.1入口保留为历史说明。Mac代理专属检查已退出P1公共CI；PG/导入/CAS/事务快照/FFmpeg/COCO的不变量保留或换成同机测试，不能把退役接口删除当成业务证明。

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
