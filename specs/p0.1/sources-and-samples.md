# P0.1 四类素材入口与公共示例规格

版本：0.2.0｜状态：暂停后的修订稿，待审核｜日期：2026-10-13

## 1. 产品行为与数据边界

素材区固定四个可见入口：公共示例、Mac素材、本地上传、公司接口。公共示例为默认入口；服务不可用只影响相关入口，不隐藏其他能力。

用户确认的映射：public_sample→local，mac/upload/dataset→mac。local表示云端演示业务主机的媒体卷。PostgreSQL保存四类来源的元数据/任务/标注；视频、JPEG、ZIP均为文件资源，不写为DB blob。

来源与存储用两个字段：sourceKind说明如何获得素材，storageProvider说明物理位置。当前部署不提供让普通用户随意把私有上传标成public_sample的选项；公共样例只能来自发布者核验的预置登记表。

不使用APP_MEDIA_PROVIDER全局开关决定所有资源；上传固定经Mac，样例固定在云端；读取/抽帧/导出按DB每条记录的provider路由。旧记录标legacy并保留当前位置，不猜测其公开性。

## 2. 四入口接口草案

| 浏览器API | 请求/响应 | 责任 |
| --- | --- | --- |
| GET /api/v1/samples | items包含sampleId/name/description/sourcePageUrl/creator/licenseName/licenseUrl/availability/width/height/durationSeconds | 列有限公开清单，不实时搜索外网 |
| POST /api/v1/samples/imports | sampleId / 201 Video；已登记返回200同Video | 只注册已缓存完整视频，幂等不重建用户标注 |
| GET /api/v1/media-sources/mac | 既有草案分页+媒体类型 | 从Mac授权清单映射可选视频，不暴露路径 |
| POST /api/v1/media-sources/mac/imports | sourceId / 202 Job | 只在Mac登记/核验媒体 |
| POST /api/v1/videos | P0既有multipart file / 201 Video | 本地上传流式到Mac，探测/发布完成才返回成功 |
| GET /api/v1/dataset/lessons | 保留P0页参数和已观察列表结构 | 云端Go调用Mac适配器，Mac访问公司API |
| POST /api/v1/dataset/imports | 保留catalogFileId/name / 202 Job | Mac获取play-url并下载，Go发布安全元数据 |
| GET /api/v1/media-storage/status | providers/sourceKinds各自的available/reasonCode | local/mac以及dataset配置分开，不返回秘密 |

public sampleId仅用于预置公开登记，不能由上传者提交任意URL；Mac sourceId与公司catalogFileId是不同域。四入口成功后进入同一个视频/批次/画布/保存/导出流程，不复制四套编辑器。

公共示例不可用返回503 SAMPLE_UNAVAILABLE；sampleId不存在404；Mac离线503 MEDIA_SOURCE_UNAVAILABLE；公司未配置503 DATASET_NOT_CONFIGURED。不能用“Mac模式不可用”关闭配置正确的公司入口。上传不在Mac离线时回落到云端文件。

## 3. 初始公共示例清单（网页信息已核对，尚未下载）

选择Wikimedia Commons两条公开课堂视频，作为初始下载登记项；它们用于真实视频工作流展示，不代替公司场景算法准确率评估。

