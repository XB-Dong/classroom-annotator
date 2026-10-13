# Test：离线镜像、停机降级与补充门槛

时间：2026-10-14T02:35:20+08:00；rootTaskId=P1-TEST-001；前置交接4841808。模型请求gpt-6.1-sol/high，实际不可验证，保持unknown。Leader继续负责发布/部署与正式修复预算，Test没有执行Windows Docker或本机PG。

## 新增检查与状态

- P1-PACK：`checks`成功后才启动amd64/arm64 AI Docker矩阵。实际容器readiness、固定Nano加载、320×240合成JPEG HTTP推理、模型SHA/输出边界/容器机器架构通过后，才docker-save/gzip为`p1-ai-{arch}-SHA`。元数据保留image-inspect、sourceSHA、平台、模型manifest/MIT及SHA256SUMS。arm64为Linux QEMU，Mac硬件验证明确not_run。全部镜像构建/容器检查当前**not_run**。
- P1-PACK：后置arm64 Go交叉编译server/migrate/import-sqlite/seed-samples/bootstrap-user；同SHA已通过checks的前端原样复用。交叉编译不冒充arm64 Go运行。setup-qemu v3.6.0实际GitHub解析SHA=`29109295f81e9208d7d86ff1c6c12d2833863392`；download-artifact v4.3.0实际SHA=`d3f86a106a0bac45b974a628896c90dbdf5c8093`。checks上限按Leader要求45分钟，镜像job30分钟、交叉编译10分钟。
- P1-LOCAL：实际FFmpeg输出先达到PG发布事务触发器屏障，然后真正cancel HTTP请求阻塞于产品job行锁；释放后完成事务胜出且恰好1批次/3帧。和原P0取消不发布测试合并，替代Mac bridge完成协议的业务不变量。精确cancel-first发布锁竞态仍未补，不能说两种排序都已独立验证。当前PG检查**not_run**。
- P1-LOWCONC：固定5独立browser context/5会话token/3账号，10分钟读写/完成/导出期间额外触发真实AI并逐项复核。外部runner每秒采样Go/Python RSS，记录机器CPU/内存及峰值；这不是2核4GB云端资源门槛。当前**not_run**。
- P1-DEGRADE：runner真正停止其拥有的Python进程，保留退出证据；API验证AI失败风险可确认、人工保存/完成/独立JPEG+COCO导出；随后再次实际跑浏览器人工闭环。当前**not_run**。
- P1-AUTH：独立发现退出失败被authGeneration转换为STALE_AUTH_RESPONSE后静默忽略，Leader确认FrontendB也独立发现并已准备修复。新增真实浏览器abort/logout必须显示撤销未确认提示。正式修复计数由Leader记P1-FRONT-001第1次，Test没有重置或额外分配预算。

## 真实证据

Windows互斥内`npm run check`退出0；`npm run list`退出0，31项/14文件收集。`python`仅AST解析两份独立检查脚本成功，未运行模型；`bash -n scripts/verify.sh`与`git diff --check`退出0。原始Node日志见外部`runtime/p1-test/portability-{check,list}.log`。工具9项通过沿用本切片未变的基础检查证据，不把收集数量当成31项通过。

本切片无业务实现变更。新增tests路径、README与已明确授权由Test单独拥有的workflow；Dockerfile/Compose仍由Leader负责，未编辑。真实GLM问答/OpenCode独立产品体验、云端部署/30分钟资源观测、退休mac资源410、AI候选接受/调整/删除统计的独立全栈fixture仍需后续验收；不以编写脚本代替通过。
