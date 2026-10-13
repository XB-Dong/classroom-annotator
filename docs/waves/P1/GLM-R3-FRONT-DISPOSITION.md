# P1 前端 GLM 审核逐项处置

时间：2026-10-14T04:08:42.5635700+08:00

结论：11条反馈中，8条按其原主张不成立，1条导航竞态缺乏复现，1条存储上限观察作为非阻断改进保留，1条措辞意见缺少可执行内容。本任务未确认新功能缺陷，未改代码，未消耗修复轮次。不得依据外部报告的FAILING标签直接增加前端第三轮修复。

基线：80705d286657f9ca6d5c879c8dea11c6f8381d34；复核结束观察HEAD：0aa5ed1db183783a719be01b1868a542009d39ca。前端及本文引用的后端规则路径在两SHA间无差异；Leader并行的annotation409 requestId修复不影响本文结论。

外部原始证据：D:/Project/cvte/runtime/glm/evidence/P1-R3-FRONT/stdout.jsonl；summary记录OpenCode/company-glm/glm-4.7-flash，27.95秒，exit 0，15个前端源文件。原始报告保留不改写。当前native复核请求Astra/xhigh，实际路由未验证。

证据口径：只读跟踪调用路径、规格和已有测试断言。父任务给出的79个前端测试及6个JobsPanel测试通过属于既有证据，本任务未重跑测试、未启动重负载、DB或服务器。实际浏览器体验和精确SHA发布验收仍pending，本文不宣称P1已通过。

## SPEC-1 — 旧authGeneration仍可展示迟到私有响应

处置：`rejected`。触发：用户A发起视频/媒体请求；退出或切换用户B后，A的fetch或图片回调才完成。

唯一业务fetch位于request；捕获generation后在fetch返回、错误body解析后及成功JSON/blob解析后再次断言。clear/activate均递增generation并abort。账户工作区卸载/按用户key重建，图片回调另查generation和active。此实现并非仅abort。

代码/规格：frontend/src/shared/api/client.ts:14-26,38-44,66-73；frontend/src/features/auth/session.ts:15-26；frontend/src/app/App.tsx:63-70；frontend/src/features/annotation/Canvas.tsx:141-164。
已有测试断言：frontend/src/features/auth/session.test.ts:17-37；frontend/src/app/App.test.tsx:152-193。
边界：未在本任务重跑真实浏览器；以上是实际调用路径与已有回归断言。

## SPEC-2 — 冲突resolve隐式丢弃草稿，UI未说明后果

处置：`rejected`。触发：保存409或加载到基于旧revision的本账号草稿，选择冲突恢复。

UI分别明确显示“查看最新并保留草稿”和“丢弃草稿，使用服务器内容”；保留分支继续写入原草稿并展示旧revision、框数、坐标。false仅由明确丢弃按钮触发，且读取latest成功后才清理。

代码/规格：frontend/src/features/annotation/Editor.tsx:371-409；frontend/src/features/annotation/saveController.ts:113-126,137-164；specs/p1/accounts-collaboration.md:29-34。
已有测试断言：frontend/src/features/annotation/saveController.test.ts:28-47,160-192。
边界：按钮理解度仍可在真实浏览器体验中评价；当前反馈所称无说明不成立。

## SPEC-3 — 候选决策调用settle但仍允许dirty正式编辑未保存就继续

处置：`rejected`。触发：人工补框后立即接受候选，或保存A期间又有编辑B。

settle等待inFlight并循环flush直到sequence===acknowledged且state===saved；error/conflict/gesture返回false。Editor同时要求canRefresh，而它也要求saved、无待确认编辑及无保留草稿，再从已确认snapshot取revision。

代码/规格：frontend/src/features/annotation/Editor.tsx:175-185,211-216,240-243；frontend/src/features/annotation/saveController.ts:87-110,167-190。
已有测试断言：frontend/src/features/annotation/Editor.review.test.tsx:143-154；frontend/src/features/annotation/saveController.test.ts:64-99。
边界：无新增失败复现。

## SPEC-4 — 批量接受仍会包含OVERLAP_EXISTING等高风险候选

处置：`rejected`。触发：同时存在无风险、LOW_CONFIDENCE和OVERLAP_EXISTING候选，或只剩高风险候选时点击批量接受。

