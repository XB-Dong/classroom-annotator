# P0.1 数据库审核处置

审核对象：`4d61ff02c5c14803063ba38a3b64d1ef019a483c`。独立审核请求 `gpt-6-astra/xhigh`，actual 配置不可核验，记 null。审核在仓库外 overlay 测试及独立 PostgreSQL schema 中复现两项问题，未修改产品文件。

| 问题 | Leader 决定 | 修复验收 |
| --- | --- | --- |
| 已关闭、无 sidecar 的 WAL 模式备份用 mode=ro 打开后产生 -wal/-shm，主文件 SHA 不变仍不足以证明来源不可变 | 采纳，迁移前修复 | 安全构造 file URI，离线 immutable 读取；来源文件及目录不变，含 URI 特殊字符的路径可用 |
| 仅比较列名，接受 CREATE TABLE AS 保留列名但删除主键/非空/外键/CHECK 的未知结构 | 采纳，迁移前修复 | 核对已知 P0 结构，未知结构在目标写入前拒绝，合法既有备份仍通过 |

这是数据库任务正式修复第 1 轮，全波次已用 3/6 轮。此前普通合成备份逐字段导入、目标非空拒绝、事务故障回滚通过，不能覆盖上述边界。

默认 collation 可能影响 canonical ID 排序的意见未完成有效复现，暂记待核验，不计为确认缺陷。CAS、快照、发布锁的静态核对不能代替并发和恢复验收。

仓库外审核证据：`runtime/p01-db-review/review.md`、`review_import_test.go`、`overlay.json`、`observed-test-output.txt`。后者为工具返回转录，明确区分于原始重定向日志。
