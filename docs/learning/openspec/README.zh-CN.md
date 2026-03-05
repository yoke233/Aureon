# OpenSpec 研究笔记

## OpenSpec 是什么

规格驱动开发框架（Fission AI），核心目的：AI 写代码之前，先与人类就"要构建什么"达成共识。

上游文档：`docs/vendor/openspec-upstream-docs/`

## 四大设计哲学

1. **流动不僵化** — 无阶段门禁，随时回退修改任何 artifact
2. **迭代不瀑布** — 边做边学
3. **简单不复杂** — 最小仪式
4. **棕地优先** — delta spec 描述变更，不重写整个规格

## 核心抽象

| 概念 | 说明 |
|------|------|
| **Spec** | 系统行为的 source of truth。Requirement + Scenario (Given/When/Then)，RFC 2119 关键字 |
| **Change** | 一个文件夹，包含 proposal / specs(delta) / design / tasks |
| **Delta Spec** | ADDED / MODIFIED / REMOVED 三段式增量规格 |
| **Schema** | YAML 定义 artifact DAG（依赖图），可自定义工作流 |
| **Archive** | 完成后 delta 合并进 specs，change 归档保留审计轨迹 |

## 目录结构

```
openspec/
├── specs/          # source of truth（当前系统行为）
├── changes/        # 进行中的变更
│   ├── add-dark-mode/
│   │   ├── proposal.md      # 为什么做、做什么
│   │   ├── specs/ui/spec.md  # delta spec
│   │   ├── design.md         # 怎么做
│   │   └── tasks.md          # 实施清单（checkbox）
│   └── archive/              # 已完成变更历史
├── schemas/        # 自定义工作流模式
└── config.yaml     # 项目配置（context + rules）
```

## Delta Spec 格式（关键创新）

```markdown
## ADDED Requirements
### Requirement: Two-Factor Authentication
The system MUST support TOTP-based 2FA.
#### Scenario: 2FA enrollment
- GIVEN a user without 2FA enabled
- WHEN the user enables 2FA in settings
- THEN a QR code is displayed

## MODIFIED Requirements
### Requirement: Session Timeout
The system MUST expire sessions after 15 minutes. (Previously: 30 minutes)

## REMOVED Requirements
### Requirement: Remember Me
(Deprecated in favor of 2FA)
```

归档时：ADDED 追加到主 spec，MODIFIED 替换，REMOVED 删除。

## OPSX 工作流

**默认快速路径：**
```
/opsx:propose → /opsx:apply → /opsx:archive
```

**扩展路径：**
```
/opsx:explore → /opsx:new → /opsx:continue|ff → /opsx:apply → /opsx:verify → /opsx:archive
```

**Artifact 依赖图（DAG）：**
```
         proposal
            │
      ┌─────┴─────┐
      ▼           ▼
    specs       design
      │           │
      └─────┬─────┘
            ▼
          tasks
            │
            ▼
        implement
```

## Schema 定制

```yaml
name: spec-driven
artifacts:
  - id: proposal
    generates: proposal.md
    requires: []
  - id: specs
    generates: specs/**/*.md
    requires: [proposal]
  - id: design
    generates: design.md
    requires: [proposal]
  - id: tasks
    generates: tasks.md
    requires: [specs, design]
apply:
  requires: [tasks]
  tracks: tasks.md
```

可自定义 schema（如 `research-first: research → proposal → tasks`）。

## 项目配置

```yaml
schema: spec-driven
context: |
  Tech stack: Go, React, PostgreSQL
  Testing: go test, Jest
rules:
  proposal:
    - Include rollback plan
  specs:
    - Use Given/When/Then format
```

- `context` 注入到所有 artifact 的 AI 提示中
- `rules` 只注入到匹配的 artifact

## Verify 三维验证

| 维度 | 验证内容 |
|------|---------|
| **Completeness** | 所有 tasks 完成、所有 requirements 有对应实现 |
| **Correctness** | 实现匹配 spec 意图、边界情况处理 |
| **Coherence** | 设计决策体现在代码中、模式一致 |

## 与 ai-workflow 的关系

详见 [集成分析](./openspec-gsd-integration-analysis.zh-CN.md)

核心映射：
- OpenSpec 的 `change` = 我们的 `Issue`
- OpenSpec 的 `specs/` = 项目 source of truth（存在 OpenViking `resources/{pid}/specs/`）
- OpenSpec 的 `schema` = 我们的 Issue template（standard / feature / epic 不同 artifact 组合）
- OpenSpec 的 `archive` = Issue done 后 delta 合并到项目 specs
