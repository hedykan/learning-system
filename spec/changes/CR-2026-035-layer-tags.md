---
id: CR-2026-035
title: "按层给生成文档打标签，支持 Obsidian 按标签筛选"
status: accepted
target_version: v0.1.8
created: 2026-09-28
---

# 变更摘要

每篇生成文档带一个固定的英文层级标签，学习者可以在 Obsidian 图谱和搜索中用 `tag:#learning/...` 按“知识、证据、学习过程、导航”筛选。新建 Vault 默认图谱只显示知识与学习者手写笔记。

## 动机

v0.1.6 测试 Vault 中，概念笔记发出的链接只有 42 条指向其他概念，却有 85 条指向原始对话、32 条指向学习记录；学习记录又链接到 123 个概念。Obsidian 把所有内部链接画成边，学习记录、原始对话和首页成为中心，真实的知识结构被淹没。另外测试 Vault 开启了图谱的“显示标签”，状态标签和教材标签本身也变成了中心节点。

## 目标行为

### 标签

| 类别 | 标签 | 文档 |
| --- | --- | --- |
| 知识 | `learning/knowledge/concept`、`learning/knowledge/question` | 概念、问题 |
| 证据 | `learning/evidence/conversation` | 原始对话 |
| 学习过程 | `learning/process/session`、`learning/process/progress` | 学习记录（解读）、教材进度 |
| 导航 | `learning/nav/home`、`learning/nav/overview`、`learning/nav/curriculum` | 首页、学习者总览、教材首页 |

- 标签固定为英文，不随界面语言变化，避免切换语言后筛选与着色失效。
- 现有 `learning/state/*`、`learning/curriculum/*` 保留，用于着色。
- 新的原始对话在创建时写入标签；已有原始对话做一次迁移：只在 frontmatter 中加入 `tags`，对话正文一个字节不改。迁移由 `learn agent update --yes` 执行，`--dry-run` 列出将改动的文件。

### 默认图谱配置

- `.obsidian/graph.json` **不存在时**，`init` 与 `agent update` 写入默认配置：
  - 筛选：`-tag:#learning/evidence -tag:#learning/process -tag:#learning/nav`（保留学习者自己的笔记）；
  - 关闭“显示标签”（`showTags: false`），开启箭头；
  - 着色：stable 绿、fragile 橙、developing 蓝、问题紫。
- 文件已存在时绝不修改；首页“怎么用”写明手动设置步骤与常用筛选写法。

### 常用筛选（写入首页说明）

| 想看 | 写法 |
| --- | --- |
| 全部生成文档 | `tag:#learning` |
| 只看知识 | `tag:#learning/knowledge` |
| 知识 + 手写笔记 | `-tag:#learning/evidence -tag:#learning/process -tag:#learning/nav` |
| 某本书还不稳固的知识 | `tag:#learning/knowledge tag:#learning/curriculum/<id> tag:#learning/state/fragile` |

## 数据与兼容性影响

- 原始对话 frontmatter 新增 `tags`；解析器忽略未知字段，旧版本 `learn` 仍能读取。
- 迁移只增不改，可重复执行。

## 风险

- 学习者已有的图谱配置仍开着“显示标签”。缓解：首页说明里第一条就是关闭它。

## 验收条件

1. 每类生成文档都带对应标签；切换界面语言后标签不变。
2. 迁移后原始对话的轮次内容与迁移前逐字节一致，只多一行标签。
3. 新 Vault 有默认 `graph.json`；已有 `graph.json` 的 Vault 升级后该文件不变。
4. 在 Obsidian 中用默认筛选打开测试 Vault，图谱只剩概念、问题与手写笔记。

## 决策

2026-09-28 接受，排入 v0.1.8。原始对话采用“只加标签”的一次性迁移；如学习者更倾向不改动原始对话，可改为按路径筛选。
