# Run Engine 规范（代码事实版）

状态：`保留`

## 1. Run 生命周期

Run 使用“状态 + 结论”双轴模型：

- `status`：
  - `queued`
  - `in_progress`
  - `action_required`
  - `completed`
- `conclusion`（仅 `completed` 时有效）：
  - `success`
  - `failure`
  - `timed_out`
  - `cancelled`

这个双轴模型可准确表达“已结束但失败类型不同”，建议保留。

## 2. 状态转换

核心转换（代码约束）：
- `queued -> in_progress`
- `in_progress -> completed | action_required`
- `action_required -> in_progress | completed`
- `completed -> in_progress`（允许重试）

## 3. Human Action

支持动作：
- `approve`
- `reject`
- `modify`
- `skip`
- `rerun`
- `change_role`
- `abort`
- `pause`
- `resume`

关键语义：
- `pause` 会将 run 置为 `action_required`。
- `resume/approve` 从 `action_required` 回到 `in_progress`。
- `abort` 收敛到 `completed + cancelled`。

## 4. Run 事件

执行主路径可观测事件：
- `run_done`
- `run_failed`
- `run_update`
- `run_started`
- `run_completed`
- `run_cancelled`
- `run_action_required`
- `run_resumed`
- `action_applied`

注：
- 调度恢复时，`completed + non-success conclusion` 统一映射为 `run_failed` 处理。

## 5. Auto-Merge

保留逻辑：
- 监听 `run_done` 触发自动合并流程。
- 失败阶段会发布 `run_failed` 并标注 phase。

当前实现限制（记录但不作为理想规范）：
- 变更包测试基线仍使用 `git diff main...HEAD`，尚未完全切换到 `project.default_branch` 驱动。
