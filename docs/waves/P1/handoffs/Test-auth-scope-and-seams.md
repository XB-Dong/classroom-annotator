# Test：工作区、退役身份与内部接口核对

时间：2026-10-14T02:49:12+08:00；rootTaskId=P1-TEST-001。前置4841808/da22122；只改Test拥有的测试与交接，不改业务模块/契约/部署。请求gpt-6.1-sol/high，actual仍unknown。

## 新增独立检查

- 注册密码加入43个中文字符（129 UTF-8字节）拒绝，规范化大小写重复账号拒绝、真实注册频率超过上限后429/Retry-After；不通过改掉实际限流器使测试方便。
- 将自己的合成video临时归属另一个QA workspace，检查视频/任务列表不会露出它，video/frame/job/review/media/HEAD/Range/batch-AI/ZIP全部403；finally恢复真实归属并移除QA workspace。
- 创建历史mac provider的隔离元数据，故意将storage_key指向实际存在的合成local文件。JPEG/ZIP/抽帧必须410 REMOTE_STORAGE_RETIRED；原始local JPEG仍200且SHA一致。保留真实FK与管理卷，未把退休身份映射到local，finally只删本测试元数据。
- 真实模型单独检查与容器检查都明确验证内部`contractVersion=1.0.0`及精确Nano modelId、SHA、输出尺寸。内部1.0.0为Leader本轮修复后口径，不修改OpenAPI。
- 两浏览器冲突测试通过真实HTTP409对应的浏览器确定败者，避免依赖React何时刷新文字而制造测试误判。

这些真实PG/API/browser/model/container新检查均**not_run**。Windows仅互斥内JavaScript语法与测试收集退出0：33项/14文件；日志为外部runtime/p1-test/scope-final-{check,list}.log。Python只AST语法成功，未运行ONNX；diff检查退出0。收集33项不是33项通过。

## 独立质量观察与CI事实

| 观察 | Leader处置 / 证据 |
| --- | --- |
| logout清本地身份后，客户端把真正网络失败也变成STALE_AUTH_RESPONSE并被忽略 | Leader接受；FrontendB独立发现同问题并修复；独立浏览器abort/logout检查已加入，当前not_run |
| Go client验证1.0.0，而已运行的Python真实Detector当时返回1.0 | Leader接受，统一1.0.0并增加真模型检查；Test没有悄悄放宽版本比较 |
| DB断连时auth/Scope原实现分别把基础设施失败映射401/403 | Leader独立确认并准备503修复；原postgre-check的503/5xx/快照不变断言保留 |
| 历史mac冻结410与Producer当时503不一致 | Leader准备统一410；Test保持410独立fixture |

正式修复预算由Leader记录：前端退出、Go帮助格式和协调R1接口修复轮次不能因切片/角色改名重置。Test新增检查不是额外的产品修复声明。

首次真实Linux CI：run38446796562，SHA e683ecbbab26a5ad93baae8a86d4c85cbb90190b。Leader原始日志`runtime/p1-preparation/ci-38446796562/11_Common verification.txt`显示前端14文件/58项通过、build通过，然后因`internal/help/config.go`出现在gofmt列表以exit1停止。Python28项及真实Nano合成图已由Leader从同次日志确认；不能据此称PG/race/完整API/browser通过。第一轮依赖/格式失败不是业务TDD RED，未生成verified发布或离线镜像工件。

本轮仍待完整Linux CI与新版AI/review接线。独立candidate接受/调整/删除/统计、AI与人工stale/取消等扩充fixture须在迁移/仓储交接后继续；产品真实GLM问答、独立OpenCode体验、生产迁移和云端30分钟资源门槛仍由Leader统筹，不以合成测试代替。
