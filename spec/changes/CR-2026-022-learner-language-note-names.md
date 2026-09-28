---
id: CR-2026-022
title: "笔记文件名与标题使用学习者的语言"
status: implemented
target_version: v0.1.6
created: 2026-09-28
---

# 变更摘要

生成笔记的文件名改用学习者的语言，不再使用内部 ID。概念笔记以概念名称命名，问题笔记以问题本身命名，教材首页以书名命名，学习者总览与学习进度使用中文固定名；Session 记录的标题与小标题改为中文。

## 动机

v0.1.6 实测中，概念笔记标题已是“缓存省去网络往返”，文件名却是 `cache-avoids-roundtrip.md`。Obsidian 的文件列表、关系图谱节点与链接都显示文件名，学习者看到的是一堆英文 ID。

## 目标行为

| 笔记 | 旧路径 | 新路径 |
| --- | --- | --- |
| 概念 | `Concepts/<id>.md` | `Concepts/<概念名称>.md` |
| 问题 | `Questions/<id>.md` | `Questions/<问题>.md`（最多 60 字） |
| 学习者总览 | `Profile/learner-state.md` | `Profile/学习者总览.md` |
| 教材首页 | `Curriculum/<id>/index.md` | `Curriculum/<id>/<书名>.md` |
| 学习进度 | `Curriculum/<id>/progress.md` | `Curriculum/<id>/学习进度.md` |
| Session | `Sessions/<id>.md` | 不变，标题改为“学习记录 YYYY-MM-DD HH:MM” |

- 内部 ID 保持不变，继续用于记录、证据与命令；ID 写入笔记 frontmatter 的 `aliases`，旧的简写链接仍可解析。
- 概念名称与问题一经创建不可修改，文件名因此稳定。
- 文件名中去掉 `/ \ : * ? " < > | # ^ [ ]` 等在某些平台或 Obsidian 链接中不合法的字符；两个名称相同时，按 ID 顺序为后一个加上 `（ID）`。
- 迁移：重建投影时，扫描 `Concepts/`、`Questions/`、`Profile/` 与各教材文件夹，由程序生成但文件名不符合新规则的笔记，其手写区搬到新文件，旧文件删除；只删除带 `generated_by: learn` 的文件。
- Session 文件名保留时间戳：它们按时间排列，并被大量原话证据链接引用。
- 固定页面的语言跟随投影语言，目前为中文。支持其他语言需要为投影增加界面语言设置，留待后续。

## 验收条件

1. 新生成的笔记文件名为概念名称、问题与书名。
2. 旧命名的生成笔记在重建后迁移，手写区保留，旧文件删除；学习者自己的文件不受影响。
3. 笔记 aliases 含内部 ID。

## 决策

2026-09-28 应用户要求加入 v0.1.6（尚未发布）。
