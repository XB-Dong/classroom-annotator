# P0-FRONT-001 前端交接

状态：实现已就绪，等待独立集成／浏览器／GLM 验收。交接时间：2026-10-13T02:07:52+08:00（实际交接检查时间）。工作树 D:/Project/cvte/worktrees/p0-frontend，分支 p0/frontend；仅 frontend/**、自身证据和本交接修改。

规格：冻结 1.0.0（API、OpenAPI、坐标、产品）。实施基线 51d05f5；基础提交 6ab8e6ac9f3e2d7266f65c75c2626692495f847b；当前实现提交 **31ccc3e386e1b6d8f62bb3fcde1f29d42dba4649**。本交接为其后文档提交，不改变该验证版本。

## 已实现与追踪

- P0-VID-001/002A：本地 multipart `file` 上传；真实公司课节加载、分页、仅 channel 4 导入、snake_case DTO、安全 catalog ID → 十进制字符串；未配置接口明确提示。
- P0 抽帧与任务：间隔／起止时间毫秒精度输入，独立批次；GET /jobs 恢复、每 2 秒轮询、取消与 cancelling 状态；导入完成前 videoId 可缺省。
- P0 标注／坐标：响应式帧列表、Konva 原图像素绘制／移动／缩放／删除；仅新建应用 2px 误触阈值；列表稳定 ID；Delete、Ctrl/Cmd+Z、Shift+Ctrl/Cmd+Z，输入焦点保护；采样时间标签不称 PTS。
- P0 保存／恢复：约 500ms debounce，同帧捕获 snapshot/editSequence 串行保存；保存 A 期间编辑 B 保留，并用 A ack revision 继续；仅匹配当前序号清草稿；旧控制器 dispose 后异步 save/resolve 不能改草稿；409 暂停 autosave，显式 latest revision 替换或丢弃。
- P0 完成／导出：零人头显式完成；完成帧编辑回到 in_progress；后端进度与帧状态；POST /exports 当前 videoId+batchId ZIP 下载，仅服务端已完成帧进入导出范围。
- 真实 loading/empty/error/save 状态；视频、标注、导出能力模块；源码 Prettier 3.9.10 格式化，画布工具动态导入。依赖精确锁定与 npm cache 在仓库外。

## 验证

证据目录 `docs/audit/evidence/P0/P0-FRONT-001/`；index.md 列明真实 Red、Green、原始失败与后续修复。2026-10-13T02:05:37+08:00 至 02:05:50+08:00，`npm run typecheck`、`npm run lint`、`npm test`（17 通过，4 文件）、`npm run build` 各退出 0。生产输出主包 236.58KB、按需 Editor 339.17KB，无 chunk warning。

显著坐标／保存／冲突行为和生命周期修复已真实 Red→Green。React 交互测试在叶边界 mock Canvas；没有将它表示为实际 Konva 浏览器手势证明。测试未依赖真实课堂媒体。

## 集成入口与限制

`frontend/README.md` 提供安装／运行／验证与同源 `/api`、`/media` 代理要求。开发 Vite 代理 localhost:8080；生产静态 dist 和 Nginx/Compose 由 Leader 集成部署，本 Worker 没有修改共享部署文件或启动云端任务。

Test selectors：upload-file/upload-submit/video-select/batch-select/interval-seconds/start-seconds/end-seconds/extract-submit/frame-item-{id}/annotation-canvas/**annotation-image-bounds**（Stage 显示图像精确边界）/draw-mode/select-mode/annotation-item-{id}/complete-frame/save-state/conflict-reload/export-zip。annotation-canvas 外框含 letterbox，像素手势请取 annotation-image-bounds。

尚未由本 Worker 执行：真实公司 API 联调、后端联通的核心 E2E、云端验收、GLM 独立审查与实际浏览器体验。没有此前应用测试基线，不声称旧应用测试已通过。请求模型 gpt-6.1-sol/high，实际路由设置未独立核实，完整 null/unverified 字段在证据索引。

建议下一步：Leader/Test 对最终实现 SHA 执行合成视频 upload→extract→draw/move/resize/delete→autosave→refresh/conflict→complete→export 的独立集成；GLM 以最终格式化源码和合成演示审核。此交接不代表整波次完成，不合并／推送／部署。

