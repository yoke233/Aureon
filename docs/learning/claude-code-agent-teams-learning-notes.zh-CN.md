# Claude Code Agent Team 学习笔记

> 整理时间：2026-03-21
> 文档类型：学习记录（联网研究，非项目规范）
> 研究主题：Claude Code 中 `agent teams` / `subagents` 的协作逻辑

---

## Executive Summary

Claude Code 里其实有两套并行协作机制：`subagents` 和 `agent teams`。前者是“单 session 内的辅助工人”，适合隔离检索、审查、测试等高上下文消耗任务；后者是“多 session 组成的团队”，适合需要互相讨论、共享任务状态、跨模块并行推进的复杂工作。[1][2]

从机制上看，`agent team` 的核心不是“多开几个 agent”这么简单，而是 **team lead + teammates + shared task list + mailbox + local persistence** 这套编排层。每个 teammate 都是独立 Claude Code 实例，拥有自己的上下文窗口；它们会加载相同的项目上下文（`CLAUDE.md`、MCP、skills），但**不会继承 lead 的对话历史**，因此能避免主会话被大规模检索/实验污染。[1][3]

Claude Code 官方明确把这套能力定位为“高并行价值但高成本”的机制：只有在任务可拆分、成员之间需要互相校验或讨论时才值得用。对顺序任务、同文件编辑、依赖链很长的任务，单 session 或 `subagents` 往往更合适。[1][4]

---

## 一、先分清两类协作：Subagents vs Agent Teams

### 1.1 Subagents 是什么

`subagents` 是 Claude Code 的专用子代理。每个 subagent 运行在**独立上下文窗口**里，带有自己的 system prompt、工具权限和权限设置；Claude 会根据 subagent 的 `description` 自动决定是否委派任务。[2]

官方内置了几类典型 subagent：[2]

- `Explore`：只读、低延迟，适合代码搜索和理解。
- `Plan`：在 plan mode 下负责只读调研，避免主会话在规划阶段塞满上下文。
- `General-purpose`：同时能探索和动手改代码，适合多步任务。

它的本质是：**把“高噪音、高 token 消耗”的子任务放进独立上下文里，只把结果摘要带回主会话**。[2][4]

### 1.2 Agent teams 是什么

`agent teams` 是实验性功能。它不是单个主 agent 调几个 helper，而是由一个 **team lead** 协调多个 **独立 teammate session** 一起工作。[1]

官方对比里给出的差异非常关键：[1]

- `subagents`：各自有独立上下文，但只把结果汇报给主 agent。
- `agent teams`：各自有独立上下文，并且**队友之间可以直接通信**。
- `subagents`：由主 agent 集中调度。
- `agent teams`：通过**共享任务列表**做部分自协调。
- `subagents`：token 成本较低。
- `agent teams`：每个队友都是一个独立 Claude 实例，token 成本显著更高。

一句话总结：

- **只需要“帮我查/帮我审/帮我测”，用 subagents。**
- **需要“多角色并行、互相讨论、共享任务状态”，才用 agent teams。**

---

## 二、Claude Code 里 Agent Team 的底层逻辑

### 2.1 启动逻辑：先判断是否值得组队

官方文档说明，agent team 有两种启动方式：[1]

1. 用户明确要求创建 agent team。
2. Claude 判断当前任务适合并行，主动建议创建 team。

但无论哪种方式，**都需要用户确认**，Claude 不会未经批准直接拉起一个 team。[1]

这意味着 Claude Code 的第一层逻辑不是“默认多 agent”，而是：

1. 先判断任务是否具备并行价值。
2. 再判断是否需要跨 agent 通信。
3. 确认收益大于协调成本后，才进入 team 模式。

这和官方 best practices 完全一致：适合 team 的是研究、评审、竞争性假设调试、跨层协作；不适合的是顺序任务、同文件编辑、强依赖链任务。[1][4]

### 2.2 编排逻辑：Lead 负责拆解，Teammates 负责执行

一个 agent team 包含四个核心组件：[1]

- `Team lead`：主 Claude Code session，负责创建团队、拆任务、综合结果。
- `Teammates`：独立 Claude Code 实例，执行被分配的任务。
- `Task list`：共享任务列表，维护待办、进行中、完成态，以及依赖关系。
- `Mailbox`：代理间消息系统。

官方还给出了本地持久化位置：[1]

