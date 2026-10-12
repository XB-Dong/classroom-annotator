# P0-TEST-002 独立验收交接

状态 `partial`。只修改测试、统一检查、部署验收说明与本任务证据；没有修改 backend/frontend/规格。工作树清洁性核对后从 `aca2b23` 重置到 Leader 授权 `24096c099fdb7116ef968b4409ec710805e1634a`。读取本工作树 1.0.0 与 Leader 根目录未提交 api.md 1.0.1（dataset 配置路由和 X-API-Key）；没有写回共享契约。

真实环境：Windows，Node 24.16.0，Go 1.25.5，FFmpeg 9.0.2 essentials；在源码外构建后端，二进制 SHA-256 见 `execution.json`。每次使用全新的外部数据目录、临时 loopback 端口，最多一个由脚本管理的后端进程；结束后进程数实际为 0。测试/数据/缓存均在源码外；没有读取 SSH 凭证或重启云端。

实际检查与原始证据位于 `docs/audit/evidence/P0/P0-TEST-002/`：

- `local-api.txt`：原 5 个 API 测试执行，3 通过/2 失败，退出 1。首次复合测试在第一个无效 selector `{}` 返回 422 时停止；初步消息曾误描述为空 frameIds，后续已经更正并分别验证所有形状。
- `api-valid-export.txt`：拆分 selector 断言后，合法完整 API 流程执行 1 通过，退出 0：上传尺寸/时长、毫秒目标与数量、非法/重复框、revision409 无写入、独立批次保留、完成/撤销完成和进度、零头完成、独立 ZIP/COCO/JPEG 字节/尺寸/关联/bbox/area。
- `api-selectors.txt`：五个 selector 用例分别执行均失败，退出 1；`{}`、`{frameIds:[]}`、video+frameIds、batch-only、batch+frameIds 均得到 422 `INVALID_EXPORT_SELECTION`，预期 400。Leader 已确认需修复；断言保留。
- `dataset.txt`：真实后端访问本任务合成上游，分页 `page_size`/dataset42 路由/X-API-Key、保留 channel1/4、play-url、下载、媒体端无 API Key、unsafe ID 502 和上游失败任务均通过，退出 0。只证明材料驱动适配器，不证明真实公司环境。
- `restart.txt`：保存 completed 快照后杀停专用后端、同一目录重启；revision/框持久化、正在运行任务 failed/INTERRUPTED、未完成批次不发布均通过，退出 0。
- `storage.txt`：五个停止后的专用目录各执行独立只读 SQLite integrity/foreign keys、视频/帧媒体关联、JPEG 实际尺寸、published job completed、无孤儿/暂存检查，全部退出 0。此样本不能推翻 Astra 已识别的取消清理竞争条件。
- `harness-and-tools.txt`：语法、6 个校验器测试、13 个 Playwright 测试发现、npm audit 0 均退出 0；独立 ffprobe 合成视频为 320×240/25fps/4.000s；后端进程数 0。

另一失败为尾部任务 `failed/JOB_FAILED`，预期 `failed/FRAME_UNAVAILABLE`，属于已知 FFmpeg9 owner 修复项，本任务未循环重跑。没有将工具/依赖失败作为 Red；这里的 API 错误码 Red 是产品实际响应的有效证据。没有宣称任何后端修复 Green。

测试改进：annotation ID 全部改为每运行 UUID，避免持久云实例重复跑的全局 ID 冲突；将五种 export selector 与合法闭环分离，保留全部断言；专用进程默认临时端口；新增 `npm run local-api/storage/monitoring`。统一 verify 现在默认运行受控 local-api，额外部署 API 仍由 `--integration/-Integration` 控制；浏览器由 `--e2e/-E2E` 控制。`run-api.mjs` 启动自己的基础后端，不依赖外部 P0_BASE_URL；脚本结束停止该进程。配置的 FFmpeg/ffprobe 所在目录加入 PATH，让后端 Go integration 不因外部路径存在但 PATH 缺失而静默 skip。

云端监控/恢复操作检查表见 `deploy/ACCEPTANCE.md`；`monitoring-check.mjs` 核对实际发布 SHA、三个 Prometheus targets、真实请求/任务/队列/主机 samples、Grafana readonly datasource/provisioned dashboard，认证仅来自外部环境。此脚本尚未对云运行，Grafana API 返回字段与登录权限需在实际部署确认。备份/恢复计划保存基准 GET snapshot/progress、独立图像/COCO 内容，停止后端备份外部卷、恢复新卷并保留原卷；ZIP 整包时间戳变化不应误判数据损失。

未运行：前端/full verify/race（本任务）、云端 API/完整浏览器、监控脚本、云端备份恢复、GLM 实际审核/体验。真实公司 API `not_run`；最终 P0 判定归 Leader。当前模型仅有请求 Sol/high；未核实实际供应商参数，不声明 Leader xhigh。

下一步：Leader 合入测试工件并释放席位给 B2 selector 修复；B1 修复完成后新 Test dispatch 用实际云端 SHA 运行全套 API/E2E/monitoring，与 Leader 协作执行备份恢复并保留新的失败/成功证据。不得以此次本地成功替代云端或 GLM 门槛。
