# P1前端切片A交接：账号、草稿隔离与工作台外壳

状态：切片代码与模块检查完成；真实接口联调、浏览器视觉验收、GLM独立体验尚未执行。

- rootTaskId：P1-FRONT-001；关联P1-UI-001、P1-AUTH-001、P1-CAS-001、P1-HELP-001。
- 基线：822eb26160958037b09542dd26623b833c50db45。
- 产品提交：b345658178d77b0c94f5c268ba040be058307829，分支p1/frontend。
- 本切片首个记录时钟：2026-10-14T01:38:09+08:00；派发时间以Leader原始审计为准。检查完成并提交：2026-10-14约02:00+08:00。
- 请求配置：gpt-6.1-sol/high，独立上下文；实际provider/model/reasoningEffort均未核验，记null。
- 路径所有权：frontend/**；本交接文件。shared/api/types.ts、client.ts已在实现前得到Leader明确协调授权。
- 修复政策：root当前正式repairCount=0。Red→Green属于行为实施；继承旧单人冲突实现按批准P1-CAS行为接续，不重置P0.1的6/6历史记录。

## 本切片行为

注册发送username/password/inviteCode，201后明确提示用户登录，不自动授予会话。登录与me使用冻结的user/csrfToken响应，Cookie由服务端管理；客户端只在内存保存身份和CSRF，所有请求credentials=same-origin，写入加X-CSRF-Token，204不解析JSON。

退出/会话失效递增authGeneration并中止旧业务请求；迟到HTTP/ZIP/body解析和原图回调检查捕获generation。账号/工作区变化重建Workspace，清空视频、批次、帧、轮询、画布等旧组件状态。退出请求自身不随本地清空而中止，以便完成服务端撤销。

草稿v2 key按deploymentOrigin/workspaceId/userId/frameId隔离，不读旧v1未分账号草稿。409暂停自动保存。“查看最新并保留草稿”只读最新快照，保留旧草稿及原始baseRevision，不更改revision重发旧快照；“丢弃草稿”明确恢复服务器。保留草稿仍阻止批次导出，需要在新版本重新编辑。

顶部使用指南默认收起，可匿名读取GET/help/document完整手册。登录后POST/help/questions，回答只按文本渲染，引用仅能导航已知合法章节；API失败保留手册。账号变化丢弃迟到回答。桌面侧栏占布局空间，不覆盖画布；窄屏移到工作台上方。测试回答是合成fixture，不是实际GLM调用证据。

新增纸白/暖灰、深墨、蓝与赭黄主题token和小型Notebook SVG。装饰限定UI外壳，画布背景无纹理，框保持精确直线。活动来源入口移除MacSourcePanel，显示尚未接入的本机目录说明；旧组件仍留作历史单测，未伪装可用本机目录。历史mac记录保持“历史远端（不可用）”，local显示“当前部署主机”。

## Red/Green原始证据

外部目录：D:/Project/cvte/runtime/p1-frontend/。以下均为实际输出，不提交原始日志。

| 文件 | 命令/事实 | 退出码 |
| --- | --- | --- |
| slice-a-red.log | 窄测：草稿key碰撞、v1错误恢复、旧快照被重发；新增auth模块当时缺失单独记为missing-feature，不当作行为断言成功 | 1 |
| auth-behavior-red.log | 暂存当前client、运行原client后finally恢复：旧账号结果被接受、401后身份未清空 | 1 |
| csrf-behavior-red.log | 同一受控原client检查：credentials缺失；当前实现恢复 | 1 |
| second-red-first-green.log | 原图迟到回调未隔离、退出请求被中止、活动Mac入口仍存在；auth表单辅助文字造成标签不可精确定位；Help模块当时缺失 | 1 |
| draft-base-red.log | retainedDraft原baseRevision=3被原构造逻辑改成0 | 1 |
| typecheck-a.log | npm run typecheck | 0 |
| lint-a.log | npm run lint；修正两个测试未使用参数后重新执行的最终输出 | 0 |
| green-a.log | npm test -- --run --maxWorkers=1 --no-file-parallelism：14文件、58测试通过 | 0 |
| build-a.log | npm run build：110 modules，生产资源生成成功 | 0 |
| typecheck-formatted-a.log | 最后纯格式整理后npm run typecheck | 0 |

npm ci仅安装现有锁定依赖，192 packages，audit未报漏洞。第一次mutex等待超时，第二次等待在Leader调度通知后取消，均没有启动npm，均不记产品Red或repair。第三次实际安装成功。最后首次Prettier命令路径错误而无文件变动，随后使用仓库根路径选择实际变更文件成功整理；未掩盖这次工具错误。

所有重检查使用Local\\CVTE_P1_BUILD，最多120秒等待，finally释放；没有启动Vite、Docker或常驻服务。02:00前只读进程检查未发现commandline含p1-frontend的node进程；正常用户进程未操作。

## 未完成与下一依赖

1. Leader冻结Review DTO后接当前帧/最多20帧AI、具体候选定位、风险过滤、接受/调整/拒绝、图片风险与统计。当前前端未宣称真实AI已就绪。
2. 按冻结Backend实际接口做真实注册/登录/退出、会话Cookie/CSRF、401/403、PG多人CAS联调；单元fixture不替代真实账户门槛。
3. 增加停留帧5秒revision检查（隐藏页面停止）和安全刷新；目前保留原任务轮询，尚未完成P1 revision提示。
4. 在Leader授权临时实际栈取得1440×900、1280×720、窄屏截图及功能E2E，核对三栏可操作面积、状态可见与指南折叠；此切片只有样式源码，不能称视觉验收通过。
5. 真实产品GLM问答/引用检查、公开数据浏览器体验与GLM代码审核，由Leader组织；均为not_run。
6. 本机目录入口待Go本机source合同，不能把历史远端sourceId映射为本机路径。没有修改Go、Python、Compose、CI或依赖清单。

请Leader对b345658进行代码/契约审阅后决定集成与后续切片；本Worker不集成、不部署。
