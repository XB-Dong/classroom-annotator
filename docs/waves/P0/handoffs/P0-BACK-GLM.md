# Backend GLM 代码审核与 Leader 处置

审核对象：`ee2c927aa99b3e30a84449c92d41dba18ffadf42`。OpenCode 1.19.0／`company-glm/glm-4.7-flash`。两次运行均退出 0，实际读取限定范围 14 个后端源码文件；没有运行命令、修改文件或访问私有媒体。

首轮复用了前端审核提示，模型错误地用后端事务证明 Konva、导航和浏览器草稿正确；该结论不采纳。Leader 修正后端专用审核范围后再执行，第二轮给出 SPEC FAILING / QUALITY MIXED。以下为逐项处置，审核整体结论不能直接用作验收结果。

| 第二轮意见 | Leader 处置与依据 |
|---|---|
| revision 检查不原子 | 拒绝。读取、条件 UPDATE、RowsAffected、框替换和 commit 在同一事务；UPDATE 带 `WHERE revision=?`。旧版本 409 无写入及竞争测试已通过。模型提出“两次都成功或都失败”的预期也不符合一胜一冲突的契约。 |
| completed 筛选可能遗漏零人头帧 | 拒绝。筛选按帧状态，不按框数量；独立 ZIP 验收明确验证完成零人头图片存在。 |
| 发下载头前没有显式关闭 ZIP | 拒绝。调用链 Create→Build 先检查 archive.Close 和 temp.Close，成功才返回路径；handler 之后设置响应头。不能只看 handler 局部推断资源未关闭。 |
| SQL 列名应使用参数占位符 | 拒绝。列名来自源码固定 SQL 或固定 FrameColumns；用户 ID 使用 `?` 参数。SQL 参数用于值，并不需要把所有固定列名变成运行配置。 |
| 取消没有区别完成与取消 | 拒绝。持久化状态更新和返回 Job 区分 completed/cancelling/cancelled/failed；已完成任务原样返回。删除失败保留 failed/CLEANUP_FAILED，锁文件及恢复测试通过。 |
| path/status 指标可能无界 | 未发现其所指实际问题。path 使用 Gin 注册模板或 unmatched，method 为枚举或 other，status 为 HTTP 状态。未知方法 100 个样本只产生一个 other 标签，独立 Astra 审核通过。 |
| 上传期限不在 multipart 读取之前 | 拒绝。startUploadDeadline 同步设置读期限后才返回，upload 随后建立 multipart reader。真实慢头和慢文件测试验证约 1 秒期限及无残留。 |
| FFmpeg Extract 没有 JPEG 验证 | 拒绝其跨函数推断。Process 使用 CommandContext，抽帧 Service 紧接调用 JPEGDimensions 完整解码并使用实际尺寸；坏输出不会发布批次。跨版本真实测试通过。 |
| 未找到 API Key 隔离 | 记录本次 GLM 源码覆盖缺口：dataset adapter/network 未列入这 14 个文件，不能由“没读到”推断不存在；也不能声称本次 GLM 完成该模块审核。材料驱动合成上游验收和 Astra 组合导入实际验证媒体端没有 API Key。模型建议拒绝不带 Key 的媒体请求误解了上游凭证隔离职责。 |
| 上传错误处理混合 guard/report 和 failure | 不采纳为缺陷。guard 识别超时，failure 映射其他领域错误；清理错误优先保留，具体响应矩阵和慢请求测试通过。 |
| Cancel 不等待任务停下 | 拒绝。异步取消按契约先返回 cancelling，再确认清理后进入终态；同步阻塞取消 API 不是当前要求。 |
| 分页默认值和上限没文档 | 拒绝。api.md 写明 page=1/pageSize=50，OpenAPI 写明 pageSize 最大 100；测试验证分页参数。 |
| frames 预分配可能内存泄漏 | 拒绝无复现判断。抽帧计划在派发前受配置 MaxFrames 上限约束；有限切片分配本身不是泄漏。 |
| INVALID_REQUEST 等错误码不同 | 拒绝。不同领域错误具有契约定义的不同 code，共享同一 Error 结构；要求把它们统一成一个 code 没有业务依据。 |

首轮其他意见同样核对：`len(nil)==0` 不需要额外 nil 判定来避免崩溃；单类别 COCO category ID=1 是冻结格式；CommandContext 已存在；固定 SQL 常量不接收用户列名；有限 float64 加法若溢出为正无穷也会超过图像边界而被拒绝。未据此制造冗余配置或镜像实现的测试。

此次审核的价值是可审计的独立意见及可核查处置，而非保证模型找出真实缺陷。后端质量结论依赖实际代码、行为测试和独立 Astra 审核。原始脱敏日志在源码外 `runtime/glm/evidence/P0-BACK-GLM-001/` 与 `P0-BACK-GLM-002/`；产品浏览器体验单独验收。
