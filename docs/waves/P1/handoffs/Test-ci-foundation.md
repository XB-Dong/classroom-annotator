# Test / Acceptance：Linux CI 基础交接

时间：2026-10-14T02:19:39+08:00；rootTaskId=P1-TEST-001。独立外部 worktree `D:/Project/cvte/worktrees/p1-test`，分支 `p1/test`；基线40b93d4，已挑入Leader的帮助/契约提交4ad2fb5。请求模型gpt-6.1-sol/high，实际provider/model/effort不可独立验证，记null/unknown。没有启动本机数据库、Docker、隧道或持久业务服务器。

## 已实现与真实验证

- P1-CI-001：Linux PostgreSQL17非管理员隔离库；完整前端/Go/race/锁定Python单测；单份作者Nano下载后核对SHA，真实ONNX对合成JPEG计算；精确SHA的server/migrate/import-sqlite/seed-samples/bootstrap-user与frontend-dist发布工件。真实模型合成计算不代表课堂准确率，私有8图证据仍由原AI交接单独保留。
- P1-AUTH/CAS/LOCAL/DATASET/HELP/GATE：独立HTTP测试已编写。20轮各自新revision、真实PG行锁屏障，分别校验恰1成功/1冲突、正式框、revision、审计增量。当前**not_run**，等待Linux CI；没有用串行请求冒充竞态。
- P1-UI/LOWCONC：真实注册/登录与三尺寸截图、两个账户浏览器冲突/草稿保留、5个独立Cookie会话/3账号的10分钟混合负载已编写，当前**not_run**。
- 本机互斥锁`Local\CVTE_P1_BUILD`内：`npm ci`成功；`npm run check`成功；`npm test`9/9通过；`npm run list`成功列出28项，原始日志见外部runtime/p1-test/foundation-{check,unit,list}.log。没有执行PG/API/browser测试。测试只短暂创建并回收Node自身合成fixture进程。

## 旧门槛替换映射

| 旧入口 | P1处置 / 保留的业务不变量 |
| --- | --- |
| migration-check | 保留真实SQLite→PG五表逐字段比对、原备份SHA、非空拒绝与触发器制造故障的全量回滚；schema版本从已提交迁移文件独立计算 |
| postgres-check/restart-check | 保留真实PG20写者CAS、跨帧ID冲突、100次并发事务快照COCO、原JPEG、DB断连失败不修改与真实进程重启；显式合成注册登录/Origin/CSRF适配 |
| mac-media-check / native-agent run-api | 退役只针对Mac代理的元数据协议/云盘无私有字节断言；同机路径由p1-run-stack+p1-sources及原核心API/browser工作流覆盖upload/FFmpeg/抽帧/保存/完成/ZIP/JPEG/COCO；未知退休mac资源不猜本机路径须后续补实测 |
| job-race-check | 旧REMOTE_CLEANUP_PENDING/bridge完成协议退出公共CI；本机实际取消且不发布批次由保留P0-JOB API覆盖。精确取消/完成PG锁竞态仍待补同机替代，当前not_run，不声称已替代全部旧证明 |

## 下一步与限制

Leader集成后才能在当前产品实现上启动完整CI。本次没有发布/部署。Go配置接线使用APP_AI_URL/APP_AI_TOKEN_FILE，Python读取外部模型与AI_INTERNAL_TOKEN；同机目录使用APP_LOCAL_SOURCE_DIRS。CI预计含10分钟固定负载，整体上限30分钟；arm64镜像/轮子验证当前not_run，供Leader后续独立构建。真实GLM产品问答与OpenCode浏览器体验均未执行，不以503降级测试替代。

旧接口不存在与越工作区均返回403以隐藏资源，独立API错误预期已对齐鉴权后行为。待Leader对新的annotation修改者字段完成接线，20轮审计断言才能获得真GREEN。任何Linux首次失败均保留原始证据，不把依赖准备失败记作TDD RED。
