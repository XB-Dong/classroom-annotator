# P2 导出前质量提醒

版本：0.1.0｜P2-QC-001｜规则版本：P2-QC-1

提醒用于定位可疑框，不判断真值，不衡量准确率，不自动修改标签。只检查当前导出范围的已完成帧，并复用原服务端快照/审核门禁。未完成帧、未审候选仍遵守原契约。

## API

`POST /api/v1/exports/quality-check`，身份/工作区/Origin/CSRF 与现有 exports 一致。请求完全沿用现有 `videoId` + 可选 `batchId` 或唯一非空 `frameIds`，两种选择互斥，未知/错误输入按现有规则拒绝。

响应包含 `rulesVersion`、`checkedFrames`、`checkedAnnotations`、`totalFindings`、`truncated`、`frameRevisions:[{frameId,revision}]`、`findings:[{code,severity,frameId,annotationIds,message}]`。severity 仅 warning。无问题返回空数组；`truncated` 必须明确表示未完整扫描，不能显示“全部通过”。错误沿用 exportError 契约和 requestId。

## 初始规则与有界执行

- TINY_BOX：框最小边小于 4 原图像素。
- LARGE_BOX：人头框面积超过原图面积的 25%。
- DUPLICATE_BOX：两个正式框 IoU ≥ 0.90。边界值按包含阈值测试。

这些是工程启发式，极小目标/近景大头和真实遮挡可能触发误报。相邻帧重复不在本规则内。每次返回最多 100 条发现，全请求重复比较最多 100000 次，按 frameId/code/annotationIds 确定顺序，输出截断事实；上下文取消与超时必须退出。计数代表已检查快照，不把没检查的内容记为零问题。

不新增写事务、不改变 revision、不调用推理/外部模型、不读取视频帧进行视觉审查。原 export 执行时仍独立检查最新快照和媒体完整性，预检 token 不可绕过任何门禁。

## 前端

导出入口点击后展开质量检查/结果，低频入口不平铺到画布主工具条。显示本次规则版本和检查数量。发现可点击定位帧，具名框 ID 保留。用户可检查后继续导出；警告不强制判错。预检不可用时清楚显示，可选择直接执行原导出；不能假装已检查。无已完成帧或存在未保存草稿仍沿用现有禁用规则。

## 验收

独立几何例子验证 IoU、包含关系、极小/大框、零人头、100条截断和比较上限；实际 PG 快照/授权/CSRF、无写入、不破坏原导出；前端检查失败与定位、不隐式下载、忙碌防重复点击、切换批次不使用旧结果。详细测试 evidence 写入波次记录。

新增路由必须纳入 `backend/internal/auth/scope.go` 对导出请求的逐资源工作区校验。`totalFindings` 仅计实际检测到的发现；截断时不代表全量。`checkedFrames/checkedAnnotations` 是已捕获快照数量，不承诺重复框已全量比较。
