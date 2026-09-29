# 想法池

这里记录尚未进入版本基线的产品想法。每条想法在开发前必须转成变更单或下一版本需求。

| ID | 想法 | 状态 | 候选版本 |
| --- | --- | --- | --- |
| IDEA-001 | PDF 文本提取与章节自动识别 | idea | v0.2（目录部分由 CR-2026-005 以 Agent 辅助方式先行） |
| IDEA-002 | 扫描版 PDF OCR | rejected | 2026-09-26：程序不做 OCR；Agent 先用模型视觉读页面，读不出再请学习者换用文字版（见 CR-2026-014） |
| IDEA-003 | 基于遗忘和检索证据的自动复习计划 | implemented | v0.1.5（见 CR-2026-012） |
| IDEA-004 | Claude/Codex 宿主 Hook 自动捕获对话 | deferred | 2026-09-26：继续由 Skill 保证 Agent 逐轮记录 |
| IDEA-005 | 真实 AI Provider 分析 Session | rejected | 2026-09-26：语义分析由 Agent 完成，程序不内置模型调用 |
| IDEA-006 | 教材版本升级后的章节位置映射 | idea | backlog |
| IDEA-007 | 将 Git 代码项目作为 Project Source 导入并按 revision 学习 | implemented | v0.1.9（见 CR-2026-030） |
| IDEA-008 | Source 验证、可恢复删除/重导入与 Session 中止 | implemented | Session 中止与位置重置已在 v0.1.1 实现；Source 验证并入 CR-2026-005；删除、归档、恢复进入 v0.1.4（见 CR-2026-001） |
| IDEA-009 | 首次学习摸底、例子优先与术语预算 | implemented | v0.1.1（见 CR-2026-002） |
| IDEA-010 | 证据驱动的知识沉淀、Cognitive History 与长期 Learner Model | implemented | v0.1.2（见 CR-2026-004） |
| IDEA-011 | 教材章节遵循：目录、位置校验、完成记录、依据原文教学 | implemented | v0.1.3（见 CR-2026-005） |
| IDEA-012 | 学习者可见话术与学习对话模型配置 | implemented | v0.1.3（见 CR-2026-006） |
| IDEA-013 | Agent 实测暴露的可用性缺陷 | implemented | v0.1.3（见 CR-2026-007） |
| IDEA-014 | 推进教材前的跨 Session 巩固规则 | implemented | v0.1.3（见 CR-2026-008） |
| IDEA-015 | Runtime 提供 PDF 指定页文本读取（如 `learn source read <id> --pages 30-38`），免去 Agent 每次自行拼凑 PDF 工具 | deferred | 用户决定暂缓 |
| IDEA-016 | 目录条目的“学过一部分”状态：讲过但未完成的小节不再显示为未开始 | implemented | v0.1.4（见 CR-2026-010） |
| IDEA-017 | Vault 首页 README.md 与清理空的旧目录 | implemented | v0.1.4（见 CR-2026-009） |
| IDEA-018 | 概念笔记增加“教材要点”一段，概括原书对该概念的讲法 | implemented | v0.1.4（见 CR-2026-011） |
| IDEA-019 | Obsidian 知识图谱：Agent 标注相关概念，程序生成概念互链与状态标签 | implemented | v0.1.5（见 CR-2026-013） |
| IDEA-020 | 扫描版教材：先用模型视觉，读不出再请学习者换用文字版 | implemented | v0.1.5（见 CR-2026-014） |
| IDEA-021 | Windows 构建与路径兼容测试 | implemented | v0.1.5（见 CR-2026-015） |
| IDEA-022 | 跨课程学习模式实测：用第二本教材验证 Learning Pattern | idea | 有第二本教材后 |
| IDEA-023 | 17 个核心验证场景全部通过 | partial | v0.1.6 复验后仅场景 7 未通过 |
| IDEA-025 | 界面语言设置：固定文字跟随学习者语言 | implemented | v0.1.7（见 CR-2026-023） |
| IDEA-026 | 学习记录未提交到 Git 时提醒 | implemented | v0.1.7（见 CR-2026-024） |
| IDEA-027 | 教材要点兜底：学完一节时提示补上本节概念的教材要点 | idea | 下一版本候选 |
| IDEA-028 | 教材文件夹整理：去掉与教材首页重复的 book.md，隐藏运行时状态文件 | idea | 下一版本候选 |
| IDEA-024 | 自发检索提示：记录中出现对早先 Session 已学概念的使用、却没有 retrieval 事件时，checkpoint 输出提示 Agent 核对是否为自发检索 | idea | 下一版本候选 |
| IDEA-029 | 通用定位与资料适配器接口，课程挂多份资料 | implemented | v0.1.8（见 CR-2026-025 至 027） |
| IDEA-030 | 没有文件的资料（视频课、纸质书）与文字类格式（HTML、EPUB、DOCX 等） | implemented | 外部资料 v0.1.8（CR-2026-028）；文字类格式 v0.1.9（CR-2026-029） |
| IDEA-031 | 网址抓取快照与 Agent 搜集资料组织课程 | implemented | 网址快照 v0.1.9（CR-2026-031）；组织课程 v0.2（CR-2026-032） |
| IDEA-032 | 对话中的数学公式用终端可读的符号 | implemented | v0.1.8（见 CR-2026-033） |
| IDEA-033 | 学习流程简化为三段：资料搜集 → 学习 → 巩固 | implemented | v0.1.8（见 CR-2026-034） |
| IDEA-034 | Obsidian 图谱整理：按层打标签、证据链接收敛、关系类型与单向链接 | implemented | v0.1.8（见 CR-2026-035 至 037） |
| IDEA-035 | Curriculum Builder：Source-aligned 与 Synthesized 课程、Topic 先修与来源、有证据的课程调整、跨课程复用已学概念（见 [proposals/curriculum-builder.md](./proposals/curriculum-builder.md)） | implemented | v0.2（见 CR-2026-038 至 042） |
| IDEA-036 | 入学：目标访谈形成目标卡、诊断摸底先于建课并影响课程设计、草案评审与目标覆盖检查、资料选择理由 | implemented | v0.2.1（见 CR-2026-043 至 046） |
| IDEA-037 | 一个总文件夹管理多个学习库：列出库与课程 | implemented | v0.2.2（见 CR-2026-047）；总文件夹的 AGENTS.md、跨库汇总暂不做 |

## IDEA-007 初步边界

- 项目是 `Project Source`，某个 Git commit 是可复现的 `Source Revision`。
- 默认使用 link，不复制整个仓库；记录本地入口、commit、branch 和 dirty 状态。
- 索引前排除 `.git`、构建产物、依赖目录、二进制、凭据和常见敏感文件。
- 用户选择学习范围和目标，例如子系统、目录、调用链或贡献任务；它们形成 Curriculum，而不是写回 Source。
- 项目变化后创建新 revision，并明确选择继续旧 revision 还是迁移学习路线。
- 正式进入版本前需要变更单明确命令、索引策略、敏感文件规则和大型仓库性能验收。
