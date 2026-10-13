# P1账号、授权与并发规格

版本：0.1.0｜待审核设计；执行R0同步唯一OpenAPI后冻结。

## 1. 身份与权限

一个共享工作区，成员可以查看/编辑同一帧。邀请码由配置提供，用户输入正确码注册；不做组织管理。管理员由显式bootstrap创建，其他账号默认为annotator；不信任请求体里的userId/role。

用户名规范化为小写ASCII，3–32字符，字母/数字/下划线/短横线，数据库唯一；密码12–128 UTF-8字节，不自动截断；Argon2id采用19MiB、t=2、p=1、每用户随机盐，有限2个哈希并发槽。邀请码校验、注册、登录有IP/用户名限流，固定安全错误，不输出摘要/凭证。

会话使用随机256bit不透明token，只存摘要于PG，12小时绝对到期，退出立即撤销；Cookie HttpOnly、SameSite=Lax、HTTPS Secure。变更请求要求匹配可信Origin并提供会话绑定CSRF token（GET /auth/me获取，前端仅存内存）；注册/登录也校验可信Origin。应用不自动信任任意X-Forwarded-*，Nginx代理地址必须配置。

需要鉴权的不只有标注API：视频列表、任务、公司API、AI、导出、原视频、缩略图、JPEG、ZIP全部验证会话/工作区。静态前端、注册登录与无敏感字段liveness可公开；metrics/管理接口仅内部。退出/换账号清空内存与缓存查询状态，媒体no-store，旧标签页不能继续读取私人素材。

退出/换账号递增authGeneration并中止旧请求，迟到HTTP/图片/轮询回调携带捕获generation，不能把旧用户结果写入新账号缓存或画布；仅abort并不够。测试覆盖登出后旧响应到达、新账号登录以及老媒体Range/HEAD/ZIP直链401。

单工作区仍用workspaces/workspace_members显式表示。视频有workspace_id，frame/job等沿关联推导权限；旧视频在迁移后分配bootstrap工作区，不让任意注册者成为管理员。最终demo只向持邀请码的授权使用者共享。

## 2. 帧级并发规则

复用现有BIGINT revision，PG transaction + `UPDATE frames ... WHERE id=$id AND revision=$expected RETURNING revision`，成功后在同一事务替换框/审核决策/修改者；零行结果409。先验证权限再读数据。所有状态写入，包括完成、撤销完成、接受/拒绝AI和风险解决，都共享帧revision。

不使用应用全局锁、不全局串行任务；同帧短事务自然串行，不同帧并行。图像推理/文件I/O不放事务内。同一请求处理多帧不形成长事务，排序为固定frameId避免互锁。

API冲突返回 `REVISION_CONFLICT`、currentRevision和latestUpdatedBy的安全显示名。前端再GET最新快照，不把陈旧409载荷当最新全量数据。权限失败401/403，不能用409伪装未授权。

两个设备同时从r写入：恰好一个成功r+1，另一个409；后者不删框、不改变状态/审核/审计。重复同一revision重试同样409，不把另一次成功据为己有。不同帧同时写入均成功。

前端每帧串行发送保存，跨帧请求带scope校验；草稿key新增deploymentOrigin/workspaceId/userId/frameId，包含baseRevision；不自动把旧未分用户的草稿应用给新账号。退出后不恢复他人草稿，登录同账号才可恢复。

409停止自动保存，保存本账号原草稿：

1. “查看最新并保留草稿”：读取服务端快照，只读保留本地差异，用户在最新版本上重新编辑。
2. “放弃草稿”：明确操作后使用服务端数据。

不提供直接改expectedRevision重新提交整份旧快照的快捷键，不自动合并删框与复核决策。停留帧每5秒轻量查revision，只有本地无未保存修改才自动刷新；忙碌时只提示新版本，隐藏页面停止轮询。

## 3. AI与导出竞态

开始AI保存baseRevision、imageSHA、modelSHA和参数快照，依赖frame未完成。用户可以在推理期间保存；候选发布事务先CAS到baseRevision+1。失败则run标stale，原AI结果只读留审计，不进入当前候选/正式框，不能覆盖手工数据或标图片完成。

同帧最多一个有效运行，PG唯一约束+Idempotency-Key防双击；指纹含baseRevision、image/model哈希、参数与风险版本；同key不同指纹409，相同指纹返回原任务，主动重跑使用新key。job取消与发布按job→frame锁顺序，终态与候选同步；只有持PG scheduler单例锁的实例恢复自己上次中断的未终态推理，记failed/INTERRUPTED，不扫仍有效owner，不伪装恢复旧子进程。

导出在一个只读 REPEATABLE READ 中读取状态、revision、正式框与风险解决状态，事务结束后打包不变原图；结果来自某个一致快照，不混合两次保存。导出owner必须属于工作区，禁止未审核标签。

## 4. 数据与接口

新增迁移（不得改已执行00001–00004）：users、sessions、workspaces、workspace_members；videos.workspace_id；frames.updated_by/updated_at；annotation_events追踪操作者/action/revision，不保存密码或原始HTTP载荷。

| 接口 | 语义 |
| --- | --- |
| POST /api/v1/auth/register | username/password/inviteCode；201，不返回密码 |
| POST /api/v1/auth/login | username/password；200，设置会话Cookie，生成CSRF绑定 |
| GET /api/v1/auth/me | 当前安全用户/workspace/CSRF；未登录401 |
| POST /api/v1/auth/logout | 204，撤销会话/清Cookie |
| GET/PUT /api/v1/frames/{id}/annotations | 延续expectedRevision，返回统一快照和安全修改者 |

固定响应错误码和新增字段由Leader写入 `specs/contracts/openapi.yaml`，前端/Python不各自定义另一份类型来源。首次freeze记录版本和SHA。

来源：[PG事务隔离](https://www.postgresql.org/docs/17/transaction-iso.html)、[OWASP密码摘要建议](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)。具体共享工作区、CAS和会话参数是本项目设计。
