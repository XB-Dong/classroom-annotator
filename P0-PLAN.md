# P0计划

版本：0.6.0｜状态：用户已批示启动，执行状态见 P0 state｜日期：2026-10-13
时区：Asia/Shanghai（UTC+08:00）

执行记录：用户已批示“启动p0”，2026-10-13T01:30:22+08:00 开始，最晚 08:30 停止。下文关于“尚未启动”的段落保留为原计划基线，现行进度以 [P0状态](docs/waves/P0/state.json) 为准。
仓库：[XB-Dong/classroom-annotator](https://github.com/XB-Dong/classroom-annotator)

> For execution agents: Choices are frozen. Do not start P0 until the user explicitly authorizes execution. Then use native Codex subagents with external Git worktrees, and OpenCode for independent GLM code and browser-experience review. Convert this plan into task packets with SDD acceptance and meaningful TDD evidence. Stop at the agreed deadline or P0 delivery gate.

用户已确认此前 14 项推荐，并以 [验收范围决定](docs/decisions/0004-local-materials-acceptance.md) 替代其中第 05 项：P0 不以 Mac/真实公司环境联调为门槛。五个核心角色、按需架构审核及关键角色极高推理强度见 [角色与模型决定](docs/decisions/0003-lean-agent-roles.md)。本稿用于启动前审核，尚未实施或排程。目录和检查入口为交付要求，不是已存在的功能。

**目标：** 交付可迭代工程底座与人工标注闭环，具备基础测试、独立智谱审核、云端验证和监控。
**架构：** 单仓库三模块；P0 实现 React 前端与 Go 业务后端，保留第三模块的架构边界，不实现或启动推理服务。Go 是唯一业务数据入口。
**技术栈：** React、TypeScript、Vite、React Konva、Go、Gin、SQLite、FFmpeg、Docker Compose、Nginx、Prometheus、Grafana、Vitest、Playwright、Go Test。研发编排固定为 Codex 原生子 Agent + OpenCode 智谱审核。

## 1. 当前事实与启动状态

已确认：单仓库、GitHub 开发、云端测试、清爽架构、P0/P1/P2、SDD/TDD、多 Agent、监控和审计。用户审阅文档改用中文，代码/注释/接口字段/执行指令保持英文。

已观察：GitHub 仓库原为空，已克隆到 `code/`；原始材料和模型评估在仓库外。云服务器此前 SSH 已连通，为 Ubuntu 22.04、2 vCPU、约 4GiB 内存；检查时 Docker/FFmpeg/Nginx 未安装。Nano/Tiny CPU 推理已跑通，不代表真实课堂准确率已验证。

新增材料：已读取仓库外 `cvte-handoff` 的接口说明、脱敏响应和媒体元数据；17 个清单文件通过大小/SHA-256 校验，8 张 PNG 尺寸核对为 1920×1080。包内为 20 秒、1080p、25fps、无音频视频及 8 张抽帧。材料说明了 Mac 内网实测链路，但本项目适配器尚未实现或完成实时联调。

P0 待验证的事实：OpenCode 智谱工具/浏览器链路、完整监控栈容量与本项目适配器的材料驱动集成。这些是执行检查，不是未定选型；验证失败必须报告，不能伪造通过或擅自换方案。项目自身公司接口的真实环境联调留到开发完成后，不影响 P0 完成判定。交接包中的真实样本仅用于 Windows 开发，不进 GitHub、个人云或第三方模型。

已冻结决定：Codex Leader/开发、OpenCode 智谱独立代码审核及合成数据浏览器体验；Windows 编排，云端运行/测试；截止 **2026-10-13 08:30**，最长 7.5 小时；单使用者、React Konva、矩形框、COCO 图片 ZIP；P0 使用 SSH 转发访问。完整 14 项决定见 [确认记录](docs/decisions/0001-confirmed-choices.md)。

## 2. P0 范围与交付状态

P0 包含底座、配置/迁移、契约、CI、视频/抽帧、框编辑、保存恢复、进度、COCO、独立智谱审核、监控、云端闭环及最小运行/恢复说明。不开发 AI 推理、跟踪传播、复杂审核产品或 YOLO 导出；后续内容集中在 [后续波次前瞻](NEXT-WAVES.md)。

P0 以本地视频、已提供接口材料和合成测试环境验收，不要求 Mac 联调或公司真实环境连通。当前全部 P0 门槛通过可交付 `status=completed`；未通过则为 `partial`/`blocked`。另记录 `realApiVerificationStatus=not_run`，明确公司 API 真实环境尚未验证；此字段是后续事实记录，不作为本次完成门槛，也不将模拟测试冒充真实联调。

Leader 每轮判断本波次是否继续。单项阻塞继续独立授权工作；核心可交付、达到截止/预算、出现必须停止的风险或已无可推进任务时，生成报告停止，等待用户审核。不得自动开始后续波次。

## 3. 仓库、架构与配置

拟建结构按需要增加，本轮不创建空壳目录：

```text
code/
  CONSTITUTION.md / P0-PLAN.md / NEXT-WAVES.md / AGENTS.md / README.md
  frontend/src/app/
  frontend/src/features/{videos,annotation,export}/
  frontend/src/shared/{api,ui}/
  backend/cmd/server/
  backend/internal/{videos,extraction,annotations,exports}/
  backend/internal/platform/{config,sqlite,observability}/
  backend/migrations/
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

## 4. 数据与 API 契约

| 实体 | 必要信息 |
| --- | --- |
| Video | 稳定 ID、来源、尺寸、时长、导入状态、内部存储引用 |
| ExtractionJob | ID、video ID、时间区间、间隔、状态/进度/错误、起止时间 |
| Frame | 稳定 ID、video ID、batch ID、时间戳（毫秒）、原图尺寸、完成状态、revision |
| Annotation | 稳定 ID、frame ID、类别、原图 xywh、来源/审核状态 |

接口前缀固定 `/api/v1`，前端只访问 Go：

- 视频上传：`POST /videos`；公司列表与导入：`GET /dataset/lessons`、`POST /dataset/imports`。
- 抽帧：`POST /videos/{videoId}/extractions`；任务查询/取消：`GET /jobs/{jobId}`、`POST /jobs/{jobId}/cancel`。
- 帧列表：`GET /videos/{videoId}/frames`。
- 标注：`GET/PUT /frames/{frameId}/annotations`；保存携带 `expectedRevision`。
- 导出：`POST /exports`，明确选定的视频/帧范围。
- 运维：`/health/live`、`/health/ready`、`/metrics`。

OpenAPI 统一类型和错误码。错误响应有 `code/message/requestId`，不泄露 SQL、路径或外部凭证。

交接包已观察的公司结构为 `{type, items, total, stats}`，视频位于 `items[].videos[]`；按数字 `channel === 4` 选择，取 `catalog_file_id` 请求 `GET /api/v1/catalog-files/{catalog_file_id}/play-url`，其独立响应的 `url` 才是媒体地址。课节的 `source_page_url` 不是视频下载地址。列表/播放地址认证与媒体下载分开，不向媒体主机自动转发公司 API Key。

以交接包实际观察结构构建使用虚构业务值的测试 fixture，通过合成 API/媒体服务验证本项目 Go 适配器的解析、分页、学生视频筛选、播放地址获取与导入。保留可配置的真实 API 接入能力，不硬编码响应或样本路径。真实环境连接/导入留到开发完成后由实际部署环境试验，P0 不要求 Mac 操作；上游材料与合成测试不证明真实联调已通过。地址的长期时效与源流可变帧率均不作未经验证的假设。

坐标为原图 xywh；间隔使用秒、帧时间存毫秒。区间端点/可变帧率的工程规格在 R0 定义并测试。重新抽帧一律创建新批次，保留旧帧与标注；列表/导出明确所属批次。标注用稳定 ID，不使用数组下标作身份。

保存使用事务与 revision 比较，冲突返回 `409`。同帧串行保存，明确状态；小型浏览器草稿恢复时比较服务端 revision，不能静默覆盖。类别固定 `head`，只标学生，排除教师；部分遮挡框住可见头部范围，完全不可见者不标，不推测身份或隐藏头部。

COCO 导出仅包含用户选定范围内的已完成帧及图片，显式完成的零人头帧也包含。未完成帧不导出；框坐标与原图对应。

## 5. P0 必须验收的内容

| ID | 行为 | 最低证据 |
| --- | --- | --- |
| P0-BASE-001 | 模块、配置、锁文件、迁移、统一检查入口和最小运行/恢复说明清晰 | 架构评审、构建/迁移检查；按文档启动/测试/备份恢复 |
| P0-VID-001 | 本地视频上传；损坏/超限被拒绝且清理 | Windows 授权本地样本检查；云端合成视频操作与媒体验证 |
| P0-VID-002A | 公司适配器按已观察结构解析、分页、筛选 channel=4、获取媒体地址并导入 | 现有接口材料驱动的合成 API/媒体集成检查，无需真实公司环境连通 |
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
| P0-REV-001 | 智谱完成代码审查与合成数据核心流程浏览器体验，Leader 处理意见 | 实际模型、工具运行、复现步骤及采纳/拒绝/暂缓记录 |

上表所有条目均为本次 P0 门槛；智谱审核、公司适配器材料驱动测试、云端闭环和监控不能缺省。真实公司环境联调不在本次验收范围，按后续事实单独记录。完成声明必须注明本地/合成材料验收范围，不宣称真实 API 已通过。原始视频的使用许可不因验收范围调整而扩大。

## 6. SDD/TDD 如何留下真实证据

SDD 工件为 `specs/` 中的产品、架构、数据、坐标、OpenAPI 和验收。任务引用需求 ID、规格版本、基准 SHA；契约变更写短决策记录，并同步消费者检查。

TDD 优先验证坐标、非法框、revision 冲突、导出关联、任务生命周期。先确认测试因目标行为缺失而失败，再实现；若发生重构，重新验证。没有重构就如实记录，不编造三步故事。

追踪示例：`P0-SAVE-001 → revision spec → stale-save test → implementation commit → green evidence → report`。证据保留命令、起止时间、环境、退出码及对应工作树/提交；安装错误不充当有效 Red。

验证入口 `scripts/verify.ps1` 与 `scripts/verify.sh` 调用同一组检查：前端类型/lint/Vitest/构建，后端格式/`go vet`/`go test ./...`、关键并发包 race 检查，契约与 SQLite/FFmpeg 集成、独立导出解析、Playwright 核心 E2E。云端另验实际部署 SHA 的业务闭环，不只重复本地单测。

这些入口尚未创建。文档变更只做文档校验，不虚构代码测试已经通过。

## 7. 已确定的多 Agent 执行机制

固定采用 **Codex 原生子 Agent + 外部 Git worktree + 版本化文件状态** 开发，**OpenCode + 智谱** 做独立审核；不引入自研编排平台。OpenCode 安装、配置、认证、缓存与临时验证放在仓库外。

R0 在仓库外合成测试页面上验证 OpenCode/智谱认证、读取限定代码、受限工具、浏览器能力、交接、超时和脱敏；此时产品尚未实现，不要求操作产品核心流程。R1–R5 在已完成契约/模块上分批进行智谱审核，可用的实际云端流程先行体验；R6 汇总并补齐实际云端核心闭环体验，才完整满足 `P0-REV-001` 的体验部分。审核模式只读产品源码，允许写分配的审核工件。只有代码文本意见而未实际操作浏览器时，体验门槛不算通过。客户端选型固定，具体可用 GLM 模型由公司 Key 实测后记录，不编造型号。

智谱接收可分享代码、规格、脱敏日志和合成测试结果；不发送交接包中的真实课堂视频/图片或公司凭证。

### 角色与所有权

| 角色 | 工作 | 文件边界 |
| --- | --- | --- |
| Leader | 兼产品澄清；契约、调度、审核处置、集成、发布与验收 | 共享规格、依赖、状态与最终报告；CI/deploy 共享文件可单写委派准备 |
| Frontend | 画布/状态/恢复；实现及组件/交互测试；前端部署准备 | 分配的 `frontend/`、相关测试与部署文件 |
| Backend | 视频/抽帧/保存/导出；实现及单元/集成测试；后端部署准备 | 分配的 `backend/`、相关测试与部署文件 |
| Test / Acceptance | 独立验收、集成/E2E、证据索引和报告草稿 | 分配的 `tests/` 与审计草稿，不覆盖实现者测试或原始证据 |
| GLM Reviewer/Experience | 使用智谱独立审查 GPT 实现，核对用户操作与潜在盲点 | 只读授权代码/合成演示，写独立审核工件 |
| Architecture Reviewer，按需 | 架构/核心契约、坐标/保存一致性和复杂跨模块问题审核 | 只读规格与代码，写分配的审核工件 |

P0 固定五个核心职责，另有一个按需专项审核；取消独立 Product 和 Evidence Assistant。角色表示职责，不代表每个角色一直运行或每轮都启动。P1 的 AI Integration 是按任务启用的专项 Worker，P0 不运行。

Leader 加最多三个 Worker，职责轮换席位；智谱客户端单独计入实际执行预算，不虚报为内置 GLM 子 Agent。工作树在仓库外，单文件同一时间只有一个写入者；共享依赖/契约由 Leader 合入。评审必须指出规格覆盖、测试与缺陷，不能只给主观评价。

交叉模型闭环：GPT 实现和测试 → 智谱独立审核/体验 → Leader 复现、核对规格并决定采纳/拒绝/暂缓 → 对采纳项修改和验证 → 必要时智谱复审。意见需要复现步骤、预期/实际行为、代码或界面位置、严重度及证据；Leader 记录处置理由，不以模型投票代替验证。

同一问题沿用修复预算，不进行无限自我博弈。智谱链路未接通时继续独立开发，但记录审核门槛未满足，不能把 Codex 自审标成已完成的跨模型审核。

### P0 固定模型与推理强度

以下是已确认的目标配置，不是已运行证明；以 [决策 0003](docs/decisions/0003-lean-agent-roles.md) 为角色/模型配置唯一权威来源，替代决策 0002。

| 角色 | 模型 | 推理强度 | P0 使用重点 |
| --- | --- | --- | --- |
| Leader | `gpt-6.1-sol` | `xhigh`（极高） | 兼产品澄清、契约/调度、证据核对、审核处置与最终验收 |
| Frontend | `gpt-6.1-sol` | `high` | 画布、状态、保存恢复与流程 |
| Backend | `gpt-6.1-sol` | `high` | 抽帧、事务/revision、导出 |
| Test / Acceptance | `gpt-6.1-sol` | `high` | 独立测试/验收、证据整理与报告草稿 |
| GLM Reviewer/Experience | 公司 Key 实测可用的智谱模型，R0 固定准确 ID | 只使用供应商实际支持的设置 | 独立代码审核和实际浏览器体验 |
| Architecture Reviewer，按需 | `gpt-6-astra` | `xhigh`（极高） | R0 底座/契约，R3/R4 坐标与保存一致性，复杂跨模块阻塞 |

Architecture Reviewer 按任务启用，占原生 Worker 席位，不增加并发上限。Leader 与专项审核使用 `xhigh`；开发/测试默认 `high`。不自动提升至 `max/ultra`，不因失败切换模型或改任务名逃避修复预算。Luna 不列入当前执行角色。

R0 将确认的模型配置落地为项目专用英文 Agent 定义/启动参数，进行最小启动验证；本轮只更新文档，不创建或宣称已经生效的运行配置。Leader 主会话需单独选为 `gpt-6.1-sol` / `xhigh`；子 Agent 设置不会替主会话切换。无法确认主会话配置时记录未确认，不能凭模型自述判断。

当前原生工具中，显式覆盖子 Agent 模型/推理强度使用 `fork_turns="none"`，加载自包含任务包；完整历史继承模式不支持此覆盖。启动时分别记录请求配置与可验证的实际配置，禁止把预期型号写成已验证实际型号。选定模型不可用时停止依赖该角色的任务，记录阻塞并推进独立任务，不静默替换型号或降低验收。

GLM 经 OpenCode 外部调用，不伪装为 Codex 内置模型；其准确模型 ID、浏览器/工具兼容性是 R0 的实测项。P1/P2 的使用重点仅在 [后续波次前瞻](NEXT-WAVES.md) 保留，不因此启动未来任务。

### 分阶段调度与负载责任

以下是启动前的负载预估，不是已运行的均衡证明。Leader 每轮用任务队列、依赖、阻塞和剩余预算决定席位，不要求各角色工时相同。

| 阶段 | 调度与责任 |
| --- | --- |
| R0 规格 | Leader 兼产品澄清；Test 提前定义独立验收；Astra 一次性审核架构/契约。前后端只推进不依赖未冻结契约的准备 |
| R1–R4 开发 | 通常 Frontend/Backend/Test 三个 Worker 并行。实现者承担自身 TDD，Test 提前设计验收并独立验证；智谱分批审核已完成工件 |
| R3/R4 核心一致性 | Astra 按任务核对坐标转换、保存/revision 与导出一致性，占用空闲或完成任务释放的席位，不与其他 Worker 超额并发 |
| R5/R6 集成 | 开发者准备构建、部署和监控配置；Test 验证指定 SHA 的闭环/监控并整理证据；Leader 审核、合入和实际发布，智谱完成产品体验 |
| R7 交付 | Test 汇总验收矩阵、证据索引和报告草稿；Leader 核实结论、最终签发报告并停止 |

后端任务优先按“视频导入/抽帧”和“标注持久化/导出”拆成自包含能力包；依赖可满足且有空闲席位时可启用两个 Backend Worker，分别拥有不同路径。仍最多三个原生 Worker；不得挤掉必需的独立验收或跳过测试。共享路由、迁移序号、契约和依赖由 Leader 协调单写后集成。

部署准备按任务包明确单文件 owner：Frontend 准备前端镜像/Nginx，Backend 准备后端镜像/FFmpeg；Compose、CI、Prometheus/Grafana 等共享配置由 Leader 指定一个 Worker 准备，交接后才切换 owner。Leader 保留共享契约和最终发布责任，避免包办全部实现/配置/证据整理。

### 上下文、soul、记忆与交接

| 工件 | 作用 |
| --- | --- |
| `docs/agents/<role>/ROLE.md` | 英文职责、质量标准、允许区域；所谓 soul 的可审查定义 |
| `docs/agents/<role>/MEMORY.md` | 已验证事实/决定/待确认项，每条有来源或提交 |
| `docs/waves/<wave>/tasks/<taskId>.md` | 自包含英文任务包，加载必要上下文，不盲目复制整段聊天 |
| `docs/waves/<wave>/handoffs/<taskId>-<timestamp>.md` | 英文成果、文件、提交、检查、阻塞、下一步、评审请求 |
| `docs/waves/<wave>/state.json` | Leader 单写的状态，可从 Git、工件与测试事实恢复 |

运行上下文可独立，但重启后必须重读记忆与交接。worktree 隔离代码、任务包隔离上下文、版本化工件保持连续性，不能只靠命名角色达到这些效果。

最小任务包包括 `taskId/waveId/roundId/rootTaskId`、角色、`requestedModel/requestedReasoningEffort`、实际 `provider/model/reasoningEffort`、`modelVerificationStatus/modelEvidenceRef`、上下文模式、基准 SHA、规格版本、允许路径、依赖、验收、时间/修复预算、评审者与交付格式。请求与实际配置分开记录；无法核实的实际字段用 `null` 并说明原因，不猜测。角色或模型变化后重新加载规格、记忆与交接，不假设自动继承前任上下文。

## 8. 夜间 P0 轮次与预算

从用户明确批示启动时计时；最晚结束时间为 `min(启动时间 + 7.5 小时, 2026-10-13 08:30 +08:00)`。最多八轮，每轮最多 45 分钟，另含最多 90 分钟发布/回归缓冲，均计入总预算。提前完成核心门槛则交付停止，不填满夜间扩展需求；若启动批示已晚于截止，不自行顺延。

| 轮次 | 目标 | 分工与门槛 |
| --- | --- | --- |
| R0 | 落地规格、契约、任务，验证 OpenCode 智谱工具链 | Leader 兼产品澄清，Test 输入验收，Astra 专项审核；任务有 owner/依赖 |
| R1 | 骨架、配置、迁移、CI、合成 fixture | 前后端与独立测试并行；构建/健康通过 |
| R2 | 上传、已知公司适配、抽帧与帧列表 | 前后端/Test 分路径；上传到帧展示通过 |
| R3 | 四种框操作、几何和标注 API | 原图坐标与契约测试通过 |
| R4 | 保存恢复、进度、COCO | revision、恢复、独立导出解析通过 |
| R5 | 云端部署与基础监控 | 指定 Worker 准备配置，Leader 发布，Test 验证指定 SHA 的闭环/监控 |
| R6 | 错误输入、故障、取消、重启及跨模型审核 | 汇总分批智谱审核并完成实际产品体验；Leader 核实处置，修复与验收 |
| R7 | 仅补 P0 缺口并封存交付 | Test 整理证据/草稿，Leader 核实完成/部分/阻塞报告后停止 |

执行按表中依赖顺序组织；Leader 可调整本波次内部调度并记理由，不能删除验收或跨波次。同一任务最多两次修复重派；短暂网络错误最多两次退避重试；单任务最多 45 分钟。超限记录阻塞，继续独立工作，必要时拆成带交接的新任务。

后继任务必须记录 `rootTaskId`，继承同一失败的累计重试计数，并消耗原波次预算；拆分或改名不能重置次数/截止。真正改变解决路径时记录原因，由 Leader 检查，仍受波次总预算约束。

执行环境固定：Windows 运行 Codex Leader/Worker 和 OpenCode，必须开机、联网、应用在线且不休眠；应用在云端运行。P0 采用持续会话执行和文件状态恢复，不增加远程 Agent 客户端，不创建无限定时编码任务。尚未启动或排程。

## 9. 日志与报告协议

```text
docs/audit/logs/events-20261012T230000+0800.jsonl
docs/audit/evidence/<wave>/<taskId>/red-<timestamp>.txt
docs/audit/evidence/<wave>/<taskId>/green-<timestamp>.txt
docs/audit/reports/report-20261013T083000+0800.md
docs/waves/P0/state.json
```

事件记录 `timestamp/runId/waveId/roundId/taskId/actor/provider/model/reasoningEffort/requestedModel/requestedReasoningEffort/modelVerificationStatus/modelEvidenceRef/contextMode/event/outcome/commitSha/specVersion/evidenceRefs`。未知实际配置用 `null`，供应商不支持的推理设置标记为 `unsupported`，不能套用 Codex 等级。日志从实际启用时开始，历史事件只能标注为回顾，不补造当时不存在的记录。

Worker 各有日志流，Leader 汇总并单写状态；记录领取、规格、Red/Green、提交、评审、发布、重试、阻塞与决策。日志追加并用 Git/文件哈希锚定，但不声称不可篡改，不记录内部思维。大量原始日志留在外部受控存储，仓库保留脱敏必要证据；敏感内容及其派生哈希不公开。

每个波次中文报告包含：时间/范围/实际角色模型；每轮成果；验收矩阵；实现/测试/部署 SHA；CI 和云端结果；架构决策；失败/未完成项；用户验收步骤、入口、数据范围、回滚办法；Leader 停止理由与下一波次建议。

实现/测试/部署 SHA 先记录；报告的封存提交在交付消息或后续事件引用，避免把自己的最终 SHA 写成同一提交的自引用。轮次交接持续记录，波次结束必须交付 `report-<timestamp>.md`。

## 10. Prometheus/Grafana 与资源预算

Compose 管理 Nginx、Go、Prometheus、Grafana 与主机 exporter；P0 不启动 AI。配置、datasource、dashboard provisioning 入仓库；数据、管理员密钥外置。初期一张仪表盘，不引入 Loki/Tempo/复杂告警平台。

| 服务 | 初始内存上限 | 约束 |
| --- | --- | --- |
| Nginx | 64MiB | 静态产物与代理 |
| Go + FFmpeg | 768MiB | 一项抽帧任务，单线程起步 |
| Prometheus | 512MiB | 30 秒抓取；3 天/512MB 数据块保留目标 |
| Grafana | 768MiB | 单实例，无图像渲染器 |
| Host exporter | 64MiB | 只读、最小挂载 |

这是起始预算，不是容量证明。Prometheus 保留限制不覆盖全部 WAL/压缩临时空间，卷需留余量；按安装版本选有效配置。以 RSS/CPU/磁盘/OOM 实测调整，不能为了完成验收静默关掉监控。

仪表盘展示健康、请求耗时/错误、任务队列/耗时/结果、保存冲突/失败、导出结果、主机资源。标签用有限枚举，避免 ID、文件名和敏感信息。运行日志轮转，与研发审计分开。

GitHub CI 构建并执行基础测试；Windows Playwright 通过 SSH 转发对云端业务执行 E2E，智谱也通过受限浏览器工具体验该合成演示。云端串行执行重任务，不同时跑构建和 Chromium。业务、Grafana 和指标检查入口均通过 SSH 转发，不开放公网管理端口。

## 11. GitHub、发布与早晨验收

任务分支完成相关测试，由 Leader 审查后合入；代码任务执行 CI，智谱审核结论在核心交付前完成。启动批示后，Leader 可在已授权仓库和演示服务器提交、合入与发布，不每轮另问用户，波次结束必须停下。CI 调用统一验证入口；未建立 CI 前的文档任务执行文档检查，不假造 CI 通过。

云端发布记录 SHA，备份持久状态、应用迁移、检查版本、执行合成闭环。P0 提供最小安装、配置、启动、测试、备份/恢复和回滚说明。提前定义迁移兼容与回滚方法，不能只回滚程序而忽略数据库。

P0 只在 GitHub 开发和已授权演示机验证，不要求配置或操作 Mac/内网 GitLab。后续代码搬运和真实环境试验由开发完成后的安排处理，不影响本次验收。

早晨交付清单：

- [ ] GitHub 提交、需求追踪与测试证据可核对。
- [ ] 云端入口及实际部署 SHA 可核对。
- [ ] 合成/授权视频的导入、抽帧、框编辑、保存恢复、COCO 可重做验收。
- [ ] 公司适配器材料驱动集成通过；报告明确真实 API 状态为未验证，不作为本次门槛。
- [ ] Prometheus target 和 Grafana 指标可查看。
- [ ] 智谱完成代码审核与浏览器体验，Leader 记录意见处置。
- [ ] 中文波次报告列明所有缺口；Leader 停止，不开始后续波次。

## 12. 启动批示与参考

此前 14 项选择已确认，其中第 05 项以决策 0004 的本地材料验收范围为准；角色配置以决策 0003 为准。本计划只等待用户审核后的明确 P0 启动批示。本轮不启动产品编码、OpenCode 安装、服务器配置或定时任务。R0 将决定变成任务包和具体工程规格，工具链/资源验证失败按阻塞处理，不重新罗列替代选型。

[后续波次前瞻](NEXT-WAVES.md) 只作核心交付后的参考，不是本次执行授权。用户可以单独批示“启动 P0”，Leader 才进入执行。

参考：[Codex 子 Agent](https://learn.chatgpt.com/docs/agent-configuration/subagents)、[后台任务条件](https://learn.chatgpt.com/docs/automations?surface=app)、[OpenCode providers](https://opencode.ai/docs/providers/)、[OpenCode CLI](https://opencode.ai/docs/cli/)、[Prometheus storage](https://prometheus.io/docs/prometheus/latest/storage/)、[Grafana 安装要求](https://grafana.com/docs/grafana/latest/setup-grafana/installation/)。GLM 本次工具链与监控全栈容量仍需验证。
