# P0.1 GLM 素材服务审核处置

审核工具为 OpenCode1.19.0 / company-glm/glm-4.7-flash。两次实际调用分别读取9584502和a95d59b快照，退出码均为0；只授权源码读取，没有公司媒体、私密凭据或浏览器访问。本文件是Leader的核验意见，原始GLM输出未改写。请求模型已记录，无法独立验证的实际模型/推理参数仍为null。

第一轮GLM给出的整体PASS不采纳为验收结论：独立Astra及受控测试随后确认了上传取消、登记续租、完整控制响应超时、媒体空闲超时及ZIP传输计数五项缺陷，已在a95d59b实施修复。GLM实际读码是证据，不能替代业务测试或真实链路。

最终轮逐条处置：

| GLM意见 | Leader处置及依据 |
| --- | --- |
| Mac不可用仍可公司导入/上传 | 拒绝。source_routes的`mac && capabilities.DatasetConfigured`先要求Mac可用；remote_upload的nil/能力读取失败分支立即返回。没有本地fallback，公司Key未进入云端。 |
| 浏览器sourceId未校验白名单 | 拒绝。source_mac先校验格式，再用findMacSource读取实际Bridge清单并精确匹配；正式登记引用Agent稳定sourceId，后续发布采用登记结果的metadata。 |
| 确认cancelled仍必须REMOTE_CLEANUP_PENDING | 拒绝。冻结契约要求未确认清理才pending；确认cancelled可记INTERRUPTED。completed未ack输出仍保留并pending，没有把准备完成当清理完成。 |
| named return的defer未传播清理错误 | 拒绝该结论。defer通过named result赋值，errors.Join包含清理pending；当前实现确实等待取消结果。持久化失败仍不宣称已清理，原ledger保留供恢复。 |
| 固定operationColumns拼接有SQL注入 | 拒绝。operationColumns为源码固定列清单；外部cursor使用$1/$2参数，不存在用户控制SQL片段。 |
| VerifyCached与Lstat构成危险发布竞态 | 未证明。发布使用原子链接/摘要验证，存在不符文件会失败。外部任意删除受管不可变文件不是本波次承诺的事务保障；真实不可用返回错误。保留为文件异常测试方向，不为推测改架构。 |
| storageStatus是过度架构 | 拒绝。显式组装四个来源的可用状态是既定产品要求，没有增加编排框架或隐藏公司入口。 |
| 10秒/5秒/2GiB均为magic number | 2GiB是冻结导出限额，清理/续租短控制上限用于限制失败处理。可维护性意见记作后续参考；没有证据支持本轮增加配置框架。 |

五项实际缺陷的专项审核在读取独立登记3/3重跑证据后通过；首次独立JOB_FAILED及短时序断言失败仍保留，没有编造环境原因。bf78eb5仅调整测试调度余量，业务代码保持a95d59b。完整云端、Mac媒体、浏览器、CI及GLM实际体验分别验收，不能从这份代码审核推出波次完成。

原始证据：runtime/glm/evidence/p01-sources-r1-attempt1、runtime/glm/evidence/p01-sources-repair1-final-attempt1；独立结论：runtime/p01-sources-review/review-final-a95d59b.md。

最后一轮107593b已实际读取22个受限文件（含审核说明），exit0，用时约94秒；GLM未提出阻塞，但其整体PASS仍只作为静态意见。关于将公司凭证检查参数化、提前校验完整文件摘要、按归档大小调整租约的建议不采纳：前者是固定隔离要求，摘要已有流式计算且必须在字节接收完成后对比，续租依据租约而非文件大小。buffer优化没有可测瓶颈，留作参考。独立Astra复核真实EOF/限流/取消与HTTP请求生命周期后通过。实际正向HTTP上传三次通过；最终原生FFmpeg API、四项Chrome、公共离线及文件观察也通过，证据分别记录。云端切换、真实Mac媒体流和GLM实际云端体验仍独立验收。
