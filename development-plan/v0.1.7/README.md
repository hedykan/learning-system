# v0.1.7 开发计划

**状态：** done  
**基线：** [v0.1.7](../../spec/versions/v0.1.7.md)

| 阶段 | 内容 | 状态 | 完成条件 |
| --- | --- | --- | --- |
| 0 | 变更单与基线冻结 | done | CR-023、CR-024 accepted |
| 1 | CR-023 固定文字对照表与语言设置 | done | 英文 Vault 页面无中文固定文字；缺省输出与 v0.1.6 一致 |
| 2 | CR-023 切换语言迁移页面 | done | 往返切换手写区保留、无残留旧页面 |
| 3 | CR-024 自动提交结果与首页提醒 | done | 失败出现提醒、`learn commit` 后消失、Git 未启用不显示、成功时工作区干净 |
| 4 | Skill、CLI 契约、CHANGELOG | done | 文档同步 |
| 5 | 真实 Agent 复验 | done | Codex 沙箱内首页出现提醒；英文学习者自动切换为 en |

## 真实 Agent 复验（2026-09-28，Codex gpt-5.6-sol 低思考，3 轮）

- 学习者用英文开口，Agent 第一轮即运行 `learn config set language en`；首页、学习者总览、教材首页、进度页的固定文字全部为英文，页面名为 `Learner overview.md`、`Progress.md`。
- 沙箱内自动提交失败，`status` 报告 `git_auto_commit: failed`；首页顶部出现英文提醒。在沙箱外运行 `learn commit` 后提醒消失、工作区干净，只产生一个提交。
- 结束学习时，Agent 在总结后补一句“学习历史尚未保存到 Git”，并给出 `learn commit` 用法；课程中间没有提及 Git。
- 注意：测试机的 Codex 全局指令要求用中文回复，所以 Agent 的对话与概念名是中文；这与界面语言无关，真实英文学习者不会遇到。
