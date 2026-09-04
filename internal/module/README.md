# 业务模块（module）

**领域层 + 持久化**：实体、枚举、领域工具、`Repository` 接口、`repo` 实现（`ClientFrom`）。  
**不含** 应用 Service、**不含** HTTP、**不含** `*Request`。

`article` 额外保留 `agent/`、`prompt/`、SSE 事件常量与 payload（编排与协议细节），应用层只负责用例与推送。
