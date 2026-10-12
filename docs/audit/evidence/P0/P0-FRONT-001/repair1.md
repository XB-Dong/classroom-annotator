# P0-FRONT-001 修复 1

收到 Leader R3 Astra 的五项实际复现；基线 09212898df5b151e2c1a2185bb3f997e616640bd。本次为 rootTaskId P0-FRONT-001 第 1 次正式审核修复，未重置最多 2 次修复预算。实际开始环境检查：2026-10-13T02:21:35+08:00。

| 问题 | 修复与回归 |
| --- | --- |
| 重复 resolve 后到 GET 清理新编辑 | 控制器 resolution 锁、busy view/按钮、捕获 editSequence 检查与 disposed 检查。延迟重复响应测试保证新编辑／草稿存留；错误在释放 busy 后仍可见 |
| debounce 内快速导航丢失服务器更新／导出旧 completed | frame/video/batch 统一等待 settle；串行等待正在保存的快照并保存途中新编辑；失败／冲突不允许导航。当前未保存／加载状态及选中批次任何持久草稿阻止导出；测试完成帧编辑快速切帧、完成立即切帧、视频／批次导航和另一帧草稿导出门槛 |
| draw → select Transformer 未连接 | effect 同时依赖 selected 与 enabled；测试实际安装 Konva Rect/Transformer 的节点连接，并直接触发 transformend 更新模型 |
| 归一化模型与节点属性不一致 | 每次有效拖动／调整立即将 normalized x/y/width/height/scale 写回真实节点；边界测试同时检查模型与安装 Konva 节点属性 |
| progress 响应作用域／顺序错误 | ScopedRequest 用 videoId+batchId 和递增代次保护接受及错误回调；初次加载与保存后的刷新均使用。测试晚到 A 响应不能覆盖 B，同作用域旧响应不能覆盖新请求 |

任务完成引发的批次刷新保留已选批次，避免后台任务完成时直接卸载正在编辑的帧。BatchBar、FramePicker 从 App 分离；格式化继续使用 Prettier 3.9.10。没有修改接口或增加业务功能。

## 原始证据

- repair1-red.log：重复 resolve／两个 progress 乱序断言真实失败；同次 Konva 2 项初次因 jsdom 缺少 canvas context 报错，这两项**不计有效 Red**。
- repair1-ui-red.log：叶环境仅替换 Konva canvas 指纹检测（实际安装节点与 Transformer 未替换）；4 项真实断言失败，覆盖快速切帧／完成与 Transformer 连接、节点归一化。
- repair1-green-attempt.log：24 项通过，退出 0。
- repair1-resolution-error-red.log：释放 resolve busy 的 finally 清除错误提示，1 项真实失败，退出 1；修复保留错误。
- repair1-verification.log：中间版本 typecheck/lint/30 tests/build 全 0。
- repair1-final-verification.log：最终行为与组件提取后，2026-10-13T02:30:36.4701772+08:00 至 02:30:50.2586266+08:00，typecheck/lint/30 tests（7 文件）/build 全 0。主包 239.27KB、按需 Editor 339.30KB，无 chunk warning。

App/Editor 测试仅在叶测试边界替换 Canvas 和 HTTP；生产不存在 mock。Konva 测试使用真实安装节点，但不包含浏览器绘制／真实指针手势，不将其表述为完整浏览器验收。独立 Astra 重审和 GLM/云端验收仍由 Leader 安排。
