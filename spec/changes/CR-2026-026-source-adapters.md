---
id: CR-2026-026
title: "资料适配器与 learn source 命令"
status: accepted
target_version: v0.1.8
created: 2026-09-28
---

# 变更摘要

每种资料格式实现同一个适配器接口，只做确定性的事：认格式、存档、出草稿目录、按定位取文字、校验定位。Agent 通过一组固定的 `learn source` 命令使用它们，不论什么格式，读到的都是统一的 Markdown。

## 动机

现在的格式判断写死在 `curriculum.sourceKind` 的 switch 里，草稿目录只有 Markdown 有，取原文全靠 Agent 自己想办法（读 PDF 时每次方法不同，慢且不稳定）。新增格式时要改多处代码。统一接口后，新格式只是新增一个适配器（开闭原则）。

## 目标行为

### 程序内部接口

```go
type Adapter interface {
    Kind() string
    Detect(path string, info fs.FileInfo) bool
    Capabilities() Caps
    Import(src, dst string) (Revision, error)        // 快照 + 哈希；代码项目为提交号
    Outline(rev Revision) (*Outline, error)          // 草稿目录；无结构返回 ErrNoStructure
    Read(rev Revision, loc Locator) (Content, error) // 统一转成 Markdown
    Validate(rev Revision, loc Locator) error
}

type Caps struct {
    Structured  bool // 能自动出草稿目录
    Extractable bool // 能按定位取文字
    NeedsVision bool // 需要 Agent 看图
    External    bool // 没有文件，内容由学习者带进对话
}
```

`Content` 为 `{locator, format: "markdown", text, images, needs_vision}`。

### 本版要改写和新增的适配器

| 适配器 | Structured | Extractable | 说明 |
| --- | --- | --- | --- |
| markdown（含文件夹） | ✓ | ✓ | 现有行为；文件夹按自然顺序排序（`ch2` 在 `ch10` 前） |
| text | ✗ | ✓ | 现有行为 |
| pdf | ✗ | ✗ | 维持现状：目录由 Agent 建，原文由 Agent 读（IDEA-015 仍暂缓） |
| external | ✗ | ✗ | 见 CR-2026-028 |

HTML、EPUB 等见 CR-2026-029，代码项目见 CR-2026-030。

### 命令（输出 JSON）

| 命令 | 作用 |
| --- | --- |
| `learn source add <路径\|网址> [--external]` | 存档一份资料，返回资料编号、类型、能力、版本 |
| `learn source list [课程]` | 列出资料与能力 |
| `learn source outline <资料>` | 草稿目录；无结构时返回 `no_structure` |
| `learn source read <资料> <定位>` | 取内容；`Extractable` 为假时返回 `unsupported` 与原因 |
| `learn source check [课程]` | 报告没有资料的节点、失效的定位 |

`learn curriculum add` 保持不变，内部改为“新建课程 + `source add` + 挂到全部节点”。

### Skill

一条规则：先看 `capabilities` 再决定怎么读——`Extractable` 用 `learn source read`；`NeedsVision` 用视觉；`External` 请学习者把内容带进对话；都不满足时按现有规则自己读。

## 数据与兼容性影响

- `Sources/<id>/` 目录保持不变；现有课程视为只有一份资料 `r1`。
- 文件夹导入的自然排序会改变**新导入**的草稿目录顺序，已确认的目录不受影响。

## 风险

- 接口一次设计过大。缓解：本版只实现上表的适配器，接口字段以它们的实际需要为准，其余格式按此接口后续接入。

## 验收条件

1. 现有 PDF、Markdown、文本导入的测试全部通过，行为不变。
2. 注册一个测试用假适配器，不修改其他代码就能被 `source add/outline/read` 使用。
3. 对 Markdown 资料 `learn source read` 按标题锚点返回对应段落。
4. 导入含 `ch1.md`…`ch14.md` 的文件夹时草稿目录按章节号排序。

## 决策

2026-09-28 接受，排入 v0.1.8。
