# P2 浏览器自适应验收

本文件对应 P2-UI-001、P2-UX-001、P2-TEST-001，并覆盖 P2-QC-001 的前端展示。所有请求由 Playwright 拦截为合成响应，图片为生成的 SVG；没有调用真实 Go、PostgreSQL、媒体服务或语言模型。这些结果不能代替真实标注、CAS、导出或独立 GLM 浏览器门槛。

## 测试与范围

入口为 `tests/e2e/p2-responsive.spec.mjs`，通过既有 Playwright Chromium project 运行，单 worker、无自动测试重试。共 30 个测试：

- 登录、注册、长错误；已加载画布/待审风险候选、长账号/文件名、指南与问答失败：320×568、360×720、390×900、768×568、1024×720、1280×568、1440×900、1920×900。
- 空库与离线、200% 文本：320×568、768×720、1280×900。文本放大逐元素将计算字体大小乘二，保留 CSS px 视口；不同于 deviceScaleFactor，也不宣称覆盖系统字体设置或浏览器所有缩放模式。
- 可选抽帧范围、批量 AI 和复核统计默认收起，聚焦后通过 Enter/Space 展开；完成、导出仍可通过纵向滚动到达。
- 320×568 下选中合成上传文件后可读取完整长名称；测试不上传该缓冲区。
- 质量检查在 320×568、768×720、1280×900 下展开，覆盖忙碌、100 条长标注 ID、截断提醒、最后一条定位按钮可达与失败后的原导出入口。
- 自动保存 503 和 409 各一个连续缩放测试：遍历上述八种尺寸，并追加 1440×568、320×900、1920×568、1280×720；逐次核对同一草稿、视频选择、原图坐标和写请求数量，缩放没有隐式提交；错误/恢复动作可达。

几何检查包括 HTML/body 宽度、控件横向边界、文字 Range 与控件边框的裁切、无全名访问的名称省略，以及关键 supporting pane 的实际交叠。交互控件之间只检查互不包含的同级几何交叠，不把祖先包含后代当作冲突。关闭 details 的未渲染内容不计入。原生输入框内部滚动与 select 弹出选项不按按钮文本检查。关键动作另检查纵向滚动后的中心点命中，防止可见但被覆盖。

## 已执行的 Red 与修复

基线产品：`0f720ddc8cf7c271028ce2e98d4533bf9d46e948`。先运行两个 320×568 用例，真实 Chromium 执行 **2 failed，退出码 1**。可观测失败包括页面宽 904 CSS px、账号动作出界、抽帧输入与导出/批次控件交叠、帧卡与画框工具交叠、长名称省略且无完整名称访问。不是依赖安装失败。

首次候选：`17d3f1a`（等价 Frontend `8555f5b` + `b79e6cd`）。完整 25 例 **20 passed / 5 failed，退出码 1**。共同缺陷为窄屏横排帧列表的 160 px 高度上限使风险卡文字越出按钮；320 的 200% 文本用例还记录了超时，原始日志保留，不改写为通过。

新增选中文件名检查在同一候选 **1 failed，退出码 1**：原生 file input 没有可读取的完整文件名文本。Frontend `e445639` 同批移除窄屏帧列表固定高度，并显示可换行的完整选择文件名。对应六例复验在本测试工作树 `190d395` **6 passed，8.7 秒，退出码 0**。

最终候选产品：`87f9c68775f3a0f67784d4bcef34e45d76dd18da`，包含 Leader `f3b58fe` 质量检查界面；`git diff eeaa3b6 HEAD -- frontend` 为空，前端源码与 Leader 候选一致。最终 29 例执行结果以本文件后续记录和外部 `final-green.log` 为准。

## 复现与证据

从仓库根目录启动 `frontend` Vite，例：

```sh
node frontend/node_modules/vite/bin/vite.js frontend --host 127.0.0.1 --port 5192 --strictPort
P01_BROWSER_CHANNEL=chromium P0_BASE_URL=http://127.0.0.1:5192 \
  node tests/node_modules/playwright/cli.js test \
  --config tests/playwright.config.mjs --project=chromium p2-responsive \
  --global-timeout 180000
```

基线 Red 同命令追加 `--grep 'login and registration 320|loaded editor review and help 320'`；修复定向复验追加 `--grep 'loaded editor review and help (320|360|390)|200 percent text 320|advanced controls|selected upload'`。每次 `P0_TEST_OUTPUT`、`P0_REPORT_OUTPUT` 设为外部独立目录，避免覆盖失败证据。

本机 Node `24.19.0`；按 `tests/package-lock.json` 执行外部 `npm ci` 成功，`@playwright/test 1.64.0`；实际 Chromium `156.0.8078.4`。浏览器缓存与 npm 缓存均在仓库外。Chromium 初次安装曾出现默认 30 秒下载超时，后续自动尝试取得完整 Chromium；冗余 headless-shell 下载停止，执行使用 `channel=chromium` 的完整浏览器。原始安装日志保留。

外部证据根目录：`/Users/kdl/.local/share/classroom-annotator/p2-20261014/evidence/responsive/`：

- `red.log`、`red-results/`、`red-report/`：基线两例失败、截图和 trace。
- `green-attempt1.log`、同名前缀结果/报告目录：20/25；失败没有被最终通过覆盖。
- `filename-red.log`、同名前缀结果/报告目录：选中文件名缺陷。
- `targeted-green.log`、同名前缀结果/报告目录：六例修复复验。
- `final-green.log`、`final-green-results/`、`final-green-report/`：最终全矩阵。
- `screenshots/`：经过几何断言后的登录、注册、空库、编辑/风险、指南、窄屏、200% 文本、保存错误/冲突与质量检查场景。每张 PNG 附 JSON，标明 `fixtureState`、`synthetic=true`、源码 SHA、时间、浏览器和视口；不同尝试中的同名精选图可被最终候选更新，原始测试目录保留各次图像。

## 交付边界

仅 Chromium 本机模拟 UI；Firefox/WebKit、真实 Go/PG/媒体闭环、真实注册登录、真实导出 ZIP 内容、真实模型候选、CI 与独立 GLM：**不属于本套证据，未执行即 not_run**。原有 `workflow.spec.mjs`、`public-samples.spec.mjs` 只增加展开“指定抽帧范围（可选）”一步，原断言不变；本次模拟矩阵不代表它们已经执行。

## 最终结果补记

第一次29例完整候选执行26通过/3失败，新增质量长列表的误报来自几何检查没有裁切overflow滚动祖先的未显示区域；修正测试器只比较实际绘制区域，保留文字/末行可达断言。未修改产品源码。随后29/29在38.0秒通过，原失败保留。

原生静态复查另发现真实产品问题：quality组件key含reviewRefresh，正常5秒相同revision轮询导致结果面板重挂载。新增轮询用例实际RED1失败，记录poll-red.log。Leader提交74f6e8e移除无关刷新计数，仍按帧ID/revision变化失效；质量组件6项定向单测通过。

最后完整 **30/30通过，44.3秒，exit0**。产品前端等价Leader74f6e8e，测试工作树3cb3d84，frontend路径diff为空。最终结果与报告目录为 `final-green3.log`、`final-green3-results/`、`final-green3-report/`。新增不变revision轮询用例验证面板和结果经过真实5秒轮询保留。

Test Agent在补充交接阶段连续两次模型容量错误，未切换型号。Leader接续保存其已编写测试，并执行最终30例；这不是独立GLM验收，不能冒充新的跨模型浏览器体验。原生请求配置和错误如实记录在P2状态。
