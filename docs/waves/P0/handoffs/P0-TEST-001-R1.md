# P0-TEST-001 R1 测试与部署准备交接

状态：准备工件完成；产品验收 `not_run`。基准 `db1f5c5`，冻结规格 1.0.0（Leader 基准 `77a54bf`）；任务模型请求 Sol/high、fresh context，实际模型参数未独立核实。没有修改前端、后端或共享规格。

工件：独立 `tests/package.json/package-lock.json`、Playwright API/真实 UI 流程、独立 ZIP/JPEG/COCO 读取器、6 个校验器测试、外部 FFmpeg 合成生成脚本、专用后端进程的重启/持久化与公司 API 合成联调脚本；统一 PowerShell/Bash 检查和 CI；artifact-only Dockerfile、Compose、Nginx、Prometheus/Grafana、来源镜像锁、外部配置示例与备份恢复说明。

检查证据见 `docs/audit/evidence/P0/P0-TEST-001/harness-checks.txt`、`preparation.json`。实际通过：JS 语法、6 个独立校验器单测、发现 5 个 API + 3 个浏览器测试（仅发现）、npm audit 0、Docker Compose config、部署 JSON、PowerShell parser、git diff whitespace。Compose config 不需要启动 Docker daemon。没有运行产品 API、专用进程脚本、完整 verify、镜像构建或云端 E2E；不能据此声称任何产品门槛通过。

| 门槛 | 准备覆盖 | 当前产品验证 |
| --- | --- | --- |
| P0-BASE-001 | 统一脚本、部署 SHA、恢复步骤 | 未运行 |
| P0-VID-001 | 元数据、损坏上传、非法抽帧 | 未运行 |
| P0-VID-002A | observed dataset/path/page_size/X-API-Key、play URL、媒体 key 隔离、unsafe ID、上游失败 | 未运行 |
| P0-FRM/JOB-001 | 精确目标/数量、批次保留、取消、FRAME_UNAVAILABLE、重启 INTERRUPTED | 未运行 |
| P0-ANN/SAVE/PROG-001 | 非法框/重复 ID、409 无写入、延迟 ack 后新编辑、UI 冲突、刷新、完成撤销、零头完成 | 未运行 |
| P0-EXP-001 | 独立 ZIP/JSON/JPEG 的 ID、关联、尺寸、字节、bbox/area、选择器错误、未完成排除 | 校验器 6 单测通过，产品未运行 |
| P0-OPS/OBS-001 | 资源上限、loopback 端口、健康、日志轮转、监控面板 | 配置验证通过，实际部署未运行 |
| P0-E2E-001 | UI 上传→抽帧→画/移/调/删→自动保存→刷新→两帧完成→ZIP | 未运行 |
| P0-AUD/REV-001 | 独立原始检查记录；GLM 单独门槛 | GLM 不由本任务替代 |

运行方法详见 `tests/README.md`、`deploy/README.md`。统一脚本默认执行前端类型/lint/Vitest/build、Go format/vet/test/build/race 与 FFmpeg/SQLite 专用进程集成；`--integration/-Integration` 访问真实部署 API；`--e2e/-E2E` 增加浏览器。交付必须在实际云端运行浏览器，不能将可选执行参数解释为可选门槛。race 显式跳过会输出未运行，需另补证据。

接续关注：媒体包必须在外部目录；Windows 当前只有 Playwright 裁剪 FFmpeg，生成尝试报缺少 lavfi，属于工具失败。Leader 已从云端生成并下载合成视频，Test 核对 22696 字节及 SHA-256（见 JSON）。未记录产品 TDD Red，不能把安装/工具失败算 Red。

测试对合作点的假设必须在真实运行中确认：使用已读到的 `video-select`、`save-state/conflict-reload` 等稳定 testid，画布以 `.konvajs-content` 边界计算坐标；首次保存 revision 0→1；重复 annotation ID 预期 422；损坏上传测试目前要求请求失败且视频列表不变；尾部无帧报 FRAME_UNAVAILABLE。取消任务用 300 帧保持可取消窗口，但终态完成竞争在规格中合法，若测试抢不到窗口应隔离调度观察后调整输入，不能更改产品规则。

Leader 已验证官方五个 linux/amd64 镜像来源及下载内容。离线 docker-load 的 RepoDigest 可能与来源不同；当前 Compose 默认保存来源 digest，需用环境文件覆盖为实际 daemon 引用并记录映射。请确认 nginx curl、Prometheus/Grafana/node-exporter wget 可用；这些镜像内的实际 healthcheck 命令未由 Test 执行。Backend 镜像明确 apt 安装 curl/FFmpeg。apt 包实际版本也需发布记录，尚未宣称 bit-for-bit 构建。

下一步：Leader 合入此准备提交、继续后端标注/导出；产品集成后新 Test dispatch 运行统一检查、合成 API/重启、云端 API/E2E 和监控/恢复，整理实际证据；GLM 代码审核及真实浏览器体验由独立任务完成。最终判定归 Leader，真实公司 API 仍 `not_run`。
