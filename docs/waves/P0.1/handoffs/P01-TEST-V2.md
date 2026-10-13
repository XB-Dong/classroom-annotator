# P0.1 Test / Acceptance v0.2 交接

时间：2026-10-13T13:26:00+08:00。任务：P01-TEST-001；R0设计已完成，Leader已通知数据库契约冻结，可开始独立DB用例。源码基线：`2414176a0f1a197d67bef9a9676a6bc8ef2be368`；工作树：`D:/Project/cvte/worktrees/p01-test`；分支：`p01/test`。恢复时间13:12:51+08:00，截止21:02:03+08:00；沿用原任务修复计数，最多2轮，不重置。

requestedModel=`gpt-6.1-sol`，requestedReasoningEffort=`high`；actual provider/model/reasoningEffort均为`null`，modelVerificationStatus=`unverifiable`，modelEvidenceRef=`null`；contextMode=`independent_task_packet`。没有执行GLM任务。

## 已核对事实与基线

已读AGENTS、CONSTITUTION、P0计划、决策0001/0003/0004、Test角色/历史MEMORY、P0.1 state/tasks/Mac交接及database/mac-media/sources-and-samples/acceptance规格。当前授权以Leader任务包与running state为准；文档中残留暂停/P0截止文字属于历史，不覆盖此次明确恢复。

- 工作树初始clean，HEAD如上。现有run-api/restart/dataset/storage验证使用SQLite、本地上传和APP_DATA_DIR；不能直接作为P0.1 PG/provider验收。
- Node实际为v24.16.0；tests/node_modules不存在；FFmpeg/ffprobe、psql/pg_dump/pg_restore不在PATH，FFmpeg可从既有外部runtime目录显式指定。
- `node scripts/check.mjs`（tests目录）exit=0，输出Acceptance JavaScript syntax checked。
- `node --test tests/unit/zip.test.mjs` exit=1，原因是未安装adm-zip；这是依赖环境失败，不是产品Red。没有运行真实DB/API/E2E，也没有操作现有演示数据库/真实媒体。

## 冻结入口与夹具

API：GET /api/v1/samples；POST /api/v1/samples/imports `{sampleId}`返回201新Video/200原Video；GET /api/v1/media-sources/mac分页；POST同路径/imports `{sourceId}`返回202 Job；POST /api/v1/videos仍为multipart但固定mac；GET /api/v1/dataset/lessons和POST /api/v1/dataset/imports仍使用P0请求；GET /api/v1/media-storage/status使用`{providers:[{provider,available,reasonCode}],sources:[{sourceKind,available,reasonCode}]}`，reasonCode可空。保存/抽帧/任务/导出沿用/api/v1既有路由。

CLI：cmd/migrate up|status；cmd/import-sqlite --source <backup> --report <external-json>，目标APP_DATABASE_URL；cmd/seed-samples --catalog <external-json> --report <external-json>，媒体根在外部。Leader授权Test独占tests/package.json/package-lock.json，核对registry后新增精确pg8.16.3。

夹具均在`D:/Project/cvte/runtime/p01-test-v2`：已知20秒320×240合成视频（[0,20)、2秒间隔10帧）；损坏/超限/断流视频；9项合成素材（video与image明确分型），稳定sourceId、独立catalogFileId；两个HTTP origin分别承载合成公司API与媒体，记录认证隔离；两个已批准公开视频由Leader提供缓存清单/hash/probe。SQLite fixture含五张表、非零revision、完成有框/完成零框/未完成帧、多批次及原样float64坐标。

## 实施计划与独立断言

