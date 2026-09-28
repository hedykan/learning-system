---
id: CR-2026-041
title: "有证据的课程调整建议与大纲版本历史"
status: implemented
target_version: v0.2
created: 2026-09-28
---

# 变更摘要

Agent 在解读记录中提交 `curriculum_proposals`：带学习者证据的课程调整建议。建议只有在学习者同意、Agent 执行 `learn curriculum accept <id>` 后才生效；每次生效都把旧大纲存入版本历史。“Model 可以改变，History 不能改变”延伸到课程本身。

## 动机

Curriculum Loop（Learner Model → 调整 Curriculum）最有价值也最危险：大纲若能被随意改动，主线就没了。调整必须有证据、经学习者同意、留版本。

## 目标行为

- 记录字段：

  ```json
  "curriculum_proposals": [{"id": "p1", "action": "mark_known", "node": "2.1",
    "reason": "学习者在 DDIA 中已能迁移该概念", "evidence": [{"turn": "t0004", "quote": "..."}]}]
  ```

  `action`：`skip`、`mark_known`（标为已完成，原因为已掌握）、`insert`（新条目 `node` 与 `title`，可带 `why`、`prerequisites`；位置由 ID 决定）、`remove`（没有子条目、未完成的条目）、`retitle`（`title`）。必须有学习者证据；ID 全局唯一。
- 类型限制见 CR-2026-038；提交时校验条目存在与动作合法。
- `learn curriculum proposals [--json]` 列出待定建议；`accept <id>` 应用到大纲并保持已确认状态；`reject <id> --reason`。决定记录在 `Curriculum/<id>/proposals.yaml`。
- 应用前的大纲存为 `Curriculum/<id>/outline-history/NNNN.yaml`，附时间、建议 ID 与理由；`learn curriculum outline history` 列出版本。
- `learn next` 在有待定建议时输出 `pending_proposals`，Skill 要求在合适时机问学习者。

## 数据与兼容性影响

Interpretation Record 新增可选字段；旧记录不受影响。建议本身来自记录（可重建），决定属于课程状态。

## 验收条件

1. 没有证据、条目不存在、类型不允许的建议被拒收。
2. accept 后大纲按动作改变，版本历史多一份旧大纲；reject 不改大纲。
3. 同一建议不能决定两次。

## 决策

2026-09-28 接受，排入 v0.2。

2026-09-28 实现：记录字段 `curriculum_proposals`；提交时按当前大纲与类型校验；`learn curriculum proposals/accept/reject` 与 `outline history`；决定存于 `proposals.yaml`，被替换的大纲存于 `outline-history/`；skip、mark_known 记为进度并在当前条目时推进位置；`next` 输出 `pending_proposals`，首页提示。
