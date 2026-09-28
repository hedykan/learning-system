# v0.2 开发计划

**状态：** done  
**基线：** [v0.2](../../spec/versions/v0.2.md)

| 阶段 | 内容 | 状态 | 完成条件 |
| --- | --- | --- | --- |
| 1 | CR-038 课程类型 | done | 书保持 source_aligned；insert 被拒 |
| 2 | CR-039 Topic 元数据与先修 | done | 非法先修拒收；`next` 越过被挡条目、给出 blocked_by |
| 3 | CR-040 目标课程 | done | 建课、无来源标注、教材要点拒收 |
| 4 | CR-041 调整建议与版本历史 | done | 各类拒收；accept、reject、历史、重复决定 |
| 5 | CR-042 跨课程复用 | done | quick_check 触发与不触发 |
| 6 | Skill、文档 | done | curriculum-builder 参考、CLI 契约、CHANGELOG、README |
| 7 | 真实 Agent 复验 | done | 从学习目标出发研究、建纲、挂资料、学一节，并触发一次调整建议 |

## 真实 Agent 复验（2026-09-28，Codex gpt-5.6-sol 低思考，开启网页搜索与网络，4 轮）

目标：“系统地学一下向量数据库，能自己选型并调优一个向量检索服务”（学习者是后端工程师，用过 Elasticsearch，没接触过 embedding）。

| 轮 | 结果 |
| --- | --- |
| 1 | 网页搜索 2 次；`--goal` 建立 synthesized 课程；提交 7 节大纲，每节都有 `why`、先修链（5 依赖 3、4）与预计概念；列出计划引用的 Faiss、pgvector 官方资料与 HNSW 论文，等学习者确认 |
| 2 | 学习者确认、跳过摸底；Agent 抓取 5 份官方资料快照（Google ML 课程、Faiss、pgvector、Elastic、ann-benchmarks），挂到全部 7 节，`source check` 无 unsourced；按章节定位读原文后，用检索场景让学习者预测 |
| 3 | 学习者回答并要求加一节多租户；Agent 讲清 embedding 与数据隔离的边界，提交 `insert` 建议（6.1，先修 4、5，证据为学习者原话），先问学习者是否加入。第一次提交把先修写成父条目 6，被拒收后自行修正 |
| 4 | 学习者同意后执行 `curriculum accept`；大纲变为 8 条且仍为已确认，旧大纲存为历史版本 1；Agent 给 6.1 挂上 pgvector 的多租户章节；结束时总结并提示 `learn commit` |

遗留观察：Codex 全局指令要求中文回复，本轮学习者也用中文，界面语言未涉及；quick_check 只由自动化测试覆盖（新学习库没有稳定概念）。
