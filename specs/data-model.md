# 数据规格

版本：1.0.0｜状态：已冻结，Astra审核通过

Video: id,name,width,height,durationSeconds,status(ready),createdAt，内部storageKey不返回物理路径。Job: id,kind(extraction/import),videoId?(导入成功后提供，抽帧必有),batchId?(仅抽帧),status(queued/running/cancelling/completed/failed/cancelled),progress,error?(安全字符串),createdAt。Frame: id,videoId,batchId,timestampMs,width,height,imageUrl,status(not_started/in_progress/completed),revision初始0。Annotation: id,frameId,bbox原图xywh,source(manual)，类别固定head。

GET /jobs 返回最近100项任务，含非终态与重启中断失败，刷新后仍可发现导入任务。GET /videos/{id}/batches 返回已完成批次及frameCount；frames/progress可按batchId过滤。导出选择恰为videoId+可选batchId或显式frameIds，不混用选择器；batch必须属于对应视频。

独立 extraction batch ID 分隔重抽结果；同一视频列表明确批次，帧按批次创建时间和timestamp排序，帧ID永不以数组下标代替。标注保存事务内读取revision与expectedRevision比较，成功整组替换并revision+1及更新frame.status。冲突返回409，不改变任何已保存内容。已完成帧继续编辑即进入in_progress，需再次确认完成；显式完成的空annotations合法。

外键、唯一约束和参数化SQL；帧/任务/视频关系必须一致。导出在一致读事务获取选定已完成帧快照，再读取不可变帧媒体；如果没有已完成帧则422。COCO images/annotations/categories IDs 数值唯一、关联完整，类别head id1，bbox xywh与area一致，iscrowd0。ZIP images/<frameId>.jpg和annotations.json，无数据库/私密来源地址。完整构造并关闭临时ZIP后才发送HTTP响应头；图片缺失/打包失败返回安全错误，临时文件请求结束清理。
