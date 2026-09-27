# v0.1 开发计划

**状态：** done

| 阶段 | 内容 | 状态 | 完成条件 |
| --- | --- | --- | --- |
| 0 | 规格与领域基线 | done | spec、词汇表和变更流程一致 |
| 1 | CLI/Vault/Git 骨架 | done | init/status/version 测试通过 |
| 2 | 教材导入与进度 | done | import/list/show/activate/position 测试通过 |
| 3 | Session 与状态 | done | start/append/end 闭环测试通过 |
| 4 | Agent Rules/Skills | done | 两类 Agent 入口生成且不覆盖 |
| 5 | 集成、发布与安装 | done | 全套检查和交叉构建通过 |

## 实现顺序

1. 建立 Cobra root command 和可注入的 stdout/stderr、工作目录。
2. 实现 Vault discovery、幂等初始化、内嵌资产和原子写入。
3. 实现 Git 边界；所有 Git 错误可解释且不损坏文件。
4. 实现 Source manifest 与 Curriculum State，再接 CLI。
5. 实现 append-only Conversation 和 Session pipeline。
6. 使用临时 Vault 完成端到端测试。
7. 补齐 README、Makefile、版本号与交叉构建。

## v0.1 风险

- PDF 解析容易扩大依赖：本版只管理原件和元数据。
- Agent 可能漏记回合：Skill 明确每轮追加；自动 Hook 延后。
- Git 环境可能缺少 identity：初始化不要求提交，Session 提交失败只报告。
- 用户已有规则文件：初始化永远保留，不做合并覆盖。

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
