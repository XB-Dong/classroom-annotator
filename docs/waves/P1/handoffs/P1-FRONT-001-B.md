# P1前端切片B交接：候选复核、本机来源与统计

状态：切片实施与前端模块验证完成；集成联调、真实浏览器截图、GLM独立代码/体验审核未执行。

- rootTaskId：P1-FRONT-001；关联P1-UI-001、P1-AI-001、P1-CAS-001、P1-AUTH-001。
- 工作树：D:/Project/cvte/worktrees/p1-frontend；基线aff4671；未reset。产品提交为本文件所在提交，精确SHA同时发送Leader。
- 本切片首个已记录时钟：2026-10-14T02:09:27+08:00；派发时间以Leader原始审计为准。最终检查完成约02:27，当前波次截止08:30，切片要求03:00前交接。
- 请求配置gpt-6.1-sol/high，独立上下文；实际provider/model/reasoningEffort均未核验，记null。
- 所有权：frontend/**及本交接文件。没有修改Go、Python、OpenAPI、CI、Compose、依赖清单或锁文件。
- 修复计数：Leader将已独立确认的退出失败反馈纳入正式repair，root P1-FRONT-001为1/2，P1共享1/8；其余首次行为Red→Green为本切片实施，不计repair。保留P0.1旧6/6历史。

## 实现与数据保护

Editor从GET /frames/{id}/review读取统一revision、正式框、候选、当前运行和图片问题。候选虚线，正式框实线，选中蓝色控制点；保留原图等比例坐标算法。候选列表可筛待审/待审风险/全部决策，具体风险文字可定位对应候选；选中已决定候选可定位其正式annotationId。

接受/调整/拒绝和图片人工确认均先settle人工保存，再用已确认revision发送PUT review，应用响应后再次GET review。任何迟到快照只在控制器仍干净、revision仍匹配时采纳，保留人工草稿、冲突草稿及进行中的Canvas手势。批量接受仅无riskReasons的pending；OVERLAP_EXISTING不能接受/调整接受，拒绝带ALREADY_LABELED。NO_DETECTIONS/AI_FAILED要求显式“已人工检查本图”。pending或未解决图片风险阻止本帧完成；服务端仍须独立执行完成/导出门禁。

候选调整是未提交预览，需明确“调整后接受”。存在调整预览时，禁止原坐标接受、批量接受和图片确认；迟到轮询不能清除该预览。重新选择候选会取消预览，UI明确说明。正式框仍可手工新增/修改，AI来源、candidateId/runId保留；草稿校验已允许manual/ai/ai_corrected，避免混合草稿被丢弃。

停留帧5秒轮询，隐藏页面停止；手势、人工dirty/in-flight/error/conflict、保留草稿、候选调整、复核写入期间不自动刷新。请求由现有authGeneration保护；frame组件卸载、scope变化及迟到响应检查独立生命周期。重推理仅明确操作，带expectedRevision与Idempotency-Key；不清空已有正式框，未决候选阻止重推理。网络失败重试同revision复用key，成功后下一次明确生成使用新key。

批量面板明确勾选最多20张未完成图片，开始前settle当前帧；请求仅frameIds并带key，失败重试相同选择复用key。Job.kind按Leader确认增加preannotation，任务列表准确显示AI任务。

本机来源GET /sources/local读取授权目录元数据；POST opaque sourceId/imports带key返回Job，不接浏览器任意路径，不启动Mac入口。503或格式错误准确显示并可重试，不编造已配置目录。统计按GET review-stats原始fraction转换百分比，null显示无样本，明确已决策候选/当前候选/有效图片分母，失败图片、人工补框单列；检测准确率显示未评估。

正式退出修复：原request在本地clear后把撤销请求的network错误转换为STALE_AUTH_RESPONSE，被AuthProvider忽略。只对logout且当前无新账号允许读取退出结果，使失败反馈可见；新账号已激活仍拒绝旧响应。撤销请求继续保留原CSRF/Cookie且不随本地clear abort。

## 实际验证证据

外部日志目录：D:/Project/cvte/runtime/p1-front-review/。所有重检查使用Local\\CVTE_P1_BUILD，最多120秒等待，finally释放。没有启动开发服务器、Docker、常驻进程或安装依赖。

| 日志 | 实际观察 | 退出码 |
| --- | --- | --- |
| red-review.log | 4项Editor复核行为缺失；控制器方法尚未实现另记missing-feature | 1 |
| green-review-first.log | 首批复核与控制器15测试通过 | 0 |
| red-local-poll.log | 本机目录2项行为缺失；轮询测试定时器fixture错误另记，不冒充产品Red | 1 |
| red-logout-green-local.log | 退出成功被当stale，退出network失败被隐藏，两项真实Auth断言失败；另一个轮询fixture错误 | 1 |
| green-logout-local-poll.log | Auth、本机来源与迟到轮询14项通过 | 0 |
| red-gesture-batch.log | 进行中手势被server刷新覆盖；控制器方法与Batch模块尚未实现另记missing-feature | 1 |
| red-ai-draft-green-batch.log | mixed AI/manual草稿被校验器忽略；其他24测试通过 | 1 |
| red-idempotency-adjust.log | AI重试key变化、调整后仍可接受原坐标，两项真实断言失败 | 1 |
| red-provisional-adjustment.log | 迟到刷新丢失候选预览、批量接受未排除预览，两项断言失败 | 1 |
| green-provisional-adjustment.log | 最终Editor复核11项通过 | 0 |
| typecheck-first.log | 新测试mock参数类型与React19 ref初始化错误；已修正，不冒充行为Red | 2 |
| typecheck-b.log | 完整typecheck通过 | 0 |
| test-full-b.log | 18文件/78测试通过，后续新增1项候选预览测试 | 0 |
| lint-b.log / build-b.log | lint通过；114模块生产build通过 | 0 |
| test-final-b.log | 最终18文件/79测试通过 | 0 |
| lint-final-b.log | 最终完整lint通过 | 0 |
| build-final-b.log | 最终tsc --noEmit与114模块生产build通过 | 0 |

格式工具仅整理实际修改的frontend文件，git diff --check通过。所有API/模型结果均为合成fixture，不是实际ONNX推理、真实GLM或真实账户联调证据。

## 后续集成验收

1. Leader集成后用真实PG/Go/Python覆盖候选决策CAS、手工保存与推理stale、完成/export门禁、当前运行统计和本机import。
2. Test取得1440×900、1280×720、窄屏实际截图，核对右侧长审核内容可滚动、中央画布操作面积与具体风险定位；本切片未声称截图通过。
3. GLM独立代码与实际浏览器体验按波次要求执行；本Worker只完成实施与模块测试，未替代跨模型审核。
4. Canonical接口由Leader维护；实施时发现Annotation.source同时const manual和enum三来源矛盾，已通知Leader，其e683ecb已修正。Job.kind singular preannotation同样经Leader确认。
