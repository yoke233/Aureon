# GitHub 集成规范（代码事实版）

状态：`保留（核心） + 观察（能力边界）`

## 1. 当前支持的 webhook 事件类型

仅支持以下三类顶层事件：
- `issues`
- `issue_comment`
- `pull_request`

动作级约束：
- `issues` 仅处理 `opened`
- `issue_comment` 仅处理 `created`
- `pull_request` 仅处理 `closed`

## 2. Slash Command（issue_comment）

当前可识别命令：
- `/run [template]`
- `/approve`
- `/review`（映射为 approve）
- `/reject <stage> [reason]`
- `/status`
- `/abort`
- `/cancel`（映射为 abort）

权限：
- 按 `author_association + username allowlist` 判定。

## 3. 触发行为

- `issues.opened`：按 issue 内容触发 run。
- `/run`：触发 run；参数按 `template` 处理（不是 `workflow_profile` 强约束）。
- `/approve|/reject|/abort`：转换为 `RunAction` 交给执行器。
- `pull_request.closed`：触发 PR 生命周期回调。

## 4. 保留与剔除

### 保留
- webhook 签名校验 + 异步分发 + DLQ/replay 能力。
- slash command ACL。
- run 与 GitHub issue/PR 的弱耦合关联。

### 观察
- 文档常见的 `issues.labeled`、`pull_request_review.submitted` 不在当前支持列表。
- `/run <profile>` 语义与实现不一致（实现是 template）。

### 剔除
- 任何未落地的 webhook 事件声明，不写入主规范。
