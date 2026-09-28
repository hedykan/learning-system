---
id: CR-2026-025
title: "通用定位：用 locator 取代页码"
status: accepted
target_version: v0.1.8
created: 2026-09-28
---

# 变更摘要

目录节点与教材要点用一个通用的 `locator`（种类 + 值）表示“在资料的哪里”，取代只适用于 PDF 的 `pages`。这是 v0.1.8 其余资料类型的地基。

## 动机

`outline.yaml` 的节点、Interpretation Record 的 `textbook_points` 和概念笔记里的“第 x–y 页”都假定资料有页码。网页、EPUB、代码项目、视频都没有页码：网页用锚点，EPUB 用章节，代码用文件与行号，视频用时间点。不先统一定位，后面每加一种资料都要改记录格式和校验。

## 当前行为

- `outline.yaml` 节点可带 `pages: [起, 止]`，校验页码递增且落在父节点范围内。
- `textbook_points.pages` 必须是两个页码，并落在所属节点的页码范围内（`session.checkPointsInNode`）。
- 概念笔记写“AI 根据原书第 x–y 页概括”。

## 目标行为

- 新增 `locator`：

  ```json
  {"resource": "r1", "kind": "page", "value": "42-45"}
  ```

  | kind | value 示例 | 用于 |
  | --- | --- | --- |
  | `page` | `42-45` | PDF、纸质书 |
  | `anchor` | `#replication-lag` | HTML、网页快照 |
  | `chapter` | `ch05.xhtml#sec2` | EPUB、DOCX |
  | `file` | `src/raft.go#L120-180` | 代码项目（提交号在资料版本里） |
  | `time` | `3/05:20-48:00`（第几集/起-止） | 视频、音频 |
  | `text` | 自由文本 | 无法结构化的外部资料 |

- `resource` 指向课程内的一份资料（见 CR-2026-027），只有一份资料时可省略。
- 校验由资料适配器负责（见 CR-2026-026）：格式合法；同一种类可比较时（page、time、file 行号）检查“要点落在节点范围内”；不可比较时（anchor、chapter、text）只检查格式与存在性。
- 学习者可见文字按种类渲染：“第 42–45 页”“第 3 讲 05:20–48:00”“`src/raft.go` 第 120–180 行”。

## 数据与兼容性影响

- 旧的 `pages: [a, b]` 继续可读，读取时视为 `{"kind": "page", "value": "a-b"}`；写入新记录时 Agent 可任选其一，程序统一成 locator。
- 已提交的 Interpretation Record 不改写。回放时对旧 `pages` 做同样换算，重建结果与 v0.1.7 一致。
- `record` 与 `outline` 的 schema 版本各加一；CLI JSON 同时输出 `locator`，并在一个版本内保留 `pages`。

## 风险

- 不可比较的种类无法做范围校验，Agent 挂错位置的可能性变大。缓解：这些种类要求 value 在资料目录或快照中真实存在。

## 验收条件

1. 旧 Vault 升级后 `learn model rebuild` 的结果与升级前逐字一致。
2. page、time、file 三种 locator 的范围校验各有接受与拒收用例。
3. anchor 在快照中不存在时拒收。
4. 概念笔记按种类渲染定位文字。

## 决策

2026-09-28 接受，排入 v0.1.8，作为本版第一项实现。
