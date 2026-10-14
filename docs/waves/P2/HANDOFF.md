# P2 阶段交接

当前P2为partial，P1仍partial_closed_fast_path；不自动进入下一波次。先读AGENTS、CONSTITUTION、P2计划、state.json及[最新报告](../../audit/reports/report-20261014T230000+0800.md)。不要让历史待办覆盖新实测，也不要让新实测抹掉历史失败。

## 当前来源与运行

- 产品源码bf18d30f8853e75cdac6abea160dde54c89dd18b，真实GitHub分支codex/p2-adaptive-quality，草稿PR #1；main基线仍033de6c。PR合并测试树a39f599与bf18产品树相同，部署使用bf18独立归档构建，不能混称直接部署CI工件。
- Mac入口http://127.0.0.1:8180/，7服务healthy；云端https://114.67.222.150/仅公开QA。原P0镜像/挂载/HTTP80 Basic门禁不变，原QA79文件哈希一致，schema8。
- bf18双端实际浏览器人工闭环、最新102单测及30尺寸用例通过。9e阶段真实DeepSeek200/ONNX任务completed且0候选按原来源保留，Go/AI源码与bf18等价；不是课堂正例或准确率评测。
- 完整PR CI38588562501结果以state/报告及GitHub实际记录为准。两次旧失败与普通页面滚动复现的Red均保留，修复仅重新测量测试画布坐标，未弱化宽度断言。main push/PR保留完整CI，feature push不再与PR重复触发。

## 规则与保留物

公司数据集仅Mac内网直接访问，列表及取址/Go课节HTTP200、未下载媒体。按[本机规则](../../deployment/MAC-COMPANY-DATASET.zh.md)使用catalog_file_id，不能与id/sourceId混淆；公司凭证、签名URL与原响应在外部私有目录，不配置云端、不发审核模型。容器公司DNS映射随网络变化重新核验，不关闭TLS。

答辩PPT/讲稿/问答/源文件仅Mac仓库外留存，不再提交GitHub。用户入口在工作区父目录“交付物/P2”；这里不复制材料。P2分支历史已清理答辩路径，旧本地完整历史bundle保留在外部私有目录，不能push --all。

运行配置、旧镜像、数据库备份、媒体manifest和原始日志在`/Users/kdl/.local/share/classroom-annotator/`，源码树无运行依赖。Mac启动见[本机runbook](../../deployment/MAC-LOCAL.zh.md)。临时Vite5192与独立测试PG已停止，数据保留；产品服务和历史launchd/旧素材不作清理。

## 尚缺门槛与下一步

独立GLM代码审核及真实合成/公开浏览器体验缺少授权环境，未运行，不能拿DeepSeek产品Key替代或静默换模型。课堂真实候选UI正例、量化检测/节时效果、生产迁移及匹配恢复/30分钟生产监控仍未完成。智能抽帧/相邻传播只有设计，无产品实现。

当前波次在阶段交付处停止。后续具体任务由用户指示；不迁移生产、不恢复隧道、不自动启动下一波次。先读取已有精确证据，再决定有新代码/新失败时的针对性验证，不反复推送已知失败，也不为审计文字反复跑无变化的完整重套件。
