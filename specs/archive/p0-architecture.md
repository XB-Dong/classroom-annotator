# P0 架构规格

版本：1.0.0｜状态：已冻结，Astra审核通过

React/TypeScript/Vite/React Konva 前端调用 Go/Gin。Go 唯一业务数据入口，SQLite 保存视频、任务、批次、帧、标注和 revision。能力包 videos/extraction/annotations/exports，平台包 config/sqlite/observability。使用显式 SQL、事务和 repository 边界，不引入 ORM。SQLite 驱动选纯 Go modernc.org/sqlite，锁定可用版本。Python 模块只保留文档边界，不安装/运行。

存储根目录配置，媒体/数据库位于仓库外。上传用临时文件、大小限制和 ffprobe 校验，成功后原子转存；失败清理，文件名不作系统路径。随机稳定ID。工作队列有界，单项 FFmpeg 抽帧，Context 取消/超时；启动时将上次 running/queued/cancelling 未完成工作标为 failed/interrupted，并清理未发布暂存，不假装恢复外部子进程。任务进度单调0–1，终态不可逆。取消进入cancelling后确认子进程退出与暂存清理再成为cancelled；成功发布必须在事务中确认仍为running。

服务地址、存储路径、数据库、FFmpeg/ffprobe路径、API地址/key、并发/超时/上传上限/最大帧数/允许媒体来源都通过经过校验的配置。错误返回安全 code/message/requestId；重启中断使用status=failed、安全error=INTERRUPTED，不新增未定义状态。外部导入默认禁止回环/私网/云元数据，实际内网或合成fixture必须显式配置允许来源；API和媒体分开HTTP客户端，API重定向只能同origin，每次媒体重定向和解析出的连接目的地址都校验，API key不得透传媒体。HTTP超时/重定向/下载体积受限。

抽帧暂存不在/media服务目录；全部JPEG验证后先rename到不可变批次目录，再事务插入frames并标任务completed。启动时清理未完成暂存与未被数据库引用的最终批次目录；DB不发布缺失媒体的frame。旋转视频以实际输出JPEG尺寸作为frame原图尺寸，画布和COCO沿用此尺寸，不盲用编码stream宽高。

Nginx 静态构建并代理 /api、/media，后端不公开存储目录；Compose 业务与监控仅绑定127.0.0.1。Prometheus/Grafana/node exporter按P0计划容量配置，指标标签仅有限枚举。前端源、后端源、端到端测试隔离所有权。实现、测试、部署各记录实际SHA。GitHub CI使用统一脚本；重任务串行以适配2核4GB演示机。
