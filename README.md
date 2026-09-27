# Personal Learning OS

用 Codex 或 Claude 跟着一本教材学习，系统记住你怎么理解、哪里错过、学到哪里，并安排复习。学习痕迹是本地的 Markdown，可用 Obsidian 打开。

> **给 AI 一个网址就能装好。** 把下面这行发给 Codex 或 Claude：
>
> 读 `https://raw.githubusercontent.com/hedykan/learning-system/main/README.md` 并按其中「给 AI 代理」一节安装，然后带我开始学习。
>
> 这需要仓库是公开的。私有仓库请用下面的「手动安装」。

## 这是什么

`learn` 是一个本地优先的命令行 Runtime。它管理教材目录、学习位置、原始对话和复习计划，并把你的理解沉淀成可追溯、可重建的 Markdown 笔记。教学由 Codex 或 Claude 完成，`learn` 只做固定的存储、校验和规则。

## 手动安装

需要 Go 1.23+ 和 Git，支持 macOS、Linux、Windows。

```bash
go install github.com/hedykan/learning-system/cmd/learn@latest
learn version
```

私有仓库先配一次拉取方式：

```bash
go env -w GOPRIVATE=github.com/hedykan
git config --global url."git@github.com:hedykan/".insteadOf "https://github.com/hedykan/"
```

找不到 `learn` 时，把 `go env GOPATH` 下的 `bin` 加入 PATH。

## 开始学习

```bash
learn init ~/Learning
cd ~/Learning
```

然后在这个目录启动 Codex 或 Claude，说「继续学习」，或「我的教材在 /path/to/book.pdf，帮我导入」。Agent 会读取 Vault 内的 Skill 自行调用 `learn`。打开 `~/Learning/README.md` 可随时看到学到哪、下一步做什么。

## 给 AI 代理

你正在为用户安装 Personal Learning OS。按顺序执行，每步用工具确认成功再继续；除必要选择外不要打断用户。

1. 确认 `go version` 为 1.23 及以上，且 `git` 可用。缺少则告知用户先安装。
2. 安装 Runtime：
   ```bash
   go install github.com/hedykan/learning-system/cmd/learn@latest
   ```
   若报鉴权或找不到模块的错误，说明这是私有仓库：让用户确认已配置 `GOPRIVATE=github.com/hedykan` 与对应的 Git SSH 拉取，或把仓库设为公开。
3. 运行 `learn version`，确认输出版本号（如 `learn v0.1.5` 或更高）。若命令未找到，提示用户把 `go env GOPATH` 下的 `bin` 加入 PATH。
4. 问用户 Vault 放在哪里，默认 `~/Learning`。运行 `learn init <路径>`。
5. `cd` 进该目录，运行 `learn status` 确认可用。
6. 告诉用户安装完成，可以说「继续学习」或提供教材路径。之后完全遵循 Vault 内 `.agents/skills/learning-os/SKILL.md`（Claude 为 `.claude/…`）的指引，不要凭记忆操作 `learn`。

已有旧 Vault 时改用 `learn agent update --dry-run` 预览、`learn agent update --yes` 升级教学规则。

## 能力

- **跟着教材学**：建立与原书一致的目录，学习位置只能指向真实章节，逐节标记完成、跳过或学过一部分。
- **引导式教学**：先摸底，再用预测、反例、修正、迁移推进理解；每轮最多引入两个新术语。
- **证据化沉淀**：每条判断都要引用你的原话，Runtime 校验后才写入；概念状态分未观察、形成中、脆弱、稳定。
- **过去影响未来**：`learn next` 依据历史推荐下一步——修误解、复习、巩固或按目录前进。
- **间隔复习**：按 1、2、4、7、15、30、60 天排期，答错回到第一档。
- **Obsidian 笔记**：概念笔记、学习者总览、教材首页与学习首页，全部可重建；「手写笔记」区永远保留。相关概念互链，可在 Obsidian 图谱按理解状态着色。

## 更多

- 完整命令：`learn <命令> --help`，或 [`spec/cli.md`](./spec/cli.md)。
- 需求与设计：[`spec/`](./spec/README.md)；分阶段计划：[`development-plan/`](./development-plan/README.md)。
- 变更历史：[`CHANGELOG.md`](./CHANGELOG.md)。

解读由 Agent 提交，Runtime 不接入网络模型，也不内置 OCR。扫描版教材由 Agent 用视觉能力读取，读不出时会请你换用文字版。
