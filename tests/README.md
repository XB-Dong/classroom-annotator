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

默认 `verify` 执行静态检查、单测、构建和专用本地后端进程的 FFmpeg/SQLite、dataset、restart、API 集成；`-Integration` / `--integration` 另外对正在运行的部署 API 验收；`-E2E` / `--e2e` 明确启用浏览器。P0 交付仍必须实际运行云端 E2E；没有启用不表示通过。

主闭环测试只通过 UI 完成变更；API 仅观测持久化与独立验证下载 ZIP。两个保存竞态测试使用 API 准备帧并模拟另一个写入者。要求 UI 提供 `upload-file/upload-submit/video-select/interval-seconds/start-seconds/end-seconds/extract-submit/frame-item-{id}/annotation-canvas/draw-mode/select-mode/annotation-item-{id}/complete-frame/export-zip/save-state/conflict-reload` 的 `data-testid`。画布坐标以容器中的 `.konvajs-content` 实际图像区域为准，避免外层留白影响坐标。选择器是实现协作点，缺失产生明确失败，不应删除断言。

重启/任务中断、超时、上游 API Key 隔离和重定向等需受控独立进程集成，详见后端测试及 `scripts/restart-check.mjs`；外部部署不能在没有 Leader 授权时由测试直接重启。合成 API 验证不代表真实公司环境联调。GLM 实际代码审核和产品浏览器体验是另外的必需门槛。

专用进程可单独运行：设置仓库外 `P0_BACKEND_BINARY`、`P0_FIXTURE_DIR`、全新不存在的 `P0_API_DATA_DIR/P0_DATASET_DATA_DIR/P0_RESTART_DATA_DIR`，依次执行 `npm run local-api`、`npm run dataset`、`npm run restart`。脚本默认分配临时 loopback 端口，每次最多一个后端进程，结束时停止自己启动的进程。`P0_EXPECTED_SHA` 应为对应二进制构建源码的完整 SHA，不能用测试分支 HEAD 代替部署版本。只在进程停止后，设置 `P0_STORAGE_DATA_DIR` 为该专用目录并执行 `npm run storage`，独立读取 SQLite 与 JPEG 检查 DB integrity、文件关联和无暂存/孤儿媒体。

`npm run monitoring` 的实际云端要求见 `deploy/ACCEPTANCE.md`，需要通过外部环境变量配置 Grafana 认证；脚本不打印凭证。`npm run check` 只验证这些脚本语法，不会运行监控验收。
