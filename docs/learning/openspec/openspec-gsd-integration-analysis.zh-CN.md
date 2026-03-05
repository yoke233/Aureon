# OpenSpec + GSD 与 ai-workflow 集成分析

## 三者定位（不同层次）

| 层次 | 工具 | 解决什么 |
|------|------|---------|
| 规格管理 | OpenSpec | WHAT — 定义做什么，delta 变更，artifact 生命周期 |
| 执行管理 | GSD | HOW — 并行执行，wave 调度，验证闭环 |
| 流水线编排 | ai-workflow | WHO/WHEN — 角色分配，状态机，事件驱动 |
| 上下文存储 | OpenViking | WHERE — spec 存取，记忆，语义搜索 |

我们不引入 OpenSpec CLI 或 GSD 本身，而是**借鉴其设计理念和数据格式**。

## 从 OpenSpec 借鉴

### 1. Delta Spec 格式

TL 生成的 spec 采用 ADDED/MODIFIED/REMOVED 结构：

```markdown
## ADDED Requirements
### Requirement: Payment Retry
The system MUST retry failed payments up to 3 times.
#### Scenario: Transient failure
- GIVEN a payment that failed due to network timeout
- WHEN the retry interval elapses
- THEN the system retries the payment

## MODIFIED Requirements
### Requirement: Order Timeout
The system MUST cancel unpaid orders after 15 minutes. (Previously: 30 minutes)
```

好处：
- Coder 精确知道要改什么（不是"读完整 spec 自己猜"）
- Done 后 delta 合并回项目 specs，系统文档持续积累
- Reviewer 只看变更部分，效率高

### 2. Issue-Artifact 关联（工具无关）

我们**不绑定 OpenSpec 的 schema 协议**。系统只记录一件事：issue 关联了哪些文件。

```
Issue #123 → files: [proposal.md, security-review.md, design.md, tasks.md, ...]
```

不管用 OpenSpec、自定义工具、还是未来任何新工具生成了多少个文件、什么类型——
我们只存 `viking://resources/{pid}/specs/{iid}/` 下的文件列表。

OpenSpec 的 schema（DAG、artifact 依赖、template）是它自己的事。
我们的系统不需要理解这些协议，只需要知道 TL 往这个目录里写了什么文件。

好处：
- 换 spec 工具不影响系统核心
- 不同项目可以用不同工具
- 系统保持简单——只做编排

### 3. Verify 三维验证

Issue done 之前，独立 agent（aggregator）验证：
- **Completeness** — tasks 全部完成，requirements 全部有对应代码
- **Correctness** — 实现匹配 spec 意图
- **Coherence** — 设计决策体现在代码中

### 4. Archive = 知识积累（Aggregator Agent）

Issue done 时，**aggregator 通过 ACP session** 合并 delta 到项目 specs。
这不是机械操作——需要大模型理解 coder 实际做了什么（vs 原始 spec 的差异）。

```
EventIssueDone → Aggregator ACP session:
  1. 读现有项目 specs
  2. 读本次 issue 原始 requirement
  3. 读 PR diff（coder 实际变更）
  4. 理解差异，生成正确的 delta（ADDED/MODIFIED/REMOVED）
  5. 更新主 spec
  6. 归档原始 spec
  7. session.commit() → 提取 aggregator 经验记忆
```

存储路径：
```
viking://resources/{pid}/specs/{iid}/   → 活跃期的 issue spec
viking://resources/{pid}/specs/auth/    → 合并后的 source of truth
viking://resources/{pid}/archive/{iid}/ → 归档保留审计轨迹
```

## 从 GSD 借鉴

### 1. Wave 执行模型

子 issue 之间有依赖关系时，按 wave 分组并行：

```
Epic Issue #100
  ├── Child #101 (无依赖)  ── Wave 1 ──┐
  ├── Child #102 (无依赖)  ── Wave 1 ──┤ 并行
  ├── Child #103 (依赖 #101) ── Wave 2 ─┤ 等 Wave 1 完成
  └── Child #104 (依赖 #102,#103) ── Wave 3
```

已有基础：DecomposeSpec 的 `DependsOn` 字段。
需要做：ChildCompletionHandler 中加 wave 感知的调度逻辑。

### 2. Model Profile（角色模型分级）

```yaml
model_profiles:
  quality:
    team-leader: opus
    reviewer: opus
    coder: opus
  balanced:          # 默认
    team-leader: opus
    reviewer: sonnet
    coder: sonnet
  budget:
    team-leader: sonnet
    reviewer: haiku
    coder: sonnet
```

决策密集角色（TL）用高端模型，执行角色（coder）用性价比模型。

### 3. Nyquist 验证（前置测试映射）

在 decompose 阶段就定义每个子 issue 的验收命令：

```yaml
child_issues:
  - title: "Add payment retry"
    verify_command: "go test ./internal/payments/... -run TestRetry"
  - title: "Update order timeout"
    verify_command: "go test ./internal/orders/... -run TestTimeout"
```

Coder 完成后自动运行验收命令，而不是等到人工 review。

### 4. Context Monitor

ACP session 长时间运行时监控上下文消耗：
- 35% 剩余：收尾当前任务
- 25% 剩余：保存状态，暂停 session
- 通过 `stageEventBridge` 上报消耗指标

### 5. Plan-Check 循环

Spec 质量门禁，reviewer 最多 3 轮审查：

```
TL 生成 spec → Reviewer 审核 → PASS? → Yes → 下一步
                    │
                    No → TL 修改（回到同一 session 保留上下文）
                    │
                    3 次失败 → 升级人工介入
```

已有基础：TwoPhaseReview + reject 回到 draft 保留 sessionID。

## 存储映射（OpenViking）

```
viking://resources/{pid}/
├── specs/                    # 项目 source of truth（OpenSpec 的 specs/）
│   ├── auth/spec.md          # 认证行为规格
│   └── payments/spec.md      # 支付行为规格
├── changes/{iid}/            # 活跃 issue 的 artifacts（OpenSpec 的 changes/）
│   ├── proposal.md
│   ├── specs/auth/spec.md    # delta spec
│   ├── design.md
│   └── tasks.md
├── archive/{iid}/            # 已完成 issue 归档
└── templates/                # schema 模板
    ├── standard.yaml
    ├── feature.yaml
    └── epic.yaml

viking://resources/shared/
└── schemas/                  # 全局共享的 schema 定义
```

## 实现优先级

### P0: 低成本高价值

1. **Delta spec 格式规范** — 定义 TL 输出格式，coder 解析格式
2. **Schema-per-template 配置** — 在 config.yaml 中定义每种 template 的 artifact 列表
3. **Model profile 配置** — 角色 → 模型的映射表

### P1: 中等投入

4. **Verify 步骤** — aggregator 角色执行三维验证
5. **Archive 合并** — issue done 时 delta 合入项目 specs
6. **Wave 执行** — 子 issue 依赖分析 + 分组并行

### P2: 长期建设

7. **Nyquist 验证** — decompose 时定义验收命令
8. **Context monitor** — ACP session 上下文监控
9. **Plan-check 循环增强** — 自动重试 + 次数限制 + 升级策略
