# 教室人员标注平台：初步开发计划

版本：0.2.0｜状态：讨论稿｜日期：2026-10-12
时区：Asia/Shanghai（UTC+08:00）
仓库：[XB-Dong/classroom-annotator](https://github.com/XB-Dong/classroom-annotator)

> For execution agents: After user review, prepare a task-level implementation plan. Use `superpowers:subagent-driven-development` for delegated work or `superpowers:executing-plans` for inline execution. Proposed paths and verification commands are not implemented functionality.

本文中的目录和检查入口均为拟建内容，确认后才进入具体实施。

**目标：** 先完成可迭代底座与人工标注闭环，再接 AI，最后完成扩展验收和答辩材料。
**架构：** 单仓库三模块；Go 是唯一业务数据入口；Python 推理可选；React 负责交互。云服务器负责运行与部署验证。
**技术栈：** React、TypeScript、Vite、Konva 候选、Go、Gin、SQLite、FFmpeg、Python、FastAPI、ONNX Runtime、Docker Compose、Nginx、Prometheus、Grafana、Vitest、Playwright、Go Test、Pytest。

## 1. 当前事实与讨论建议

已确认：单仓库、GitHub 开发、云端测试、清爽架构、P0/P1/P2、SDD/TDD、多 Agent、监控和审计。用户审阅文档改用中文，代码/注释/接口字段/执行指令保持英文。

已观察：GitHub 仓库原为空，已克隆到 `code/`；原始材料和模型评估在仓库外。云服务器此前 SSH 已连通，为 Ubuntu 22.04、2 vCPU、约 4GiB 内存；检查时 Docker/FFmpeg/Nginx 未安装。Nano/Tiny CPU 推理已跑通，不代表真实课堂准确率已验证。

新增材料：已读取仓库外 `cvte-handoff` 的接口说明、脱敏响应和媒体元数据；17 个清单文件通过大小/SHA-256 校验，8 张 PNG 尺寸核对为 1920×1080。包内为 20 秒、1080p、25fps、无音频视频及 8 张抽帧。材料说明了 Mac 内网实测链路，但本项目适配器尚未实现或完成实时联调。

未完成：项目自身公司接口的真实联调、智谱审核执行链路、夜间执行配置。本轮只提交规范、计划和讨论报告，不实现产品、不安装监控、不创建夜间日程。交接包注明样本仅用于 Windows 开发，不进 GitHub、个人云或第三方模型；本次不扩大使用范围。

建议待讨论：

- Codex 承担主要开发与 Leader；智谱复用为独立审核/体验 Agent。客户端验证通过后接入，不能用 GPT 自审冒充智谱交叉审核。
- 夜间目标截止为 **2026-10-13 08:30（北京时间）**；这是建议，尚未排程。
- 矩形框、原图像素坐标、COCO 图片 ZIP；初期单使用者流程。
- 云端通过 SSH 转发或已有 HTTPS 边界访问；具体入口在发布报告记录。

## 2. 波次范围与停止

| 波次 | 主要交付 | 不进入本波次的内容 |
| --- | --- | --- |
| P0 | 仓库底座、配置/迁移、契约、CI、视频/抽帧、框编辑、保存恢复、进度、COCO、基础监控、云端闭环及最小运行/恢复说明 | AI 推理、跟踪传播、复杂审核、YOLO 导出 |
| P1 | Nano 模型、候选框、阈值、人工审核、失败降级、效果/资源评估 | 自行训练、未经审核的自动训练标签 |
| P2 | 扩展全栈回归、修复、运行说明、演示脚本、架构与研发证据、答辩材料 | 超出考核的新业务平台 |

后续 AI 提效方向，仅列入候选，不扩展 P0 或在此展开设计：

- 自动预标注，减少重复框选。
- 可疑、需人工复检标记，定位到图片及具体标注框。
- 可视化识别成功率与可疑率，帮助发现质量问题。

后续统计口径基于人工复检/评估结果确定，不直接把模型置信度或候选框数量当作识别成功率；交接样本没有标签真值。

Leader 每轮基于证据决定继续本波次或结束。全部必须项通过才标记 `completed`。单项阻塞时保留该项状态并继续独立授权任务；只有波次完成、达到截止/预算、出现必须停下的风险，或已无可推进任务时，才报告 `completed`、`partial` 或 `blocked` 并停止等待审核。**完成 P0 不自动进入 P1。**

## 3. 仓库、架构与配置

拟建结构按需要增加，本轮不创建空壳目录：

```text
code/
  CONSTITUTION.md / PLAN.md / AGENTS.md / README.md
  frontend/src/app/
  frontend/src/features/{videos,annotation,export}/
  frontend/src/shared/{api,ui}/
  backend/cmd/server/
  backend/internal/{videos,extraction,annotations,exports}/
  backend/internal/platform/{config,sqlite,observability}/
  backend/migrations/
  ai-service/app/{api,inference}/
  ai-service/tests/
  specs/{product,architecture,data-model,coordinates,acceptance}.md
  specs/contracts/openapi.yaml
  docs/decisions/
  docs/agents/
  docs/waves/
  docs/audit/{logs,evidence,reports}/
  tests/{integration,e2e}/
  scripts/
  deploy/
  .github/workflows/
```

按能力划分，只有重复需求成立才提取共享代码。UI、业务规则、SQL 与外部命令分层；只在需要替换或测试的边界定义接口。接近 300 行的源码检查职责，不强制机械拆分。契约与依赖文件由 Leader 负责。

服务器地址、存储根目录、凭证、上传上限、并发、超时和阈值统一配置并校验。规范常量不滥用配置。依赖锁定实际版本，发布引用提交与镜像版本，避免浮动 `latest`。

运行数据/媒体/模型/监控放在仓库外目录或持久卷。缓存、安装工具、临时评估、worktree 不污染 `code/`。项目脚本和脱敏审计属于必要内容，不复制第三方工具源码。

## 4. 数据与 API 建议

| 实体 | 必要信息 |
| --- | --- |
| Video | 稳定 ID、来源、尺寸、时长、导入状态、内部存储引用 |
| ExtractionJob | ID、video ID、时间区间、间隔、状态/进度/错误、起止时间 |
| Frame | 稳定 ID、video ID、时间戳（毫秒）、原图尺寸、完成状态、revision |
| Annotation | 稳定 ID、frame ID、类别、原图 xywh、来源/审核状态 |

建议接口前缀 `/api/v1`，前端只访问 Go：

- 视频上传：`POST /videos`；公司列表与导入：`GET /dataset/lessons`、`POST /dataset/imports`。
- 抽帧：`POST /videos/{videoId}/extractions`；任务查询/取消：`GET /jobs/{jobId}`、`POST /jobs/{jobId}/cancel`。
- 帧列表：`GET /videos/{videoId}/frames`。
- 标注：`GET/PUT /frames/{frameId}/annotations`；保存携带 `expectedRevision`。
- 导出：`POST /exports`，明确选定的视频/帧范围。
- 运维：`/health/live`、`/health/ready`、`/metrics`。

OpenAPI 统一类型和错误码。错误响应有 `code/message/requestId`，不泄露 SQL、路径或外部凭证。

交接包已观察的公司结构为 `{type, items, total, stats}`，视频位于 `items[].videos[]`；按数字 `channel === 4` 选择，取 `catalog_file_id` 请求 `GET /api/v1/catalog-files/{catalog_file_id}/play-url`，其独立响应的 `url` 才是媒体地址。课节的 `source_page_url` 不是视频下载地址。列表/播放地址认证与媒体下载分开，不向媒体主机自动转发公司 API Key。

以观察结构构建使用虚构业务值的测试 fixture。真实连接与导入由获准内网环境验收，不能将 Mac 的上游接口记录当成本项目 Go 适配器已经通过。地址的长期时效与源流可变帧率均不作未经验证的假设。

坐标为原图 xywh；间隔使用秒、帧时间存毫秒。冻结区间端点、可变帧率、重复抽帧语义后实现。标注用稳定 ID，不使用数组下标作身份。

保存使用事务与 revision 比较，冲突返回 `409`。同帧串行保存，明确状态；小型浏览器草稿恢复时比较服务端 revision，不能静默覆盖。首个类别为 `head`；教师排除、遮挡框规则在真实样本到达后确认，不假称模型能区分师生。

## 5. P0 必须验收的内容

| ID | 行为 | 最低证据 |
| --- | --- | --- |
| P0-BASE-001 | 模块、配置、锁文件、迁移、统一检查入口和最小运行/恢复说明清晰 | 架构评审、构建/迁移检查；按文档启动/测试/备份恢复 |
| P0-VID-001 | 合成视频上传；损坏/超限被拒绝且清理 | 媒体验证与云端操作 |
| P0-VID-002 | 真实公司 API 筛选 channel=4 并导入授权视频 | 实际契约、适配测试、授权网络实测 |
| P0-FRM-001 | 间隔、帧时间、尺寸和数量准确 | 已知时长合成视频的 FFmpeg 集成检查 |
| P0-JOB-001 | 进度、失败、取消、超时与重启状态可解释 | 生命周期测试 |
| P0-ANN-001 | 画/移动/调整/删除框，缩放坐标准确 | 几何单测与交互测试 |
| P0-SAVE-001 | 保存、刷新、后端重启一致，拒绝旧 revision | SQLite/API 集成和恢复检查 |
| P0-PROG-001 | 未开始/进行中/完成明确，零人头可显式完成 | 状态转换测试 |
| P0-EXP-001 | COCO/图片关联、面积、数量、坐标正确 | 独立解析导出包 |
| P0-OPS-001 | 云端版本明确，持久化和回滚可验证 | 云端 SHA、重启与发布记录 |
| P0-OBS-001 | Prometheus 抓取、Grafana 业务/主机指标可看 | target 状态和查询结果 |
| P0-E2E-001 | 上传→抽帧→编辑→保存→刷新→导出 | 自动 E2E 与云端闭环 |
| P0-AUD-001 | 需求、任务、提交、验证及智谱独立审核可追溯 | 日志、交接、GLM 审核意见与 Leader 处置、报告 |

若公司 API 仍未取得真实结构/连接：继续其他独立任务，只测试已观察的脱敏结构，不编造兼容性。`P0-VID-002` 保持阻塞，完整 P0 不能标为完成。

## 6. SDD/TDD 如何留下真实证据

SDD 工件为 `specs/` 中的产品、架构、数据、坐标、OpenAPI 和验收。任务引用需求 ID、规格版本、基准 SHA；契约变更写短决策记录，并同步消费者检查。

TDD 优先验证坐标、非法框、revision 冲突、导出关联、任务生命周期。先确认测试因目标行为缺失而失败，再实现；若发生重构，重新验证。没有重构就如实记录，不编造三步故事。

追踪示例：`P0-SAVE-001 → revision spec → stale-save test → implementation commit → green evidence → report`。证据保留命令、起止时间、环境、退出码及对应工作树/提交；安装错误不充当有效 Red。

拟建验证入口 `scripts/verify.ps1` 与 `scripts/verify.sh` 调用同一组检查：前端类型/lint/Vitest/构建，后端格式/`go vet`/`go test ./...`、关键并发包 race 检查，契约与 SQLite/FFmpeg 集成、独立导出解析、Playwright 核心 E2E。P1 增加 Pytest/模型适配；P2 扩展全栈覆盖。云端另验实际部署 SHA 的业务闭环，不只重复本地单测。

这些入口尚未创建。文档变更只做文档校验，不虚构代码测试已经通过。

## 7. 多 Agent 的技术选择

| 方案 | 价值与代价 | 建议 |
| --- | --- | --- |
| Codex 原生子 Agent + Git/worktree + 文件状态 | 已有工具、独立上下文；当前含 Leader 最多四席，不能原生选 GLM | P0 默认 |
| Codex Leader + OpenCode 等成熟 GLM 客户端 | 独立模型审核减少同模型自审盲点；本次 Key、端点、模型与工具执行需实测 | 智谱优先承担审核/体验职责，验证后启用 |
| 自研编排平台 | 灵活，但消耗本来用于业务的开发时间 | P0 不采用 |

GLM 审核客户端先在仓库外临时任务证明：读取限定的可分享规格/代码，执行允许的检查或体验流程，输出可核实问题与交接，支持超时和凭证脱敏。审核模式默认只读；以后若承担编码，再验证受限修改和测试能力。只有文本意见时如实称为代码评审，不声称做过浏览器体验。

智谱接收可分享代码、规格、脱敏日志和合成测试结果；不发送交接包中的真实课堂视频/图片或公司凭证。

### 角色与所有权

| 角色 | 工作 | 文件边界 |
| --- | --- | --- |
| Leader | 契约、任务排序、评审集成、发布、继续/停止 | 共享规格、依赖、CI/deploy、状态与报告 |
| Product | 用户流程、例子、验收和范围 | 分配的产品草稿，Leader 合入 |
| Frontend | 画布、状态、草稿恢复与流程 | 分配的 `frontend/` 文件 |
| Backend | 视频、抽帧、持久化与导出 | 分配的 `backend/` 文件 |
| Test | 规格驱动的独立集成/E2E | 分配的 `tests/`，不同时修改实现者的测试 |
| GLM Reviewer/Experience | 使用智谱独立审查 GPT 实现，核对用户操作与潜在盲点 | 只读授权代码/合成演示，写独立审核工件 |
| AI Preannotation | P1 模型、候选框、资源/效果 | 分配的 `ai-service/` 文件 |

Leader 加最多三个 Worker，职责轮换席位；智谱客户端单独计入实际执行预算，不虚报为内置 GLM 子 Agent。工作树在仓库外，单文件同一时间只有一个写入者；共享依赖/契约由 Leader 合入。评审必须指出规格覆盖、测试与缺陷，不能只给主观评价。

交叉模型闭环：GPT 实现和测试 → 智谱独立审核/体验 → Leader 复现、核对规格并决定采纳/拒绝/暂缓 → 对采纳项修改和验证 → 必要时智谱复审。意见需要复现步骤、预期/实际行为、代码或界面位置、严重度及证据；Leader 记录处置理由，不以模型投票代替验证。

同一问题沿用修复预算，不进行无限自我博弈。智谱链路未接通时继续独立开发，但记录审核门槛未满足，不能把 Codex 自审标成已完成的跨模型审核。

### 上下文、soul、记忆与交接

| 工件 | 作用 |
| --- | --- |
| `docs/agents/<role>/ROLE.md` | 英文职责、质量标准、允许区域；所谓 soul 的可审查定义 |
| `docs/agents/<role>/MEMORY.md` | 已验证事实/决定/待确认项，每条有来源或提交 |
| `docs/waves/<wave>/tasks/<taskId>.md` | 自包含英文任务包，加载必要上下文，不盲目复制整段聊天 |
| `docs/waves/<wave>/handoffs/<taskId>-<timestamp>.md` | 英文成果、文件、提交、检查、阻塞、下一步、评审请求 |
| `docs/waves/<wave>/state.json` | Leader 单写的状态，可从 Git、工件与测试事实恢复 |

运行上下文可独立，但重启后必须重读记忆与交接。worktree 隔离代码、任务包隔离上下文、版本化工件保持连续性，不能只靠命名角色达到这些效果。

最小任务包包括 `taskId/waveId/roundId`、角色、实际 provider/model、基准 SHA、规格版本、允许路径、依赖、验收、时间/修复预算、评审者与交付格式。未知模型标明未确认，不猜测。

## 8. 夜间 P0 建议轮次与预算

建议最多八轮，每轮约 45 分钟，另留最多 90 分钟发布/回归缓冲；总预算最多 7.5 小时，且受早晨截止约束。提前通过则报告停止，不为填满夜间扩展需求。

| 轮次 | 目标 | 分工与门槛 |
| --- | --- | --- |
| R0 | 冻结规格、契约、数据范围、任务与验收 | Product/Test 输入，Leader 冻结，任务有 owner/依赖 |
| R1 | 骨架、配置、迁移、CI、合成 fixture | 前后端与独立测试并行；构建/健康通过 |
| R2 | 上传、已知公司适配、抽帧与帧列表 | 前后端/Test 分路径；上传到帧展示通过 |
| R3 | 四种框操作、几何和标注 API | 原图坐标与契约测试通过 |
| R4 | 保存恢复、进度、COCO | revision、恢复、独立导出解析通过 |
| R5 | 云端部署与基础监控 | Leader 发布；指定 SHA 的闭环/监控通过 |
| R6 | 错误输入、故障、取消、重启及跨模型审核 | 智谱独立评审，Leader 核实处置；修复与验收 |
| R7 | 仅补 P0 缺口并封存交付 | 完成/部分/阻塞报告后停止 |

轮次为建议顺序，Leader 可按真实依赖调整本波次任务，但必须记理由，不能删除验收或跨波次。同一任务最多两次修复重派；短暂网络错误最多两次退避重试；建议单任务最多 45 分钟。超限记录阻塞，继续独立工作，必要时拆成带交接的新任务。

后继任务必须记录 `rootTaskId`，继承同一失败的累计重试计数，并消耗原波次预算；拆分或改名不能重置次数/截止。真正改变解决路径时记录原因，由 Leader 检查，仍受波次总预算约束。

执行环境先明确：若用当前本地 Codex 编排，Windows 要开机、联网、应用在线并避免休眠；应用在云端不代表 Agent 已迁到云端。脱离笔记本运行需另验远程客户端。本轮没有创建日程；确认方案后再决定持续会话或绑定波次的恢复任务，不建无限定时编码循环。

## 9. 日志与报告协议

```text
docs/audit/logs/events-20261012T230000+0800.jsonl
docs/audit/evidence/<wave>/<taskId>/red-<timestamp>.txt
docs/audit/evidence/<wave>/<taskId>/green-<timestamp>.txt
docs/audit/reports/report-20261013T083000+0800.md
docs/waves/P0/state.json
```

事件记录 `timestamp/runId/waveId/roundId/taskId/actor/provider/model/event/outcome/commitSha/specVersion/evidenceRefs`。未知写明未知。日志从实际启用时开始，历史事件只能标注为回顾，不补造当时不存在的记录。

Worker 各有日志流，Leader 汇总并单写状态；记录领取、规格、Red/Green、提交、评审、发布、重试、阻塞与决策。日志追加并用 Git/文件哈希锚定，但不声称不可篡改，不记录内部思维。大量原始日志留在外部受控存储，仓库保留脱敏必要证据；敏感内容及其派生哈希不公开。

每个波次中文报告包含：时间/范围/实际角色模型；每轮成果；验收矩阵；实现/测试/部署 SHA；CI 和云端结果；架构决策；失败/未完成项；用户验收步骤、入口、数据范围、回滚办法；Leader 停止理由与下一波次建议。

实现/测试/部署 SHA 先记录；报告的封存提交在交付消息或后续事件引用，避免把自己的最终 SHA 写成同一提交的自引用。轮次交接持续记录，波次结束必须交付 `report-<timestamp>.md`。

## 10. Prometheus/Grafana 与资源预算

Compose 管理 Nginx、Go、Prometheus、Grafana 与主机 exporter；P0 不启动 AI。配置、datasource、dashboard provisioning 入仓库；数据、管理员密钥外置。初期一张仪表盘，不引入 Loki/Tempo/复杂告警平台。

| 服务 | 建议内存上限 | 约束 |
| --- | --- | --- |
| Nginx | 64MiB | 静态产物与代理 |
| Go + FFmpeg | 768MiB | 一项抽帧任务，单线程起步 |
| Prometheus | 512MiB | 30 秒抓取；3 天/512MB 数据块保留目标 |
| Grafana | 768MiB | 单实例，无图像渲染器 |
| Host exporter | 64MiB | 只读、最小挂载 |
| Python AI（P1） | 512MiB 起步 | 单 Worker，CPU 单线程 |

这是起始预算，不是容量证明。Prometheus 保留限制不覆盖全部 WAL/压缩临时空间，卷需留余量；按安装版本选有效配置。以 RSS/CPU/磁盘/OOM 实测调整，不能为了完成验收静默关掉监控。

仪表盘展示健康、请求耗时/错误、任务队列/耗时/结果、保存冲突/失败、导出结果、主机资源。标签用有限枚举，避免 ID、文件名和敏感信息。运行日志轮转，与研发审计分开。

主要在 CI 构建；云端串行验证。E2E runner 可从外部受控环境访问云端业务，避免小服务器同时跑构建、Chromium、FFmpeg 与监控。指标端口和管理面板采用 SSH 转发或受控代理，不无边界暴露。

## 11. GitHub、发布与早晨验收

任务分支完成相关测试，由 Leader 审查集成；CI 调用统一验证入口。云端发布记录 SHA，备份持久状态、应用迁移、检查版本、执行合成闭环。P0 提供最小安装、配置、启动、测试、备份/恢复和回滚说明；P2 扩展交付与答辩材料。提前定义迁移兼容与回滚方法，不能只回滚程序而忽略数据库。

Mac 从 GitHub 拉稳定版本再推内网 GitLab，保留提交历史；Actions、PR、secrets 不随 Git 搬运，必要时以同一脚本接 GitLab CI。Windows 无需连接内网 GitLab。

早晨交付清单：

- [ ] GitHub 提交、需求追踪与测试证据可核对。
- [ ] 云端入口及实际部署 SHA 可核对。
- [ ] 合成/授权视频的导入、抽帧、框编辑、保存恢复、COCO 可重做验收。
- [ ] 公司 API 单独说明通过或阻塞，不用演示代替。
- [ ] Prometheus target 和 Grafana 指标可查看。
- [ ] 中文波次报告列明所有缺口；Leader 停止，不继续 P1。

## 12. 下一步讨论与资料

智谱独立审核/体验是已确认要求。下一步讨论具体客户端和审核覆盖，再确认 P0 矩阵、截止预算、样本范围、标注政策和访问方式。确认后生成具体任务包与实施清单；不把尚未确认的执行建议当作已获批准。

本版已按用户要求增加智谱独立审核/体验职责及后续 AI 提效方向；具体客户端、审核覆盖和夜间执行配置仍需落实。交接资料留在仓库外，后续优先用其观察结构验证接口适配。

参考：[Codex 子 Agent](https://learn.chatgpt.com/docs/agent-configuration/subagents)、[后台任务条件](https://learn.chatgpt.com/docs/automations?surface=app)、[OpenCode providers](https://opencode.ai/docs/providers/)、[OpenCode CLI](https://opencode.ai/docs/cli/)、[Prometheus storage](https://prometheus.io/docs/prometheus/latest/storage/)、[Grafana 安装要求](https://grafana.com/docs/grafana/latest/setup-grafana/installation/)。GLM 本次工具链与监控全栈容量仍需验证。
