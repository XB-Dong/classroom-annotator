# P1 R8 独立测试与验收审计（草稿）

记录时间：2026-10-14T09:23:38.000+08:00；精确提交：`950083081de7f6590be8eeb3edf0c46225c2d45a`。请求配置为 gpt-6.1-sol/high；实际 provider/model/effort 不可核验，均记 null。

R8 静态审核通过，未发现新的已证实缺陷。P1 整体仍是待验收的部分交付：当前提交完整 CI、实际 GLM 浏览器体验、真实 Go 问答、云端正式迁移/恢复及30分钟观测、录屏等强制门槛尚无通过证据，不能标 completed。本报告不查询 CI、不运行测试或产品环境，仅为 Leader 提供独立验收索引草稿。

## R8 精确范围

获批补丁 SHA256 为 `14bc58c80922c7857b5e03aa73ac8806a8e1daa6e9653f4fca49f15d882a253a`，实测文件哈希一致。新测试 hunk 与获批补丁一致：只在现有无效导出选择器用例中，通过 upload→extract 创建有权限的视频、批次和第一帧，再替换相应 ID。mixed video/frames、batch without video、mixed batch/frames 的无效结构保留；missing selector 和 empty explicit frames 仍原样进入检查。

`safeError(..., 400)` 原断言完整保留；safeError 的 Error schema 和路径/SQL/Authorization/API-key泄漏检查未改。已有缺失资源403、未登录API/媒体/ZIP401、外工作区媒体/HEAD/Range/ZIP403测试保留。会话、Origin/CSRF和真实抽帧助手未改。

冻结路径先通过 auth.Protect/Scope；Scope 对 video/batch/frame 逐项核对实际工作区，缺失或越权资源都403。以前的未创建 synthetic IDs 因此到不了 exports.selection 的无效参数400。现在的真实拥有ID可进入业务选择器校验，预期400依据充分；这只是静态结论，运行Green仍待当前CI。

相对旧CI提交 `dcf046e2ca4bf916d0faa83aaaa55c7d86d51932` 的全部差异只有7份文档/审计文件和1份测试文件；backend、frontend、AI、deploy、契约、helper无变化。本次提交本身为6文件。初读工作树clean；git diff --check退出0。未删改旧Red或把旧CI标为Green。

## 证据资格与当前门槛

