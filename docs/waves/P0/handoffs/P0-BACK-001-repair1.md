# P0-BACK-001 修复 1 交接

记录时间：2026-10-13T02:50:00+08:00。rootTaskId=P0-BACK-001；repairDispatch=1，沿用累计最多2次修复预算，没有重置。工作树p0/backend，以a4b7b1d为本次起点；集成端24096c0及main.go由Leader负责，本任务未修改main.go、schema、标注或导出实现。

本次代码提交：`88b6f6d7159a8da073d70c8900843028154a83e2`。四项Astra阻塞均实际复现并修复。最终单工作树能力检查通过，集成端完整回归和独立复审由Leader继续。

| 问题与需求 | Red、根因及修复 | Green |
| --- | --- | --- |
| 清理失败却返回cancelled；P0-JOB-001 | Windows测试真实锁定暂存JPEG，原实现返回cancelled且文件残留；defer/rollback删除错误被丢弃，FinishJob无条件把cancelling转成cancelled。暂存和回滚删除现在传播安全ErrCleanup；取消期间该错误保留failed/CLEANUP_FAILED，只有成功清理才允许cancelled。导入/上传回滚也检查删除结果。 | 真实Windows锁定文件测试通过；文件解锁后Recover清除保留暂存；无frames发布。SQL终态和普通成功取消回归通过。 |
| method指标标签无界；P0-OBS-001 | 100个自定义动词产生100个直方图序列；router使用原始Request.Method。现在仅GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS/CONNECT/TRACE，其他统一other。 | 100自定义动词对两类请求指标各只产生一个other序列。 |
| FFmpeg9尾部编码错误；P0-FRM-001 | 实际FFmpeg9.0.2运行原测试失败EXTRACTION_FAILED；MJPEG编码需明确全范围像素格式。抽帧命令仅增加-pix_fmt yuvj420p，仍保留输入后的-ss。 | FFmpeg9蓝帧/无帧尾部/旋转尺寸断言通过；云端4.4.2与容器5.1.9相同测试实际通过。 |
| 读取上传不受超时约束；P0-VID-001 | 真实HTTP停在multipart头或文件内容，150ms预算超过700ms仍阻塞；Context建立在NextPart之后且Copy不检查Context。现在在任何multipart读取前建立Context和ResponseController读取期限，超时关闭body，Copy逐次读检查Context，探测后再检查期限；成功请求撤销期限，超时返回503/UPLOAD_TIMEOUT并清理。 | 原150ms回归通过；实际APP_JOB_TIMEOUT_SECONDS=1配置的头/文件停滞测试和150ms用例重复10轮全部通过（40子用例），1秒配置约1.000–1.003秒返回；无staging/video/DB残留。 |

清理语义：OS拒绝删除时任务不能声称已取消；转为安全终态failed/CLEANUP_FAILED，暂存/未发布批次不公开，启动重整继续尝试清理。后续重整成功不会反向改写该终态。所有错误响应和Job.error均不返回OS物理路径。

超时修复另捕获了期限边界竞争：socket超时可能先取消父Request.Context，随后copy观察到Canceled或BodyReadAfterClose。只检查DeadlineExceeded偶尔返回500；现在同时比较已保存的Context期限，明确超时响应。首次失败及后续10轮Green保存在独立证据中。

FFmpeg测试素材兼容修正：9.0.2的旧`-metadata rotate=90`生成命令没有写Display Matrix；支持该选项的版本用输入`-display_rotation:v:0 90`，旧版本继续原metadata命令。没有放宽蓝帧、尾部或24×32尺寸断言，增加了ffprobe.rotation==90断言。首次新验证查询`-show_entries stream_side_data=rotation`在4.4.2不支持，实际Red保留；改用通用`-show_streams -of json`并解析元数据，跨版本通过。选项行为依据[FFmpeg官方文档](https://ffmpeg.org/ffmpeg.html#Video-Options)，实际素材用ffprobe核实。

审计目录：`docs/audit/evidence/P0/P0-BACK-001/repair1/`。Red/Green、实际命令、带时区时间、exit code、云端安全原始结果均已提交。关键最终检查：

- `PATH=<FFmpeg9.0.2>; go test -race -count=1 ./...`：exit0，真实FFmpeg测试实际执行，没有skip。
- `go vet ./...`：exit0。
- `go build -o <external-runtime>/repair1/backend-server.exe ./cmd/server`：exit0。
- `go test -count=10 -v ./internal/httpapi -run TestUploadDeadline`：exit0，40个慢速真实HTTP子用例通过。
- 实际云端FFmpeg4.4.2：0.42s；容器5.1.9：0.69s；exit0，原始结果green-cloud-crossversion.json。

跨版本Linux/amd64纯Go测试程序SHA256：`D9FF24BD4A2E849006535580F3DE7D5F1B9C423B410DAC4F58B6AAD915CF3CA5`。源码构建时尚未提交；media包逐文件哈希与提交88b6f6d一致，保存在cloud-binary-provenance.txt。运行由Leader协调，没有上传课堂媒体。

API响应结构、路由、DB schema和标注扩展接口保持兼容；新增错误码使用现有503/500响应类型。`media.Service.Remove`现在返回error，全部本能力调用点已检查；其他模块没有使用该清理方法。标准Go HTTP server和Gin1.12 Unwrap提供本次验证的读取期限能力，无需修改main.go。

剩余工作：Leader合入集成端后运行包含标注/导出的全量检查并安排Astra独立复审；GLM产品审核/体验、完整云端应用验收、真实公司环境等既有波次门槛不由本修复证据替代。未发现其他本次审核阻塞待修复；真实公司API仍not_run。模型实际路由仍无额外可核实证据，沿用初次交接的unverified记录。
