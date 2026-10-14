# P1 R10 GLM 独立审查Leader审核处置

本稿只做本地原始证据与源码静态复核，不运行测试、不查询当前 CI、不改 code 或交付运行手册。审查基准 `29108b2b37d46a405b940e1d366ff04e1aa49413`。请求 native 配置 `gpt-6.1-sol/high`，实际 model/effort 未核实，保持 null/unknown。

原始证据位于 `runtime/glm/evidence/P1-R10-scheduler-barrier/`。summary 记录 OpenCode/company-glm/glm-4.7-flash，thinking disabled，2026-10-14 10:02:14.546904–10:02:37.657562（+08:00），23.110658 秒、exit0、timeout=false；stderr 空。stdout JSONL 实际只有 4 次 read：REVIEW.md 22行、process.mjs 77行、p1-run-stack.mjs 202行、process.test.mjs 61行；均 completed，元数据无截断/错误。3份源码各读取1次；没有 edit/bash/network/browser 或运行测试工具。opencode 权限仅放行这4个文件，其他工具/目录 deny。该结果只能证明记录的外部审查运行，不能据此核实 native model 路由。

三源码快照 SHA256 与当前文件相同，且 `git hash-object --no-filters` 精确匹配 source SHA 的3个 Git blob。stdout SHA256：`294a34d70f17d8a1f0e69a8effc53cbbd2d9aaa7ebddc52179f00212f6b1fc2d`。具体源码/hash/读计数见 machine-index.json。

GLM 原结论 SPEC PASS、QUALITY PASS，未提出负面可执行缺陷。Root 处置为“限定三文件静态范围接受，修正事实表述，不作为运行/全P1验收通过”。

| 原结论/陈述 | 处置及精确依据 |
| --- | --- |
| marker跨chunk、状态有界、当前进程隔离、可选默认兼容 | 接受静态结论。process.mjs:37–50 校验最多1024字符，每实例闭包初始化false，未观察时仅留marker长度减1的尾串；无marker仍ignore stderr。unit:29–60 有分段、128KiB噪声、重启false和有界未出现等待场景。源码支持有界保留，测试未直接断言内部内存/尾串大小，不称“全面实测内存界限”。 |
| full stderr drain引用process.mjs:10–13 | 引用错误，该处是command()输出聚合。专用进程实际依据37–51：pipe和持续data监听让stderr保持流动；markerObserved后回调return但未移除监听，继续消费，不继续积累。修正引用即可，不作为产品缺陷。 |
| stop()“5秒内未停止才SIGKILL” | 拒绝该描述。process.mjs:54–68 先注册exit与5000ms拒绝计时器，65行立即SIGKILL；5秒是等待exit的上界，不是延迟kill或优雅停机期限。幂等stopping promise/已终止早退接受。这段stop行为在R10前已存在，不能归为本轮新修复。 |
| 所有async均有合适timeout | 不接受全称。仅command 60s、readiness 30s/单fetch 1s、stop退出等待5s、新marker等待5s是源码可见事实；p1-run-stack:120–126子进程等待未在该函数配置timeout。没有本次运行证据，不能由此推断全部异步边界或新增修复必要性。 |
| 无任意sleep、等待当前primary真实转变 | 限定新故障路径接受。stack:77–78、140–147 同一startPrimary配置marker，断DB前assert false，保持故障直到当前backend marker，waitFor 5000ms，重启创建新实例；不是随意延时/降低503断言。176行另有已有61s认证限流等待，不把整个脚本说成无sleep。 |
| unit“全面覆盖核心要求” | 限定为测试设计支持split/噪声/重启/已终止stop；GLM没有执行测试，也未读取scheduler-fixture、proxy或Go产品代码，因此不接受据此声称503/200/manual equality/restart inference已通过。 |

本静态复核另读Git diff及当前Go lease/main以校核审查说明，不能计为GLM覆盖：source commit只有3个测试文件及审计/state变化，没有产品源码变化；lease每1秒watch，先owned=false，再onLoss调用cancelWork并打印marker。新测试保持故障到观察点符合这份说明，不改变产品永久失去租约后需操作者重启的契约。

后续只需 Root 保留这些表述修正与限定范围，读取同SHA完整CI实际结果，确认lease-loss readiness503/恢复DB200/manual不丢失/extraction503/preannotation503/重启后inference及后续浏览器矩阵真实通过。GLM审查不是测试执行；完整CI、独立公开浏览器、真实产品帮助API、迁移/恢复、TLS、30分钟观察、录屏及Mac实机仍按各自证据门槛处置。本次不新增修复、重置计数或更新P1运行手册。
