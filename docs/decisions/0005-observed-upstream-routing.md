# 决策 0005：补齐已观察上游路由与数据集配置

日期：2026-10-13｜状态：P0 执行中材料核对，业务 API 不变

R0 规格描述了课节列表的响应结构，遗漏上游列表路径中的 datasets/{datasetId}。后端实现查询时发现缺口，Leader 根据既有交接包和原题接口记录核对补齐；不凭空采用 /api/v1/lessons。

上游列表为 GET /api/v1/datasets/{datasetId}/lessons，查询 page 与 page_size，认证 X-API-Key。播放地址为 GET /api/v1/catalog-files/{catalogFileId}/play-url。资料中的数据集 ID 为 13，运行值通过 APP_DATASET_ID 配置，不硬编码。APP_DATASET_API_URL 配置上游 origin；API URL/数据集未配置时返回 DATASET_NOT_CONFIGURED。

业务侧 GET /api/v1/dataset/lessons 的 page/pageSize 和响应结构保持不变；新增部署配置字段已通知 Backend 和 Test。前端不读取上游凭证。测试 fixture 必须使用真实观察到的路径形状和虚构值；未执行真实公司环境联调。

API 契约附录版本更新为 1.0.1；其余冻结规格 1.0.0 的行为不变。此为观察材料遗漏的补充，不扩展 P0 范围。原始材料、实际公司 API Key 不进入仓库。
