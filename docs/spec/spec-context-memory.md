# Context & Memory 规范（OpenViking 集成）

## 目标

定义 ai-workflow 的上下文存储与角色记忆系统，基于 OpenViking 实现。
解决三个核心问题：

1. Spec 文件跨角色、跨 worktree 流转
2. 角色记忆的积累与隔离
3. Agent 权限边界

## 非目标

- 不自建 L0/L1/L2 摘要生成（交给 OpenViking）
- 不自建语义检索引擎（交给 OpenViking）
- 不自建记忆提取逻辑（交给 OpenViking session.commit）
- 不自定义目录骨架（使用 OpenViking 默认目录结构）

## 核心映射：OpenViking 三元组 = 我们的角色模型

OpenViking 的 `UserIdentifier(account_id, user_id, agent_id)` 直接映射到我们的系统：

```
account_id  =  "default"          部署实例（单租户）
user_id     =  "system"           ai-workflow 系统用户
agent_id    =  角色名              coder / reviewer / tl / decomposer / aggregator
```

OpenViking 用 `md5(user_id + agent_id)[:12]` 生成 `agent_space`，
**不同 agent_id 的记忆、指令、技能自动物理隔离**。

我们不需要自定义 URI 结构——直接使用 OpenViking 的默认目录。

## 控制模型：三分架构

```
Scope（作用域）   → 系统控制，ACP 初始化时通过 agent_id 确定
Query（查询）     → Agent 主动，通过 scoped MCP tools + target_uri 搜索
Memory（记忆）    → 系统自动，ACP session 结束后 commit，OpenViking 自动提取
```

## 目录结构（OpenViking 默认 + 资源组织约定）

OpenViking 默认提供以下目录结构，我们只需定义 `resources/` 下的项目组织方式。
其余目录由 OpenViking 自动初始化和管理。

```
viking://
├── resources/                          # account 级共享，所有角色可读
│   ├── {pid}/                          # 项目（用 ID，不用名称）
│   │   ├── docs/                       #   项目文档（架构、约定）
│   │   ├── specs/                      #   Issue 规格
│   │   │   └── {iid}/                  #     单个 Issue
│   │   │       ├── requirement.md
│   │   │       └── api-design.md
│   │   └── references/                 #   参考资料
│   └── shared/                         # 全局共享（编码规范、模板）
│       └── common-docs/
│
├── agent/                              # 按 agent_id 自动隔离
│   ├── memories/                       #   session.commit() 自动提取
│   │   ├── cases/                      #     问题-方案对（不可变）
│   │   └── patterns/                   #     行为模式（可追加）
│   ├── instructions/                   #   角色指令 / prompt template
│   └── skills/                         #   角色可用工具（MCP 自动转换）
│
├── user/                               # 按 user_id 隔离
│   └── memories/
│       ├── .overview.md                #   用户画像（profile）
│       ├── preferences/                #   偏好
│       ├── entities/                   #   实体（人、项目）
│       └── events/                     #   事件
│
└── session/{sid}/                      # 会话级
    ├── messages.jsonl
    ├── .abstract.md                    #   L0 摘要（commit 时生成）
    ├── .overview.md                    #   L1 概览
    ├── tools/                          #   工具调用记录
    └── history/                        #   归档历史
```

### 为什么不需要自定义 agent 子目录

之前设计的 `viking://agent/{role}/p/{pid}/memories/` 是多余的：

1. **角色隔离**：`agent_id = role` → OpenViking 自动按 agent_space 物理隔离
2. **项目记忆**：不需要按项目分子目录。记忆内容自然包含项目上下文，
   语义搜索 + target_uri 定向即可召回项目相关记忆
3. **跨项目学习**：扁平记忆让角色的通用经验自动跨项目可用

## L0/L1/L2 分层加载

每个目录节点由 OpenViking 自动生成 `.abstract.md`（L0, ~100 tokens）
和 `.overview.md`（L1, ~1k tokens）。子节点的 L0 聚合成父节点的 L1。

### 角色实际查询路径

