# TaskPlan 能力学习沉淀（planning-with-files / OpenSpec / writing-plans）

> 更新时间：2026-03-01  
> 适用项目：`ai-workflow`（Go 编排内核）  
> 目标读者：后续接手同类需求的工程师 / Agent

---

## 1. 这份文档解决什么问题

我们需要把“用户聊天需求 -> 可执行 task plan -> 并行执行”做成稳定能力。  
这份文档沉淀了三个项目/技能的定位差异与组合方案，避免后续重复调研。

涉及对象：
- `planning-with-files`
- `OpenSpec`
- `writing-plans`（superpowers）

---

## 2. 三者定位（先分工，再集成）

### 2.1 OpenSpec：规范与任务骨架生成器

核心价值：
- 把需求落为结构化变更产物（proposal/spec/design/tasks）
- 能提供清晰的“从想法到任务清单”的标准流程
- 适合作为 `taskplan_generate` 的上游主能力

在本项目里的角色建议：
- 插件槽位：`Spec`（现有 `spec-openspec` 可扩展）
- Pipeline 阶段：`requirements -> spec_gen -> spec_review -> taskplan_normalize`

### 2.2 writing-plans：实施级计划细化器

核心价值：
- 把“已经确认的需求/设计”拆成非常细粒度、可立即执行的步骤
- 强调明确文件路径、测试命令、验收结果，适合交给执行器直接消费

在本项目里的角色建议：
- 作为 `taskplan_refine` 阶段的计划细化规则
- 输出结果用于驱动执行器 PR/提交策略

### 2.3 planning-with-files：执行期长上下文记忆层

核心价值：
- 用文件做持久记忆（计划、发现、进度），降低长任务漂移
- 在多轮执行、恢复执行、跨 Agent 协作时价值明显

在本项目里的角色建议：
- 不替代 OpenSpec，不负责“规范建模”
- 放在执行阶段（implement/fixup）做过程管理和恢复辅助

---

## 3. 推荐组合（V1 可落地）

推荐主链路：
1. 聊天需求进入 `requirements`
2. `spec_gen` 使用 OpenSpec 生成规范与 `tasks.md`
3. `taskplan_normalize` 将 `tasks.md` 转为统一 JSON（内部 TaskPlan）
4. `taskplan_refine`（可选）按 writing-plans 规则细化到“可执行步”
5. 执行阶段引入 planning-with-files（记录 `task_plan.md` / `findings.md` / `progress.md`）
6. 调度器按依赖关系并行分发

为什么这样组合：
- OpenSpec 负责“定义做什么”
- writing-plans 负责“定义怎么做”
- planning-with-files 负责“执行时别跑偏”

---

## 4. 与现有 ai-workflow 架构的映射

### 4.1 新增/调整阶段建议

- 新增：`taskplan_generate`（可复用 `spec_gen` 结果，也可合并进 `spec_gen`）
- 新增：`taskplan_normalize`
- 可选新增：`taskplan_refine`
- 现有 `implement/fixup` 阶段内启用文件化进度记录

### 4.2 统一数据结构（建议）

建议在核心模型里引入 `TaskPlan`：
- `plan_id`
- `source`（openspec/manual/chat）
- `tasks[]`
- `tasks[].id`
- `tasks[].title`
- `tasks[].description`
- `tasks[].acceptance`
- `tasks[].depends_on[]`
- `tasks[].labels[]`
- `tasks[].status`

并将其持久化到：
- `pipelines.artifacts_json`（最小改动）
- 后续可升级单独表（例如 `task_nodes`）

---

## 5. GitHub 可选（不是必选）的实现原则

结论：把 GitHub 作为适配层，不作为内核前提。

落地原则：
- 内核调度只依赖本地 `TaskPlan + Store + EventBus`
- `github.enabled=false` 时，系统仍可完整执行
- `github.enabled=true` 时，再做 Issue/PR/Label 镜像同步

一句话：先保证“本地闭环可跑通”，再保证“GitHub 协作可对齐”。

---

## 6. V1 最小实施清单（按优先级）

P0（必须）：
1. 定义 `TaskPlan` 数据结构与序列化
2. 在 `spec_gen/spec_review` 后产出标准化任务清单
3. 调度器支持基于 `depends_on` 的就绪判定（本地）
4. 执行器消费 `ready` 任务并回写状态

P1（建议）：
1. 增加 `taskplan_refine`（writing-plans 风格）
2. 实现执行期三文件沉淀（plan/findings/progress）
3. 增加失败重试与人工介入的任务级视图

P2（可选）：
1. `github-tracker` 镜像 Issue/Label
2. Webhook 反向同步状态
3. PR 审核与任务依赖可视化增强

---

## 7. 踩坑与经验

1. 不要把依赖关系只编码在 GitHub 标签里。  
应先有内核 DAG，再决定如何映射到标签。

2. 不要让执行器直接解析自然语言需求。  
执行器应只消费结构化 `TaskPlan`。

3. `taskplan_refine` 需要有上限。  
避免把任务拆到过碎导致调度成本高于收益。

4. 长任务必须有恢复语义。  
至少要记录“当前任务、最近决策、阻塞点、下一步”。

5. 先保守后扩展。  
V1 优先“稳定完成”，而不是“一步到位全自动”。

---

## 8. 给后来者的执行建议

如果你要继续推进这条线，建议顺序：
1. 先读本文件与 `spec-*` 文档，确认边界
2. 先实现 `TaskPlan` 内核模型和本地调度
3. 再接 OpenSpec 产物映射
4. 然后再接 GitHub 镜像，不要反过来

---

## 9. 参考入口（便于复查）

- `OpenSpec` 官方：`https://openspec.dev/`
- `writing-plans`（superpowers）：本地路径  
  `C:\Users\yoke\.codex\superpowers\skills\writing-plans\SKILL.md`
- `planning-with-files`：社区仓库（用于文件化执行记忆方案）

