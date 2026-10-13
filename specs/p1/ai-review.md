# P1人头预标注与风险复核规格

版本：0.1.0｜待审核；目标是减少画框与寻找风险的操作，人工决定训练标签。

## 1. 模型与服务边界

默认PINTO YOLOv9 Wholebody34 Nano、640×640、ONNX CPU；使用旧评估记录的精确3,221,705字节权重，SHA-256见P1主计划。以前纯模型合成张量基准不证明课堂准确率。R0核对模型文件、作者类别表（预期head=7）、权重许可/来源与锁定版本，保存可追溯模型清单；许可不明确停止发布该模型，不把仓库代码MIT自动当权重许可。

Python模块 `ai-service/` 本次新增，现有仓库只有边界文档，不声称服务已存在。FastAPI只接Go传入的图像字节及数值参数，不接任意URL或文件路径、不访问业务DB、不持有公司Key。接口内部token与网络限制；模型挂载只读，图像不写日志/临时磁盘。JPEG/PNG限制15MiB、16MP，解码失败返回明确422；不安装PyTorch/CUDA做运行依赖。

`POST /v1/detections`：multipart image/confidenceThreshold/nmsThreshold；200返回contractVersion、modelId/modelSHA256、image.width/height、detections[]（bbox原图xywh、confidence、classId）、timingMs。Go提供requestId，Python失败有安全code/message。CPU单槽，忙则429，30秒调用超时；GET /health/live与/health/ready区分存活/模型可用。

使用等比例letterbox，记录实际缩放和padding，输出逆映射后裁剪到真实图片边界。仅保留head，NMS IoU默认0.50，置信阈值0.25；拒绝NaN/Inf/退化框，不用身体或脸扩张猜人头。无候选是有效推理结果，但图片必须人工确认。旋转视频沿用抽帧后实际JPEG尺寸。

模型不能可靠区分学生和教师：候选是“可能的人头”，正式标签仍按学生可见头部规则，人类审核剔除教师/无关人物。小头和遮挡可能漏检，不能把高置信度当自动完整标注。

## 2. 持久化和工作流

正式annotations与候选分开。新增 ai_runs、ai_candidates、frame_review_issues、annotation_provenance，记录模型/input哈希、参数、riskPolicyVersion、baseRevision、状态、候选原始bbox/score/风险、最终决策和操作者。人工补框source=manual，由候选接受的正式框source=ai，调整后source=ai_corrected；保留candidateId、runId追踪，服务端验证引用，不信任客户端伪造模型证据。provenance对annotationId及candidateId各有唯一约束，一个候选只预留一个正式框ID，删除保留tombstone；导出校验支持合法manual/ai/ai_corrected，仍拒绝未审候选或伪造来源。

run状态 queued/running/completed/stale/failed/cancelled；job继续既有终态语义，stale表示计算完成但结果未应用，UI明确需要重新生成。发布候选在短事务CAS，仅成功后激活run。重新生成只在无pending且用户明确操作时替换当前run，历史不删除。

幂等范围为workspace/frame/明确requestKey与请求指纹：指纹包含expectedRevision/imageSHA/modelSHA/options/riskPolicyVersion。重复同key同指纹返回同run，包括终态，同key不同指纹409。已完成后用户明确“重新生成”必须用新key，不能用相同image/model/options永久阻止重跑；同帧仍最多一个未终态任务。批次key保存子frame→子key映射，重试不再重新创建子运行。

首次/重新推理均保留已有手工与历史AI正式框，不能清空已完成的人工工作。新候选与现有正式框IoU≥0.35时标OVERLAP_EXISTING并定位已有框；P1不自动合并/替换或直接再次接受产生重复标签，用户将重叠候选以ALREADY_LABELED原因拒绝，或先明确处理已有正式框再审核候选。此类拒绝单列原因，不宣称为模型误检。旧run产生的正式框继续属于原run，新run统计只计其自己的候选；手工补框统计不纳入旧正式框。

