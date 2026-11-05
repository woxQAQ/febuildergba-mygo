# 历史记录

此目录保留已完成的一次性审查及被替代的历史方案，不作为当前开发规则、待办或功能状态。当前文档入口见[文档索引](../README.md)。归档后不随产品变更更新，旧行号和测试结果只说明当时的观察。报告的截图与历史源码链接指向对应实现的固定提交；截图已退出当前工作树，不作为当前界面的验证证据。

| 记录                                                       | 基线／对应实现提交           | 当前替代文档                                                                              |
| ---------------------------------------------------------- | ---------------------------- | ----------------------------------------------------------------------------------------- |
| [人物与职业审查](record-editor-deslop-review.md)           | `a164bd7`                    | [人物与职业设计](../record-editor-design.md)                                              |
| [头像审查](portrait-editor-deslop-review.md)               | `a164bd7`                    | [头像设计](../portrait-editor-design.md)                                                  |
| [测试实施记录](test-implementation.md)                     | `a77fea5`                    | [测试说明](../../tests/README.md)                                                         |
| [临时验证历史发掘](temporary-verification-history.md)      | `88402d0` 加核对时工作区修改 | [测试说明](../../tests/README.md)、[AGENTS](../../AGENTS.md#命令与验证)                   |
| [MyGo v0.6.3 → v0.7.3 评估](mygo-0.6.3-to-0.7.3-review.md) | `44cdc8f` 加调研时工作区修改 | `go.mod`、[架构](../architecture.md)、[开发说明](../development.md)                       |
| [MyGo v0.7.3 → v0.7.4 评估](mygo-0.7.3-to-0.7.4-review.md) | `32de81f` 加调研时工作区修改 | `go.mod`、[架构](../architecture.md)、[开发说明](../development.md)                       |
| [应用日志与诊断记录设计](logging-design.md)                | 设计基线 `0f82791`           | [错误设计](../error-design.md)、[架构](../architecture.md)、[开发说明](../development.md) |
| [文档 harness 巡检](document-harness-audit.md)             | `90ab062`                    | [AGENTS](../../AGENTS.md)、[文档索引](../README.md)、[测试说明](../../tests/README.md)    |

[Format 与编辑流程原始提案（Opus）](format-editing-opus-proposal.md)和[另一版提案（Codex）](format-editing-codex-proposal.md)保留不同设计方向的原始讨论。写作提交未记录，归档核对基线为 `640f0fb`；当前实现与规则由[架构](../architecture.md)和 [AGENTS](../../AGENTS.md)替代，状态由[覆盖矩阵](../coverage.md)替代。

已退出维护的实现汇总，其代码导航归入[架构](../architecture.md)，格式与写入语义归入[专题参考](../README.md#格式与行为参考)，状态归入[覆盖矩阵](../coverage.md)，不再保留另一份状态副本。此前主动删除的验收报告不恢复。
