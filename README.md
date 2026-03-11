# AI Workflow

AI Workflow 是一个面向多 Agent 编排的本地开发与执行系统，后端使用 Go，前端使用 React + Vite。

当前仓库的主线结构已经稳定在以下模块：

- `cmd/ai-flow`：主程序入口，提供 `server` 和 `executor` 命令。
- `internal/application`：流程编排、调度、规则与业务服务。
- `internal/adapters`：HTTP、存储、执行器等外部适配层。
- `internal/core`：领域模型与核心接口。
- `internal/platform`：配置、启动、运行时管理。
- `internal/runtime`：Agent 运行时与会话管理。
- `web/`：前端控制台。
- `configs/`：提示词、配置模板、技能相关配置。
- `scripts/test/`：后端、前端、集成与 smoke 脚本。
- `docs/`：计划、规格、学习资料与归档文档。

历史方案和旧实现保留在 `archive-src/` 与 `docs/archive/`，不属于当前主线。

## 环境要求

- Go 1.23+
- Node.js 20+
- Git

## 快速开始

### 1. 安装前端依赖

```powershell
npm --prefix web install
```

### 2. 启动后端

```powershell
go run ./cmd/ai-flow server --port 8080
```

服务启动后会自动：

- 读取运行时数据目录下的配置文件 `config.toml`
- 如果配置文件不存在，则写入一份默认配置
- 暴露健康检查 `/health`
- 在 `/api` 前缀下提供后端接口

默认运行时数据目录：

- 优先使用环境变量 `AI_WORKFLOW_DATA_DIR`
- 未设置时使用仓库根目录下的 `.ai-workflow/`

因此当前项目默认的运行时配置文件通常是：

```text
.\.ai-workflow\config.toml
```

### 3. 启动前端

```powershell
npm --prefix web run dev -- --strictPort
```

### 4. 访问地址

- 前端开发环境：`http://127.0.0.1:5173`
- 后端健康检查：`http://127.0.0.1:8080/health`
- 后端 API 前缀：`http://127.0.0.1:8080/api`

## 配置说明

运行时配置以数据目录中的 `config.toml` 为准，而不是源码中的内嵌默认文件。

- 源码里的默认模板：`internal/platform/config/defaults.toml`
- 运行时实际加载文件：`.ai-workflow/config.toml`

后端启动后会监听运行时配置文件变更；直接修改 `.ai-workflow/config.toml` 会触发热重载并同步更新运行时 agent registry。

敏感信息与 token 独立存放在 secrets 文件中，不应提交到仓库。

## CLI

查看版本：

```powershell
go run ./cmd/ai-flow version
```

启动服务：

```powershell
go run ./cmd/ai-flow server --port 8080
```

启动执行器：

```powershell
go run ./cmd/ai-flow executor --nats-url nats://127.0.0.1:4222 --agents claude,codex --max-concurrent 2
```

## 测试

常用测试脚本位于 `scripts/test/`。

后端全量测试：

```powershell
pwsh -NoProfile -File .\scripts\test\backend-all.ps1
```

前端单测：

```powershell
pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1
```

前端构建验证：

```powershell
pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
```

端到端集成回归：

```powershell
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
```

浏览器 E2E：

```powershell
pwsh -NoProfile -File .\scripts\test\project-admin-e2e.ps1
```

更多脚本说明见 `scripts/test/README.md`。

## 桌面版

仓库保留了 Tauri 桌面打包入口，根目录 `package.json` 提供：

```powershell
npm install
npm run tauri:icons
npm run tauri:dev
```

构建桌面包：

```powershell
npm install
npm run tauri:build
```

更多说明见 `docs/spec/tauri-desktop.md`。

## 文档入口

- 当前计划：`docs/plans/README.md`
- 测试脚本说明：`scripts/test/README.md`
- 规格与学习资料：`docs/spec/`、`docs/learning/`
- 历史归档：`docs/archive/`
