# P01-MEDIA-AGENT-001 修复交接

状态：媒体修复第 2/2 轮已提交，沿用全局修复计数 4/6（含验收脚本修复 1 次）；待独立复审。第 1 轮代码提交 `b39d408be5e08e58fc89673ea56bd6595e732b7a`，基于原 Agent 提交 `4efd3e5398d6ec504a64f46f56e48530ad557117`。请求模型/推理强度为 `gpt-6-sol/high`；运行时实际 provider、model、reasoningEffort 无可核验元数据，均记为 `null`。

## 修复范围

1. **P1 媒体身份**：GET、HEAD、单 Range 在发响应头前用同一个打开的文件描述符核对常规文件、长度和 SHA-256；抽帧入口、执行前和输出发布前核对原视频身份。上传、源登记和公司视频准备在 FFprobe 前后与原始流摘要比对；图片归档仍在打包时逐字节核验。相同长度的文件改写返回 `RESOURCE_CHANGED`，不发布帧。原有路径与符号链接检查保留。
2. **P1 上传接收**：从 multipart 文件头取得 `UploadIntent{videoId,name}`，先持久化 operation、deadline、lease 和 staging 归属，再读取视频 bytes。接收并发受 `queueSize` 限制；取消/到期关闭阻塞的请求体，清理完成后才进入终态。流式 SHA-256 和长度单独持久化并与准备后的 Registration 绑定；同 operationId、同意图但不同内容重放返回 409，`requestFingerprint` 创建后不变。能力响应增加仅由配置计算的 `datasetConfigured`、`sourceConfigured`。
3. **P2 清理归属**：删除失败时保留 `Staging` 和 `CLEANUP_FAILED`，重启恢复与后台定期重试；对失败操作再次调用 cancel 可显式重试清理。即使后来清理成功，操作仍为 failed，不虚报 cancelled。

## 验证证据

- 原提交上先加入回归测试并执行：`go test -p 1 ./internal/mediaagent -run 'TestSameLengthMediaMutationRejectsReadAndExtraction|TestPausedUploadCanBeObservedCancelledAndCleaned|TestSameUploadOperationChangedBodyConflicts' -count=1 -timeout=30s`，退出码 1；实测相同长度改写 GET 返回 200、暂停接收时操作 GET 返回 404。随后修复并通过 Windows 媒体 Agent 包测试，包括暂停上传取消、接收中断重启清理、锁住文件导致删除失败后的重启/显式重试、相同 operationId 不同内容冲突和关闭进程停止接收。
- `go test -p 1 ./internal/mediaagent -count=1 -timeout=120s`，提交前最终运行退出码 0。
- `TEST_FFMPEG` 和 `TEST_FFPROBE` 指向 `D:/Project/cvte/runtime/ffmpeg/ffmpeg-9.0.2-essentials_build/bin/` 中实际 FFmpeg 9 程序，`TestActualFFmpegExtraction` 退出码 0，真实生成 MP4 并提取 3 张 JPEG。原独立 Windows 10 帧合成验收证据仍在 `D:/Project/cvte/runtime/p01-test-v2/mac-agent-independent-attempt2.txt`，本轮未用桩替换。
- `go test -p 1 ./... -count=1 -timeout=180s`、`go vet -p 1 ./...` 和 `go build -p 1 ./cmd/media-agent` 退出码 0。最终提交的同命令复核日志见 `D:/Project/cvte/runtime/p01-media-repair-evidence/`。

## 精确代码来源的产物

构建时使用 `-ldflags '-X main.version=b39d408be5e08e58fc89673ea56bd6595e732b7a'`，`GOTOOLCHAIN=local`、Go 1.25.5。产物均在仓库外。

| 目标 | 文件 | SHA-256 |
| --- | --- | --- |
| Windows amd64 | `D:/Project/cvte/runtime/p01-media-repair-evidence/media-agent-b39d408-windows-amd64.exe` | `0D46AAF10A2DC34372139BD31C9AFEF5BC6FE704F697BA3BB7B0BF413966B652` |
| Darwin arm64 | `D:/Project/cvte/runtime/p01-media-repair-evidence/media-agent-b39d408-darwin-arm64` | `E0841C2125F981D6F42A2F1D8B8D207E279C75CED9E32AC97287F61EE138A7D3` |

尚未在 Mac 上启动或联调修复包；不宣称 Mac/云端完整链路通过。旧 `4efd3e5` Mac 包保持原样，待 Leader 独立复核后重新打包与激活。没有将真实媒体、公司凭证或运行时目录提交到 Git。

## 第 2 轮：真实 HTTP 连接中断修复

独立复审在真实 `net/http` TCP 连接中复现：上传请求发送合法 multipart 文件头及部分视频 bytes 后停止发送，取消接口超过 1 秒、租约/总时限后仍为 running、`Agent.Close` 等待客户端主动断开。外部证据在 `D:/Project/cvte/runtime/p01-media-rereview/`。本轮先把三项真实 TCP 测试纳入 Agent 包，原 `b39d408` 上运行得到 3 项失败、退出码 1；随后修复并补充独立的租约到期及到期前 renew→cancel 测试。

修复代码提交：`0dff5895c87b2bc4ed2898cfecd8883f155b1378`。取消或到期时先通过 `http.ResponseController.SetReadDeadline` 使底层网络读取返回，再关闭请求体；若响应包装器不支持读 deadline，尝试 Hijack 并关闭连接，最后关闭请求体。已有 `httptest`/`io.Pipe` 退路仍通过。操作只在接收停止和 staging 清理后进入 cancelled/failed，租约到期保留 `TIMEOUT`，关闭时保留 `PROCESS_FAILED`。未更改上传意图、内容摘要、接收上限或原有清理归属逻辑。

验证：真实 TCP 的 cancel、总 deadline、仅租约到期、关闭和续租后取消 5 项通过；包含这些回归及实际 FFmpeg 9 MP4→JPEG 抽帧的 `go test -p 1 ./internal/mediaagent -count=1 -timeout=180s -v` 退出码 0；`go vet -p 1 ./internal/mediaagent ./internal/mediaapi ./cmd/media-agent` 退出码 0；两平台构建退出码 0。Windows 测试不代表 Mac 实机联调。直接使用标准 `net/http` 响应写入器的中断已由真实 TCP 证明；对同时不提供读 deadline 和 Hijack、且请求体 Close 无法唤醒读取的第三方包装器，没有通过本轮证明中断能力，部署时不可据此宣称支持。

第 2 轮构建使用 `-ldflags '-X main.version=0dff5895c87b2bc4ed2898cfecd8883f155b1378'`，Go 1.25.5，产物在仓库外：

| 目标 | 文件 | SHA-256 |
| --- | --- | --- |
| Windows amd64 | `D:/Project/cvte/runtime/p01-media-repair2-evidence/media-agent-0dff589-windows-amd64.exe` | `819308A5EDC93F42E1E5DC841ADA9141A42F6619E9F105D5C93F4539EA780F57` |
| Darwin arm64 | `D:/Project/cvte/runtime/p01-media-repair2-evidence/media-agent-0dff589-darwin-arm64` | `1EF90FC8A7F6A773EA166E53003A78C07F58F7FC25A9FE5D6D570782D8A7F5D0` |

本轮尚未在 Mac 启动或部署。必须先由 Leader 完成独立复审，再决定重新打包与激活；不能继续增加媒体修复轮次或重置全局计数。
