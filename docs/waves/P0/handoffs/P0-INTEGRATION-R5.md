# P0 R5 整合与云端检查点

记录日期：2026-10-13。当前检查点不是 P0 最终完成声明。

Frontend 修复后提交 `08180d528b932acc846968e2e981cdaaf8cc7314` 经独立 Astra 复核 SPEC/QUALITY PASS：33 项测试通过，原五项问题及上传旧回调竞态均关闭。Backend B1 代码 `88b6f6d7159a8da073d70c8900843028154a83e2` 经独立 Astra 复核 SPEC/QUALITY PASS：清理失败、有限指标标签、真实 FFmpeg 和慢上传期限均通过；另外验证停顿下载与锁文件组合不会泄漏凭证或错误发布。B2 导出 selector 修复 `e042da97477fb26971b5753dcbcab2c6e9a74b6b` 有 23 项 HTTP 矩阵与 Red/Green；独立云端 API 再次验证五类无效参数 400。

Leader 在整合提交 `ee2c927aa99b3e30a84449c92d41dba18ffadf42` 执行统一 verify.ps1：2026-10-13T02:50:40+08:00 至 02:52:46+08:00，退出 0。实际包含前端 typecheck/lint/33 Vitest/build，后端 Go 格式/vet/tests/build/race，独立 6 个导出校验器、合成 API 导入、重启和 10 个 API 流程。原始安全日志在 `docs/audit/evidence/P0/P0-INTEGRATION/`。

云端发布 `64b65df305daebf09cf0b1563cf87750a714222b` 与上述 frontend/backend 源码一致；新增交接文件及 Dockerfile 离线构建兼容修正。包和 Linux 二进制 SHA 验证后启动。首次旧 Dockerfile 的 COPY --chmod 不支持 legacy builder，实际失败保留；改为 COPY 后 RUN chmod 0555，最终仍以 10001:10001 运行。

官方镜像 manifest/layers/diffIDs 已在允许网络验证并离线转移。导入后的标签和 config/image ID 均实际核对；Docker run 按记录中的 config digest 仍尝试联网，不将 config ID 冒充原始 manifest pin。部署使用已验证本地标签、pull_policy never，启动前检查 Image ID。镜像来源与导入 ID 两套记录保留。

云端五服务均 healthy。一次空闲观测约为 Backend 6MiB、Nginx 3MiB、Prometheus 22MiB、Grafana 153MiB、node-exporter 8MiB；主机可用约 3.1GiB。这是空闲快照，不是压力或 AI 推理测量。无 AI 服务。

云端独立 10 API、3 Playwright E2E、实际 Prometheus/Grafana 检查及新卷恢复后的标注/revision/进度/JPEG/COCO 比对通过，详见 P0-TEST-003。恢复曾因外部 Windows CRLF 导致 sed 不匹配而未切卷；通过挂载核对发现并修正，不将原卷可读误计作恢复成功。原卷保留，当前 `/data` 实际挂载 `classroom-p0-app-restored-20261013t0308`。

GitHub 已发布候选分支 p0/leader 的 `64b65df`，实际 CI [38294607717](https://github.com/XB-Dong/classroom-annotator/actions/runs/38294607717) completed/success。首次 Git Data API 发布途中遇到 502；重试从经服务端验证的已上传提交继续，没有改写历史、force push 或泄漏 Git 凭证。主分支尚未更新到产品版本，最终审核门槛完成后由 Leader 发布。

授权真实课堂样本只在 Windows 专用短时后端验证上传/抽帧：1920×1080、20 秒，目标 0/5000/10000/15000ms 四帧均成功。不创建训练标签，不上传云端或外部模型。公司真实 API 状态 `not_run`，不构成当前 P0 阻塞。

尚待：GLM 第二次实际产品体验及 Leader 逐项核验，发布/回滚核验、最终报告与主分支交付。主会话实际 Sol/high 与请求 xhigh 的偏差仍在 model-check.json 公开记录，用户选择尚未返回；没有把子 Agent 设置当作主会话切换。