- [ ] tests/helpers/postgres.mjs：只从仓库外私密DSN文件读取；数据库名严格p01_test前缀；缺文件/非专用目标直接失败；连接/查询/停止均有有限超时。tests/helpers/process.mjs管理自己启动的专用进程，不重启现有实例；输出保留源码SHA、schema、数据类型与exit。
- [ ] tests/scripts/migration-check.mjs（DB001/006）：真PG迁移up/status与重复无变更；SQLite只读摘要和备份SHA；逐列规范化五表数据比较；第二次导入非空拒绝且目标不变；全新专用目标临时trigger在中途拒绝选定frame，断言五表整体回滚、源SHA不变。恢复/启动的状态变化另记，不掩盖导入摘要差异。
- [ ] tests/scripts/postgres-check.mjs（DB002/003/004/005）：20请求同expectedRevision，各自不同框，恰好1个200/19个409，revision仅+1且DB与成功请求完全一致；两帧并发以相反输入顺序争用两个新annotation ID，恰好一个200/一个400，败方无部分替换，旧revision/框保留。持续保存与导出并行，跨两帧原子切换关联cohort，每份ZIP只能含完整单cohort，保留零框完成帧并校验JPEG原字节；记录实际观察次数，不把随机未撞见竞态当确定性证明。
- [ ] DB005确定性取消/发布：合成bridge操作屏障和PG job行锁控制两种排队顺序；取消先赢时无batch/frames，发布先赢时completed及对应batch保持一致。通过pg_stat_activity观察确实等待，而不是单靠sleep猜时序；不新增产品测试端点。
- [ ] 调整restart/storage/restore（DB007/REG001）：专用PG进程保存→停止→启动，快照/revision/进度/JPEG/ZIP语义不变；running中断终态与清理另核对；Leader受限cloud adapter执行pg_dump -Fc/pg_restore至另一个新专用库，恢复后重复API/媒体/ZIP对比，dump SHA留外部。
- [ ] tests/scripts/samples-check.mjs与mac-media-check.mjs（SRC/SAMPLE/DATASET/MAC）：四来源/provider字段、样例重复seed/import不重置标注、版本更新旧图不变、credits/LICENSES；Mac和样例源离线后public_sample全链路可用；Mac失败upload/dataset不得fallback；分页/channel4/play-url/API Key隔离；Range401/206/416、幂等/lease/ack丢失与重启；新mac payload文件扫描与真实Mac门槛单独记录。
- [ ] tests/e2e保留P0画/移/缩/删、串行保存/旧ack、冲突/草稿、刷新/完成零框/ZIP断言，公共示例为默认准备入口；增加四入口可见、离线Mac已加载图片编辑/PG保存及未加载图片503、恢复后加载。UI选择器与媒体契约完整冻结后实现。

## 当前依赖与阻塞

Leader正在准备真实隔离PG和仓库外DSN；未收到前数据库门槛均not_run，绝不跳过当成功。需要分别提供API、import rollback、restore新目标的连接配置/权限或受限创建方案。pg_dump/restore由Leader提供仅可操作专用目标的cloud transport；本机无客户端。测试依赖安装经Leader已授权；registry核验结果和lock完整性另记。产品migration/import/seed/server二进制及精确构建SHA未提供，不能宣称新行为通过。真实Mac/GLM代码与浏览器/运维门槛仍由各责任方交接，合成测试不替代。

所有代码仅修改tests/（含刚获授权清单）及本文；不改backend/frontend/shared/workflows/root scripts。正式交付将附新增文件、提交、逐项退出码和外部原始证据索引。

## 首次DB脚本准备检查（2026-10-13T13:35:37+08:00）

Leader给定冻结规格SHA：`07eea66203680a7421b79c5d3ed8a1d4de968822`。registry实际核验pg8.16.3及其dist.integrity；精确版本写入授权的package.json/lock，npm install exit=0（25packages，0vulnerabilities）。新增helpers/{postgres,process,http}.mjs、fixtures/legacy-schema.sql、scripts/{migration-check,postgres-check}.mjs；注册npm migration/postgres入口。`npm run check`和`npm test`均完成，组合命令exit=0，6个原ZIP/contract单测通过、0skip；git diff --check无错误。这只证明准备脚本语法/原独立单测，不是新PG行为Green。

