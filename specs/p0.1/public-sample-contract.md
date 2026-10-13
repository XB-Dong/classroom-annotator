# P0.1 公共示例与导出元数据契约补充

版本：1.0.0｜2026-10-13｜当前P0.1实施契约。只补齐已有规格未明确的表示，不增加产品范围。

- `Sample.availability` 为 `ready | unavailable`，与数据库缓存状态一致，不使用布尔值。
- 有限预置清单为 `{catalogVersion:"1.0.0",items:[...]}`。每项包含 `sampleId,version,name,description,creator,sourcePageUrl,licenseName,licenseUrl,downloadUrl,sha256,sizeBytes,width,height,durationSeconds,sourceFile`。`sourceFile` 是相对清单所在目录的文件路径，不是浏览器可提交的URL。发布者校验完整文件后才可seed；媒体、部署生成清单和实际路径在仓库外。源码清单仅保存相对路径和公开元数据。
- `GET /api/v1/samples` 的 `items` 不返回downloadUrl/sourceFile，只返回已有规格中的公开展示字段及availability。选用已缓存的sampleId后返回Video；重复操作返回原Video且不重置标注。
- 公开示例ZIP除原JPEG和COCO外，包含 `credits.json` 与 `LICENSES.txt`。私有资源导出不强加公开示例出处。
- `credits.json` 为 `{schemaVersion:"1.0.0",items:[{sampleId,version,videoId,creator,sourcePageUrl,licenseName,licenseUrl,changes}]}`。items只含本次实际导出涉及的公开示例，按sampleId/version/videoId稳定排序。changes为 `Frames extracted from the original video; no rendered annotation overlays.`。许可文本根据记录中的作者/页面/许可生成，不把所有数据默认标成CC0。
- media Operation 的 `resourceId`：extraction为batchId；registration/upload/dataset_import为videoId；archive为null。jobId仅在实际业务任务存在时提供；不伪造业务job。

公开页面与许可已核对；两个视频已实际下载、完整解码并预览人头。文件身份和环境证据位于仓库外的 `runtime/p01-public-samples/prepared-catalog.json`，不等同于业务seed/云端闭环已通过。
