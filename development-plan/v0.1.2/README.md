# v0.1.2 开发计划

**状态：** done（阶段 8 的真实 Vault 回填留给下一次真实学习）  
**基线：** [v0.1.2](../../spec/versions/v0.1.2.md)

| 阶段 | 内容 | 状态 | 完成条件 |
| --- | --- | --- | --- |
| 0 | Schema 与 CLI 冻结 | done | 规范、CR-004、版本基线评审通过 |
| 1 | Turn ID 与 Conversation 解析器 | done | 新旧格式解析、`append --json`、`session turns` 测试通过；assessment 复用新解析器 |
| 2 | Interpretation Record 校验 | done | 证据、Concept 去重、状态门槛、策略与 Pattern 规则的正反用例全部通过 |
| 3 | 记录持久化与模型回放 | done | checkpoint、end、annotate 幂等；回放确定性；retraction 回退正确 |
| 4 | Detour 命令 | done | start、end、日志、baseline gate 测试通过 |
| 5 | Learning Policy | done | R0 至 R6 与策略选择每条规则都有测试；`state`、`state concept` 输出稳定 |
| 6 | Markdown 投影与重建 | done | 四类投影、用户手写区保护、`model rebuild` 逐字一致、`projections` 状态 |
| 7 | 回放验收夹具 | done | 12 Session 双教材夹具，对照组与实验组断言通过 |
| 8 | Agent Skill 升级与真实 Vault 验证 | in-progress | Skill 覆盖记录提交与 `learn next`；在 DDIA Vault 回填旧对话并完成一次真实 Session |

## 实现约束

- 新增包：`internal/conversation`（turn 解析）、`internal/record`（Schema 与存储）、`internal/mdblock`（用户手写区保护）、`internal/learner`（回放与模型）、`internal/policy`（`learn next`）、`internal/projection`（Markdown 投影）、`internal/replay`（验收夹具）。
- `model.Provider` 与 `ConservativeProvider` 保留，只用于 `--no-analysis` 结束时生成兼容 Session 摘要。
- 所有写命令沿用现有锁与原子写入工具；模型派生必须是纯函数，便于回放测试。

## 发布检查

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
make build-all
go install ./cmd/learn
learn version
```