safe只包含riskReasons.length===0的pending候选；safe为空时按钮禁用，批量payload只映射safe。服务端另拒绝含高风险的多项决策，实时重算已有正式框重叠并要求ALREADY_LABELED拒绝。

代码/规格：frontend/src/features/review/ReviewPanel.tsx:44-48,132-165,179-186；backend/internal/preannotation/repository.go:182-199。
已有测试断言：frontend/src/features/annotation/Editor.review.test.tsx:131-166；backend/internal/preannotation/pg_test.go:395-447。
边界：PG测试仅阅读断言，本任务未启动数据库。

## SPEC-5 — 正式AI框编辑/删除/撤销provenance不完整，理由是UI无来源视觉区分

处置：`rejected`。触发：接受AI候选后移动正式框、删除，再撤销删除并保存。

几何修改通过spread保留source/candidateId/runId；history克隆完整snapshot，撤销走同一保存接口。服务端Sync验证不可变映射并重算accepted/adjusted、ai/ai_corrected，删除保留tombstone，恢复验证原映射，与revision同事务。UI要求候选虚线/正式框实线已实现。缺少正式来源徽标的观察属可选UX增强，不证明provenance破坏。

代码/规格：frontend/src/features/annotation/Canvas.tsx:249-255；frontend/src/features/annotation/Editor.tsx:240-269；frontend/src/features/annotation/history.ts:5-18；backend/internal/annotations/repository.go:123-136,163-170；backend/internal/reviewstate/state.go:28-87；specs/p1/ai-review.md:19,31-39；specs/p1/ui.md:30-31。
已有测试断言：backend/internal/preannotation/pg_test.go:140-196；frontend/src/features/annotation/drafts.test.ts:34。
边界：未在本任务实跑端到端AI删除/撤销；现有PG测试包含该链条。

## SPEC-6 — ExportButton仅检查完成数量，导致未复核/部分应用结果作为最终标注导出

处置：`rejected`。触发：批次含已完成帧和未决候选帧，或试图将pending帧直接设为completed再导出。

按钮只是入口；App另阻断未保存/草稿，完成按钮另阻断pending/图片问题。服务端只读取completed帧，并在REPEATABLE READ内重新Gate、Validate、Verify。直接篡改完成状态仍过不了export Gate。批次可以合法导出其中已完成子集，无需所有帧完成。partial applied显示“当前候选已生成，仍需人工复核”，未冒充正式标签。

代码/规格：frontend/src/features/export/ExportButton.tsx:17-21,39；frontend/src/app/App.tsx:109-115,318-325；frontend/src/features/annotation/Editor.tsx:314-315,451-474；backend/internal/annotations/repository.go:133-136；backend/internal/exports/repository.go:72-103,121-136；backend/internal/reviewstate/state.go:14-23；frontend/src/features/videos/JobsPanel.tsx:79-103。
已有测试断言：frontend/src/app/App.test.tsx:195-205,276-287；backend/internal/preannotation/pg_test.go:148-150,439-447；frontend/src/features/videos/JobsPanel.test.tsx:33-52。
边界：本任务未独立下载ZIP；真实导出和GLM体验仍需原计划验收。

## QUALITY-1 — HelpPanel的generation比较不能处理请求中途session.clear

处置：`rejected`。触发：提问发出后退出，再收到旧答案/错误。

clear与activate调用同一transition递增generation；HelpPanel成功、catch、finally均比较捕获generation，session变化effect清空答案/问题/busy；request层还会拒绝旧响应。

代码/规格：frontend/src/features/help/HelpPanel.tsx:16-35,46-63；frontend/src/features/auth/session.ts:15-26；frontend/src/shared/api/client.ts:23-26,41-44,72。
已有测试断言：frontend/src/features/help/HelpPanel.test.tsx:93-137；frontend/src/features/auth/session.test.ts:17-37,90-99。
边界：现有HelpPanel组件用账户切换测试，clear语义由SessionScope同一transition及session单测支持；本任务没有单独补测logout-only HelpPanel。

## QUALITY-2 — navigationLock在快速连续导航时存在竞态

处置：`unsubstantiated`。触发：dirty保存未确认时快速选择帧B/C，或异步上传完成与切换同时触发。

