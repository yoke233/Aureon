# Get Shit Done (GSD) 研究笔记

## GSD 是什么

基于 Claude Code / Gemini CLI 的 AI 项目管理与执行框架。覆盖从需求到代码的全生命周期。

上游文档：`docs/vendor/get-shit-done-upstream-docs/`

## 核心设计理念

1. **Fresh context per agent** — 每个子 agent 获得干净的 200K 上下文窗口
2. **并行执行 + Wave 依赖** — 无依赖任务并行，有依赖按 wave 排序
3. **Plan verification loop** — 最多 3 轮自动验证计划质量
4. **极度偏向行动** — 做完再说，提供 `yolo` 模式自动审批

## 完整生命周期

```
/gsd:new-project → /gsd:discuss-phase → /gsd:plan-phase → /gsd:execute-phase
       │                                                         │
       ▼                                                         ▼
 PROJECT.md                                              Wave 并行执行
 REQUIREMENTS.md                                               │
 ROADMAP.md                                                    ▼
                                                      /gsd:verify-work
                                                               │
                                                               ▼
                                                    /gsd:audit-milestone
                                                               │
                                                               ▼
                                                   /gsd:complete-milestone
```

## 多 Agent 架构

| Agent 角色 | 职责 | quality | balanced | budget |
|-----------|------|---------|----------|--------|
| planner | 生成执行计划 | Opus | Opus | Sonnet |
| executor | 写代码 | Opus | Sonnet | Sonnet |
| phase-researcher | 阶段研究（×4 并行） | Opus | Sonnet | Haiku |
| verifier | 验证执行结果 | Sonnet | Sonnet | Haiku |
| plan-checker | 验证计划质量（3 轮） | Sonnet | Sonnet | Haiku |
| codebase-mapper | 分析现有代码库 | Sonnet | Haiku | Haiku |

## 关键机制

### Wave 执行模型

```
Wave 1 (无依赖): Executor A → commit    (并行)
                  Executor B → commit
Wave 2 (依赖 W1): Executor C → commit
Verifier: 验证代码匹配 phase 目标
```

### Nyquist 验证层

规划阶段就映射自动化测试覆盖到每个需求：
- 输出 `{phase}-VALIDATION.md`
- Plan checker 的第 8 个验证维度
- 确保执行前已有验证机制

### Plan-Check 循环

```
Planner → Plan → Checker → PASS? → Yes → 执行
                    │
                    No → 重做（最多 3 轮）
```

8 个检查维度，第 8 个是 Nyquist 合规性。

### Context Monitor（上下文窗口监控）

解决问题：Agent 不知道自己上下文快用完了。

```
Statusline Hook → /tmp/claude-ctx-{session_id}.json → Context Monitor Hook
                                                            │
                                                     additionalContext 注入警告
```

| 级别 | 剩余上下文 | Agent 行为 |
|------|----------|-----------|
| Normal | > 35% | 无警告 |
| WARNING | ≤ 35% | 收尾当前任务 |
| CRITICAL | ≤ 25% | 立即停止，保存状态 |

防抖：首次立即触发，后续间隔 5 次工具调用，级别升级绕过防抖。

### 状态持久化

```
.planning/
├── PROJECT.md          # 项目愿景（always loaded）
├── REQUIREMENTS.md     # 需求 + ID
├── ROADMAP.md          # 阶段分解 + 状态
├── STATE.md            # 决策、阻塞、会话记忆
├── config.json         # 工作流配置
├── phases/
│   └── XX-phase-name/
│       ├── XX-YY-PLAN.md       # 原子执行计划
│       ├── XX-YY-SUMMARY.md    # 执行结果
│       ├── CONTEXT.md          # 实现偏好
│       ├── RESEARCH.md         # 研究发现
│       └── VERIFICATION.md     # 验证结果
└── codebase/           # 棕地分析（map-codebase）
```

## 与 ai-workflow 的关系

详见 [集成分析](../openspec/openspec-gsd-integration-analysis.zh-CN.md)

核心映射：
- GSD 的 `phase` = 我们的 child issue（decompose 后的子任务）
- GSD 的 `wave execution` = 我们的子 issue 并行执行
- GSD 的 `plan-check loop` = 我们的 TwoPhaseReview
- GSD 的 `Nyquist validation` = 需求到验收测试的前置映射
- GSD 的 `model profile` = 角色级别模型选择
- GSD 的 `context monitor` = ACP session 上下文监控
