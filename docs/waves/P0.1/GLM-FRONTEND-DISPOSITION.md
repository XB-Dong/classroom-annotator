# P0.1 GLM 前端审核处置

OpenCode 1.19.0 使用请求模型 `company-glm/glm-4.7-flash`，于北京时间 16:26–16:28 完成提交 `ab66dead4dcb39477312b5ca15493332d156fc28` 的只读前端审核，exit 0，未超时。实际模型响应遥测未独立核实，actual 记 null。原始调用与响应在仓库外 `runtime/glm/evidence/p01-frontend-r1-attempt1/`。

Leader 逐条核对意见，不直接采用其“SPEC PASS / QUALITY FAIL”总评：

| 意见 | 处置与核对依据 |
| --- | --- |
| Canvas 缺少 error boundary | 作为后续体验改善建议；未证明此变更引入崩溃，不列为本波次阻塞 |
| ResizeObserver 未清理 | 拒绝事实断言：effect cleanup 已执行 disconnect；未提供断开后回调造成实际错误的复现 |
| pointer 可能 undefined | 拒绝：start/move 已检查 p，无坐标则不绘制 |
| img.onerror 无 active 检查 | 拒绝：代码明确为 `if (active) setImageError(true)` |
| frame 尺寸无校验 | 当前帧来自已核验媒体/帧 DTO；恶意损坏 DTO 的额外前端防护记后续，不凭读取推定实际失效 |
| normalizeBox 未验证正尺寸 | 拒绝：调用其规范化结果，仅 box 存在才添加；审核未读取几何实现，无法据此下结论 |
| 无 PropTypes/TypeScript strict | 拒绝：已有 TypeScript props，配置 strict；无需重复引入 PropTypes |
| 视觉常数需抽取、异常变量命名不统一 | 不采纳：无业务缺陷依据，避免与任务无关整理 |
| 公司加载无 loading 状态 | 拒绝：busy 驱动界面加载/禁用，未要求把内部状态传父组件 |
| legacy 隐藏实现不存在 | 拒绝：实现位于导入组件之外、App 使用的 VideoLibrary 过滤与“显示历史素材”开关；本次受限文件选择不能证明全仓库缺失 |

图像身份切换是否残留旧图的总评依据不足：保存 settle 只证明草稿切换守卫，不能证明 Canvas 绘图身份。本问题由 Leader 对 Canvas 图片身份及浏览器流程另行核对，不使用 GLM 总评代替验收。

本记录证明跨模型代码审核确实执行，并保留意见处置。它不能代替 GLM 实际浏览器体验、真实 Mac 联调、云端部署或数据库验收；这些仍有各自独立门槛。
