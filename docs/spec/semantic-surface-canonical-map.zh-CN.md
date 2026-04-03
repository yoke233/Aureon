# Public Surface Canonical Map

> 状态：现行
>
> 最后按代码核对：2026-04-03
>
> 适用范围：本文件是当前仓库 public surface / canonical 语义分层的唯一权威定义。

## 1. 目的

本文件回答两件事：

1. 当前仓库真实公开了什么 surface
2. 这些 surface 在产品叙事上应归到哪一层

所有 `README`、索引文档、专题文档、执行计划都只能**引用**本文件，
不能再各自维护独立的 canonical 清单。

## 2. Evidence Level

### `code`

- 一手源码证据
- 例如：CLI 根命令、HTTP 路由、Web 路由、导航定义
- 判断“当前公开存在”时优先级最高

### `docs`

- 当前仓库内、带明确现状属性的文档证据
- 只能用于说明“当前文档如何叙述现状”
- 不能替代源码证明某个运行面存在与否

准入规则：

- 默认只接受带 `状态` 与 `最后按代码核对` 的非历史文档
- `docs/spec/README.md` 作为索引页可例外引用，但只能用于说明索引/维护约定，不用于替代一手代码事实
- `历史`、`草案`、`计划中`、`runbook` 默认不能作为 Matrix A 的 primary evidence

### `inference`

- 基于 `code/docs` 的归纳判断
- 只允许用于目标归类、升级条件、风险判断、治理规则
- 不得单独支撑 Matrix A 的“当前观察”

### 使用规则

1. Matrix A 只记录当前观察
2. Matrix A 每行必须至少有一个一手直接锚点
3. Matrix A 的 `Evidence Level` 只能取：
   - `code`
   - `docs`
   - `code+docs`
4. Matrix B 允许使用 `inference`
5. 若 canonical 分层与专题文档叙述冲突，以本文件为准

## 3. Canonical 分层

### 一级公开业务语义

- `WorkItem`
- `Thread`
- `Deliverable`
- `Project`
- `Chat`

### 二级公开业务语义

- `Inbox`
- `Review`

### 独立 Ops/Admin 轴

- `Monitoring`
- `Runtime`
- `Profile`

说明：

- 上述概念可以公开可见
- 但 `Ops/Admin` 轴不与业务主轴混写为同一层业务对象

### Public Capability, Not Primary Narrative

- `Requirement`
- `CEO submit`
- `Proposal`
- `Initiative`

说明：

- 这些能力可以有公开 API、深链、帮助文档或专题文档
- 但它们不是默认一级产品叙事对象

#### 对 `Proposal / Initiative` 的正式规则

允许公开出现的位置：

- API docs / HTTP 资源说明
- 高级帮助、专家文档、专题架构文档
- 详情深链、局部操作入口、关联对象上下文

禁止升格的位置：

- 根 `README.md` 的 Core Concepts 主表
- 全局顶层导航
- 默认 Landing / Home 主对象区
- CLI 根帮助中的一级产品对象集合

表达规则：

- API docs / Help 中可写为：
  - `public capability`
  - `advanced workflow object`
- 专题/架构/运行手册可以把 `Proposal / Initiative` 写成当前 workflow approval chain
- 这不构成 `public primary narrative` 升格

升格条件：

- 同时满足以下条件才可升级为一级或二级公开语义候选：
  - 至少两个 surface 出现稳定一级入口
  - 不再主要依附 `Thread / Project / WorkItem`
  - Architect 明确批准
  - 本文件先完成更新
  - README / spec / help 文案同步完成

## 4. Matrix A：Current Observed Surface

