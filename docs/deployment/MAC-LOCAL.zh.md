# Mac arm64 本机运行与复现

2026-10-14。当前Mac已完成独立整栈部署和登录冒烟；这是本机验证，不是生产迁移。具体版本/证据以P2报告为准。

## 前提与结构

Apple Silicon主机运行支持arm64的Docker Engine。当前使用专用Colima profile（4CPU、6GiB内存、40GiB数据盘），未配置开机自动启动。应用只绑定127.0.0.1，Go/PG17/ONNX/FFmpeg同机，Prometheus/Grafana仅loopback。FFmpeg/ffprobe使用各自可执行路径，不能因默认PATH缺失判断未安装。

仓库外分别存配置、媒体、PG/监控卷、模型、构建副本/缓存和原始证据。源码目录不放node_modules、权重、数据库或.env。Go/前端/AI工件必须对应精确源码版本；本机独立空库，不自动导入旧P0数据。

## arm64核验

P1通用Compose中若某digest指向amd64，不能只设置platform后继续使用。需要在仓库外overlay中锁定同版本官方arm64 manifest及arm64 Go工件，逐个检查 `docker image inspect --format '{{.Architecture}}' <image>`，再执行合并后的compose config --quiet。PG旧amd64卷不复用为新的arm64空库；遇到已有数据先保留并停止，不能当成空库删除。

迁移使用独立migration角色，app角色只授DML/sequence；显式bootstrap首个管理员，不以首个公开注册者提权。固定loopback HTTP Origin可配CookieSecure=false，LAN访问仍需匹配可信TLS。AI内部token与模型SHA须一致，不向宿主机发布PG/AI端口。

## 产品指南助手

Go配置外部HTTPS endpoint、model和key-file路径。当前本机DeepSeek单题真实Go路由已返回200；这不等于四题全验收或研发独立审核。密钥在仓库外0600文件，容器通过只读专用Docker卷获得UID10001可读的0400文件；不要直接把Mac0600主用户文件假定为容器UID可读。浏览器和AI服务不得拿到供应商Key。

容器与Mac终端网络不同。本机曾出现容器无法解析供应商域名；确认Mac实际代理和Colima到宿主机的可达地址后，在外部backend overlay设置代理，并通过NO_PROXY排除localhost/127.0.0.1/postgres/ai/backend。代理地址按当前Mac实测，不照抄Windows配置。停用代理后外部助手可能失败，但静态手册和人工标注仍应可用。不得关闭TLS验证来绕过网络问题。

## 本机候选升级

先在独立工作树/外部构建目录运行相关单测、类型/lint/build、真实PG与实际浏览器尺寸矩阵，保存失败/成功。新镜像使用独立P2标签和精确本地提交版本，保留旧镜像与外部配置备份。只重建Go与前端时不重置PG卷、模型卷或问答Key卷。检查ready版本、匿名拒绝、登录、核心人工闭环、质量提醒与导出、七服务健康后再给出本机入口。

新代码不改schema时可以回退旧兼容镜像并保留PG/媒体；不能用删除数据卷“回滚”。跨schema升级需另行匹配快照恢复方案。测试库使用独立临时容器和独立凭据，禁止对本机产品库跑破坏性验收。
