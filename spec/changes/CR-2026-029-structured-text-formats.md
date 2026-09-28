---
id: CR-2026-029
title: "文字类格式：HTML、EPUB、DOCX、Jupyter、LaTeX 等"
status: accepted
target_version: v0.1.8
created: 2026-09-28
---

# 变更摘要

为本身就是文字、带标题结构的格式各写一个适配器：程序自动出草稿目录、按定位提取为 Markdown；理解内容仍由 Agent 负责。

## 动机

很多技术书有 EPUB 版，文档站可以存成 HTML，讲义常是 DOCX 或 LaTeX，数据课程是 Jupyter。它们的结构和文字都能确定性地拿到，比 PDF 好处理得多，却现在全部报“不支持的格式”。

## 目标行为

| 格式 | 扩展名 | 草稿目录来源 | 定位 |
| --- | --- | --- | --- |
| HTML | `.html` `.htm` | `<h1>`–`<h3>` 及其 id | `anchor` |
| EPUB | `.epub` | 包内导航文件（nav / NCX） | `chapter` |
| DOCX | `.docx` | 标题样式 Heading 1–3 | `chapter`（标题序号） |
| Jupyter | `.ipynb` | Markdown 单元格标题 | `anchor` |
| LaTeX | `.tex` | `\part` `\chapter` `\section` `\subsection` | `anchor`（标签或标题） |
| reStructuredText / AsciiDoc / Org | `.rst` `.adoc` `.org` | 各自标题语法 | `anchor` |

- 能力全部为 `Structured: true, Extractable: true`；EPUB/HTML 内的图片列入 `Content.images`，公式图片标 `needs_vision`。
- 提取只做格式转换（剥标签、保留标题层级、列表、代码块、表格与链接），不做摘要。
- EPUB、DOCX 用标准库 `archive/zip` 与 `encoding/xml`；HTML 引入 `golang.org/x/net/html` 一个依赖。
- 文件夹导入接受上述格式混合，按自然顺序排序。
- 带 DRM 的 EPUB 报错并提示换用无 DRM 版本。

## 数据与兼容性影响

纯新增适配器；已有课程不受影响。新增一个 Go 依赖。

## 风险

- 各种 EPUB/HTML 的结构差异大，草稿目录可能不准。缓解：草稿永远需要 Agent 审核、学习者确认；解析失败时退回 `ErrNoStructure`，由 Agent 手建目录。

## 验收条件

1. 每种格式至少一个真实样例：导入、出草稿目录、按定位读出正确段落。
2. 一本真实 EPUB 技术书的草稿目录与书内目录一致。
3. 带 DRM 的 EPUB 给出明确错误。
4. 解析失败时导入仍成功，`source outline` 返回 `no_structure`。

## 决策

2026-09-28 接受，排入 v0.1.8。
