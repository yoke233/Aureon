# Frontend API 契约现状（以调用代码为准）

状态：`观察`

## 1. 当前“可用主路径”

前端 `apiClient` 实际工作方式：
- 读接口：偏向 `/api/v2/*`
- 写接口：偏向 `/api/v1/*`

### 高频调用（当前有效）
- `GET /api/v2/issues`
- `GET /api/v2/issues/{id}`
- `GET /api/v2/runs`
- `GET /api/v2/runs/{id}`
- `GET /api/v2/runs/{id}/events`
- `GET /api/v2/workflow-profiles`
- `GET /api/v2/workflow-profiles/{type}`
- `POST /api/v1/projects/{projectID}/issues`
- `POST /api/v1/projects/{projectID}/issues/from-files`
- `POST /api/v1/projects/{projectID}/issues/{id}/review`
- `POST /api/v1/projects/{projectID}/issues/{id}/action`
- `POST /api/v1/projects/{projectID}/issues/{id}/auto-merge`
- `GET /api/v1/projects/{projectID}/chat*`
- `POST /api/v1/projects/{projectID}/chat*`
- `GET /api/v1/projects/{projectID}/repo/*`

## 2. 兼容别名（短期可保留）

`createPlan/listPlans/getPlanDag/...` 在前端是 issue 接口的别名封装。

建议：
- 迁移期保留别名能力。
- 新增页面与新代码统一使用 issue 命名。

## 3. 明确过时或不一致项

以下定义在前端存在，但后端当前未注册对应路由：
- `POST /projects/{projectID}/Runs`
- `POST /projects/{projectID}/Runs/{runID}/action`
- `GET /projects/{projectID}/Runs/{runID}/logs`
- `GET /projects/{projectID}/Runs/{runID}/checkpoints`

说明：
- 上述路径大小写与后端现状不匹配（`Runs`）。
- 在新规范中不应当成“可用 API”。

## 4. 规范建议

- 前端 API 契约应分为：
  - `implemented`（可调用）
  - `legacy_alias`（迁移保留）
  - `stale_unimplemented`（待清理）
