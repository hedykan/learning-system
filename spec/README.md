# Spec 管理

`spec/` 是产品需求的唯一入口。代码、计划和 Agent 指令不得成为未记录需求的来源。

## 阅读顺序

1. [产品目标](./product.md)
2. [Learner Model 与知识沉淀](./learner-model.md)
3. [Interpretation Record 与 Learning Policy](./interpretation-record.md)
4. [领域与数据边界](./domain-and-data.md)
5. [CLI 契约](./cli.md)
6. [AI Agent 集成](./agent-integration.md)
7. [质量属性](./quality.md)
8. [核心验证场景](./validation-scenarios.md)：每个版本发布前必须用真实 Agent 复现的 17 个场景
9. [v0.1.8 基线](./versions/v0.1.8.md)（下一版本：资料接口）；[v0.1.7 基线](./versions/v0.1.7.md)（当前版本）；[v0.1.6 基线](./versions/v0.1.6.md)；[v0.1.5 基线](./versions/v0.1.5.md)；[v0.1.4 基线](./versions/v0.1.4.md)；[v0.1.3 基线](./versions/v0.1.3.md)；[v0.1.2 基线](./versions/v0.1.2.md)；[v0.1.1 基线](./versions/v0.1.1.md)

## 需求状态

- `idea`：只进入 [想法池](./ideas.md)，不能直接开发。
- `proposed`：已形成变更单，等待评审。
- `accepted`：已进入某个版本基线。
- `implemented`：代码和测试已满足验收条件。
- `deferred`：排入后续版本。
- `rejected`：保留决策理由，不进入开发。

## 变更规则

v0.1 基线冻结后，新增或改变行为必须先按 [需求变更流程](./changes/README.md) 建立变更单。只有修复与既有验收条件明显不符的缺陷可以直接进入当前版本；扩大能力范围的内容默认进入下一版本。