```
TL 写 spec：
  1. overview("resources/{pid}/docs/")      → L1 了解项目架构
  2. find("编码规范", target_uri="resources/shared/")  → 搜索全局标准
  3. add_resource(spec, target="resources/{pid}/specs/{iid}/")
  4. wait_processed()                       → 等 L0/L1 生成完毕

Reviewer 审 spec：
  1. abstract("resources/{pid}/specs/")     → L0 扫描所有需求
  2. overview("resources/{pid}/specs/{iid}/") → L1 看需求概览
  3. read("resources/{pid}/specs/{iid}/requirement.md") → L2 读全文
  4. search("类似需求的审查经验", session=s) → 带会话上下文的智能搜索

Coder 执行：
  1. find("相关经验", target_uri="agent/memories/cases/") → 搜索历史案例
  2. Materialize("resources/{pid}/specs/{iid}/") → 落盘到 worktree
  3. 本地读文件，开干

Decomposer 拆分：
  1. read("resources/{pid}/specs/{iid}/requirement.md") → 读完整 spec
  2. overview("resources/{pid}/docs/")      → 了解项目架构
  3. 拆分后 add_resource 子 issue specs
```

## 角色权限矩阵

权限由系统在 ACP 初始化时通过 scoped MCP tools 控制。

| 角色 | Read resources | Read agent memory | Write resources | Write instructions |
|------|---------------|-------------------|-----------------|-------------------|
| team-leader | `{pid}/docs/`, `shared/` | 自己的 | `{pid}/specs/`, `{pid}/docs/` | 自己的 |
| reviewer | `{pid}/specs/{iid}/`, `{pid}/docs/` | 自己的 | — | — |
| decomposer | `{pid}/specs/{iid}/`, `{pid}/docs/` | 自己的 | `{pid}/specs/`（子 issue） | — |
| coder | `{pid}/specs/{iid}/` | 自己的 | — | — |
| aggregator | `{pid}/specs/`, `{pid}/archive/` | 自己的 | `{pid}/specs/`（更新主 spec）, `{pid}/archive/`（归档） | — |

说明：
- 每个角色的 agent memory/instructions/skills 由 agent_id 自动隔离，不需要额外控制
- resources 是 account 级共享，读权限由 MCP tools 的 target_uri 参数限定
- coder 不写 resources，代码变更走 git
- aggregator 负责 issue 完成后的 spec 归档：读 PR diff + 现有 specs → 生成 delta → 更新主 spec → 归档原始 spec

## OpenViking 原生能力利用

### 1. Instructions — 角色 Prompt Template

每个角色的系统指令存储在 `viking://agent/instructions/`（按 agent_id 自动隔离）：

```
# 系统初始化时写入
client = ov.NewHTTPClient(agentID="coder")
client.Write("viking://agent/instructions/system-prompt.md", coderPrompt)
client.Write("viking://agent/instructions/coding-rules.md", codingRules)

# ACP 启动时读取
instructions = client.Find("", target_uri="viking://agent/instructions/")
```

格式为 Markdown + YAML Frontmatter：

```markdown
---
name: coder-system-prompt
tags: [system, prompt]
---

# Coder 角色指令

你是一个代码编写 Agent，负责根据 spec 实现代码...
```

### 2. Skills — MCP 工具注册

OpenViking 自动将 MCP tool 的 JSON Schema 转换为技能格式：

```python
# 注册 MCP 工具为 OpenViking Skill（可被语义搜索发现）
client.AddSkill({
    "name": "query_issues",
    "description": "查询项目中的 issues",
    "inputSchema": { ... }
})

# Agent 可通过搜索发现相关工具
results = client.Find("查询需求", target_uri="viking://agent/skills/")
```

### 3. Relations — Spec 与文档的关联

spec 写入后建立与项目文档的关联，提升搜索相关性：

```python
client.Link(
    from_uri="viking://resources/{pid}/specs/{iid}/",
    to_uris=["viking://resources/{pid}/docs/architecture.md"],
    reason="该需求涉及架构变更"
)
```