ref锁在首个await前同步置true，覆盖整个settle和同步action；重入直接返回，finally位于action后。已存在快速切帧、延迟上传/冲突/失败使用最新controller的断言。报告未提供锁释放后可丢草稿的事件次序，也未复现。丢弃重复点击并不等于数据竞态。

代码/规格：frontend/src/app/App.tsx:118-143,290-304,331-350,387-405；frontend/src/features/annotation/saveController.ts:167-179。
已有测试断言：frontend/src/app/App.test.tsx:195-273；frontend/src/features/annotation/saveController.test.ts:64-99。
边界：未单独执行同tick重复导航或真实浏览器压力重放；可作为已要求浏览器验收的观察点，不据此申请修复或宣称已复现。

## QUALITY-3 — 草稿localStorage没有数量/大小上限，可能达到quota

处置：`accepted_nonblocking_observation`。触发：大量帧/账户长期保留未保存或冲突草稿，直到浏览器setItem触发QuotaExceededError。

确实未实现应用级TTL/字节总额/条目数策略；但readDraft只是读取固定frame/account key，实际写入异常已捕获并显示“浏览器草稿存储不可用，请保持页面打开直到保存成功”。已确认保存清理对应草稿，历史保留草稿不能任意自动删以免违反恢复承诺。未展示静默丢失或无限内存的实际复现。

代码/规格：frontend/src/features/annotation/drafts.ts:8-42；frontend/src/features/annotation/Editor.tsx:93-105,356-359；frontend/src/features/annotation/saveController.ts:101-108,147-155；backend/internal/annotations/repository.go:18-20,33-35。
已有测试断言：frontend/src/features/annotation/saveController.test.ts:134-202。
边界：未模拟quota，也未证实实际存储压力。若未来改清理策略须先定义哪些草稿可丢弃；当前不作为P1阻断项、不消耗第三次前端修复。

## QUALITY-4 — geometry没有坐标校验，非法框可保存导出

处置：`rejected`。触发：绘制/拖动/resize产生NaN、越界或零尺寸框，或者直接提交非法坐标。

geometry明确要求输入/尺寸有限、尺寸正数、裁剪边界且返回正宽高；新框还要求最小2原图像素。创建和变换均调用它，后端保存与导出再次验证有限、正值、不越界。外部报告承认未读该文件，其“缺失”推断不成立。

代码/规格：frontend/src/features/annotation/geometry.ts:2-17；frontend/src/features/annotation/nodeGeometry.ts:3-19；frontend/src/features/annotation/Canvas.tsx:193-200,249-257,285-288；backend/internal/annotations/repository.go:33-64,126-128；backend/internal/exports/repository.go:98-103。
已有测试断言：frontend/src/features/annotation/geometry.test.ts:3-15；backend/internal/annotations/repository_test.go:17-33。
边界：readDraft只检查normalizeBox有非空结果而未用归一化结果替换原草稿；手工篡改localStorage可造成保存422，但这不是正常几何路径缺失校验或非法导出。本任务未将这一防御性边界扩成新修复。

## QUALITY-5 — 多处中文错误消息技术词不清楚、不一致

处置：`non_actionable_opinion`。触发：外部报告未给出具体消息、错误场景或期望替代措辞。

缺少可复现问题和规格违反；已有错误均有可见区域。浏览器体验若产生具体理解障碍，再按实际触发与影响评估；不以宽泛风格意见判FAILING。

代码/规格：frontend/src/shared/api/client.ts:51-63；frontend/src/features/annotation/Editor.tsx:348-364；frontend/src/features/help/HelpPanel.tsx:119-122。
已有测试断言：无具体测试场景。
边界：真实用户理解度/全站措辞一致性没有在本任务测量。

## 预算与后续

任务派发时global 5/8；复核中state.json已记录Leader现有第6轮annotation409 requestId修复，故观察值global 6/8、frontend 2/2、auth 2/2。本任务不修改state、不重置预算、不申请新功能修复。Leader可采纳以上逐项结论；若后续浏览器证实前端新缺陷，应明确报告该root已达2/2，不能静默追加。

完整机器可读证据及各项不确定性见同目录disposition.json。
