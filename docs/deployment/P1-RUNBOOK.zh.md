# P1 同机部署操作手册

本版平台使用当前主机的 Go、PostgreSQL、FFmpeg、Python ONNX 和媒体卷。没有云端–Mac隧道、relay或远程媒体代理。云端只使用公开示例与获得许可的临时单段样本；Mac后续运行整套服务，真实实机结果单独验收。

## 1. 构建与运行边界

Docker Engine / Compose运行在部署主机。Windows/macOS需要用户自行准备Docker环境；本次Windows执行端不启动Docker Desktop或持久开发服务。架构匹配Linux amd64/arm64镜像和Go工件；不能将amd64二进制装入arm64基础镜像。AI容器固定CPU单进程，权重精确SHA记录在ai-service/model-manifest.json；权重不进仓库。

将.env.example复制到**仓库外**受限配置文件，填入镜像精确Git SHA、独立媒体/PG/监控卷、数据库角色DSN、邀请码、准确浏览器Origin、内部token文件、公开catalog与同机授权目录。真实密码/DSN/key仅保存在外部文件。AI_INTERNAL_TOKEN必须与AI_TOKEN_HOST_FILE内容相同；公司GLM Key只有启用compose.help.yml时挂入Go，不能给Python/浏览器/GLM体验实例。

Compose基本命令（`<external-config>`、`<release>`是操作者替换的路径）：

```sh
docker compose --env-file <external-config> -f <release>/docker-compose.yml -f <release>/deploy/compose.postgres.yml -f <release>/deploy/compose.sources.yml --profile ai config --quiet
docker compose --env-file <external-config> -f <release>/docker-compose.yml -f <release>/deploy/compose.postgres.yml -f <release>/deploy/compose.sources.yml --profile ai up -d
```

初始化时先创建外部卷并赋予容器UID10001媒体卷权限，启动PG后使用**迁移角色**运行镜像内`migrate up`；授予app角色业务DML和sequence权限，应用启动不执行DDL。明确执行`bootstrap-user --username <admin>`并提供APP_BOOTSTRAP_PASSWORD_FILE创建首个管理员；现有账号不覆盖或提权。公开注册只创建annotator且必须邀请码，不自动登录。

## 2. HTTPS与本机访问

云端添加deploy/compose.tls.yml，映射80/443及仓库外证书bundle/ACME webroot。证书bundle包含fullchain.pem/privkey.pem，密钥保持受限权限。配置公网HTTPS准确Origin、Secure Cookie；未装TLS不得明文开放注册。PG/Python只有内部expose；Grafana/Prometheus只loopback，运维通过普通SSH执行管理命令，不创建转发链路。

本机127.0.0.1访问可显式APP_AUTH_COOKIE_SECURE=false及准确HTTP loopback Origin；LAN多人访问仍需对应可信TLS，云IP证书不能当Mac地址证书。旧P0 Basic overlay在隔离QA验证认证之前保留，新应用会话验收后才在同一发布切换中替换；不要叠加旧password/public overlays冒充完成新认证。

云IP证书由Certbot5.4.0成功完成staging与正式签发。原P0未挂载新webroot，因此引导阶段采用官方支持的manual auth/cleanup hook，将公开challenge送入原Nginx容器；原业务仍401。P1完成切换后改为永久绑定的webroot和部署hook，使用短期证书自动续期与Nginx reload；这只是首次challenge传递方式调整，无应用隧道。当前签发成功不等于HTTPS已安装，实际握手证据在发布报告。

## 3. 备份、旧数据与恢复

现有SQLite生产数据只有按P1部署规格执行一致备份、匹配媒体哈希清单、精确9fafbea schema4工具迁移与只读导入比对后，才升级到新迁移。新空库初始化可直接使用P1 migrator；历史库不得改00001–00004。旧mac provider记录不可用，返回REMOTE_STORAGE_RETIRED；无真实文件映射不得自动改为local。

新PG写入后只回退兼容PG的代码/匹配PG+媒体备份，不能切回旧SQLite丢弃新标注。`pg_dump`/`pg_restore`使用独立restore目标和匹配媒体清单验证，不在当前可写库覆盖恢复。真实文件缺失/哈希变化/旧帧尚未回填SHA均拒绝训练导出。

## 4. 去隧道与故障降级

云端专用classroom-mac-bridge-relay已停止并取消restart，classroom-tunnel只禁用专用登录/forward权限，原管理SSH保留。Mac既有launchd不由Windows卸载；后续Mac交接应显式移除旧隧道任务和19092代理，再部署新整套服务。日志快照及公钥备份受限保留，不删除原媒体或旧审计。

AI/profile未启动时仍可人工画框、保存与导出；已产生的AI_FAILED图片问题需要人工明确检查。指南供应商无额度、繁忙或超时返回HELP_UNAVAILABLE，静态全文始终可读，不把本地固定答案当API成功。资源/认证/模型/DB故障分开报告，结束状态只依据实际验收。