| 拟定sampleId | 素材页面/作者 | 页面标注的许可与元数据 | 展示说明 |
| --- | --- | --- | --- |
| public-classroom-01 | [DeutschunterrichDikome2014.webm](https://commons.wikimedia.org/wiki/File:DeutschunterrichDikome2014.webm)，Frosch79618 | CC BY-SA 3.0，320×240，约86秒，约5.51MB | 公开课堂教学视频；低分辨率如实显示，不放大冒充1080p |
| public-classroom-02 | [Acrobatics at school, 2009.webm](https://commons.wikimedia.org/wiki/File:Acrobatics_at_school,_2009.webm)，H3lla3x | CC0 1.0，320×240，约42秒，约2.58MB | 公开教室活动视频，用于可重复演示 |

核验时间2026-10-13。以上时长/尺寸来自页面，实际值以恢复后ffprobe和解码验证为准；当前没有文件SHA-256，不能编造。许可链接分别为 [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) 与 [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/)。

初始清单在R0下载后逐条实际预览人头是否可见、检查decode与原图；没有合适画面则停止发布该条并记录原因，提出可核验的清单修订，而不是用黑块图替代。公共示例数量目标至少两条，达到后不继续无限搜素材。

## 4. 初始化和更新

1. 恢复开发后发布者从上述文件页的实际原文件链接获取素材，校验跳转来源/体积/解码；下载在Windows外部准备目录或授权云端样例目录，不写code。
2. 记录页面URL、作者、许可URL、原文件URL、retrievedAt、SHA-256、字节大小、probe元数据和sample version，形成可审计清单。
3. 将媒体一次性转移云端样例卷；cmd/seed-samples依据登记表与hash幂等初始化数据库，默认不抽帧、不伪造已完成标注。浏览器点击选用后走正式抽帧接口。
4. 同版本重复初始化不新增Video，不删除/重置旧标注。原内容变更以新版本/key登记，旧标注关联旧图保留。
5. 初始化失败或未缓存只展示明确不可用状态；成功缓存后外网源站关闭、Mac关闭也能完成核心流程。

不提供普通浏览器任意URL下载入口，不做线上爬虫、不实时热链播放；网络获取只在有限发布初始化阶段发生。清单/出处为项目文档和配置，媒体及下载工具在仓库外。

## 5. 示例导出与许可信息

ZIP保留原JPEG与COCO，额外带公开样例的credits.json/LICENSES.txt，列作者、页面、许可、是否抽帧/剪辑。CC BY-SA样例及其派生媒体按相应许可处理；不把公共示例当成公司真实数据，也不把数据许可当成应用源码许可。

原JPEG不烧框，bbox仍在COCO JSON。标注与许可信息不能出现在公司素材的原始路径/鉴权URL中。sample scope只导出用户选定的已完成帧，显式完成零框帧保留。

## 6. 公司接口链路

入口保持P0业务API；目标链路：Browser→Go→受限Mac桥接→公司API/media→Mac文件→Go元数据/任务→PG。

Mac复用项目已观察的Go数据集适配器：datasets/{datasetId}/lessons分页、items[].videos[]筛数字channel=4，以catalog_file_id获取catalog-files/{id}/play-url，再使用url下载。公司Key只在Mac；API与媒体鉴权分开；签名url不入云端DB/日志。具体真实响应证据以原交接包为依据，不猜测新结构。

项目Mac桥接新增拟定控制路由：GET /bridge/v1/dataset/lessons、POST /bridge/v1/dataset/imports，返回安全列表/操作结果。它们是本项目拟定契约，不声称Mac目前v1服务已有；完整响应冻结与R0字段核对后再实施。

P0.1要求材料驱动的合成公司API从四入口走通、channel/分页/play-url/Key隔离和Mac存储验证。真实公司内网API仍可单独not_run；显示未配置不能冒充适配成功。真实测试需用批准的数据范围与环境，暂停期间不执行。

## 7. 验收

- P01-SRC-001：四入口可见，source/provider映射与返回字段一致，原核心画布共用。
- P01-SAMPLE-001：至少两条已缓存真实公开视频，出处/许可/hash/probe完整，可选择使用，无占位假图。
- P01-SAMPLE-002：Mac/公网素材源离线时，公开示例登记后的抽帧→标注→保存→刷新→重启→ZIP完整可用。
- P01-SAMPLE-003：重复seed/import不重置标注；更新不改旧帧身份；导出保留credits/license。
- P01-SRC-002：本地上传和公司媒体在Mac保存，云端无相应payload文件；Mac失败无静默云端fallback。
- P01-DATASET-001：实际项目适配的合成内网API链路通过，真实内网结果单独记录。

这些门槛在[验收矩阵](acceptance.md)追踪；产品实现与新数据下载均等待用户恢复批准。
