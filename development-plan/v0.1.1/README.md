# v0.1.1 开发计划

**状态：** done

| 阶段 | 内容 | 状态 | 完成条件 |
| --- | --- | --- | --- |
| 0 | CR-002 与版本基线 | done | v0.1.1 范围冻结 |
| 1 | Session kind 与 baseline gate | done | 默认选择、skip、position gate 测试通过 |
| 2 | Assessment schema 与证据校验 | done | end/持久化/status 测试通过 |
| 3 | Guided Socratic Agent Skill | done | baseline/lesson 策略可发现且有完成条件 |
| 4 | Agent 资产安全升级 | done | dry-run、备份、更新测试通过 |
| 5 | 迁移、安装与真实 Vault 验证 | done | v0.1 兼容、全套发布检查通过 |
| 6 | 学习期间静默记录 | done | 普通回合不播报 Runtime 操作，异常和确认仍可见 |

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
