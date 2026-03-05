# Context & Memory（后端主链路现状）

状态：`观察`

## 1. 当前已实现范围

主链路后端中，尚未发现 OpenViking 的深度运行时集成（例如：
- `viking://` 资源读写主链路
- session commit 自动记忆提取
- context_* MCP tool 直接面向 OpenViking URI）

## 2. 当前可见资产

仓库中已有与 OpenViking 相关的辅助资产：
- `cmd/viking`（探测与规划辅助 CLI）
- `configs/openviking/*`（部署配置模板）

这些资产可作为后续集成起点，但不应被视为“已接入生产主链路”。

## 3. 规范结论

### 保留
- 把 OpenViking 视为后续演进方向（Roadmap），保留模板与辅助命令。

### 剔除
- 把 OpenViking 描述成“当前已全面落地”的规范描述。

## 4. 后续接入建议（仅方向）

如果后续要纳入主规范，应先满足以下最小落地条件：
1. 运行期有明确的 OpenViking client 初始化与故障处理。
2. 有可验证的 context read/write 接口闭环（非 demo）。
3. 有端到端测试覆盖（至少 chat -> context -> commit 的 smoke）。
