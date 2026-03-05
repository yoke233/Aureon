# OpenViking Start Here（5 分钟）

最后更新：2026-03-05

## 1. 你只需要先记住三件事

1. `ai-workflow` 不自建上下文系统，直接对接 OpenViking。
2. 默认是项目级记忆池，不先做复杂角色目录拆分。
3. 工程策略是“薄封装 + 可回退”：OpenViking 为主，sqlite 为 fallback。

## 2. 最短阅读顺序

1. 本文（5 分钟）
2. [`ops-runbook.zh-CN.md`](./ops-runbook.zh-CN.md)（怎么跑起来）
3. [`integration-spec.zh-CN.md`](./integration-spec.zh-CN.md)（怎么接到系统里）
4. [`06-architecture-deep-dive.zh-CN.md`](./06-architecture-deep-dive.zh-CN.md)（需要深挖时再读）

## 3. 最小闭环命令

```powershell
# 1) 准备项目私有配置目录
New-Item -ItemType Directory -Force D:\project\ai-workflow\.runtime\openviking | Out-Null
Copy-Item D:\project\ai-workflow\configs\openviking\ov.conf.example D:\project\ai-workflow\.runtime\openviking\ov.conf
Copy-Item D:\project\ai-workflow\configs\openviking\ovcli.conf.example D:\project\ai-workflow\.runtime\openviking\ovcli.conf

# 2) 启服务（docker 方式）
Set-Location -LiteralPath D:\project\ai-workflow\configs\openviking
docker compose -f docker-compose.example.yml up -d

# 3) 探活
Set-Location -LiteralPath D:\project\ai-workflow
go run ./cmd/viking probe --base-url http://127.0.0.1:1933 --timeout 3s
ov status
```

## 4. 目录与职责（当前策略）

- `secretary`：允许写入/提交记忆。
- `worker/reviewer`：只读上下文，不提交记忆。
- 上下文主路径：
  - `viking://resources/shared/`
  - `viking://resources/projects/{project_id}/`
  - `viking://memory/projects/{project_id}/`

## 5. 常见误区

1. 误区：先设计复杂层级再落地。  
   建议：先跑通项目级最小路径，再按冲突情况增量拆分。
2. 误区：在 Go 侧重做摘要/检索。  
   建议：摘要（L0/L1）和语义检索由 OpenViking 提供，Go 只做薄封装。
3. 误区：把文档和实现耦死。  
   建议：接口保持最小集（Read/Write/List/Materialize），高级能力按阶段启用。
