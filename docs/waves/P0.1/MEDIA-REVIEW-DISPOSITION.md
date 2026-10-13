# P0.1 媒体服务审核处置

审核对象：`4efd3e5398d6ec504a64f46f56e48530ad557117`。请求审核模型 `gpt-6-astra/xhigh`；可核验实际模型信息缺失，记为 null。以下为静态代码发现，不能描述为审核者已执行运行时复现。

| 发现 | Leader 决定 | 验收要求 |
| --- | --- | --- |
| 同长度素材修改后，GET/HEAD/Range 可能沿用旧摘要，抽帧前也未核对内容 | 采纳，启用 Mac 前修复 | 实际同长度改字节后拒绝读取、抽帧；原资源仍可读取 |
| 上传在持久化操作前已建立 staging 并接收字节，无法在接收中取消/续租，重启可能失去清理归属 | 采纳，启用 Mac 前修复 | 接收前保存身份、限定容量，阻塞接收中 cancel/renew，重启后清理已有归属 staging |
| 清理失败仍清空 staging 字段，重启只核对 queued/running | 采纳，启用 Mac 前修复 | 保留失败路径，重启后可重试，未清理前不冒充 cancelled |

上述为 `P01-MEDIA-AGENT-001` 正式修复第 1 轮。旧源码二进制私密交接包仅为草稿，暂不启用。修复需独立复审及重新构建核验摘要，再交给 Mac。

第 1 轮修复提交 `b39d408be5e08e58fc89673ea56bd6595e732b7a` 已通过完整单元测试及独立 FFmpeg 流程，但复审用真实 loopback TCP 上传复现新的取消/超时阻塞：Go HTTP Body.Read 与 Close 共享锁，接收取消只调用 Body.Close 无法中断正在读取的连接，取消接口超时、租约/总超时后仍 running，Agent.Close 等待客户端主动断开。三个真实 HTTP 用例 exit 1，证据在 `runtime/p01-media-rereview/real-http-result.txt` 和 `real_http_test.go`。Leader 采纳，进入媒体任务第 2/2 轮，全波次已用 4/6 轮；不得以原 io.Pipe 用例 Green 代替真实 HTTP 清理成功。

合成独立媒体验收已通过真实 FFmpeg 10 帧、Range、操作重启保留与确认、完整原图/COCO ZIP、公司 API/媒体 Key 隔离。这是 Windows 合成运行证据，不能替代真实 Mac、隧道及云端不落盘验收。原验收清单空列表由测试夹具错误地使用绝对 URL 导致，已改回实际相对路径契约，不计为产品故障。