DSN环境文件名已获Leader认可：P01_DATABASE_URL_FILE（API）、P01_IMPORT_DATABASE_URL_FILE、P01_ROLLBACK_DATABASE_URL_FILE、P01_RESTORE_DATABASE_URL_FILE；对应独立p01_test_数据库。当前API脚本只声称DB003/004：20请求CAS；PG行锁+pg_stat_activity确认两个跨帧writer实际阻塞；反向两个新ID竞态；30张真实合成抽帧组成100次导出与两帧以上原子cohort写入竞争。DB002故障/readiness、DB005取消发布和DB007恢复仍须补齐后实际运行，未跳过当通过。

## 续跑准备检查（2026-10-13T14:25:23+08:00）

已保留并检查此前未提交的postgres helper/postgres-check/restart/storage修改；没有覆盖原证据或改产品源码。沿用冻结规格SHA `07eea66203680a7421b79c5d3ed8a1d4de968822`、running授权及21:02:03截止。repairCount仍为0；实际model字段仍不可核验，不把请求配置当实际模型证据。

- Leader提供4个仓库外DSN文件及受限专用角色。只读连接核验exit=0：目标`p01_test_api`、角色`p01_test_owner`、实际PostgreSQL17.11；未读取/打印凭证，未访问演示业务库。只读连接成功不是DB业务门槛Green。
- DB002脚本准备了仅影响自己server连接的loopback TCP relay，检查默认并发池上限5/空闲上限2、运行时schema mismatch readiness503、断连请求有限超时/安全错误且保存值不变、恢复后ready200。DB004新增100次标注读取，revision必须与同一原子事务的bbox cohort对应；保留100次导出、零框完成帧、未完成排除和原JPEG字节检查。
- DB005新增`job-race-check`与synthetic bridge barrier：取消先排队时实际job行锁观察双方等待；发布先获得job锁时临时专用DB batch trigger/advisory lock阻塞发布，再观察取消等待该产品事务。测试不添加产品端点或hook，finally移除trigger/function。该脚本尚未在产品二进制上运行。
- DB007改为对新独立restore库启动自己server，先比较五表逐字段值，再核对保存快照/revision、进度、JPEG摘要和独立COCO语义。Leader负责受限`pg_dump -Fc`/`pg_restore --exit-on-error --no-owner --no-privileges`；外部dump通过PGDMP头和SHA核对。dump/restore尚未执行。
- 固定保留原4秒夹具，另生成20秒320×240/25fps视频和JPEG供Mac测试；所有文件在`D:/Project/cvte/runtime/p01-test-v2/fixtures`。source fixture精确使用当前只读服务字段`id/url/size_bytes/content_type/sha256`与`/v1/media/{id}`，9个opaque object id分2视频/7图片，不把id直接当sourceId。独立company API/media origins继续检查分页/channel4/play-url及Key隔离。fixture smoke exit=0，认证Range206返回16原字节；这是合成夹具自检，不是Mac产品门槛。
- 2026-10-13T14:20之后`npm run check`、`npm test`和`git diff --check`组合exit=0；6个独立ZIP/contract单测通过、0skip。原始日志在`D:/Project/cvte/runtime/p01-test-v2/harness-20261013T142000/{check,unit}.log`。此后新增job-race脚本又执行`npm run check`/`git diff --check` exit=0；提交前再次完整验证。

待接收migration/import/server实际二进制及构建SHA后依次执行迁移/并发/故障/重启/取消发布/恢复。真实Mac、实际新Mac payload落盘范围、GLM代码与浏览器、云端运维、CI仍为not_run，不以准备脚本或旧P0结果填passed。前端四入口与sample selector尚待冻结；原P0画/移/缩/删、旧ack/冲突/零框/ZIP断言全部保留。

续补：`mac-media-check.mjs`已准备独立原生Go Agent过程，使用外部源服务与公司API合成夹具，断言9素材类型/sourceId映射、登记摘要/幂等、Range206/416、实际FFmpeg10帧、ack重放/冲突、未ack完成资源跨重启保留、原JPEG与COCO零框导出、API/media认证隔离；尚未运行该产品脚本，明确不代表真实Mac或Go→PG完整公司链路。新增迁移字段类型核对及restore的source/provider/媒体引用摘要比较。`npm run check`、`npm test`、`git diff --check`最新组合exit=0，6pass/0skip；本次只提交验收准备工件。
