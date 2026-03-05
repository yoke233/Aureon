# ai-workflow × OpenViking 集成规范（压缩版）

最后更新：2026-03-05

> **完整规范**：`docs/spec/spec-context-memory.md`
> 本文为压缩版，保留核心决策和接口定义。

## 1. 背景与目标

Issue 流水线中，spec 要跨角色流转：

`TL 生成 -> Review 审核 -> Decompose 拆分 -> Coder 执行`

关键约束：

1. Coder 在独立 worktree，默认看不到主目录文件
2. 多 Coder 并行执行必须互相隔离
3. 不能把临时 context 污染到主分支
4. TL/Reviewer/Decomposer/Coder 都要可访问同一份 spec

结论：不自建 context 系统，直接对接 OpenViking。

## 2. 核心映射

OpenViking 的 `UserIdentifier(account_id, user_id, agent_id)` 直接映射：

```
account_id  =  "default"            部署实例
user_id     =  "system"             ai-workflow 系统用户
agent_id    =  角色名                coder / reviewer / tl / decomposer / aggregator
```

不同 agent_id → 自动物理隔离（agent/memories、agent/instructions、agent/skills）。
resources/ 是 account 级共享，所有角色可读。

## 3. 目录结构

使用 OpenViking 默认目录，只定义 `resources/` 下的项目组织方式。

```text
viking://resources/{pid}/specs/{iid}/       # Issue 规格（用 project ID）
viking://resources/{pid}/docs/              # 项目文档
viking://resources/shared/                  # 全局共享（best practice 推荐）

viking://agent/memories/cases/              # 角色记忆（按 agent_id 自动隔离）
viking://agent/memories/patterns/           #
viking://agent/instructions/                # 角色指令 / prompt template
viking://agent/skills/                      # 角色工具（MCP 自动转换）

viking://session/{sid}/                     # 会话级
```

不需要 `agent/{role}/` 自定义路径——`agent_id` 参数已处理隔离。

## 4. Store 接口

```go
type Store interface {
    Read(ctx context.Context, uri string) ([]byte, error)
    Write(ctx context.Context, uri string, content []byte) error
    List(ctx context.Context, uri string) ([]Entry, error)
    Remove(ctx context.Context, uri string) error

    Abstract(ctx context.Context, uri string) (string, error)
    Overview(ctx context.Context, uri string) (string, error)
    Find(ctx context.Context, query string, opts FindOpts) ([]Result, error)
    Search(ctx context.Context, query string, sessionID string, opts SearchOpts) ([]Result, error)

    AddResource(ctx context.Context, path, targetURI, reason string, wait bool) error
    Link(ctx context.Context, from string, to []string, reason string) error

    CreateSession(ctx context.Context, id string) (Session, error)
    Materialize(ctx context.Context, uri, targetDir string) error
}

type Session interface {
    AddMessage(role string, parts []Part) error
    Used(contexts []string) error
    Commit() (CommitResult, error)
}
```

## 5. MCP 工具

```
context_read(uri)              L2 全文
context_list(uri)              目录列表
context_overview(uri)          L1 概览
context_abstract(uri)          L0 摘要
context_find(query)            简单语义搜索
context_search(query)          带会话上下文的智能搜索
context_write(uri, content)    写入（仅授权角色）
context_link(from, to, reason) 建立关联（仅授权角色）
```

Agent 直接使用 `viking://` URI，系统在 MCP handler 中做白名单校验。

## 6. ACP 集成要点

```
runACPStage:
  1. client = ov.NewHTTPClient(agentID=role)    ← agent_id = 角色名
  2. session = client.Session(stageID)
  3. [可选] 加载 instructions → 注入 system prompt
  4. [coder] Materialize spec → worktree/.ai-workflow/context/
  5. 注册 scoped MCP tools
  6. 运行 ACP session（对话自动记录）
  7. session.Commit() → 自动提取 cases/patterns
```

## 7. 配置

```yaml
context:
  provider: openviking
  openviking:
    url: "http://localhost:1933"
    api_key: ""
  fallback: sqlite
```

## 8. 实现阶段

- **P0**：Go HTTP client、ACP 集成（agent_id=role）、MCP tools、Materialize
- **P1**：L0/L1 查询、语义搜索、Instructions 加载、Skills 注册、Relations
- **P2**：Session.Used()、启动时自动加载记忆、sqlite fallback、记忆质量监控
