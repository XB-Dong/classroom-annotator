# P1验收矩阵

版本：0.1.0｜待用户审核。所有产品门槛当前not_run；历史P0/P0.1结果单列，不能自动转成P1通过。

| ID | 验收方法与必需结果 | owner |
| --- | --- | --- |
| P1-BASE-001 | 核对候选SHA、数据快照、模型文件/许可/类别映射；新增迁移显式执行，保存旧ID/revision/标签，导入比对与独立恢复通过 | Leader / Backend / Test |
| P1-LOCAL-001 | 未配置任何Mac变量/无relay/隧道进程也能示例→抽帧→标注→AI→ZIP；上传和同机目录实际走local；历史mac资源返回已退役而非猜路径 | Backend / Test |
| P1-DATASET-001 | 已观察schema的合成API分页、channel4/play-url/密钥隔离导入本机通过；公司真实API未配置记not_run，不阻塞 | Backend / Test |
| P1-UI-001 | 1440×900/1280×720/窄屏真实截图与操作，手绘外壳不挡画布；快捷键、缩放原图坐标、焦点、错误/空状态无退化 | Frontend / Test |
| P1-AUTH-001 | 注册重复/错误邀请码/过限/登录/退出/过期；2实际账户独立会话；未登录所有API/媒体/ZIP拒绝，用户体role/userId无效，CSRF/Origin校验；草稿跨账号不泄漏 | Backend / Test |
| P1-HELP-001 | 供应商请求包含完整正确版本手册而非片段，引用合法；实际产品GLM至少3个操作问题/1个文档外问题；注册登录后真实提问/章节跳转、API断开仍可读全文；鉴权/CSRF/限流/非法引用/超时/退出迟到结果可控，不发送业务数据或密钥 | Backend / Frontend / Test |
| P1-AI-001 | 真实ONNX加载、至少8个真实示例JPEG推理与非零有效头候选观察；原图输出合法，模型SHA/输入SHA/耗时可追踪，不能用mock代替 | AI / Test |
| P1-COORD-001 | 原始output/head过滤/NMS/letterbox/横竖图/裁剪/非有限值测试；已知框逆映射误差≤1原图像素，真实框与图对应 | AI / Frontend / Test |
| P1-REVIEW-001 | pending→接受/调整/拒绝与补框、再编辑/删除/撤销同步合法provenance；重跑不清正式框，重叠不重复接受；高风险逐项定位/批量跳过/图片问题确认，pending时人工保存可用，所有写入CAS | Backend / Frontend / Test |
| P1-GATE-001 | pending/未解风险不能完成或进入COCO，直接PUT/exports也不能绕过；0头显式完成包含，rejected/pending均不导出；独立读取ZIP/JPEG/COCO验证 | Backend / Test |
| P1-STATS-001 | 根据可计算fixture独立校验分母、重复run/删除/调整、0分母/失败状态；UI真实值与DB一致，未有人类参考标签时准确率未评估 | Backend / Frontend / Test |
| P1-CAS-001 | 两浏览器账户同revision同步保存恰1成功/1冲突，败者草稿保留/无静默覆盖；20次独立同步API竞态每轮恰1胜，DB/审计无副作用；不同帧并行均成功 | Test |
| P1-AIRACE-001 | AI执行时人工写入，返回stale且不影响正式框；同key幂等/参数不符409，新key明确重跑；取消/完成/失败写图片问题竞态均无旧结果覆盖，重启失败状态可解释 | Backend / AI / Test |
| P1-LOWCONC-001 | 5独立会话、至少3账户，10分钟混合读/编辑/完成/审核/导出；同帧冲突为预期409，不计异常，最终正确；无重复资源/跨账号草稿/未授权媒体 | Test |
| P1-DEGRADE-001 | AI进程停止/超时/错误输出，人工新增/保存/刷新/完成/导出仍通过；批次部分失败不冒充全部完成；5会话加AI任务不OOM | Test |
| P1-CI-001 | 精确提交完整CI：前端type/lint/test/build、Go vet/test/race、Pytest、真实PG集成、契约、真实ONNX独立步骤与浏览器；依赖准备失败不是TDD Red | Leader / Test |
| P1-GLM-001 | 固定实际GLM通过OpenCode读代码并完成独立公开数据实例的真实注册登录/预标注/风险/保存/导出体验；Leader逐项处置，截图/下载和退出码保留 | GLM / Leader |
| P1-OPS-001 | 无隧道云端同机版本/HTTPS/Cookie/媒体鉴权/备份恢复；Prometheus目标/Grafana实查，连续30分钟健康资源观测，无OOM，可用内存目标≥512MiB | Leader / Test |
| P1-PACK-001 | amd64及arm64构建/依赖模型加载分别记录；Mac实机未做就not_run；启动/迁移/备份说明可复做，录屏文件真实可播放且不含凭证 | Leader / Test |
| P1-AUD-001 | requirement→task→Red/Green→commit→原始证据→每轮report与最终report；模型requested/actual分开；旧失败/修复计数不删除 | All / Leader |

## 并发与性能口径

固定20轮屏障同步竞态，不用串行两个请求冒充同时。相同revision的全部写入只准一个赢家；失败者的数据、candidate决策和审计不能残留。同帧完成与审核竞争、AI stale、两账户切换在真实PG上验证。

5会话不是5个标签共享同一Cookie。低并发使用5个独立browser context，至少3账户；图像内容/期望最终标注用合成或公开素材独立校验。压测仅专属QA工作区，不污染正式标注；实际授权课堂截图不提交Git/发GLM。

正常读/保存目标p95≤2s；推理单帧执行目标p95≤5s，队列等待单列；等待120秒、开始执行后批次统一120秒分别为硬截止，逐帧timeout取min(30秒,剩余执行时间)，部分提交/未执行帧与failed批次准确记录，不宣称全部完成。记录样本数、机器、并发、冷/热、失败/429；未达性能目标如实列限制，正确性/人工可用不能降低。

录屏默认公开示例，授权视频用于真实模型验证。1段视频抽出的8张图高度相关，不当8个独立课堂评估；通过本矩阵不等于证明跨课堂准确率或保证省时。

全部必需门槛通过才completed。真实公司API与最终Mac实机验收为明确后续not_run；真实模型、注册/冲突、高风险门禁、GLM体验、正式迁移/云端监控不得以“后续P2”推迟。截止08:30自动停止并交partial报告。
