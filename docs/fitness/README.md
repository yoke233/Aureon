# Fitness Rulebook

> 目标：把当前项目里已经存在的测试、构建、桌面交付、线程运行时与审计约束，整理成一套可执行的质量门禁。

## 适用范围

- 当前先覆盖后端、前端、桌面打包三条主线
- 目标不是追求“大而全”，而是让关键变更有可执行证据
- 当前版本优先落地“能跑的规则书”，再逐步扩展
- 设计方式参考 `routa/docs/fitness`：规则文档 + frontmatter 指标 + 统一执行器 + 硬门禁
- 但不照搬其 OpenAPI 单一真相源，因为本项目当前契约分布在 handler、WebSocket 类型、spec 与测试之间

## 防御原则

- 证据优先：规则必须能指向具体命令或可验证文件
- 硬门禁优先：关键检查失败直接阻断，不参与加权
- 增量演进：先复用现有脚本，不重造测试基础设施
- 当前项目优先：先覆盖 `Go + React + Tauri` 主路径，而不是泛化抽象

## Quick Start

```powershell
# 只查看将执行什么
pwsh -NoProfile -File .\docs\fitness\scripts\fitness.ps1 -DryRun

# 实际执行
pwsh -NoProfile -File .\docs\fitness\scripts\fitness.ps1

# 失败时输出更多详情
pwsh -NoProfile -File .\docs\fitness\scripts\fitness.ps1 -VerboseOutput
```

## 当前维度

| 维度 | 权重 | 说明 | 证据文件 |
|------|------|------|----------|
| `testability` | 45 | 复用现有 PowerShell 测试脚本，保证后端/前端闭环 | [testability.md](D:/project/ai-workflow/docs/fitness/testability.md) |
| `code_quality` | 20 | 复用现有 `gofmt` / `go vet` / frontend lint 能力 | [code-quality.md](D:/project/ai-workflow/docs/fitness/code-quality.md) |
| `api_contract` | 15 | 以 REST/WS 类型定义、handler 注册、测试用例三者一致性作为当前契约证据 | [api-contract.md](D:/project/ai-workflow/docs/fitness/api-contract.md) |
| `runtime_contract` | 10 | 线程运行时、任务事件、前端订阅路径的运行时约束 | [runtime-contract.md](D:/project/ai-workflow/docs/fitness/runtime-contract.md) |
| `security` | 10 | 先收口当前已经存在的审计、redaction、admin scope 证据 | [security.md](D:/project/ai-workflow/docs/fitness/security.md) |
| `desktop_delivery` | 20 | 校验 Tauri Windows 打包链路是否具备稳定配置 | [desktop-delivery.md](D:/project/ai-workflow/docs/fitness/desktop-delivery.md) |

## Hard Gates

当前有这些硬门禁：

- 后端单测通过
- 前端单测通过
- 前端构建通过
- `gofmt` 检查通过
- `go vet` 通过
- 前端 lint 通过
- Thread WebSocket 关键后端测试通过
- audit HTTP 关键测试通过

## 当前规则

### 1. 测试规则

- 后端改动至少能通过后端单测
- 前端改动至少能通过前端单测与构建
- 无法执行的检查必须标记为 `BLOCKED`，不能默认为通过

### 2. 代码质量规则

- Go 代码格式必须保持 `gofmt` 干净
- 后端最少通过 `go vet`
- 前端最少通过现有 `eslint`

### 3. 契约规则

- 当前项目没有单一 OpenAPI 契约源时，必须至少满足三者一致：
  - handler / 事件分发已注册
  - 前端类型已声明
  - 后端或前端测试已覆盖关键路径
- 不允许只改 `spec` 或只改 `web/src/types/ws.ts` 而不补对应实现/测试

### 4. 运行时规则

- `thread.send`、`subscribe_thread`、`thread.task.*` 这类线程运行时协议，必须在后端事件处理、前端订阅和测试中同时存在证据
- 审计时间线相关路由必须有后端回归测试，不允许只有 handler 注册没有行为校验

### 5. 安全规则

- 当前先以“已有安全/审计能力不回退”为目标
- 至少保证 audit redaction 相关测试存在并可执行
- admin-only 的审计读取接口必须仍在 admin scope 保护下

### 6. 桌面交付规则

- Tauri Windows 产物目标必须是当前约定的安装器格式
- GitHub Actions 必须存在独立的 Windows 桌面打包工作流
- 先校验配置闭环，后续再把真机构建纳入硬门禁

## 评分模型

```text
Fitness = Σ(维度得分 × 权重) / Σ权重

< 80  : BLOCK
80-90 : WARN
>= 90 : PASS
```

硬门禁失败时直接退出，不看总分。

## 文件职责

- `README.md`：规则总览
- `testability.md`：测试维度与命令
- `code-quality.md`：代码质量维度与命令
- `api-contract.md`：当前 REST / WebSocket 契约证据
- `runtime-contract.md`：线程运行时证据
- `security.md`：审计 / redaction / admin scope 证据
- `desktop-delivery.md`：桌面交付维度与命令
- `scripts/fitness.ps1`：执行器

## 下一步建议

后续适合继续补的维度：

- `observability.md`
- `migration.md`
- `ci-release.md`
