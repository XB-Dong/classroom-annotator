# P1 首轮 GLM 审查处置

2026-10-14 02:24（北京时间）；实际外部 OpenCode / company-glm/glm-4.7-flash，退出码0，约32秒。精确快照4ad2fb5；原始工具/回答存于仓库外 `runtime/glm/evidence/p1-r1-auth-ai-code/`。模型确实运行，意见不是发布结论。Leader对照代码及现有检查独立判断如下。

| GLM意见 | 处置与依据 |
| --- | --- |
| 删除sessions不算即时退出，仍有效12小时 | 拒绝。Authenticate每次查询PG；DELETE即时撤销，该断言与repository.go/SessionByDigest条件矛盾。真实PG重启/退出测试仍需CI。 |
| workspace JOIN没有校验 | 拒绝其概括。AuthorizeResource明确比较服务器Actor.WorkspaceID，Scope逐项调用。另有新batch路由尚待接入Scope，已分配同机后端角色，作为真实集成注意事项。 |
| 注册/登录缺登录会话CSRF | 拒绝。这两条在未登录时没有会话CSRF，使用严格Origin；受保护业务写入同时检查Origin与CSRF，与冻结契约一致。 |
| Repository接受role即角色伪造 | 拒绝。客户端未知字段拒绝，公开注册固定annotator；Bootstrap仅独立管理命令创建admin。内部受信调用不是客户端输入。 |
| class7/模型格式无验证 | 拒绝。精确文件SHA、签名检查、作者类别证据及head通道过滤分别存在；Go对服务输出再做独立验证。 |
| 推理锁允许无限并发/HTTP超时后释放 | 拒绝。threading.Lock非阻塞单槽；infer finally在底层执行结束才释放，shield+callback处理超时。30个通过测试含超时槽保持。 |
| 原图坐标/分块不验证 | 拒绝笼统判断。letterbox逆映射、clip和tile offset/global NMS已有实际坐标单元测试；全栈画布对应仍未验收。 |
| Python还应校验浏览器会话 | 拒绝。Python只接Go内部Bearer，公网无端口；业务会话属于Go，此意见违反模块边界。 |
| h.limit不存在，运行时method not found | 拒绝。limiter.go定义该方法且Go全包编译通过；首轮快照未给该文件，属于审查输入遗漏，后续补齐上下文。 |
| export body未限大小/参数 | 拒绝。Scope显式1MiB读取限制，业务Decode/selector继续验证；只读取鉴权选择器不替代业务验证。 |
| Cookie和会话没有同一事务 | 拒绝。数据库会话成功后才发送Cookie；Cookie不能加入SQL事务。失败留下不可知随机token的过期会话不是标签丢失竞态。 |
| DB错误与无权限混用403 | 记录限制。当前鉴权失败关闭且不泄漏存在性；可用性错误分类可改善，不因此关闭鉴权。需真实故障验收确认表现。 |
| non-existent403泄漏存在性 | 拒绝。不存在与其他工作区均同一403，恰是统一不可区分响应。 |
| token随机碰撞/数据库连接泄漏 | 拒绝无证据的错误结论；随机256位并有PG主键，QueryRow.Scan与有界context正常释放。真实资源观测尚未执行。 |
| 图片准备异常仍有推理内存泄漏 | 未证实；同函数infer finally释放锁，异常路径已有测试。继续保留独立故障/资源观测门槛。 |

尚未验收的跨模块候选决策、审计、统计、媒体SHA门禁、真实浏览器和正式迁移不会因上述意见被拒绝就标记通过。后续按完整包补审并逐项登记真实发现。
