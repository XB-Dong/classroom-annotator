# P0.1 媒体第 2 轮交付前收敛

第 2 轮主体 `0dff5895c87b2bc4ed2898cfecd8883f155b1378` 的原三项真实 TCP P1 已由独立审核复验关闭。复审发现受控响应包装器中 AfterFunc 回调可能晚于 handler 返回，属非阻塞 P2 生命周期问题；100 次标准入口 TCP 压力检查未复现生产错误。

Leader 在本轮尚未验收交付时进行一次限定收敛，未重置第 2/2 轮及全局计数：以完成 channel 同步已经启动的 Body 取消回调，handler 返回前等待其退出；保存独立审核的受控回归测试。原报告与失败日志未改写。

独立受控探针原始 exit 1 位于 `runtime/p01-media-rereview/callback-lifetime-result.txt`。Leader 自己的首次修复前探针 exit 0，保留为 `runtime/p01-media-final-evidence/lifetime-prepatch-attempt1.txt`，不将其称作 Red。修复后生命周期及真实 TCP 用例、完整 mediaagent 包回归、vet 全部 exit 0；另显式设置 TEST_FFMPEG/TEST_FFPROBE，实际 FFmpeg 9 抽取 3 帧退出码 0，未以缺工具时的 skip 冒称通过。

证据：仓库外 `runtime/p01-media-final-evidence/{lifetime-green,full-tests,vet,actual-ffmpeg}.txt`。新的二进制须从本收敛提交的干净工作树构建，并重新核验原生 Darwin arm64 与摘要；旧 0dff 二进制不代表本次收敛版本。真实 Mac 验收仍未执行。