候选状态 pending/accepted/adjusted/rejected：

- 仅展示pending候选不会写正式标签。
- 接受：创建正式框并写决策；调整后接受写adjusted；拒绝：无正式框、保留原候选证据。
- 后续移动来自AI的正式框更新其决策为adjusted；删除同步为rejected，审计保留先前动作，避免统计还把已删除框当通过。
- 手工补框直接写正式框，并保持AI原始输出不变。
- 每次决策/编辑都执行帧CAS；完成状态下修改需回到in_progress。不能由前端独自更新审核状态。

全量PUT /annotations由服务端对照当前快照与不可变provenance映射，不依赖客户端source/candidateId/runId声明：保留的AI框坐标变化同步adjusted，删除同步rejected，新普通框只准manual。AI框ID↔原candidate/run归属在删除后仍保留tombstone，合法撤销删除仅能恢复同帧、同原映射的ID并重算accepted/adjusted；不得以旧ID绑定另一候选或跨帧恢复。映射、正式框、候选决策、revision、操作者审计在同一事务更新。所有写入路径必须调用同一服务端规则，拒绝伪造AI来源或重置风险。

冻结业务接口（具体字段以specs/contracts/openapi.yaml 2.0.0为准）：POST /api/v1/frames/{id}/preannotations（expectedRevision + Idempotency-Key）返回202/jobId；GET /frames/{id}/review返回统一revision、当前run、候选、正式框、图片问题；PUT /frames/{id}/review携带expectedRevision、decisions与问题确认，在同一事务生效。批次 POST /batches/{id}/preannotations 输入最多20个frameId，每帧捕获独立revision，返回批次job，逐帧结果可见。参数/字段在执行R0与OpenAPI同步冻结。

所有保存共用鉴权/CAS/合法provenance校验；只有请求将图片设为completed时应用pending/未解决风险门禁。in_progress下允许人工保存、补框和逐项复核，即使还有pending。普通PUT /annotations也不能绕过完成门禁，导出独立再验证。更改阈值生成新run，不悄悄删除旧候选。

推理失败不直接在新revision上写入旧帧问题：持久化run/job失败；仅当当前frame仍是baseRevision时，以CAS添加AI_FAILED问题并推进revision。若已有人保存则不改变其帧/问题，失败只作run级提示。用户转人工确认/解决图片问题也必须CAS。取消时不自动制造失败问题。模型失败与人工完成竞态和问题写入均需真实PG测试。

P1只允许一个任务调度实例，通过PG advisory锁维护scheduler单例；第二实例可以做普通API但不能同时恢复/调度任务。记录scheduler owner，失去锁即停止启动任务。只有持锁实例可以恢复本调度器上次中断的未终态任务，不扫仍有效的另一owner。API多实例CAS正确性照常验证；不声称多调度器高可用。事务锁顺序统一job→frame；所有发布/取消/失败路径遵守同一顺序。

批次等待与执行分别记录。入队等待最多120秒，到期标QUEUE_TIMEOUT；开始执行后统一120秒hard deadline。每张请求超时取min(30秒,批次剩余时间)，到期不启动后续帧，已提交结果保留，未执行/被中断帧逐项failed或cancelled并说明，批次job为failed且有partialResults，不能叫completed。429只准一次受剩余deadline约束的退避重试，不无限积压。性能目标p95与这些硬截止分别验收。

## 3. 高风险规则

所有候选都需要人工决策；高风险的接受、调整、拒绝均需逐项检查和提交，不代表其他框自动正确。