| 术语 | 当前观察 | Evidence Level | 一手直接锚点 |
| --- | --- | --- | --- |
| WorkItem | Web 顶层导航存在 `/work-items`；Web 路由存在列表、新建、详情 | `code` | [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):27; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):77; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):78; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):79 |
| Thread | Web 顶层导航存在 `/threads`；Web 路由存在列表、详情 | `code` | [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):29; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):81; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):82 |
| Deliverable | HTTP 已直接暴露 deliverable / artifact 读取接口；当前给定 `README.md` Core Concepts 主表未把其列为主概念项 | `code+docs` | [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):221; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):222; [README.md](D:\project\ai-workflow\README.md):123; [README.md](D:\project\ai-workflow\README.md):127; [README.md](D:\project\ai-workflow\README.md):133 |
| Project | README Core Concepts 当前把 `Project` 列为主概念；Web 顶层导航存在 `/projects`；HTTP 公开 project 资源；CLI 根命令当前未见一级 `project` | `code+docs` | [README.md](D:\project\ai-workflow\README.md):130; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):30; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):86; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):87; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):156; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):157; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):258; [cmd/ai-flow/root.go](D:\project\ai-workflow\cmd\ai-flow\root.go):57 |
| Chat | HTTP 当前直接暴露 `/chat*`；Web 顶层导航存在 `/chat`；CLI 根命令当前未见一级 `chat` | `code` | [chat.go](D:\project\ai-workflow\internal\adapters\http\chat.go):24; [chat.go](D:\project\ai-workflow\internal\adapters\http\chat.go):27; [chat.go](D:\project\ai-workflow\internal\adapters\http\chat.go):28; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):28; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):75; [cmd/ai-flow/root.go](D:\project\ai-workflow\cmd\ai-flow\root.go):57 |
| Monitoring | Web 顶层导航存在 `/monitoring`；Web 路由下含 dashboard / analytics / usage / inspections / scheduled-tasks；HTTP 直接暴露 analytics 与 inspections 公开接口；CLI 根命令当前未见一级 `monitoring` | `code` | [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):31; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):91; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):97; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):228; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):285; [handler.go](D:\project\ai-workflow\internal\adapters\http\handler.go):289; [cmd/ai-flow/root.go](D:\project\ai-workflow\cmd\ai-flow\root.go):57 |
| Runtime | CLI 一级命令存在 `runtime`；Web 顶层导航与 `/runtime` 路由存在 | `code` | [cmd/ai-flow/root.go](D:\project\ai-workflow\cmd\ai-flow\root.go):182; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):32; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):100 |
| Profile | CLI 一级命令存在 `profile`；HTTP 通过 `/agents/profiles/*` 公开；Web 当前不在全局顶层导航，而位于 `AgentsPage` 的 profiles 区块 | `code` | [cmd/ai-flow/root.go](D:\project\ai-workflow\cmd\ai-flow\root.go):229; [agents.go](D:\project\ai-workflow\internal\adapters\http\agents.go):40; [agents.go](D:\project\ai-workflow\internal\adapters\http\agents.go):44; [AgentsPage.tsx](D:\project\ai-workflow\web\src\pages\AgentsPage.tsx):310; [AgentsPage.tsx](D:\project\ai-workflow\web\src\pages\AgentsPage.tsx):325; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):25 |
| Proposal | HTTP 当前公开完整 proposal 路由族；当前顶层导航与 README Core Concepts 主表未见其升格为一级主对象 | `code+docs` | [proposal.go](D:\project\ai-workflow\internal\adapters\http\proposal.go):41; [proposal.go](D:\project\ai-workflow\internal\adapters\http\proposal.go):50; [README.md](D:\project\ai-workflow\README.md):123; [README.md](D:\project\ai-workflow\README.md):127; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):25 |
| Initiative | HTTP 当前公开完整 initiative 路由族；Web 当前存在 `Initiative` 详情深链；当前顶层导航与 README Core Concepts 主表未见其升格为一级主对象 | `code+docs` | [initiative.go](D:\project\ai-workflow\internal\adapters\http\initiative.go):43; [initiative.go](D:\project\ai-workflow\internal\adapters\http\initiative.go):52; [App.tsx](D:\project\ai-workflow\web\src\App.tsx):83; [README.md](D:\project\ai-workflow\README.md):123; [app-sidebar.tsx](D:\project\ai-workflow\web\src\components\app-sidebar.tsx):25 |

## 5. Matrix B：Target Canonical Mapping

| 术语 | 目标 canonical 归类 | 默认对外表达 | Evidence Level | Promotion Criteria | 依据 |
| --- | --- | --- | --- | --- | --- |
| WorkItem | 一级公开业务语义 | 默认工作主轴 | `inference` | 已满足，继续保持 | Matrix A |
| Thread | 一级公开业务语义 | 默认协作容器 | `inference` | 已满足，继续保持 | Matrix A |
| Deliverable | 一级公开业务语义 | 统一结果对象正式名称 | `inference` | 若未来进入根 README Core Concepts 主表，则视为 narrative 完整落位 | Matrix A |
| Project | 一级公开业务语义 | 组织容器 | `inference` | 不要求 CLI 一级命令；要求 README / Web / HTTP 叙事一致 | Matrix A |
| Chat | 一级公开业务语义 | 直接对话入口 | `inference` | 保持 Web / HTTP 一级入口即可 | Matrix A |
| Monitoring | 独立 Ops/Admin 轴 | 监控控制面 | `inference` | 除非成为普通用户第一工作对象，否则不升格为业务主轴 | Matrix A |
| Runtime | 独立 Ops/Admin 轴 | 运行时控制面 | `inference` | 维持控制面，不升格为业务主轴 | Matrix A |
| Profile | 独立 Ops/Admin 轴 | Runtime / Agents 语境下的配置对象 | `inference` | 不进入全局顶层导航；除非用户心智转为普遍业务对象 | Matrix A |
| Proposal | `public capability`, not `primary narrative` | API / 高级帮助 / 专题文档可公开；不进入默认主叙事 | `inference` | 仅当满足“公开入口至少两面 + 不再依附 Thread/Project/WorkItem + Architect 批准 + 本文件先更新”时可升格 | Matrix A + 第 3 节规则 |
| Initiative | `public capability`, not `primary narrative` | API / 深链 / 专题文档可公开；不进入默认主叙事 | `inference` | 同 Proposal | Matrix A + 第 3 节规则 |