- Team 配置：`~/.claude/teams/{team-name}/config.json`
- Task 列表：`~/.claude/tasks/{team-name}/`

这说明 Claude Code 的 team 不是纯 prompt 幻觉式协作，而是有一层**本地状态存储**来承载团队拓扑和任务状态。

### 2.3 任务流转逻辑：共享任务表 + 自主抢单

任务列表里至少有三种状态：[1]

- `pending`
- `in progress`
- `completed`

任务还可以带依赖。若依赖未完成，任务即使是 `pending` 也不能被 claim。[1]

任务分配有两种模式：[1]

- `Lead assigns`：lead 显式指派任务给某个 teammate。
- `Self-claim`：teammate 做完后自己领取下一个未阻塞任务。

更关键的是，官方写明 **task claiming uses file locking**，用文件锁避免多个 teammate 同时抢到同一个任务的竞争条件。[1]  
这很像一个轻量本地任务队列，而不是只靠自然语言约定。

### 2.4 通信逻辑：自动投递，不靠 lead 轮询

agent teams 的通信不是“只有队长收集汇报”，而是支持真实的 agent-to-agent messaging：[1]

- `message`：发给一个指定 teammate
- `broadcast`：发给所有 teammate

文档还明确了三点机制：[1]

- 队友发出的消息会自动投递到收件人，lead 不需要轮询。
- teammate 空闲/结束时会自动通知 lead。
- 所有 agent 都能看到共享任务状态并领取可执行任务。

这就是 Claude Code 里“team”与“subagent”最本质的差异：  
**subagent 是“主从式返回结果”；agent team 是“有共享任务状态和直接通信能力的多 session 系统”。**

### 2.5 上下文逻辑：共享项目级上下文，不共享对话历史

这部分是最值得借鉴的设计。[1][2][3]

每个 teammate 启动时会加载与普通 Claude Code session 相同的项目上下文：

- `CLAUDE.md`
- MCP servers
- skills
- lead 提供的 spawn prompt

但它**不会继承 lead 的 conversation history**。[1]

这个设计的作用很明显：

- 保留项目级制度与知识：编码规范、命令、架构规则、工具接入仍然共享。
- 避免主会话污染：lead 前面读过的一堆日志、文件内容、失败尝试，不会全部复制给每个队友。
- 降低上下文耦合：队友只拿“稳定上下文 + 当前任务说明”，而不是整段历史。

Claude Code 的 memory 文档也验证了这个方向：跨 session 持久上下文主要依赖 `CLAUDE.md` 与 auto memory，而不是依赖会话历史本身。[3]

### 2.6 质量控制逻辑：先计划、再执行、再过钩子

Claude Code 给 team 提供了两层质量闸门：[1][2]

第一层是 **plan approval**。  
对于复杂或高风险任务，可以要求 teammate 先在只读 plan mode 下提交方案；lead 审批通过后，teammate 才能进入实现阶段。[1]

第二层是 **hooks**。  
agent teams 支持至少两种与完成态相关的 hook：[1]

- `TeammateIdle`：teammate 即将空闲时触发；若 hook 以 code 2 退出，可反馈并让其继续工作。
- `TaskCompleted`：任务即将被标记完成时触发；若 hook 以 code 2 退出，可阻止完成并反馈。

而 subagent 层面还能配 `PreToolUse` / `PostToolUse` 钩子，对具体工具调用做约束，比如限制 Bash 只能执行只读 SQL。[2]  
也就是说，Claude Code 的质量控制思路是：

1. 用 plan gate 控方向。
2. 用 tool hooks 控操作。
3. 用 completion hooks 控出关。

---

## 三、为什么 Claude Code 要把协作设计成这样

### 3.1 根因一：上下文窗口是最稀缺资源

Claude Code 官方 best practices 反复强调，**context window 是最重要的资源约束**。文件读取、命令输出、调试过程都会快速塞满上下文，性能也会下降。[4]

因此它引入 subagents 和 agent teams，本质上都是为了解决同一个问题：

- 把高噪音探索隔离在单独上下文中。
- 让主会话只保留关键摘要和决策。
- 在复杂任务中把不同工作流拆到多个独立窗口中并行推进。

这也和 Claude Code 的 agentic loop 一致：先收集上下文，再行动，再验证，并根据反馈不断循环。[5]

### 3.2 根因二：项目知识应该共享，但临时过程不该全部共享