旧CI [38463091137](https://github.com/XB-Dong/classroom-annotator/actions/runs/38463091137) 的source为 `dcf046e2ca4bf916d0faa83aaaa55c7d86d51932`，整体 failed。仓库报告记录：Python28、真实ONNX合成JPEG、前端19测试文件/type/lint/build、Go真实PG全部模块及race、Node11、schema8合成SQLite五表忠实导入/回滚、20轮屏障CAS、DB恢复就绪及重启JPEG/COCO通过；实际API19通过、3个旧导出选择器测试失败。浏览器、多架构与发布工件未运行。本审计读取报告和索引，不读取原始CI日志；这些均只登记历史证据。

模块TDD按原交接保留：AI坐标/HTTP/tiling等真实Red→Green及30项历史通过；auth stub/security seam的真实Red→Green；frontend账号、草稿、review、退出、local来源等行为Red→Green。依赖、格式、收集和fixture算术错误不当产品Red。当前提交没有独立运行Green，不将历史模块通过复制为当前强制门槛通过。

真实模型的8张JPEG来自同一受限视频的相关帧（PNG在内存转JPEG），不是8个独立课堂样本。历史实测候选及明显漏检观察保留；没有独立precision/recall、跨课堂准确率或省时比例。CI合成JPEG模型运行与这份真实图观察单列。此次未查看/复制私有像素或overlay。

实际GLM代码审核为 OpenCode/company-glm/glm-4.7-flash，R1/R2/R3/R4均有逐项处置；它们不等于真实公开隔离实例的注册登录/预标注/风险/保存/导出浏览器体验，该项not_run。供应商单题直连HTTP200/10.75秒也不等于实际Go产品API的3操作问题+1文档外问题，后者not_run。

全部20项强制矩阵行逐项见 `acceptance-index.json`。每项status指当前SHA完整门槛，现均not_run/pending；每项保留历史证据引用与缺少的运行范围。P1-CI旧失败明确单列，不掩盖。无隧道源码、证书准备、外部恢复helper准备都不能替代P1云端HTTPS、媒体鉴权、正式PG备份/匹配恢复、Prometheus/Grafana实查和30分钟健康资源观测。amd64/arm64容器构建/模型加载及录屏待验收。真实公司业务API与最终Mac实机保持明确非阻断后续not_run。

## 已知诊断与预算

继续披露已有 `P1-DIAGNOSTIC-001/P3`：backend/internal/preannotation/http.go:126 的增强review409 JSON缺requestId；触发为可读latest snapshot时以过期expectedRevision提交review PUT或单帧preannotation。公共X-Request-ID、409/currentRevision及CAS仍在。未运行复现，没有新证据表明数据损坏；这是既有明确非阻断暂缓项，不能由套件通过声称已修好。

用户获批续作允许完成R8验收和达标发布，旧08:30窗口作为历史保留，没有新截止。波次8/8、Test3/3为明确例外，其余父任务默认上限2，旧P0.1为6/6。此次消耗0修复；不增加、重置或换名避开预算，不进入P2。

本次只读审计未进行产品/仓库修改、API、DB、密钥、浏览器、build、SSH或CI查询；没有启动服务/持久进程。仅在仓库外生成本报告、`review.json`与`acceptance-index.json`，由Leader核对并整合真实后续证据后决定发布。


## 当前SHA R8 GLM实际代码审核补充

OpenCode/company-glm/glm-4.7-flash实际运行于09:14:41–09:17:15（+08:00），source9500830，exit0，153.160254秒，未超时。原始stdout SHA256为 `7cdb73017bc4d0d51837973bbd0f2cf23b1636824ffa5a4a2d7e65a44eef55bd`。1次记录运行、11次read工具尝试，全部completed；9个唯一文件为1份REVIEW说明及8份源码/测试；读取错误0、截断标记0、其他工具调用0。两次重复读取是说明和scope，不冒充额外文件。8份快照均与当前仓库SHA256一致。GLM未运行测试；叙述中的“查看git log”等没有对应实际工具调用，不算覆盖。

外部SPEC/QUALITY的PASS标签不作为全P1验收。五项逐条处置如下，准确源码引用与边界详见review.json的glmR8：

1. 外工作区batch覆盖“缺失”：原auth.spec确实创建实际批次、变更所属video workspace并断言批次preannotations403；source_scope_test另覆盖batch/review，建议operations不是产品已注册路由。拒绝具体缺失主张；更宽泛的未来路由覆盖没有可复现缺陷证据。
2. body读取1MiB+1允许超大请求：多1字节用于溢出探测，后续立即len>1MiB返回400，尚未授权/处理业务；拒绝。此次没有发超大API请求。
3. 无video job的默认workspace不是实际工作区/任何workspace可访问：migration00005已插入同一默认Shared workspace；公开注册和会话只允许该workspace，import任务在完成发布前合法无video。Repository仍比较Actor.WorkspaceID，外workspace不匹配；拒绝。此单workspace设计不扩称未来多租户支持。
4. videoId无frameIds产生空导出：video scope合法，Snapshot零completed帧返回ErrEmpty，HTTP映射422，api.spec:44已有独立422断言；拒绝。此次不声称实际下载通过。
5. Retry-After60“硬编码”为缺陷：当前10/min固定限流与明确Retry-After60就是现有可观察契约，交接记录相同；审核未提供要求可配置窗口的冻结需求，拒绝作为缺陷。改变策略属于后续契约选择，不申请额外修复。

当前SHA的有界代码审核可登记passed子项；P1-GLM整项仍not_run，因为实际公开隔离实例浏览器体验缺失。未增加修复轮次。

Leader另报告公开sample01第一张320×240 JPEG在Windows真实Nano预览返回0候选。本审计已读取外部public01-model-preview.json摘要：JPEG SHA256为5d783ef0ff00681792ca00c95a16f633158c4e720d69865be8c0ab52de29749f、实际model timing94.971ms；未读取像素或重跑，仅作为Windows低分辨率限制，不能计云端验收或非零AI输出。零候选时图片问题确认与人工补框流程合法。此前授权1080p相关8图的非零候选是私有模型观察，不向GLM提供，不宣传准确率保证。


