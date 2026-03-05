# OpenViking 架构深度理解

最后更新：2026-03-05

## 1. 定位

OpenViking 是字节跳动（火山引擎 Viking 团队）开源的 **AI Agent 上下文数据库**。
Apache 2.0 协议，GitHub: volcengine/OpenViking。

核心主张：用**文件系统范式**统一管理 Agent 的三类上下文（Memory、Resource、Skill），
取代传统 RAG 的碎片化向量存储。

## 2. 架构分层

```
Client Layer (SDK / CLI / HTTP)
    ↓
Service Layer
    ├── FSService       文件系统操作
    ├── SearchService   语义检索
    ├── SessionService  会话管理
    ├── ResourceService 资源导入
    ├── RelationService 关系管理
    ├── PackService     导入/导出 (.ovpack)
    └── DebugService    调试
    ↓
Processing Modules
    ├── Retrieve    检索（意图分析 + 重排序）
    ├── Session     会话（消息记录 + 记忆管理）
    ├── Parse       解析（文档提取）
    └── Compressor  压缩（记忆去重压缩）
    ↓
Dual-Layer Storage
    ├── AGFS         内容层（完整文件、L0/L1/L2、关系）
    └── Vector Index 检索层（URI + 向量 + 元数据，不存文件内容）
```

### 双层存储的关键设计

- **AGFS**：存完整内容，支持本地文件系统 / S3 / 内存（测试用）
- **Vector Index**：只存 URI、向量、元数据。删除文件时自动清理对应向量记录
- 两层通过 URI 关联，AGFS 是 source of truth

## 3. 三类上下文

| 类型 | 说明 | 谁控制更新 | URI 示例 |
|------|------|-----------|---------|
| **Resource** | 外部知识（API 文档、代码仓库） | 用户添加 | `viking://resources/my-project/docs/` |
| **Memory** | Agent 学习的记录 | Agent 提取 | `viking://agent/memories/cases/` |
| **Skill** | Agent 可调用的能力（MCP 工具等） | Agent 管理 | `viking://agent/skills/search-web/` |

### Memory 六分类

用户记忆（可追加）：
- **profile**：用户画像
- **preferences**：偏好设定
- **entities**：重要实体（人、项目）

Agent 记忆（不可变记录）：
- **events**：关键事件
- **cases**：问题-方案对
- **patterns**：行为模式

## 4. URI 寻址体系

格式：`viking://{scope}/{path}`

| Scope | 用途 | 生命周期 |
|-------|------|---------|
| resources | 独立资源 | 长期 |
| user | 用户级数据 | 长期 |
| agent | Agent 级数据 | 长期 |
| session | 会话级数据 | 会话周期 |
| queue | 处理队列 | 临时 |
| temp | 临时文件 | 解析期间 |

特殊文件约定：
- `.abstract.md` — L0 摘要（~100 tokens）
- `.overview.md` — L1 概览（~2k tokens）
- `.relations.json` — 关联资源
- `.meta.json` — 元数据

## 5. L0/L1/L2 分层加载（核心创新）

```
L0 (Abstract)   ~100 tokens   自动生成   用于快速过滤（向量检索命中）
L1 (Overview)   ~1k tokens    自动生成   用于重排序和导航
L2 (Detail)     无限制         原始内容   按需加载完整文件
```

生成过程：
1. SemanticProcessor 自底向上遍历目录
2. 为每个节点生成 L0/L1
3. 子节点的摘要聚合成父节点的概览
4. 解析后或会话压缩时异步生成

实际使用建议：**先用 L1，需要时才加载 L2**。

多模态处理：L0/L1 始终是文本（Markdown），L2 可以是任何格式。
二进制内容（图片、视频）在高层获得文字描述。

## 6. 检索机制

### find() — 简单向量检索
```
Query → 向量匹配 → 返回结果
```
无意图分析，无会话上下文。适合精确查询。