Claude Code 对“稳定上下文”和“临时上下文”做了明显分层：[1][3]

- 稳定上下文：`CLAUDE.md`、rules、skills、MCP 配置。
- 临时上下文：某个 session 的对话历史、命令输出、实验过程。

agent team 复用前者，不复制后者。  
这比“把主会话全文转发给每个 worker”更便宜，也更稳定。

### 3.3 根因三：并行不是越多越好

官方建议多数场景从 **3-5 个 teammates** 起步；并强调 token 成本线性增长、协调开销增加、收益会递减。[1]

文档还建议：

- 每个 teammate 维持约 `5-6` 个任务较合适。[1]
- 新手先从研究/评审类任务开始，不要一上来就做并行写代码。[1]
- 避免多个 teammate 编辑同一个文件，否则容易互相覆盖。[1]

也就是说，Claude Code 并不把 agent team 设计成“越多越强”，而是强调**任务拆分质量 > 队伍规模**。

---

## 四、从官方文档抽象出的协作状态机

可以把 Claude Code 的 agent team 逻辑抽象成下面这套状态机：

1. **Need Parallelism?**  
   判断任务是否值得并行，以及是否需要 agent 间通信。[1][4]
2. **Approval**  
   用户确认创建 team；若高风险，可要求 plan approval。[1]
3. **Spawn**  
   lead 创建 teammates；每个 teammate 加载 `CLAUDE.md`/MCP/skills，并接收 spawn prompt，但不继承 lead 历史。[1][3]
4. **Decompose**  
   lead 生成共享任务列表，标注依赖关系。[1]
5. **Claim / Execute**  
   teammates 被指派或自主 claim 任务；claim 过程由 file locking 防并发冲突。[1]
6. **Communicate**  
   teammates 通过 mailbox `message` / `broadcast` 交换信息；idle/completion 自动通知 lead。[1]
7. **Gate**  
   通过 plan approval、hooks、权限控制检查结果是否达标。[1][2]
8. **Synthesize**  
   lead 汇总各 agent 结果，必要时重分配任务、补充分工。[1]
9. **Shutdown / Cleanup**  
   先逐个关闭 teammates，再由 lead 执行 cleanup，移除共享团队资源。[1]

如果只保留一句最核心的话：  
**Claude Code 的 agent team 不是“多 agent 推理技巧”，而是“建立在本地任务状态、显式角色分工、上下文隔离和质量闸门上的轻量多会话编排系统”。**

---

## 五、Anthropic 官方实践里能看到的补充信号

Anthropic 官方 PDF《How Anthropic teams use Claude Code》虽然不专讲 `agent teams` 功能，但给出了几条很强的设计信号：[6]

- 他们会在不同仓库/项目里并行打开多个 Claude Code 实例，做长期并行任务管理。[6]
- 他们强调让 Claude 自己运行测试、lint、构建，形成自验证 loop。[6]
- 他们把 `Claude.md`（现文档体系里对应 `CLAUDE.md`）当作持续改进的操作说明书，在每次实践后迭代更新。[6]

这说明 agent team 并不是一个孤立功能，而是 Anthropic 整体工作流的一部分：  
**文档记忆、上下文隔离、自验证闭环、并行实例管理** 是同一套方法论的不同侧面。

---

## 六、局限与风险

截至 **2026-03-21**，官方仍明确将 agent teams 标记为 **experimental**，并列出若干限制：[1]

- in-process teammate 不支持通过 `/resume` / `/rewind` 恢复。
- task 状态有时会滞后，导致依赖任务被错误阻塞。
- shutdown 可能较慢，因为 teammate 会等待当前请求或工具调用结束。
- 一个 lead session 同一时间只能管理一个 team。
- 不支持 nested teams，teammate 不能再生成自己的 team。
- teammate 的权限模式在 spawn 时继承 lead，不能在 spawn 前按 teammate 粒度细配。
- split-pane 模式依赖 `tmux` 或 iTerm2；并非所有终端都支持。[1]

这些限制说明：  
Claude Code 目前的 team 机制已经可用，但仍然更适合**高价值的探索/评审/分层协作任务**，不适合无脑替代单 agent 流程。

---

## 七、对我们做 Agent/Harness 的直接启发

如果把 Claude Code 的做法抽象成可复用设计，最值得抄的不是 UI，而是下面这几条：