### 4. Session.Used — 上下文使用追踪

```python
# 标记 Agent 实际使用了哪些上下文
session.Used(contexts=["viking://resources/{pid}/specs/{iid}/requirement.md"])

# 标记使用了哪个工具
session.Used(skill={
    "uri": "viking://agent/skills/query_issues/",
    "input": "...",
    "output": "...",
    "success": true
})
```

### 5. 事务保护 — 多 Agent 并发安全

OpenViking 的 TransactionManager 自动保护写操作：
- 所有写操作自动加路径锁（`.path.ovlock`）
- 多 Agent 同时写不同路径不会冲突
- 写同一路径时自动排队，超时释放
- 我们不需要额外实现并发控制

### 6. find() vs search() 选型

| 场景 | 用哪个 | 原因 |
|------|--------|------|
| 角色启动加载指令 | `find()` | 精确查询，无需会话上下文 |
| 角色搜索历史经验 | `search()` | 带会话上下文，意图分析更准 |
| Reviewer 搜索相似 spec | `search()` | 利用当前审查上下文扩展查询 |
| Materialize 读取 spec | `read()` | 精确路径，不需要搜索 |

## MCP 工具（Agent 侧接口）

Agent 看到的 MCP 工具。系统通过 target_uri 参数控制搜索范围。

```
context_read(uri)              读取文件内容（L2）
context_list(uri)              列出目录内容
context_overview(uri)          读取 L1 概览
context_abstract(uri)          读取 L0 摘要
context_find(query)            简单语义搜索（在授权范围内）
context_search(query)          带会话上下文的智能搜索
context_write(uri, content)    写入文件（仅授权角色）
context_link(from, to, reason) 建立资源关联（仅授权角色）
```

### URI 处理

Agent 直接使用 `viking://` URI（系统在 MCP handler 中校验权限）：

```
Agent 调用: context_read("viking://resources/42/specs/101/requirement.md")
系统校验: URI 前缀在 ReadPrefixes 中 → 放行
```

决策变更：之前设计的"相对路径 + 系统加前缀"过于复杂。
Agent 直接用完整 URI 更直观，权限校验在 MCP handler 中做白名单即可。

## Go 接口定义

### ContextStore

系统与 OpenViking 交互的核心接口。P0 只实现 OpenViking 后端，P2 加 sqlite fallback。

```go
// ContextStore 是 OpenViking 的 Go 客户端抽象。
// 所有 URI 参数使用 viking:// 格式。
type ContextStore interface {
    // 基础 CRUD
    Read(ctx context.Context, uri string) ([]byte, error)
    Write(ctx context.Context, uri string, content []byte) error
    List(ctx context.Context, uri string) ([]Entry, error)
    Remove(ctx context.Context, uri string) error

    // 分层查询（L0/L1）
    Abstract(ctx context.Context, uri string) (string, error)
    Overview(ctx context.Context, uri string) (string, error)

    // 语义搜索
    Find(ctx context.Context, query string, opts FindOpts) ([]Result, error)
    Search(ctx context.Context, query string, sessionID string, opts SearchOpts) ([]Result, error)

    // 资源管理
    AddResource(ctx context.Context, path, targetURI, reason string, wait bool) error
    Link(ctx context.Context, from string, to []string, reason string) error

    // 会话
    CreateSession(ctx context.Context, id string) (ContextSession, error)

    // 物化（coder 专用）
    Materialize(ctx context.Context, uri, targetDir string) error
}

type Entry struct {
    URI   string
    Name  string
    IsDir bool
}

type Result struct {
    URI     string
    Score   float64
    Content string // 摘要或全文
}

type FindOpts struct {
    TargetURI string // 限定搜索范围
    Limit     int
}

type SearchOpts struct {
    TargetURI string
    Limit     int
}
```

### ContextSession

ACP session 期间与 OpenViking 交互的会话接口。

