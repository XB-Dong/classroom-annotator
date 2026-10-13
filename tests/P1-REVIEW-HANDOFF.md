# Test：独立候选复核与双 API 验收切片

时间：2026-10-14T03:28:15+08:00；rootTaskId=P1-TEST-001；slice=B。外部worktree为`D:/Project/cvte/worktrees/p1-test-review`，分支`p1/test-review`。基线66ffd10，另挑入Leader已集成的5499893为7fd0769、7fd047f为44158a8、79d8491为564bfe6、67717c5为1fb97ce，仅供review/scheduler/schema8/canonical接线的只读上下文。Leader只需挑入本切片Test提交，不重复挑入这些祖先提交。

请求gpt-6.1-sol/high；实际provider/model/reasoningEffort不可独立验证，记null/unknown；fresh independent context，无额外子agent。截止仍为2026-10-14T08:30:00+08:00；初始切片到03:45。继承P1-TEST-001与全局修复计数；本切片未修改产品源码，也未重置任何计数。

## 用例与证据边界

- P1-REVIEW/STATS/GATE：每次创建独立合成视频/帧。已有manual框不计manualAdded，新run之后新增计入；三个固定框的原始风险、逐步pending分母、1/3接受/调整/拒绝、移动→adjusted、删除→rejected、原ID撤销恢复accepted，以及重新生成只统计激活run且保留旧正式框。pending/图片问题不能完成或导出；最终读取ZIP/COCO/实际JPEG独立核对标签、尺寸、框与原字节。
- P1-REVIEW：明确的高风险逐项接受允许，包含风险的批量接受拒绝。客户端抹除riskReasons、null review bbox、新ID绑定他人candidate、旧AI ID转换manual、错误candidate映射、跨帧恢复均拒绝；失败请求前后比较本视频全部frames/annotations/candidates/provenance/issues/audit行，不能只检查HTTP状态。
- P1-AIRACE：真实HTTP响应屏障控制晚成功/晚失败，先完成的人工标签/revision/状态不被覆盖；旧激活run在新stale尝试之后仍保留候选与统计，currentRun显示latest而activeRunId显示真正激活来源。取消必须实际断开本次HTTP请求且不产生候选/AI_FAILED；同key终态重复仍同job，变更payload409，新key明确rerun。
- P1-AIRACE：两个实际HTTP同key入队，在真实PG advisory事务屏障上同时到达，不能用串行请求冒充竞争；只生成一个job/request/run。批次一个成功一个失败，job必须failed，逐帧结果与统计分别保留；批次key重试不重复创建子run，payload变化409。
- P1-CAS：第二个真实Go API进程在首实例AI尚被HTTP屏障挂起时启动，首job仍running且只发布一次；20轮双进程PG行锁屏障中一个200/一个409、一份审计、两API读取同一最终快照。第一轮同时竞争review与annotation保存，败者不能留下candidate/provenance副作用。
- P1-DEGRADE：仅断开首实例自己的loopback DB故障代理；连接恢复后人工保存/刷新仍可用，失锁的首实例任务入口503，重启后才恢复调度。没有触碰其他实例、生产数据库或远程链路。

真实Python阶段先单独执行。之后只停止runner拥有的Python，再以相同AI端口临时绑定确定性fixture；证据全部标`fixture_not_real_model`。fixture完成/失败后回收并恢复真实Python，浏览器/负载/最终真Python停机降级阶段才能继续。私有8图、真实Nano/ONNX、GLM或准确率均不能以本fixture替代。

## 实际已运行

Windows只运行npm依赖准备、JavaScript语法、Node单测和Playwright收集，均在`Local\CVTE_P1_BUILD`单槽内，finally释放；没有本机PG、Docker、业务持久服务器或隧道。Node单测对fixture真实loopback HTTP进行认证、独立control认证、响应挂起/显式释放及空/失败/null输出检查，属于测试设施证据。

初次检查：语法exit0、既有Node9/9、收集43项/16文件exit0。随后增加fixture单测与stale历史run用例：语法exit0、Node11/11、收集44项/16文件exit0。最后批次/风险用例后的检查为语法exit0、Node11/11、收集46项/17文件exit0；外部`runtime/p1-test-review/{check-delivery,unit-delivery,list-delivery}.log`保留。此前日志`{npm-ci,check-final,unit-final,list-final}.log`保留。收集数量不是通过数量。

schema8及发布取消补充之后：语法exit0、Node11/11、收集47项/18文件exit0，日志为`{check-metrics,unit-metrics,list-metrics}.log`。挑入Root canonical后，外部纯合成`job-evidence-smoke.mjs`验证3组合法时间/逐帧结果以及3组损坏数据确实被断言拒绝，exit0；日志`job-evidence-smoke.log`。它没有发送API/PG/model请求，不提升产品门槛状态。canonical上下文后的最后核对日志为`{check-canonical,unit-canonical,list-canonical}.log`。

所有本切片的产品PG/API/browser断言当前**not_run**；没有伪造Red/Green或实际模型结果。需Leader在接好真实scheduler、API与canonical activeRunId字段后执行精确提交Linux CI。发布和部署只由Leader完成。

## 提前交给Leader的差异

1. Leader canonical67717c5已同步activeRunId与schema8 Job字段，本worktree已读同一契约。严格schema测试保持不放宽。
2. Leader明确冻结任何高风险复核决策均须逐项检查，包含批量拒绝；安全无风险候选可以批量处理。确定性用例逐项处理风险，另明确测批量接受拒绝；既有真实Python用例也改为逐项拒绝，且后加manual与候选重叠时按原图IoU>=0.35标ALREADY_LABELED。
3. schema8已由Metrics B提交79d8491提供，本worktree挑入为564bfe6仅供上下文。对单帧成功/失败/stale/取消、真实Python闭环和fixture批次新增terminal startedAt/finishedAt有序RFC3339与逐帧partialResults检查；applied只允许completed且真正为当前激活run，失败errorCode必须安全非空。单独计算queueWaitMs/executionMs，不混为模型timing；真实120秒队列/执行截止与性能仍not_run，普通短任务的执行时间上限断言不能替代截止边界实验。
4. 读取当前frontend未发现跨帧复制动作；未把不存在的流程记成问题或修改frontend。服务端跨帧ID/来源伪造已有独立用例。

后续第一次完整CI失败必须保留日志并据实际失败区分产品缺陷、测试缺陷和依赖/准备问题。首轮格式失败e683/38446796562与已确认的Python/Frontend局部通过事实保持原样，不移植为本切片完整通过。

Leader同时确认follower普通readiness为200、任务入口503；首实例失锁后恢复DB仍可人工保存、任务503直到重启。既有postgres-check连接上限更新为两个idle加一个真实持锁会话；额外独立读取pg_locks核对恰一个scheduler session（两键namespace1129731141/key1），未宽松提高总连接peak<=5。

补充AI发布↔取消真实PG屏障：限定本测试frame的ai_candidates INSERT trigger，在publisher已持job/frame锁后挂起；实际cancel的jobs查询必须被该publisher阻塞。释放后允许job完成或取消，但已发布run/candidates必须保留且恰一次，只有一份ai_publish审计。finally回收本测试trigger/function，证据独立写入`ai-publication-cancel-evidence.json`。输入JPEG bytes、fixture receipt requestId/SHA与ai_runs input/model/pipeline三项血缘也逐项独立比对。本补充产品实验仍not_run。
