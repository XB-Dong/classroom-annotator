# 云端监控与恢复验收步骤

本文件是待执行步骤，不能代表已经部署或验收通过。以实际发布目录的 Compose、外部配置和完整 SHA 为准；不读取或提交 SSH 凭证。业务/监控通过已有授权 SSH 转发访问 `127.0.0.1:8180/9090/3000`。

1. Leader 在云端保存 `docker compose --env-file <external-env> ps --format json`、应用镜像 ID、已加载官方镜像来源 digest→实际 RepoDigest/ID 映射、`ffmpeg -version` 与 `dpkg-query -W ffmpeg`；不能只记录源码 HEAD。测试设置 `P0_EXPECTED_SHA` 为实际发布 SHA，先运行 API 与真实浏览器流程，确保生成过请求/任务指标。
2. 等待至少一次 30 秒 Prometheus scrape。设置外部 `P0_BASE_URL/P0_PROMETHEUS_URL/P0_GRAFANA_URL`（默认 loopback 上述端口）和 `P0_GRAFANA_TOKEN`，或 `P0_GRAFANA_USER/P0_GRAFANA_PASSWORD`。在 `tests/` 执行 `npm run monitoring`；脚本校验 ready 的精确 SHA、三个 targets、请求/任务/队列/内存样本及 Grafana readonly datasource/provisioned dashboard，不打印凭证或 API 返回全文。
3. Leader 实际打开 Grafana `classroom-p0` 仪表盘，确认请求、任务、主机面板有数据；保存脱敏截图/时间戳。静态配置或 Prometheus HTTP 200 不替代 Grafana实际数据检查。超过 5 分钟采集后检查 CPU rate/p95 图；无请求时某些 rate 可能为零，不把初始 NaN 当成容量数据。
4. Leader 记录云端 `docker stats --no-stream` 和主机可用内存/磁盘，在抽帧中及空闲时各取样；Compose 内存限制分别后端 768 MB、Nginx 64 MB、Prometheus 512 MB、Grafana 768 MB、node-exporter 64 MB。Prometheus 保留 `3d/512MB`，实际磁盘还包含 WAL。确认端口仅监听 loopback，日志轮转 `10m×3`，无 Docker socket 挂载。

恢复验证使用已完成的合成帧，不操作真实课堂数据：

1. 经 API 保存一个带框 completed 帧和一个显式零头 completed 帧。记录两份 GET annotations（含 revision/status/box），scoped progress，下载 ZIP 的 SHA-256，以及独立 ZIP reader 验证结果。保留 videoId/batchId/frameIds，但不记录数据物理路径或凭证。
2. Leader 停止后端，按照 `deploy/README.md` 从实际 `APP_DATA_VOLUME` 生成仓库外备份并计算 SHA-256，然后启动原后端。备份期间禁止其他写入，不复制仍在写的 SQLite/WAL。
3. 创建另一个新的外部卷，将备份恢复到新卷；保留原卷。Leader 修改外部环境 `APP_DATA_VOLUME` 指向恢复卷；Windows 生成的环境文件先规范为 LF，不能依赖 `sed ...$` 匹配 CRLF 行尾。先检查 `docker compose --env-file <external-env> config` 解析出的实际卷，再使用 `up -d --no-build --force-recreate backend`。必须用 `docker inspect` 明确核对新容器 `/data` 的 Mounts.Name 是新卷，不能把仍连接原卷的健康/数据检查当作恢复通过。确认实际挂载后才检查 health 精确 SHA，重读相同 frameIds 的 annotations/progress，与基准 JSON 深度比较。
4. 再次导出同一范围并运行独立 ZIP reader，验证每张 JPEG 字节、尺寸、COCO 关联和 bbox/area 与基准一致。ZIP 整包时间戳可能不同，应以 JSON 和图片内容比较，不能仅凭两个 ZIP 哈希不同判定恢复失败。
   可使用已通过浏览器主流程生成的仓库外 `restore-baseline.json`：设置 `P0_RESTORE_BASELINE/P0_BASE_URL/P0_EXPECTED_SHA`，执行 `npm run restore`。此脚本只观察健康/snapshot/progress/media 并请求导出，不执行停止、重启或切换卷；分别保存备份前和恢复后的命令/时间/退出码。
5. Leader 记录备份 SHA、源/恢复卷的脱敏标识、发布 SHA、检查命令/时间/退出码及回滚决定；若失败，回到保留的原卷，保留失败证据。验收结束只由 Leader 决定清理临时卷。

Test 不直接对任意部署执行重启。`npm run restart` 只管理自行启动的唯一本地后端子进程，使用全新的仓库外目录和临时 loopback 端口；它已经验证了本地 24096c0 的保存持久化与 interrupted 状态，但不替代以上云端备份/恢复门槛。
