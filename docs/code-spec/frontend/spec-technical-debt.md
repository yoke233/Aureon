# Frontend 技术债清单（用于后续重构）

状态：`观察`

## 1. 类型系统漂移

- `web/src/types/workflow.ts` 的 `RunStatus/WorkflowRunStatus` 仍是旧状态集合。
- 与后端 `core.RunStatus/core.RunConclusion` 不一致。

处理建议：
- 新增统一的前端适配层，把后端双轴状态映射到 UI 徽标，而非在 domain type 中伪造状态。

## 2. API Client 漂移

- `apiClient` 中含未落地接口（`/Runs` 大写路径相关）。
- 仍保留 `plan` 命名别名（迁移期可接受，长期应收敛）。

处理建议：
- 拆分 client 方法标签：`stable/legacy/stale`，并在 CI 做调用检查。

## 3. 规范归一建议

建议后续按两步走：
1. 先按本目录标注清理 `stale_unimplemented` 调用入口。
2. 再把 `legacy_alias` 收敛为 issue 命名的单一路径。

## 4. 不做的事（当前阶段）

- 不为了“看起来统一”而改写现有可用交互。
- 不把后端未实现接口提前写成前端必需契约。
