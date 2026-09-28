---
id: CR-2026-019
title: "学习事件判别规则"
status: implemented
target_version: v0.1.6
created: 2026-09-28
---

# 变更摘要

在 Skill 中给出学习事件类型的判别规则与正反例，纠正实测中反复出现的误标。这是语义判断，由 Agent 执行，程序不改动。

## 动机

v0.1.5 实测误标：

- 场景 5：学习者自己推出“文件指纹比对”，被记成 insight；学习者对自己学习方式的观察反而被记成 learner_proposed_method。
- 场景 7：学习者主动调用三天前的知识，被记成 connection，且没有引用旧 Session 的原话。
- 场景 11：与原题同类的题被标成 transfer。
- 场景 3：普通概括被标成 insight。

## 目标行为

Skill 的 interpretation workflow 新增判别表：

- application：与学过的题同一类型、同一领域的新题。
- transfer：换到明显不同的领域或问题类型，学习者自己完成映射。
- learner_proposed_method：学习者自己提出的解法、推导路径或设计，即使与教材答案相同。
- insight：学习者自发说出的区别或规律，不是对刚讲内容的复述或概括。
- retrieval：没有提示，学习者主动用上以前 Session 学过的知识；证据同时引用当前轮次与旧 Session 中学到它的轮次（`<session-id>#tNNNN`）。
- 学习者对自己学习方式的观察记为 pattern observation，不记为任何事件类型。

## 验收条件

1. Skill 含判别表与正反例。
2. 实测复验场景 5、7、11 通过。

## 决策

2026-09-28 接受，进入 v0.1.6。