```go
type ContextSession interface {
    // 记录对话消息（ACP session 中自动调用）
    AddMessage(role string, parts []MessagePart) error

    // 标记实际使用了哪些上下文（提升搜索权重）
    Used(contexts []string) error

    // 提交：归档消息 + 生成摘要 + 提取记忆
    Commit() (CommitResult, error)
}

type MessagePart struct {
    Type    string // "text" | "context" | "tool"
    Content string
    URI     string // context 类型时填写
}

type CommitResult struct {
    Status            string // "committed"
    MemoriesExtracted int
    Archived          bool
}
```

### ContextToolsFactory

为每个 ACP stage 创建 scoped MCP tools 的工厂。

```go
type ContextToolsFactory interface {
    // 根据角色权限创建 MCP tools
    // readPrefixes/writePrefixes 控制 URI 白名单
    NewScopedTools(
        store ContextStore,
        session ContextSession,
        readPrefixes []string,
        writePrefixes []string,
    ) []MCPTool
}
```

## ACP 集成流程

```
runACPStage(role="coder", projectID=42, issueID="101")
│
├── 1. 创建 OpenViking client
│      client = ov.NewHTTPClient(
│          url:     cfg.OpenViking.URL,
│          apiKey:  cfg.OpenViking.APIKey,
│          agentID: role,                    // ← 关键：agent_id = 角色名
│      )
│
├── 2. 创建 OpenViking session
│      session = client.Session("acp-stage-{stageID}")
│
├── 3. [可选] 加载角色指令
│      instructions = client.Find("", target_uri="viking://agent/instructions/")
│      // 注入到 ACP system prompt
│
├── 4. [coder only] Materialize spec 到 worktree
│      specs = client.List("viking://resources/42/specs/101/")
│      for each spec:
│          content = client.Read(spec.URI)
│          write to worktree/.ai-workflow/context/
│
├── 5. 注册 scoped MCP tools
│      tools = newScopedContextTools(client, session, allowedPrefixes)
│
├── 6. 运行 ACP session
│      // Agent 通过 MCP tools 与 OpenViking 交互
│      // session.AddMessage() 自动记录对话
│      // session.Used() 追踪上下文使用
│
└── 7. session 结束 → commit
       result = session.Commit()
       // OpenViking 自动：
       //   - 归档消息到 history/
       //   - 生成会话 L0/L1 摘要
       //   - 提取 cases/patterns → agent/memories/
       //   - 去重：CREATE / UPDATE / MERGE / SKIP
       log("memories extracted: %d", result.MemoriesExtracted)
```

### Materialize（物化）

仅 coder 需要。worktree 内 `.ai-workflow/context/` 目录已 gitignore：

```
worktree/
├── .ai-workflow/
│   └── context/           # gitignored，系统物化的 spec 文件
│       ├── requirement.md
│       └── api-design.md
├── src/
└── ...
```

其他角色直接通过 MCP tools 读取 OpenViking，无需物化。

### 写入 spec 后的处理

TL 写入 spec 后需等待 OpenViking 完成 L0/L1 生成：

```go
client.AddResource(specPath, AddResourceOpts{
    Target: fmt.Sprintf("viking://resources/%d/specs/%s/", pid, iid),
    Reason: "Issue spec for " + issueTitle,
    Wait:   true,  // 阻塞直到 L0/L1 生成完毕
})
```

## 记忆系统

### 记忆提取（全自动）

`session.Commit()` 触发 OpenViking 的记忆提取流程：

```
对话消息 → LLM 提取候选记忆
    → 向量预过滤（找同分类相似记忆）
    → LLM 去重决策（CREATE / UPDATE / MERGE / SKIP）
    → 写入 agent/memories/（cases 或 patterns）
    → 向量化建索引
```

### 6 种记忆分类

| 分类 | 位置 | 归属 | 可更新 | 我们用到 |
|------|------|------|--------|---------|
| profile | `user/memories/.overview.md` | user | 追加 | 暂不用 |
| preferences | `user/memories/preferences/` | user | 追加 | 暂不用 |
| entities | `user/memories/entities/` | user | 追加 | 暂不用 |
| events | `user/memories/events/` | user | 不可变 | 暂不用 |
| **cases** | `agent/memories/cases/` | agent | **不可变** | **核心** |
| **patterns** | `agent/memories/patterns/` | agent | **追加** | **核心** |

