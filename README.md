# Personal Learning OS

> **AI 可以回答几乎任何问题，但“得到答案”并不等于“真正学会”。**

今天问数据库，明天聊分布式系统，后天研究微积分。

每次 AI 都可能回答得很好，但时间久了，学习很容易变成一堆互不相连的对话：

**学了很多，却越来越散。**

Personal Learning OS 想做的是：

> **把零散的 AI 对话，变成一条连续、系统、可以长期积累的学习路径。**

它跟着教材保持学习主线，记录你怎么理解、哪里错过、哪些知识真的能够应用，并让过去的学习影响 AI 下一步怎么教你。

```text
教材 / 学习目标
        ↓
真实学习过程
        ↓
认知变化
        ↓
Learner Model
        ↓
下一步学习
        ↺
```

学习历史保存在本地 Markdown 中，可以直接用 Obsidian 打开。

---

> **给 AI 一个网址就能装好。**
>
> 把下面这句话发给 Codex 或 Claude：
>
> `读 https://raw.githubusercontent.com/hedykan/learning-system/main/README.md 并按其中「给 AI 代理」一节安装，然后带我开始学习。`
>
> 这需要仓库是公开的。私有仓库请使用下面的「手动安装」。

## 为什么需要它

普通 AI Chat 更像：

```text
问题 → 回答

问题 → 回答

问题 → 回答
```

Learning OS 更关心的是：

```text
我正在学什么？
        ↓
我已经理解了什么？
        ↓
哪里其实还没理解？
        ↓
这次学习改变了什么？
        ↓
新知识和以前学过的什么有关？
        ↓
下一步应该怎么学？
        ↺
```

它不仅想记住：

> “你学过 Replication。”

还希望逐渐知道：

> 你原来怎么理解它？  
> 哪里理解错过？  
> 什么问题让你改变了理解？  
> 现在能不能自己解释？  
> 过几天还能不能想起来？  
> 能不能应用到新的系统设计问题里？

最终形成的不是一堆 AI 总结，而是一份持续变化、可以追溯证据的 **Learner Model**。

## 这是什么

`learn` 是一个 **local-first Learning Runtime**。

Codex 或 Claude 负责教学、提问和分析；`learn` 负责确定性的部分：

- 教材与学习位置；
- 原始学习历史；
- 学习证据与认知变化；
- Learner Model；
- 下一步学习动作；
- 间隔复习；
- Markdown / Obsidian 投影。

一个重要原则是：

> **Model 可以改变，History 不能改变。**

AI 可以重新判断你是否真正理解了一个概念，但已经发生过的学习过程不会被重新润色或覆盖。

你的学习数据保存在自己的 Vault 中，不依赖 Learning OS 的云服务。

## 手动安装

需要 Go 1.23+ 和 Git，支持 macOS、Linux、Windows。

```bash
go install github.com/hedykan/learning-system/cmd/learn@latest
learn version
```

私有仓库先配置一次拉取方式：

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

然后在这个目录启动 Codex 或 Claude，说：

> 继续学习

或者：

> 我的教材在 `/path/to/book.pdf`，帮我导入。

Agent 会读取 Vault 内的 Skill，自行调用 `learn`。

打开 `~/Learning/README.md` 可以随时看到：

- 正在学什么；
- 学到哪里；
- 当前理解状态；
- 下一步做什么。

### 用 Obsidian 查看

在 Obsidian 里选择「打开文件夹作为仓库」，选中 Vault 目录（如 `~/Learning`）即可。

- `README.md` 是学习首页，从这里可以跳到所有页面；
- `Concepts/` 是每个概念的笔记，`Questions/` 是你提出的问题，`Profile/学习者总览.md` 汇总你的理解状态；每本书的文件夹里有以书名命名的教材首页和「学习进度」；
- 笔记都以你学习时使用的语言命名，例如 `Concepts/缓存省去网络往返.md`；
- 页面上的固定文字也跟随你的语言：用英文学习时会自动切换为英文（如 `Profile/Learner overview.md`），也可以运行 `learn config set language en` 手动切换；
- 打开「关系图谱」可以看到概念之间的连接。在图谱设置的「分组」中按标签着色，例如 `tag:#learning/state/stable`、`tag:#learning/state/fragile`，就能一眼看出哪些已经稳定、哪些还脆弱。

