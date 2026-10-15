# Mac 公司内网数据集导入规则

2026-10-14，用户补充授权。适用范围为公司网络中的Mac本机部署；公开云端QA继续只使用公开或合成素材。此规则不恢复旧Mac媒体服务、relay或隧道，也不改变P1/P2的partial状态。

## 调用顺序与身份

1. Go后端以`X-API-Key`请求`https://kindvideo.gz.cvte.cn/api/v1/datasets/13/lessons?page=1&page_size=50`，取得课节及其视频。继续分页时使用上游`page`/`page_size`，不猜测未返回的视频。
2. 从响应中的视频条目取得播放接口所需标识，再请求`https://kindvideo.gz.cvte.cn/api/v1/catalog-files/{catalogFileId}/play-url`。用户口述称`video_id`；当前真实响应同时包含`id`和`catalog_file_id`，本次观察两者不同。既有适配器及本次HTTP200验证使用**`catalog_file_id`**，不得把视频条目`id`、桥接白名单对象ID或业务`sourceId`相互替换。
3. 从播放接口响应的`url`取得实际媒体地址，由Mac当前主机的Go下载到同机媒体卷，再进行FFprobe/FFmpeg处理。API key仅发给配置的公司API origin，不附加到媒体下载请求；播放URL可能带签名，不能写入公开日志、源码、云端数据库或聊天。

当前既有前端选择课堂`channel=4`的视频，并向Go业务入口`POST /api/v1/dataset/imports`提交`catalogFileId`/`name`；浏览器不直接持有上游API key。业务课节入口为`GET /api/v1/dataset/lessons?page=1&pageSize=50`。上游dataset13是外部配置值，不改为代码默认值。

## 本机配置与边界

Go现有配置使用`APP_DATASET_API_URL`、`APP_DATASET_ID`、`APP_DATASET_API_KEY`。凭证仅位于仓库外权限0600配置/密钥文件，不复制到规格、提交、PPT、测试fixture或截图；不输出完整docker inspect Env/Compose渲染结果。当前实现读取环境变量，不得虚称已支持`APP_DATASET_API_KEY_FILE`。

公司API和媒体域名解析到内网时，仅为明确的完整origin配置`APP_ALLOWED_PRIVATE_ORIGINS`。媒体origin使用独立的`APP_ALLOWED_MEDIA_ORIGINS`精确限制；本次实际play-url返回`https://kdl-nas.gz.cvte.cn`。不要允许任意内网地址、关闭TLS验证或把公司请求转发到公网代理。Go数据集客户端按已有直接连接/地址校验策略访问，不依赖文档助手的外部代理。

只能在用户范围内选择需要的单段素材，不自动批量下载。公司视频不得上传到公网QA、GitHub、GLM或其他审核模型；本机可在许可范围内完成标注/导出。公司服务离线或凭证失效时给出失败反馈，人工标注、已导入素材和导出继续可用。

## 本次观察及限度

Mac直接TLS验证通过，课程列表HTTP200：8课节、40视频；播放URL接口HTTP200，未下载视频。响应原件和临时签名URL只存本机仓库外私有目录。此记录证明元数据和取址接口可达，不等于某段真实视频已导入、抽帧或通过候选审核。应用Go入口与下载结果须分别按后续实际证据记录，不能把curl连通冒充完整业务联调。

应用Go入口第一次502，原因是Colima容器无法解析公司的分域DNS；Mac宿主机能解析并不代表容器能解析。已在仓库外的Mac专用Compose overlay中，为两个精确公司域名配置当前Mac解析结果的extra_hosts，不关闭TLS、不开放通用内网、不走公网代理。网络或DNS地址变化后应重新核验并刷新该外部映射，不能当成永久固定地址。修正后真实Go课节入口HTTP200，8课节/40视频；仍未发起视频下载任务。

## 2026-10-15 实际单段验证补记

上述“未下载”是早期观测，后续已在Mac实际Go业务入口导入单段约1.6GB公司视频，60–63秒按1秒间隔抽取3张JPEG，任务均completed，鉴权读取三图HTTP200。未进行该公司素材的人工标注、完成或导出；不声称课堂候选正例验收。

实际单段超过原500MiB限制，Mac外部配置APP_MAX_UPLOAD_BYTES已设为2147483648（2GiB），公网QA未改变。NAS服务未提供RapidSSL中间证书导致容器首次TLS校验失败；从证书AIA取得官方RapidSSL TLS RSA CA G1，使用可信根验证后补充到保留原系统根证书的bundle，以Mac专用只读卷提供给Go的SSL_CERT_FILE。未禁用TLS、未信任任意叶证书。网络/DNS或证书轮换后须重新核验；外部配置与原始证据在本机runtime，不能复制凭证、播放URL或公司媒体到源码。
