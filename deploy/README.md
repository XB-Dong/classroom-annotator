# P0 部署与恢复

目标是 Ubuntu / linux-amd64 / 2 CPU / 4 GiB 演示环境。Leader 负责实际发布。无 AI 服务、Docker socket 或公开业务端口。依赖 Docker Compose、已有的官方运行时镜像，以及本地构建的 Go 二进制和 Vite dist；云端不需要 Node/Go 构建镜像。

1. 在开发机运行统一检查，记录 Git 完整 SHA。在 `frontend/` 执行 `npm ci && npm run build`。在 `backend/` 使用 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../deploy/artifacts/backend-server ./cmd/server`；PowerShell 使用环境变量赋值。`deploy/artifacts/` 已忽略，不能提交二进制。
2. 将受控代码/配置、`frontend/dist/` 和二进制传到云端发布目录。将 `.env.example` 复制为仓库外、权限 `0600` 的环境文件，设置完整 SHA、应用镜像唯一版本标签、外部卷名称和独立 Grafana 密码。公司 API Key 只在外部文件配置；`APP_DATASET_ID` 必须明确配置，未配置应显示 503。
3. 若镜像仓库不可达，先从允许网络验证官方 `linux/amd64` manifest/layers/diffIDs、保存到仓库外并转移 `docker load`。来源锁见 `image-sources.json`。导入后使用 `docker image inspect` 记录实际 ID/RepoDigests；Compose 的镜像环境变量可覆盖为实际导入 digest 或唯一版本标签。所有服务使用 `pull_policy: never`；离线构建时也为 Dockerfile base ARG 设置实际可解析的本地镜像引用，不使用 `--pull`。保留来源 digest 与实际 daemon pin 的关系。
4. 创建三个外部卷（名称与环境文件一致）：`docker volume create classroom-p0-app`、`docker volume create classroom-p0-prometheus`、`docker volume create classroom-p0-grafana`。外部卷独立于发布目录；不要用 `down -v` 清理数据。
5. `docker compose --env-file /external/p0.env config --quiet`，然后 `docker compose --env-file /external/p0.env build` 和 `docker compose --env-file /external/p0.env up -d --no-build`。后端镜像中安装 FFmpeg、ffprobe、CA 与 curl，以非 root 用户运行。记录实际 `ffmpeg -version`、Debian FFmpeg 包版本和应用镜像 ID；apt 包解析结果是发布证据的一部分，当前没有宣称 bit-for-bit 镜像可复现。

端口默认仅 `127.0.0.1:8180/3000/9090`。开发机通过 `ssh -L 8180:127.0.0.1:8180 -L 3000:127.0.0.1:3000 -L 9090:127.0.0.1:9090 <authorized-host>` 访问。Nginx 代理 `/api/`、`/media/`、`/health/`，只服务前端 dist；媒体由后端控制。Nginx 上传限制为 1 GiB；如调整后端上传上限，同步修改 Nginx 限制。

Prometheus 每 30 秒采集后端、自身和主机指标，保留 3 天且不超过 512 MB（WAL 等开销仍需实测）；512 MB 内存限制。Grafana 768 MB，预置不可编辑的数据源/仪表盘；node-exporter 64 MB；Nginx 64 MB；后端 768 MB。全部日志轮转 10 MB × 3。设置的是限制值，实际容量与监控健康必须在云端观测。

发布检查：`docker compose ps` 全部 healthy；`/health/ready` 版本等于实际 SHA；Prometheus `/api/v1/targets` 三个 targets 为 up；Grafana 预置 `p0-prometheus` 与 `classroom-p0`；用合成媒体运行 API、真实 Playwright 核心流程及 GLM 实际浏览器体验。首页可访问不能代替这些检查。数据卷使用合成资料，真实课堂素材只在已授权 Windows 环境。

备份在停止后端后进行，避免复制 SQLite/WAL 中间状态。以下示例名字要与外部配置核对；备份目录在源码外，镜像引用使用已经加载的 Debian runtime。

```sh
docker compose --env-file /external/p0.env stop backend
mkdir -p /external/backups/p0
docker run --rm --user 0 -v classroom-p0-app:/source:ro -v /external/backups/p0:/backup debian:bookworm-slim tar czf /backup/app-YYYYMMDDTHHMMSSZ.tgz -C /source .
docker compose --env-file /external/p0.env start backend
sha256sum /external/backups/p0/app-YYYYMMDDTHHMMSSZ.tgz
```

恢复到新卷，保留原卷以便回滚：`docker volume create classroom-p0-app-restored`，然后 `docker run --rm --user 0 -v classroom-p0-app-restored:/restore -v /external/backups/p0:/backup:ro debian:bookworm-slim tar xzf /backup/app-YYYYMMDDTHHMMSSZ.tgz -C /restore`。在外部配置中将 `APP_DATA_VOLUME` 改为新卷，重建后端；核对已保存 revision、框、完成进度和导出图片字节。记录备份 SHA、应用 SHA、恢复检查结果。监控卷可独立备份；Grafana/Prometheus 配置已版本化。

回滚保留上一版代码、dist、二进制和应用镜像。停止后端，备份当前卷，恢复上一版配置与镜像标签后 `up -d --no-build`，检查健康和已保存内容。数据库结构不兼容时使用上一版备份恢复到新卷；不要猜测迁移可逆。