| 级别 | reasonCode | 初始规则 |
| --- | --- | --- |
| 框 | LOW_CONFIDENCE | 0.25≤score<0.55 |
| 框 | SMALL_HEAD | 映射原图后 min(width,height)<12px |
| 框 | CLIPPED_BOX | 原模型框逆映射后超出图片边界并发生裁剪 |
| 框 | TILE_BOUNDARY | 同一精确Nano权重分块推理时头框靠近分块边缘，需人工核查 |
| 框 | POSSIBLE_DUPLICATE | NMS后仍存在IoU≥0.35的候选对 |
| 框 | OVERLAP_EXISTING | 新候选与已有正式框IoU≥0.35，必须明确处理，避免重复接受 |
| 图片 | NO_DETECTIONS | 有效推理返回0个候选 |
| 图片 | AI_FAILED | 模型/请求失败，提示人工核查，不计为模型可疑候选 |

风险策略版本固定保存，配置范围校验；原始风险不能靠客户端score/riskReasons更改抹除。未解决风险点击可定位图片/框、查看原因并决策；pending/未解图片问题阻止完成与训练导出。

模型失败不能妨碍人工保存：用户选择“转人工检查”后实际确认图片级问题，完成后可导出人工标签；该操作不是AI已成功。NO_DETECTIONS要求明确人工确认零头或人工补框，不把0个候选当0个真实人头。

风险不用于推断全部漏检，P1不实现未经校准的“漏检概率”“学生身份”或“已准确识别人数”。

## 4. 指标的定义

按工作区/视频/批次范围统计**当前激活run**，历史run独立可追踪，避免反复推理重复统计。0分母显示“无样本”，不是100%。

- 无需调整通过率 = accepted / (accepted+adjusted+rejected)。这属于人工审核结果，不命名为检测准确率。
- 修改率/拒绝率用同一已决策分母，pending单列，尚未审核时不进入分母。
- 框可疑率 = 原始有风险的候选数 / 当前run候选总数；被拒绝仍保留于原始风险分母。
- 图片可疑率 = 有原始框风险或NO_DETECTIONS的有效已推理图片数 / 有效已推理图片数；失败图片单列，不算有效推理。
- 复核进度 = 已决策候选 / 当前候选总数，同时显示高风险未解决框/图片数。
- 人工补框量 = 当前run产生后新增且保留的manual正式框数；不把run之前已有手工框算模型漏检。
- precision/recall需要独立人工参考标签、匹配IoU/样本范围；没有就显示“未评估”，不从confidence换算。

真实示例记录候选数量、人工修改/删除/补充次数与有依据的耗时。若没有人类实际计时，只交付操作日志/样本结果，不宣称省时百分比或跨课堂准确率。任何少量人工参考标注仅用于功能评估，不扩为训练项目。

## 5. 必须覆盖的测试

固定原始模型输出测letterbox逆映射/旋转/裁剪/非有限值/head过滤/NMS；真实ONNX+实际JPEG至少8张记录结果，不用固定bbox fixture冒充模型准确率。候选接受→调整→删除、批量排除高风险、0候选/失败确认、完成/导出门禁、AI stale与取消/重复请求均有行为TDD。

全栈验收读取实际导出ZIP/原JPEG/COCO，验证不包含pending/rejected候选且坐标与原图一致。AI停机时人工画框/保存/完成/导出仍通过。Go/Python只在短事务外做图像I/O。

模型来源线索：[作者README](https://github.com/PINTO0309/High-Angle_Robust_Fast_FaceAlignment/blob/main/README.md)、[现行模型目录471](https://github.com/PINTO0309/PINTO_model_zoo/tree/main/471_YOLO-Wholebody34)、[该目录MIT许可](https://raw.githubusercontent.com/PINTO0309/PINTO_model_zoo/main/471_YOLO-Wholebody34/LICENSE)。作者README中的455引用已失效；现行zoo索引455是另一模型，不能照旧下载。471的LICENSE和作者下载脚本已读取，但不重新下载巨型模型集合；优先核对已评估的精确Nano文件，权重归属/类别清单与实物一致性仍是R0门槛，不把目录MIT阅读当课堂效果验证。

线程设置依据：[ONNX Runtime官方线程说明](https://onnxruntime.ai/docs/performance/tune-performance/threading.html)。上述风险阈值为项目启发式，不是作者性能结论。
