# 旧 spec 到 code-spec 映射

用于后续逐项评审“保留/替换/删除”。

| 旧文件 (`docs/spec`) | 新位置 (`docs/code-spec`) | 说明 |
|---|---|---|
| `spec-overview.md` | `backend/spec-overview.md` + `frontend/spec-overview.md` | 拆成后端主基线 + 前端次基线 |
| `spec-api-config.md` | `backend/spec-api-config.md` + `frontend/spec-api-contract.md` | 后端可用路由与前端调用现状分离 |
| `spec-run-engine.md` | `backend/spec-run-engine.md` | 以后端状态机真实实现为准 |
| `spec-team-leader-layer.md` | `backend/spec-team-leader-layer.md` | 以 chat+issue 现状为准 |
| `spec-github-integration.md` | `backend/spec-github-integration.md` | 仅保留已支持事件与命令 |
| `spec-agent-drivers.md` | `backend/spec-agent-drivers.md` | 以 chat/ws/acp 事件驱动事实为准 |
| `spec-context-memory.md` | `backend/spec-context-memory.md` | 从“已实现”改为“现状边界+路线” |

后续评审建议：
1. 先审 `backend/*`，作为最终规范候选。
2. 再审 `frontend/*`，只吸收可保留交互，不反向约束后端。