对我们最重要的是 `cases`（问题-方案对）和 `patterns`（行为模式），
它们按 agent_id 自动隔离，每个角色积累自己的经验。

### 项目级记忆的召回

不按项目分子目录。通过语义搜索 + target_uri 定向：

```python
# Coder 接到项目 42 的任务，搜索相关经验
results = client.Search(
    query="支付系统错误处理",
    session=currentSession,                    # 带当前任务上下文
    target_uri="viking://agent/memories/",     # 在自己的记忆中搜
)
// 因为 agent_id="coder"，只搜 coder 的记忆
// 记忆内容自然包含 "项目 42"、"支付系统" 等信息
// 语义搜索自动召回相关经验
```

## 配置

```yaml
context:
  provider: openviking          # openviking | sqlite
  openviking:
    url: "http://localhost:1933"
    api_key: ""                 # dev 模式留空
    agent_id_prefix: ""         # 可选前缀，多环境隔离
  fallback: sqlite              # OpenViking 不可用时降级
```

降级到 sqlite 时：
- Read / Write / List / Materialize 可用（基于 issue_attachments 表）
- L0/L1/Overview 不可用（返回空）
- Search 不可用（返回空）
- Memory 自动提取不可用
- Instructions / Skills 不可用

## Archive（知识积累）— Aggregator 角色

Issue 完成后，**aggregator 通过 ACP session** 将变更合并到项目 source of truth。
Spec 文件的格式由外部工具决定（见"Issue Artifact 关联"章节），我们不规定。
但合并过程需要大模型理解代码变更和 spec 的语义关系。

### 事件触发

```
EventIssueDone
  → 系统启动 Aggregator ACP session (agent_id="aggregator")
```

### Aggregator 的输入

| 输入 | 来源 | 怎么获取 |
|------|------|---------|
| 项目现有 specs | OpenViking | `context_read("viking://resources/{pid}/specs/")` |
| 本次 issue 的 spec 文件 | OpenViking | `context_list/read("viking://resources/{pid}/specs/{iid}/")` |
| PR diff | 系统注入 | 系统在启动 ACP session 时，将 `git diff` 结果写入 session context |
| PR commit messages | 系统注入 | 同上 |

PR diff 不通过 MCP 工具获取——aggregator 不需要访问 git。
系统在启动 ACP session 前，先通过 git API 拿到 diff，作为初始 context 注入。

### Aggregator 的工作

```
Aggregator ACP session:
  1. 读 session context 中的 PR diff          — coder 实际做了什么
  2. 读 specs/{iid}/ 下所有文件               — 原始 spec 说了什么
  3. 读 specs/ 下对应领域的现有 spec           — 项目当前的 source of truth
  4. 对比规划 vs 实际，理解差异
  5. 更新项目 specs（格式由 aggregator 的 instructions 决定）
  6. 移动原始 spec 到 archive/
  → session.commit() — OpenViking 自动提取 aggregator 的经验记忆
```

### 为什么必须用 Agent

- Coder 实际实现可能和原始 spec 有偏差（scope 缩减、发现额外需求）
- 需要理解代码变更的语义，而不是文本匹配
- 需要判断哪些变更反映在了 spec 中、哪些是新增的、哪些 spec 中有但实际没做
- 这和 OpenSpec 里用户通过 CLI 与 Agent 对话生成 delta 是同一性质的工作

### 存储路径

```
viking://resources/{pid}/specs/{iid}/   → 活跃期的 issue spec 文件
viking://resources/{pid}/specs/{domain}/ → 项目 source of truth（按领域组织）
viking://resources/{pid}/archive/{iid}/ → 归档保留审计轨迹
```

## Issue Artifact 关联（工具无关）

我们的系统**不关心** spec 工具的内部协议（OpenSpec 的 schema、DAG、template 等）。
不管用什么工具（OpenSpec、自定义工具、未来的任何工具）生成了什么文件，
我们只做一件事：**记录 issue 和它关联的文件列表**。

