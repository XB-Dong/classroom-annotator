# P1 风险图片索引：GLM逐项处置

时间：2026-10-14T04:35:32.8396752+08:00；基线：dcf046e2ca4bf916d0faa83aaaa55c7d86d51932；工作树核查为clean。

结论：6条具体缺陷主张均不成立，未确认新功能缺陷，不申请额外修复。每项仅给出源代码、既有测试断言和明确边界；本任务没有运行测试、浏览器、API、数据库或重负载命令。

## 外部审核实际覆盖

原始stdout.jsonl记录OpenCode/company-glm/glm-4.7-flash约20.96秒、exit 0。逐条统计10次read均completed且有文件内容：1份REVIEW.md、9个唯一源码/测试文件（5个产品源文件、4个测试文件）；读取错误0、截断0。9份快照文件与当前仓库逐一SHA256相同。文件清单和计数方法见JSON。

外部原始证据：D:/Project/cvte/runtime/glm/evidence/p1-risk-index-dcf046e/stdout.jsonl；SHA256 cf85fc43edd10531ab6bb282d0bd569f67482449aa7c9ed741fc3889ce954b71。原结论为SPEC PASSING、QUALITY PARTIAL PASS。实际读到了所提供文件，但没有读取保存controller/HTTP saver/写事务；本次补查这些边界。外部SPEC标签不是全P1运行验收。native复核请求沿用Astra/xhigh，实际路由未知。

## GLM-RISK-1：App.tsx:118的localStorage读取失败将崩溃应用

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：localStorage访问/getItem因不可用或策略限制抛出异常。

getItem与localStorage访问均在try内，catch返回true，异常不会逃出filter使App崩溃。失败时把该帧计为未确认草稿并保守阻断导出；报告自身摘录也包含catch。写草稿quota错误由Editor捕获并显示存储警告。

代码/契约：frontend/src/app/App.tsx:116-122；frontend/src/app/App.tsx:325-332,379-383；frontend/src/features/annotation/Editor.tsx:93-105。
已有断言：frontend/src/app/App.test.tsx:292-303。
限定：存储不可访问时计数可能高于真实草稿数，提示可能不够具体；这是可解释的保守降级，不支持所称崩溃，也未证明数据损坏。未新增修复项。
验证边界：未模拟浏览器存储异常；引用的现有测试验证持久草稿阻断导出，不冒充quota覆盖。

## GLM-RISK-2：navigationLock异常时不清理，且快速连续点击缺少保护

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：保存待确认期间重复切帧，或settle/action抛出异常。

入口先同步检查ref锁并置true，再进入await；重入返回。try覆盖settle及action，finally在成功、提前return及抛异常时都重置锁和busy，恰好处理报告所说场景。报告没有给出绕过入口锁的具体事件次序。

代码/契约：frontend/src/app/App.tsx:134-150；frontend/src/features/annotation/saveController.ts:167-179。
已有断言：frontend/src/app/App.test.tsx:257-289；frontend/src/features/annotation/saveController.test.ts:64-99。
限定：重复点击被忽略而不是排队属于当前导航语义；不能仅凭没有排队推导数据竞态。
验证边界：已有源码断言覆盖快速切换、保存失败/冲突与in-flight编辑；本任务未运行同tick压力重放或真实浏览器。

## GLM-RISK-3：ReviewStatsPanel清理risk IDs的effect未依赖videoId/batchId，换批次旧ID会残留

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：refresh维持0，batchId从old变为new，旧请求后返回。

第40行依赖完整包含videoId、batchId、refresh、onRiskFrames；每次effect先发布undefined，cleanup令旧响应active=false。App还以video/batch给组件key，风险结果携scope且仅在scope相符时传给FramePicker，故不能把旧scope结果套到新批次。现有组件测试明确在refresh=0时换batch并断言迟到old结果被忽略。

代码/契约：frontend/src/features/review/ReviewStatsPanel.tsx:17-40；frontend/src/app/App.tsx:108-114,360-365,395-398；frontend/src/features/annotation/FramePicker.tsx:31-38,46-58。
已有断言：frontend/src/features/review/ReviewStatsPanel.test.tsx:74-123；frontend/src/app/App.test.tsx:342-394。
限定：加载/失败风险数据为unknown，UI明确禁用风险筛选并继续显示帧，不把unknown冒充零风险。
验证边界：仅检查实际代码及已有断言，没有执行新测试。