### 7.1 把“单会话子代理”和“多会话团队”分成两层能力

不要把所有并行都塞进一个工具里。  
`subagent` 解决“隔离上下文、减少主线程污染”；`agent team` 解决“多角色协作、共享任务状态、互相辩论/校验”。[1][2]

### 7.2 共享的是项目规范，不是完整聊天记录

让 worker 继承 `CLAUDE.md`/skills/MCP/规则，但不要默认继承 orchestrator 的全文历史。  
这样更稳定，也更省 token。[1][3]

### 7.3 任务队列必须是显式状态机

至少要有：

- 任务状态
- 依赖关系
- claim 机制
- 冲突控制
- cleanup 逻辑

Claude Code 用文件锁和本地任务目录做了一个很轻量但足够可靠的实现。[1]

### 7.4 高风险任务一定要有“计划闸门”和“完成闸门”

先 plan approval，再允许执行；完成前再跑 hooks / tests / policy checks。  
否则多 agent 只会更快地产生错误。[1][2][4]

### 7.5 并行前先问一句：真的值得吗

Claude Code 官方反复强调，agent teams 成本高、协调重、收益递减。  
这点很重要：**不是能并行就应该并行，而是要先确认任务天然可拆分。**[1][4]

---

## Areas of Consensus

- Claude Code 的并行协作是明确分层的：`subagents` 负责单 session 内的上下文隔离，`agent teams` 负责多 session 协作。[1][2]
- 上下文窗口管理是设计核心，几乎所有协作机制都围绕“减少主上下文污染”展开。[2][4][5]
- `CLAUDE.md` / memory / skills / MCP 共同构成项目级稳定上下文，是团队协作的一致性来源。[1][3]
- 质量保障依赖显式闸门，而不是单纯相信模型：计划审批、工具钩子、任务完成钩子、自验证测试缺一不可。[1][2][4]

## Areas of Debate / Uncertainty

- 官方没有公开更底层的调度实现细节，例如 lead 如何决定拆任务粒度、何时更偏向 `subagents` 还是 `agent teams`，这些更像产品层启发式而非完整公开算法。[1][2][5]
- `agent teams` 仍处于实验阶段，文档列出的恢复、任务状态、关闭流程问题意味着它的内部协议和运行时模型未来仍可能变化。[1]
- Anthropic 公开材料更多说明“怎么用”和“适合什么场景”，而不是完整公开其内部 orchestration runtime 源码，因此对某些实现细节只能做基于文档的合理推断。[1][6]

---

## Sources

[1] Claude Code Docs, **Orchestrate teams of Claude Code sessions**. 官方产品文档，直接定义 `agent teams` 的架构、状态、权限、上下文与限制。  
https://code.claude.com/docs/en/agent-teams

[2] Claude Code Docs, **Create custom subagents**. 官方产品文档，解释 `subagents` 的独立上下文、自动委派、内置代理与 hooks。  
https://code.claude.com/docs/en/sub-agents

[3] Claude Code Docs, **How Claude remembers your project**. 官方产品文档，解释 `CLAUDE.md`、auto memory 与项目级持久上下文。  
https://code.claude.com/docs/en/memory

[4] Claude Code Docs, **Best Practices for Claude Code**. 官方最佳实践文档，重点说明 context 管理、subagents 调研、并行 session 与 fan-out 模式。  
https://code.claude.com/docs/en/best-practices

[5] Claude Code Docs, **How Claude Code works**. 官方架构说明，定义 Claude Code 的 agentic loop 与工具/上下文管理定位。  
https://code.claude.com/docs/en/how-claude-code-works

[6] Anthropic PDF, **How Anthropic teams use Claude Code**. 官方经验材料，提供多实例并行、文档记忆、自验证 loop 等实际使用信号。  
https://www-cdn.anthropic.com/58284b19e702b49db9302d5b6f135ad8871e7658.pdf

---

## Gaps and Further Research

- 如果后续要继续深挖，建议补看 `hooks`、`permissions`、`checkpointing`、`Git worktrees` 相关文档，进一步理解 Claude Code 如何把多 agent 协作与代码安全/回滚机制串起来。[1][2][4]
- 如果目标是把这套逻辑迁移到本仓库的 agent harness，可以继续拆成三个主题研究：
  1. 任务状态存储模型
  2. 跨 agent 消息协议
  3. plan gate / quality gate / cleanup 的工程落地

