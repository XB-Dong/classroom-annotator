# P0-TEST-003 实际云端验收交接

本任务请求的实际云端 API、浏览器、监控和 Leader 控制的备份恢复观察全部通过。实际部署 SHA 为 `64b65df305daebf09cf0b1563cf87750a714222b`，各次 ready 检查精确相符；不是本地后端替代。最终 P0/GLM 决定仍归 Leader。

工作树核对清洁后重置到授权部署基准；读取 API 1.0.1/验收 1.0.0。只改测试、部署验收操作说明和本任务证据；没有改 frontend/backend/共享规格，也没有直接停止、重启或切换云端卷。模型只有 Sol/high 请求依据，实际参数未独立核实；不声明 Leader xhigh。

实际结果与原始命令/时间/退出码位于 `docs/audit/evidence/P0/P0-TEST-003/`：

| 检查 | 实际结果 | 证据 |
| --- | --- | --- |
| 云端 API | 10/10 通过，退出 0 | cloud-api.txt |
| 核心真实浏览器流程、延迟保存 ack、新编辑保留、冲突显式解决 | 3/3 通过，退出 0 | cloud-e2e-job-wait.txt |
| 实际监控 | 三个 target up；请求/任务/队列/主机 samples；Grafana readonly/provisioned 通过 | cloud-monitoring.txt |
| 备份前基准观察 | 2 个 completed 帧（1 头/零头）、revision、progress、JPEG hash、COCO 语义通过 | restore-before-backup.txt |
| 确认实际恢复新卷后的观察 | 全部基准保持，退出 0 | restore-after-new-volume.txt |
| 恢复后 ready/targets | 精确部署 SHA，backend/prometheus/node up，退出 0 | post-restore-readiness.txt |
| JS 语法/独立校验器 | 退出 0，6 单测通过 | harness-checks.txt |

API 覆盖合法上传/尺寸/时长、损坏/非法请求、准确目标/数量、批次保留、非法/重复框、revision409 无写入、完成/撤销完成/零头、独立 ZIP/JSON/JPEG 的 byte/dimension/ID/association/bbox/area、五种 selector400、取消不发布及稳定终态、不可用尾部 FRAME_UNAVAILABLE。浏览器完整流程确实经过 UI 上传→抽帧→画框→移动→调整→删除→串行自动保存→刷新→两帧显式完成→下载 ZIP；API 仅观察/独立验证主流程，另外两个竞态测试用 API 准备帧及模拟另一写入者。

保留的失败与修复分类：

1. 初次浏览器未能启动：Playwright1.64.0 需要 headless-shell1248，而现有缓存是1247。`cloud-e2e.txt` 保留实际错误；它属于工具失败，不算产品 Red，也不能称浏览器执行。安装匹配版本到已配置仓库外缓存成功（Chromium156.0.8078.4/1248），没有改变锁文件或硬编码机器路径。
2. 匹配浏览器执行时 2 个保存测试通过，核心测试失败。原始 trace 证明测试在 job queued 时请求 frames?batchId 得到 404，而规格仅公开 completed 批次；然后 harness 读取不存在的 frames.length。`cloud-e2e-matching-browser.txt` 与外部 trace/失败截图保留。这是 harness 时序问题，不是产品缺陷。
3. 单次 harness 修复使用现有 terminalJob 观察任务 completed 后再检查 200/3 帧，所有 canvas/save/export 断言保留。重新完整跑 3 项全通过。任务根的修复预算由 Leader 继承记账，没有重置预算或修改产品行为。

成功截图与恢复基准只在仓库外；`artifact-index.json` 保存路径/大小/SHA-256。Test 已实际查看成功 screenshot：选中第一帧，显示已保存、completed、1 个人头框及批次进度 2/3。没有把 PNG/视频/ZIP 或凭证提交到 Git。

恢复基准：video `911c61fa-e29b-4778-83e9-cd42e769690d`，batch `dfbeff94-6a86-4603-8135-16c6ae582b59`；frame `f37593aa-1bec-4c00-8204-ac9aac01d148` revision6/1头与 `d18a290d-b2dd-4446-880c-deabd9fc047a` revision1/零头均 completed。基准还含 scoped progress 3/2、完整 COCO 与两张 JPEG SHA-256。

Leader 确认实际新卷 `classroom-p0-app-restored-20261013t0308` 已挂载后才执行 post-restore；原卷 `classroom-p0-app` 保留。Leader 提供备份 SHA-256 `880a0042528fb549277e7ddaa9c38e0c43b19c581cd05884738f1507d61dd5ba` 与外部 operator 原始证据。Leader 前两次操作因 Windows CRLF/sed 没有切换卷，**不计入恢复通过**；第三次规范 LF、检查 Compose config、force-recreate、inspect Mounts.Name 确认为新卷。本任务只在这份确认之后观察，没有覆写 Leader 原始记录。

新增 `npm run restore` 为只观察脚本：检查 health/snapshot/progress/JPEG hashes，并请求导出后独立解析，比较 COCO IDs/关联/几何；不执行重启/切换卷。恢复后没有新增帧、任务或标注写入。Prometheus 进程内 jobs counters 会随后端重启初始化，因此恢复后只补 ready/targets 检查；完整 samples/Grafana 检查的真实通过证据来自备份前的 `cloud-monitoring.txt`，不伪造重启后完成过新任务。

`deploy/ACCEPTANCE.md` 已补充 LF 外部 env、Compose 实际卷核对、`--force-recreate`、docker inspect `/data` Mounts.Name 和前后观察命令，避免“旧卷仍在而数据可读”被误当成恢复证明。Grafana 凭证只从指定外部文件的两个允许字段读入过程环境，用后清理，没有打印值或环境全文。

边界：本任务没有代替 GLM 实际代码审核/浏览器体验，没有运行真实公司 API（`not_run`），没有将 Leader 报告的 common verify/race 当作自己的执行证据。本任务仅为实际 cloud64 的验收证据；Leader 的其他门槛与最终状态需按其原始证据汇总。
