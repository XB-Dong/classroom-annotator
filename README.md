# Classroom Annotator

面向课堂视频的学生人头标注平台，提供视频导入、定时抽帧、人工框选、AI 辅助复核和 COCO 数据集导出，支持多人在同一工作区协作。

[使用手册](docs/product/ANNOTATOR-GUIDE.zh.md) · [部署说明](docs/deployment/P1-RUNBOOK.zh.md) · [接口文档](specs/contracts/api.md)

## 核心功能

- **视频导入**：支持本地上传、预置公开示例、部署主机上的授权素材目录，以及按需配置的公司数据集接口。
- **定时抽帧**：按固定时间间隔提取图片，可指定起止范围；每次抽帧生成独立批次，保留已有图片和标注。
- **人工标注**：支持新增、移动、缩放、删除人头框，以及撤销、重做、自动保存和浏览器草稿恢复。框坐标按原始图像像素存储。
- **AI 辅助复核**：支持单张或批量生成候选框，标记低置信度、裁切与重叠等复核线索，由人工接受、调整或拒绝。AI 服务不可用时仍可继续人工标注。
- **多人协作**：提供邀请码注册、登录和共享工作区；同一图片发生并发修改时提示版本冲突，保留本地草稿供处理。
- **数据集导出**：导出已完成图片的原始 JPEG 与 COCO JSON，包含人工确认的零人头图片；未审核候选不会成为训练标签。
- **操作指南**：提供内置使用手册和可选的文档问答助手。问答服务不可用时，静态手册仍可查阅。

## 使用流程

1. 使用邀请码注册账号并登录工作区。
2. 导入视频，设置抽帧间隔和可选的时间范围，创建抽帧批次。
3. 选择图片进行人工标注，或生成 AI 候选后逐项复核、补充漏框。
4. 确认保存成功并处理待审核项，将图片标记为完成。
5. 选择视频和批次，下载包含已完成图片及标注的 COCO ZIP。

AI 候选需要人工审核。标注对象为学生可见人头，部分遮挡时框选可见部分；模型置信度不等同于准确率。详细操作与冲突处理见[使用手册](docs/product/ANNOTATOR-GUIDE.zh.md)。

## 系统架构

| 模块 | 技术 | 职责 |
| --- | --- | --- |
| 前端 | React、TypeScript、Konva | 视频与批次管理、标注画布、复核和导出交互 |
| 业务后端 | Go、Gin、PostgreSQL 17、FFmpeg | 账号会话、媒体处理、任务调度、标注保存和数据导出 |
| 推理服务 | Python、FastAPI、ONNX Runtime CPU | 人头检测与候选生成 |
| 运行环境 | Docker Compose、Nginx | 服务编排与统一 Web 入口 |
| 监控 | Prometheus、Grafana | 服务指标与运行状态 |

后端是业务和媒体访问的统一入口。数据库、媒体文件与推理服务部署在同一主机，推理服务仅供后端调用；模型权重和运行数据独立于源码存放。

## 部署与运行

### 环境准备

- 在部署主机安装 Docker Engine 和 Docker Compose，选择与主机架构匹配的镜像和构建产物。
- 准备 PostgreSQL 17、独立的数据库与媒体持久卷，以及对应版本的前后端镜像。
- 启用 AI 时，按[模型清单](ai-service/model-manifest.json)准备 ONNX 权重、推理镜像与内部访问令牌。
- 将[配置模板](.env.example)复制到仓库外，配置数据库连接、邀请码、浏览器访问地址、镜像版本及所需目录。密码、令牌和 API 密钥保存在受限文件中。
- 首次使用前，执行数据库迁移并通过 `bootstrap-user` 创建管理员；应用数据库账号使用业务读写权限。初始化与构建步骤见[部署说明](docs/deployment/P1-RUNBOOK.zh.md)。

### 启动服务

在仓库根目录执行以下命令，将 `/path/to/app.env` 替换为已准备好的外部配置文件。镜像、持久卷、数据库初始化和模型配置应先完成。

```sh
docker compose --env-file /path/to/app.env \
  -f docker-compose.yml \
  -f deploy/compose.postgres.yml \
  -f deploy/compose.sources.yml \
  --profile ai config --quiet

docker compose --env-file /path/to/app.env \
  -f docker-compose.yml \
  -f deploy/compose.postgres.yml \
  -f deploy/compose.sources.yml \
  --profile ai up -d
```

本机默认入口为 `http://127.0.0.1:8180/`，可通过 `APP_PORT` 调整端口。使用 HTTP 本机入口时，需设置对应的 `APP_AUTH_TRUSTED_ORIGINS` 和 `APP_AUTH_COOKIE_SECURE=false`；局域网或公网访问应配置 HTTPS、准确的访问来源与安全会话 Cookie。

文档问答和公司数据集接口按需启用。问答密钥仅由 Go 后端读取；公司接口需要部署主机具备对应网络访问权限。

## 数据与维护

- 视频、抽帧图片、标注数据库、模型权重和访问凭证均不随源码分发；导入素材前应确认使用权限。
- 备份应同时覆盖 PostgreSQL 和对应媒体文件；升级前保留可恢复的备份与原镜像。
- 导出使用已保存的一致快照。图片缺失、内容变化或复核未完成时，按界面提示处理后再导出。

## 文档与源码

| 内容 | 入口 |
| --- | --- |
| 标注操作与常见问题 | [使用手册](docs/product/ANNOTATOR-GUIDE.zh.md) |
| 服务配置、初始化与恢复 | [部署说明](docs/deployment/P1-RUNBOOK.zh.md) |
| HTTP 接口与数据约定 | [接口文档](specs/contracts/api.md) |
| 前端运行与构建 | [frontend](frontend/README.md) |
| 后端配置与运行 | [backend](backend/README.md) |
| 推理服务与模型要求 | [ai-service](ai-service/README.md) |
| 测试环境与验证命令 | [tests](tests/README.md) |

开发环境的统一检查入口为 `bash scripts/verify.sh`，Windows 使用 `scripts/verify.ps1`。数据库相关测试应使用独立测试库。
