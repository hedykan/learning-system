# AI Agent 集成

## 目标体验

用户在 Vault 中启动 Codex 或 Claude，然后说“继续学习”。Agent 应读取适配入口，调用 `learn status --json`，恢复教材和学习者上下文，启动 Session，教学并记录对话，最后结束 Session。

## 生成文件

`learn init` 从二进制内嵌资源生成：

- `AGENTS.md`：Codex 等 Agent 的轻量入口。
- `CLAUDE.md`：Claude 的轻量入口。
- `.agents/skills/learning-os/SKILL.md`：通用模型可发现 Skill。
- `.claude/skills/learning-os/SKILL.md`：Claude Skill 入口。
- `references/`：按教材导入、会话记录和安全边界拆分的细节。

核心行为只在通用 Skill 中定义；适配入口只负责触发和定位，减少规则漂移。初始化发现同名文件时保留用户内容，并报告 `preserved`。

## Agent 工作流

1. 在任何学习操作前执行 `learn status --json`。
2. 用户提供教材路径时，先执行 `curriculum import --dry-run --json`。
3. 根据用户意图执行正式导入；不猜测不可访问路径。
4. 教学前执行 `session start`。
5. 每个用户和助手学习回合通过 stdin 调用 `session append`。
6. 正常完成时调用 `session end`。
7. 中断后发现 active session 时，优先询问继续还是结束。

以上 CLI 操作属于后台 Runtime 行为。正常教学回复直接承接学习内容；仅在命令失败、需要用户选择、涉及覆盖/删除确认，或用户主动询问记录状态时说明内部操作。Session 结束时报告学习总结和下一步，不默认暴露命令、文件路径或产品名。

## 边界

CLI 无法自动读取宿主 Agent 的私有聊天历史。v0.1 依赖 Skill 让 Agent 显式追加对话；未来可以增加宿主 Hook，但不得绕过用户权限或秘密采集聊天。

Agent Rules 和 Skill 不保存绝对路径、凭据、主机名或其他机器敏感信息。Agent 只通过 `learn` 修改 Runtime 管理的状态，用户 Markdown 的普通编辑仍由用户控制。