### 数据模型

```go
// Issue 关联的 artifact 文件列表
// 系统不关心文件是什么类型、谁生成的、用什么工具生成的
type IssueArtifacts struct {
    IssueID string   // issue ID
    Files   []string // viking:// URI 列表
}
```

### 存储

```
viking://resources/{pid}/specs/{iid}/
├── proposal.md              # 可能有
├── security-review.md       # 可能有
├── design.md                # 可能有
├── api-contract.md          # 可能有
├── tasks.md                 # 可能有
├── research.md              # 可能有
└── ...                      # 任意文件，数量不限
```

系统只知道 `viking://resources/{pid}/specs/{iid}/` 下有文件。
文件叫什么名字、有几个、什么格式、谁生成的——全部不管。

### 各角色怎么用这些文件

| 角色 | 怎么用 |
|------|--------|
| TL | 往 `specs/{iid}/` 里写文件（用什么工具生成都行） |
| Reviewer | 读 `specs/{iid}/` 下所有文件来审核 |
| Decomposer | 读 `specs/{iid}/` 来拆分子 issue |
| Coder | 读 `specs/{iid}/` 来理解要做什么（或 Materialize 到 worktree） |
| Aggregator | 读 `specs/{iid}/` + PR diff → 更新项目 source of truth |

### 为什么不绑定具体工具

1. **灵活性** — 不同项目可以用不同的 spec 工具
2. **演进** — 未来换工具不影响系统核心
3. **简单** — 我们只是编排层，不需要理解 spec 工具的协议

## 实现阶段

### P0：基础可用

1. `ContextStore` 接口 + OpenViking HTTP client 实现
2. ACP 集成：`agent_id = role`，session 创建/commit
3. Scoped MCP tools：context_read / context_list / context_write
4. Coder worktree 物化（Materialize：List + Read + 写本地文件）
5. TL 写入 spec（AddResource + wait）
6. Issue-artifact 关联（`specs/{iid}/` 目录即关联）

### P1：智能检索 + 指令

1. context_overview / context_abstract（L0/L1 查询）
2. context_find / context_search（语义搜索）
3. Instructions 加载（角色 prompt 从 OpenViking 读取）
4. Skills 注册（MCP tools → OpenViking skills）
5. Relations 建立（spec ↔ docs 关联）

### P2：记忆闭环

1. Session.Used() 上下文追踪
2. 角色启动时自动搜索相关记忆并注入 context
3. sqlite fallback 完整实现
4. 记忆质量监控（定期 review cases/patterns 的有效性）

### P3：归档 + 验证 + 优化

1. Aggregator 归档流程（ACP session：读 PR diff + specs → 更新 source of truth）
2. Verify 三维验证（Completeness / Correctness / Coherence）
3. Model profile（角色 → 模型分级配置）
4. ACP session context monitor（上下文窗口监控）

## 未来方向（不在 P0-P2 范围，仅记录想法）

### Verify 步骤

Issue done 前，独立 agent 验证三个维度：

| 维度 | 验证内容 |
|------|---------|
| Completeness | 所有 spec 中的 requirement 都有对应实现 |
| Correctness | 实现匹配 spec 意图，边界情况处理 |
| Coherence | 设计决策体现在代码中，模式一致 |

需要新增状态 `verifying`（在 `executing` 和 `done` 之间）和对应事件。
可选增强：decompose 时定义验收命令，coder 完成后自动运行（Nyquist 模式）。

### Model Profile

角色 → 模型的分级配置（借鉴 GSD）：

| 角色 | quality | balanced | budget |
|------|---------|----------|--------|
| team-leader | opus | opus | sonnet |
| reviewer | opus | sonnet | haiku |
| decomposer | opus | sonnet | sonnet |
| coder | opus | sonnet | sonnet |
| aggregator | sonnet | sonnet | haiku |

需要定义配置格式、存储位置、ACP stage 启动时的读取逻辑。