### search() — 高级检索
```
Query → 意图分析 → 查询扩展 → 向量检索(L0) → 重排序(L1) → 结果
```
支持会话上下文，理解对话意图。适合对话式检索。

### grep() — 正则搜索
传统模式匹配，支持大小写控制。

### glob() — 文件发现
通配符模式匹配文件路径。

## 7. 会话记忆管理

```
对话消息 → session.add_message()
    ↓
达到阈值（~10 条）→ session.commit()
    ↓
压缩归档：
    ├── 生成 .abstract.md / .overview.md
    ├── 历史消息归档到 history/{timestamp}/
    └── 自动提取长期记忆：
        ├── events（关键事件）
        ├── cases（问题-方案对）
        └── patterns（行为模式）
```

存储结构：
```
viking://session/{session-id}/
    messages.jsonl       当前消息
    .abstract.md         压缩摘要
    .overview.md         压缩概览
    tools/               工具调用记录
    history/             归档历史
```

## 8. REST API 概要

| 类别 | 端点 | 方法 |
|------|------|------|
| 健康检查 | `/health` | GET |
| 系统状态 | `/api/v1/system/status` | GET |
| 目录列表 | `/api/v1/fs/ls` | GET |
| 创建目录 | `/api/v1/fs/mkdir` | POST |
| 删除 | `/api/v1/fs` | DELETE |
| 移动 | `/api/v1/fs/mv` | POST |
| 读取内容 | `/api/v1/content/read` | GET |
| 读取摘要 | `/api/v1/content/abstract` | GET |
| 读取概览 | `/api/v1/content/overview` | GET |
| 添加资源 | `/api/v1/resources` | POST |
| 语义搜索 | `/api/v1/search/find` | POST |
| 高级搜索 | `/api/v1/search/search` | POST |
| 正则搜索 | `/api/v1/search/grep` | POST |
| 模式匹配 | `/api/v1/search/glob` | POST |
| 创建会话 | `/api/v1/sessions` | POST |
| 提交会话 | `/api/v1/sessions/{id}/commit` | POST |
| 导出 | `/api/v1/pack/export` | POST |
| 导入 | `/api/v1/pack/import` | POST |
| 创建关联 | `/api/v1/relations/link` | POST |
| 查询关联 | `/api/v1/relations` | GET |
| 账户管理 | `/api/v1/admin/accounts` | POST/GET |

认证：`X-API-Key` header 或 `Authorization: Bearer` token。

响应格式：
```json
{
  "status": "ok",
  "result": { ... },
  "time": 0.123
}
```

## 9. 支持的文件格式

| 格式 | 处理方式 |
|------|---------|
| PDF | 文本+图片提取 |
| Markdown | 原生支持 |
| HTML | 清洗文本提取 |
| JSON/YAML | 结构化解析 |
| 代码 (.py .js .ts .go .java 等) | 语法感知解析 |
| 图片 (.png .jpg .gif .webp) | VLM 描述 |
| 视频 (.mp4 .mov .avi) | 帧提取 + VLM |
| 音频 (.mp3 .wav .m4a) | 转写 |
| Word (.docx) | 文本提取 |

## 10. 部署模式

- **Embedded**：自动启动 AGFS 子进程，适合本地开发
- **HTTP**：独立服务进程，支持多语言/多环境接入

配置文件：`~/.openviking/ov.conf`，包含 Storage、Logging、Embedding、VLM 四个主要段。

## 11. 关键设计决策总结

1. **文件系统范式** > 向量数据库范式 — 直觉友好，可浏览
2. **内容与索引分离** — AGFS 存内容，Vector 只存引用
3. **自动分层生成** — L0/L1 无需手动维护
4. **URI 统一寻址** — 所有上下文一个地址空间
5. **会话自动压缩** — 不丢信息的前提下控制 token 消耗
6. **记忆自动提取** — 对话自然沉淀为结构化知识
