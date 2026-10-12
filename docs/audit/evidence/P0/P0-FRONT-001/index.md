# P0-FRONT-001 验证证据索引

规格版本：1.0.0。实施基线：51d05f5。当前工具环境：Node v24.16.0 / npm 11.13.0（开始时实际命令观察）。依赖版本来自 npm registry 查询并精确锁定；完整依赖树见 frontend/package-lock.json。Prettier 3.9.10 整理源码，标注工具使用动态导入减少初次加载。

| 证据 | 命令与观察 |
| --- | --- |
| install.log | `npm install --cache D:\Project\cvte\runtime\npm-cache`，2026-10-13T01:52:13+08:00 至 01:53:08+08:00，退出 0；安装不是 Red |
| red.log | 坐标与捕获快照保存叶测试首次执行，7 失败 / 1 通过，退出 1；未实现行为导致断言失败 |
| green.log | 实现坐标归一化与保存控制器后，8 通过，退出 0 |
| foundation-checks.log | 初次 typecheck/lint/build 各退出 0；当时 574KB chunk 警告，后续动态导入消除 |
| disposed-red.log | 旧保存回调在 dispose 后清理草稿的回归 Red，1 失败 / 4 通过，退出 1 |
| completed-edit-red.log | 编辑完成帧仍保持 completed 的回归 Red，1 失败，退出 1 |
| interaction-green.log | 增加交互测试的首次运行失败：window 事件目标缺少 closest；1 失败 / 14 通过，退出 1。不是全部行为的 TDD Red |
| interaction-green-2.log | 生命周期防护、完成状态重置、快捷键目标检查后，16 通过，退出 0 |
| conflict-disposed-red.log | 冲突替换读取最新版本期间离开帧，旧控制器覆盖新草稿的 Red，1 失败，退出 1 |
| refactor-verification.log | 2026-10-13T02:05:37+08:00 至 02:05:50+08:00；typecheck/lint/17 tests/build 全部退出 0；无 chunk 警告 |

真实 Red/Green 覆盖显著坐标、保存竞态、草稿冲突与两个生命周期回归。React Editor/ImportPanel 行为测试补充删除/撤销/焦点保护、零人头完成、multipart 上传、未配置公司接口、channel 4 筛选、catalog ID 字符串和导入任务缺省 videoId。Konva 在 Editor 叶测试中替换为边界组件；实际 Konva 浏览器手势由独立 Test 验收，不把叶测试当作浏览器证明。

模型记录：requestedModel=gpt-6.1-sol；requestedReasoningEffort=high；actualProvider=null；actualModel=null；actualReasoningEffort=null；modelVerificationStatus=unverified；modelEvidenceRef=null；contextMode=fresh。此记录不使用模型自述核实实际路由。

真实公司 API 联调、云端部署、GLM 独立代码与浏览器体验均未由本 Worker 执行。没有原始课堂媒体、凭证或签名 URL 进入源码／审计。
