# Curriculum Builder 核心想法

**状态：** idea（IDEA-035，候选 v0.2）  
**来源：** 学习者提出，2026-09-28

> 把任何一个学习目标，组织成一条有来源、有结构、有上下文、能够持续演化的系统学习路径。

## 1. 教材只是 Curriculum 的一种来源

- 有成熟教材：解析原有结构 → Curriculum。
- 没有成熟教材：学习目标 → Agent Research（教材、官方文档、论文、课程、GitHub 等可靠来源）→ 组织知识与先修关系 → Curriculum。
- 两者进入同一个 Learning Engine。

## 2. 两种 Curriculum

- **Source-aligned**：忠于原作者的章节和教学主线。Learner Model 主要决定“怎么学”，不让 AI 随意改变“学什么”。
- **Synthesized**：由 Agent 搜集多个可靠来源，按知识依赖组织。每个重要 Topic 保留来源、为什么要学、prerequisite、在整体课程中的位置，避免凭空生成看似合理的课程。

## 3. Curriculum 不是静态目录

学习中发现缺少先修 B 时，暂时进入 B，完成最小必要学习后返回 A，并记录 Detour Reason、Detour Topic、Return Point。

## 4. Learner Model 同时影响“怎么学”和“学什么顺序”

- Source-aligned：尽量保持教材顺序，Learner Model 影响教学策略和必要的先修绕行。
- Synthesized：Learner Model 可以更深地参与课程设计；同样学 Kubernetes，不同基础的人路径不同。

## 5. 两个核心循环

- **Learning Loop**：Curriculum → Learn → Cognitive Change → Learner Model → Learning Policy → Next Action → Learn。解决“怎么让一个人真正学会”。
- **Curriculum Loop**：Learning Goal → Sources → Curriculum → Learn → Learner Model → 调整 Curriculum。解决“怎么让一个人长期、系统地学下去”。

教材是 Source，Curriculum 是结构，Learner Model 是个人化基础，Learning Policy 决定下一步，而持续形成的系统性学习才是产品本身。

## 讨论记录（2026-09-28）

转为变更单前需要明确：

1. **课程类型影响规则**：`type: source_aligned | synthesized`。前者只能跳过或绕行，后者可以插入、删除、调换 Topic；两者的调整都需学习者确认。
2. **Topic 元数据由程序校验**：`why`、`prerequisites`、`sources`、`sourced`。先修必须存在且无环；`learn next` 只推荐先修已完成的节点；无来源节点标“AI 综合”。
3. **课程调整有证据、有版本**：Agent 在解读记录中提交带证据的课程调整建议，学习者确认后生效，大纲保留版本历史。
4. **跨课程复用已学概念**：新课程 Topic 涉及的概念若在其他教材中已 stable，默认改为快速检索验证。
5. **范围控制**：第一版只做“目标 → 大纲 → 挂资料 → 学”，课程自动调整放在第二步；待定：小调整是否允许不经确认自动生效。