## 给 AI 代理

你正在为用户安装 Personal Learning OS。按顺序执行，每步用工具确认成功再继续；除必要选择外不要打断用户。

1. 确认 `go version` 为 1.23 及以上，且 `git` 可用。缺少则告知用户先安装。

2. 安装 Runtime：

   ```bash
   go install github.com/hedykan/learning-system/cmd/learn@latest
   ```

   若报鉴权或找不到模块的错误，说明这是私有仓库：让用户确认已配置 `GOPRIVATE=github.com/hedykan` 与对应的 Git SSH 拉取，或把仓库设为公开。

3. 运行：

   ```bash
   learn version
   ```

   确认输出版本号（如 `learn v0.1.5` 或更高）。

   若命令未找到，提示用户把 `go env GOPATH` 下的 `bin` 加入 PATH。

4. 问用户 Vault 放在哪里，默认：

   ```text
   ~/Learning
   ```

   然后运行：

   ```bash
   learn init <路径>
   ```

5. `cd` 进该目录，运行：

   ```bash
   learn status
   ```

   确认可用。

6. 告诉用户安装完成，可以说「继续学习」或提供教材路径。

之后完全遵循 Vault 内 `.agents/skills/learning-os/SKILL.md`（Claude 为 `.claude/…`）的指引，不要凭记忆操作 `learn`。

已有旧 Vault 时：

```bash
learn agent update --dry-run
learn agent update --yes
```

用于预览和升级教学规则。

## 核心能力

### 📖 保持学习主线

跟着真实教材学习，维护章节、当前位置和学习状态。

允许为了先修知识暂时绕出去，但学习不会因此失去主线。

### 🧠 记录“你是怎么理解的”

不只是记录“学过什么”，还记录：

- 你的原始理解；
- Misconception；
- Insight；
- Understanding Revision；
- Retrieval；
- Application；
- Transfer。

重要判断必须能够追溯到真实学习证据。

### 🔄 让过去影响未来

`learn next` 不只是寻找教材下一页。

它会结合学习历史判断下一步更适合：

```text
继续教材
修正误解
补先修知识
要求预测
提供反例
测试迁移
复习旧概念
切换教学方式
```

Learning OS 的核心不是：

> **“这个问题应该怎么回答？”**

而是：

> **“对于现在的你，这个东西下一步应该怎么学？”**

### 🧭 引导式学习

首次学习可以先摸底，再通过：

```text
当前理解
↓
具体预测
↓
解释理由
↓
反例 / 条件变化
↓
修正模型
↓
新场景验证
```

逐步建立理解。

“我懂了”只是一条证据，不等于真正掌握。

### ⏳ 间隔复习

按：

```text
1 → 2 → 4 → 7 → 15 → 30 → 60 天
```

安排复习，答错后重新进入第一档。

### 🗂 本地、开放、可追溯

学习数据保存在本地：

```text
Markdown
YAML
JSON
Git
```

可以直接使用 Obsidian 查看。

概念笔记、学习者总览、教材首页和学习首页都可以重新生成；「手写笔记」区域始终保留。

相关概念可以互相连接，并在 Obsidian Graph 中查看。

## Learning OS 想解决什么

当知识变得触手可及时，真正困难的问题开始从：

> **“我怎么找到答案？”**

变成：

> **“我怎么长期、系统地真正学会它？”**

Learning OS 希望把：

```text
碎片化 AI 对话
```

逐渐变成：

```text
连续的学习路径
        +
相互连接的知识
        +
可追溯的 Cognitive History
        +
持续更新的 Learner Model
        +
越来越适合你的学习方式
```

最终，AI 不只是越来越了解知识。

**它也越来越了解你是怎么学会知识的。**

## 更多

- 完整命令：`learn <命令> --help`，或 [spec/cli.md](./spec/cli.md)
- 需求与设计：[spec/](./spec/README.md)
- 分阶段计划：[development-plan/](./development-plan/README.md)
- 变更历史：[CHANGELOG.md](./CHANGELOG.md)

解读由 Agent 提交。

Runtime 不接入网络模型，也不内置 OCR。扫描版教材由 Agent 使用视觉能力读取；无法可靠读取时，会请用户换用文字版。