## GLM-RISK-4：PostgreSQL count(*) FILTER无匹配时返回NULL，扫描int失败

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：合法视频/批次没有帧、没有active run，或没有匹配风险候选。

COUNT是返回0的聚合；FILTER只限制计数输入，零匹配的COUNT仍为0，并非SUM等聚合的NULL语义。该查询无GROUP BY，空输入仍有一行零计数，可扫描int。代码真正需要null的是零分母比率，fraction显式返回nil，UI显示无样本。风险ID切片初始化为空非nil，测试包含无active run及无帧视频。

代码/契约：backend/internal/preannotation/stats.go:33-38,48-63,82-92。
已有断言：backend/internal/preannotation/risk_image_index_pg_test.go:24-48,124-137；frontend/src/features/review/ReviewStatsPanel.test.tsx:45-71。
限定：查询错误会被逐次检查并返回；不存在该反馈所称缺失NULL处理。
验证边界：没有连接PostgreSQL或执行这些测试；这里依据COUNT语义、查询结构和已有fixture断言，不宣称本SHA真实PG已通过。

## GLM-RISK-5：冲突阻断导航测试没有检查localStorage草稿保留

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：加载旧baseRevision草稿，延迟上传完成触发导航，冲突阻止切换。

被点名的同一测试第255行就是expect(localStorage.getItem(draftKey(...))).not.toBeNull()，并检查没有保存请求。保存失败场景第266行也检查草稿。风险筛选测试检查隐藏当前帧仍保留草稿；controller测试另覆盖读取latest且保留原草稿不重交。

代码/契约：frontend/src/app/App.test.tsx:236-256；frontend/src/features/annotation/saveController.ts:113-126,167-171。
已有断言：frontend/src/app/App.test.tsx:253-255,263-266,318-340；frontend/src/features/annotation/saveController.test.ts:160-178。
限定：若建议增加再次点击/恢复后的组合用例，可作为测试增强讨论，但不能声称现有草稿保留断言不存在。
验证边界：只读检查覆盖范围，不主张所有重试交错均已覆盖或实跑。

## GLM-RISK-6：App.saved只检查revision/status，正确revision但错误CAS token可通过

处置：拒绝原主张；严重性：未证实缺陷，不分配P级。
触发：构造保存请求，revision正确但附带所谓独立CAS token错误。

冻结契约使用frame expectedRevision作为CAS比较值，没有独立CAS token字段；认证/CSRF token是另一职责。App.saved是成功快照后的展示/统计刷新回调，不是写入授权点。saveSnapshot发送当前revision，服务端持帧锁比较ExpectedRevision并条件UPDATE；不符返回409，controller进入conflict且保留草稿。新增另一token校验既无契约依据，也不是本次风险索引缺陷。

代码/契约：specs/p1/accounts-collaboration.md:21-27,56；specs/contracts/openapi.yaml:3443-3465；frontend/src/app/App.tsx:273-292；frontend/src/features/annotation/Editor.tsx:84-90；frontend/src/features/annotation/saveController.ts:91-105,113-126,196-206；frontend/src/shared/api/client.ts:85-93；backend/internal/annotations/repository.go:119-128,148-157；backend/internal/preannotation/repository.go:155-163。
已有断言：frontend/src/features/annotation/saveController.test.ts:64-99；backend/internal/preannotation/pg_test.go:232-269。
限定：诊断requestId不是CAS令牌。既存review409 JSON requestId缺失已单独记P3，公共X-Request-ID与409/currentRevision/CAS仍在，本复核不重开该项。
验证边界：现有20轮同步CAS测试源码可见，但本任务未执行，不以该引用宣称运行通过。

## 预算和保留项

保持global 7/8，FRONT/Auth/AIReview/TEST均2/2。本次消耗0轮，不改state，不重置root预算。风险索引是已批准需求的首次实施，不是新增修复分配。review409 JSON requestId诊断项继续按state.json的P1-DIAGNOSTIC-001/P3暂缓：公共X-Request-ID仍在，无已知数据影响，本任务不重开。

后续仍按原门槛完成精确SHA真实PG与浏览器验收；本报告不制造运行通过结论。
