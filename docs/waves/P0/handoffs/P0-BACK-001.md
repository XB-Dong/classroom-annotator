# P0-BACK-001 交接

后续审核与修复结果见[修复1交接](P0-BACK-001-repair1.md)，本文件保留初次交接时的原始状态。

记录时间：2026-10-13T02:17:00+08:00。角色 Backend；rootTaskId P0-BACK-001；R1–R2；冻结规格 1.0.0。实际工作树基线 51d05f5，规格树与 Leader 冻结版本一致。仅修改 backend/** 和本任务审计/交接。未合入主分支、推送或部署。

实现提交：`2c41709cc1044bb070b7e2a5eda2fc78d8857b10`（底座检查点）、`d44e6fdbeb7998d2a956f3266660fdc86e02d1a8`（本任务最终代码）。当前任务实现与所列检查通过；P0 整体验收仍由 Leader 判定。

模型记录：requestedModel=gpt-6.1-sol，requestedReasoningEffort=high，contextMode=fresh。实际 provider/model/reasoningEffort 无独立运行配置证据，记录为 null；modelVerificationStatus=unverified。外部审核修复轮次尚未使用；不重置 root 修复预算。

| 需求 | 实现和可观察证据 |
| --- | --- |
| P0-BASE-001 | Go1.25.5、Gin1.12.0、modernc.org/sqlite1.39.1、Prometheus client1.23.2 锁定；显式 SQL、外键和 schema1；配置校验覆盖必需外部绝对存储、源码内存储拒绝、上限、超时、端口、origin；DB重开和schema重复初始化测试通过。 |
| P0-VID-001 | 流式 multipart、文件/请求大小限制、随机存储ID、ffprobe元数据校验、异常清理；非法/超限素材和路径名称测试通过。 |
| P0-VID-002A | 已观察 snake_case DTO、数字安全ID、分页、配置 datasetId 的真实上游路径、play-url、API/媒体独立客户端；API key隔离、API跨origin重定向、媒体跳转私网目的地、scheme/default-port边界测试通过。 |
| P0-FRM-001 | 整数毫秒计划、半开区间、最大帧预算、输入后准确seek、逐JPEG完整解码/实际尺寸；批次独立发布；云端真实FFmpeg检查目标时间、尾部不可用与旋转尺寸通过。 |
| P0-JOB-001 | 持久化任务、最近100项、单worker/有界队列、取消等待清理、超时、重启INTERRUPTED、终态比较与批次发布事务；竞争、重开、孤儿清理测试通过。 |
| P0-PROG-001 基础 | batches/frames/progress按video+batch校验归属；frame.status/revision已建模，当前所有新帧not_started。完整标注完成状态由下一任务实现。 |
| P0-OBS-001 基础 | health/live、health/ready/version、Prometheus请求/任务指标实现；Grafana与部署不在本任务路径权限内。 |

## 验证

审计目录：`docs/audit/evidence/P0/P0-BACK-001/`。保存了实际命令、带时区时间、退出码和原始结果；首次依赖安装不计 Red。核心 Red/Green 对包括 config-plan、lifecycle、upload-adapter、manager-observed-route、publication-router、origin-data-root、config-overflow-origin-port、malformed-upload。

- `go test -race -count=1 ./...`：exit0；Windows GCC可用，全部能力包执行，真实FFmpeg本地因不可用跳过。
- `go vet ./...`：exit0。
- `go build -o <external-runtime>/backend-server.exe ./cmd/server`：exit0，构建产物在源码外。
- 合成 HTTP 上传→抽帧→帧/媒体/进度，公司API→play-url→媒体→成功import任务：exit0。媒体进程边界使用明确的测试Runner；HTTP、SQLite和网络客户端为实际实现。
- 云端 `./media.test -test.v -test.run TestFFmpegAccurateSeekTailAndRotation`：exit0，0.33s。检查150ms取200ms首个蓝帧、950ms尾部无图像、旋转JPEG24×32。此证据仅证明该合成媒体集成，不代表完整云端应用验收。

云测试程序为Linux/amd64、CGO_ENABLED=0，SHA256 `EBE3E3F0AA04FB9DB10A49C42F32A608F5645197B0924C93CC4C40F030394BA2`。构建时工作树为检查点加尚未提交integration_test.go；其media包文件内容与最终代码提交d44e6fd完全一致，逐文件哈希保存在cloud-media-provenance.txt。实际云端执行由Leader协调完成，原始脱敏结果cloud-media.json已复制；没有上传真实课堂视频。

## 下一任务接口与所有权

`store.Store` 导出 `DB *sql.DB`、`Root string`，连接数1，foreign_keys/WAL/busy_timeout启用。`schema.sql` 的frames具有status/revision；annotations列为id全局主键、frame_id外键、x/y/width/height、source='manual'。`FrameColumns`、`ScanFrame`、`Frame`、`Frames`、`CheckScope` 支持下一任务。事务内避免再次调用Store方法以免单连接等待自己。

`domain.Annotation`、`Snapshot`、`Frame` 已导出。`httpapi.Error`、`Decode` 可复用。`httpapi.New(..., extensions ...httpapi.Extension)` 的Extension为 `func(*gin.RouterGroup,*store.Store)`；传入扩展时不注册501标注/导出占位，扩展须注册所有相关路由。下一任务在main传入组合注册闭包。`jobs.Manager.Metrics.Registry` 可注册下一任务的有限标签指标。

Leader已把backend/internal/annotations/**、backend/internal/exports/**、backend/cmd/server/main.go交给P0-BACK-002。本任务自检查点交接后未再修改这些路径；模型/store/httpapi接口保持稳定。

运行配置与备份说明在backend/README.md：APP_ADDR=:8080、APP_DATA_DIR=/data、APP_VERSION、APP_FFMPEG/APP_FFPROBE；APP_DATASET_API_URL为origin，APP_DATASET_ID必需；上游固定 `/api/v1/datasets/{datasetId}/lessons?page=&page_size=`。默认拒绝私网，合成/公司内网需明确APP_ALLOWED_PRIVATE_ORIGINS；API key只用于API。媒体/数据库/缓存/构建产物均在源码外。

## 明确剩余门槛

标注/导出实现、相关指标和全流程浏览器验收属于P0-BACK-002及独立Test；当前路由明确501。真实公司环境未运行，realApiVerificationStatus=not_run。CI、Compose、Grafana、完整应用云端部署/恢复、GLM独立代码审核与浏览器体验由Leader/其他任务继续，不把本任务检查冒充这些门槛。当前 schema 仅创建版1，跨未来迁移回滚需匹配备份。
