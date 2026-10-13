# P1计划审核处置

日期：2026-10-14｜状态：设计意见已处置，产品未实现/验收

专项审核请求gpt-6-astra/xhigh，实际路由不可核实；仅只读文档和已有代码。原始报告在仓库外 `runtime/p1-preparation/architecture-review.md`，不改写原发现。原报告列1 blocker、6 major、3 minor；以下是Leader对草案修订的处置，不声称专项角色已复审或产品已通过。

| 发现 | 处置 | 具体修订/后续行为测试 |
| --- | --- | --- |
| F1，快照保存与AI来源不一致 | accepted | ai-review §2统一服务端事务、唯一provenance/tombstone，移/删/撤销同步决策；导出支持合法AI来源。测试原样保存、伪造/丢字段、跨帧、删后undo与COCO |
| F2，已有标签/重跑重复框 | accepted | 保留正式标签，重叠已有框单列OVERLAP_EXISTING，不自动合并或重复接受；ALREADY_LABELED拒绝原因单列，当前run统计与旧正式标签区别 |
| F3，幂等导致不能主动重跑 | accepted | key表示一次明确意图；指纹含revision/input/model/风险版本与参数；新操作新key，网络重试旧key，批次子映射稳定 |
| F4，pending阻止人工保存 | accepted | in_progress保存允许pending，只有completed/训练导出执行审核门禁；不能靠请求体抹风险 |
| F5，失败/取消/重启写旧风险 | accepted | 成功/失败图片状态都CAS；失败过期仅run提示；单scheduler advisory锁与owner，统一job→frame锁序，不恢复仍有效另一owner |
| F6，迁移工具不能停schema4 | accepted | 权威源为生产SQLite；精确9fafbea工具镜像up到4→同版本import与report核对→P1 up身份/AI回填，不虚构target-version CLI，不导入测试PG |
| F7，旧Basic与新会话冲突 | accepted | 新会话/TLS/媒体鉴权全验证后原子替换Basic overlay；旧实例保护不取消；覆盖决策0006/0007实现选择，迟到响应authGeneration隔离 |
| F8，任务过大与晚发布 | accepted | 45分钟slices继承root计数；07:15完整产物、07:35GLM完成、07:45正式ready；错过保持原公网/partial，30分钟观察不压缩 |
| F9，批次120s与每帧30s | accepted | 等待/执行各120s硬截止，单帧取剩余时间；429最多1次受限重试，部分结果明确failed，不假全部完成 |
| F10，数据库快照不保证图片 | accepted | 正式JPEG不可变管理副本、SHA/size；旧无SHA先登记；完整ZIP校验后才下载，图被改/缺失失败，不能回部分包 |

此外明确GLM使用独立QA数据库/媒体卷的公开实例，不能只在正式共享工作区换账号。已修正模型出处：作者旧455链接失效，现行471目录有MIT文件；实际权重许可/类别/哈希验证仍是R0门槛，不把网站阅读当模型已验收。

剩余执行前条件：用户审核批准/明确启动、批准P1有限修复和模型fallback建议；R0确认真实权重/许可、可用依赖镜像/TLS和剩余关键路径；唯一OpenAPI字段和迁移顺序同步冻结。没有未知项由各Agent自行随意选择，也没有准备期开发/部署授权。

## 0.2.0追加范围的Leader自审

用户补充整文档引导、不做向量化并再次要求注册登录。新增document-help规格、受控用户手册、P1-HELP-001任务/验收，检查全文发送、文档/引用一致性、身份/CSRF、限流超时、信息外发边界及失败降级。产品引导调用Go→GLM，不复用OpenCode为线上服务；不得读业务数据、执行标注工具或替人完成风险复核。

此追加仅经过Leader文档自审，未重新调用Astra/GLM，也未运行产品API。先前专项审查只覆盖0.1.0主体，不能据此声称新增问答已独立通过。实际API上下文/回答/引用与注册登录验收仍是执行期门槛。
