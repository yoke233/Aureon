# OpenViking 运行手册（安装/配置/检索/联调）

最后更新：2026-03-05

## 1. 目标

在 `ai-workflow` 仓库内完成以下最小闭环：

1. 服务可启动
2. 探活通过
3. 可导入资料
4. 可检索命中

## 2. 前置条件

- Windows PowerShell
- Python 3.10+（建议 3.12）
- Docker Desktop（若走 compose 方式）
- 可访问 embedding / VLM 模型提供方 API

## 3. 项目私有配置（推荐）

```powershell
Set-Location -LiteralPath D:\project\ai-workflow
New-Item -ItemType Directory -Force .runtime/openviking | Out-Null
New-Item -ItemType Directory -Force .runtime/openviking/data | Out-Null
Copy-Item configs/openviking/ov.conf.example .runtime/openviking/ov.conf
Copy-Item configs/openviking/ovcli.conf.example .runtime/openviking/ovcli.conf
```

填写 `.runtime/openviking/ov.conf` 的模型参数：

- `embedding.dense.provider/api_base/api_key/model/dimension`
- `vlm.provider/api_base/api_key/model`

## 4. 启动方式

### 4.1 方式 A：本地 Python 环境

```powershell
Set-Location -LiteralPath D:\project\ai-workflow
py -3.12 -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
pip install openviking

$env:OPENVIKING_CONFIG_FILE = "D:/project/ai-workflow/.runtime/openviking/ov.conf"
$env:OPENVIKING_CLI_CONFIG_FILE = "D:/project/ai-workflow/.runtime/openviking/ovcli.conf"
openviking-server
```

### 4.2 方式 B：Docker Compose

```powershell
Set-Location -LiteralPath D:\project\ai-workflow\configs\openviking
docker compose -f docker-compose.example.yml up -d
```

## 5. 探活与状态

```powershell
Set-Location -LiteralPath D:\project\ai-workflow
go run ./cmd/viking probe --base-url http://127.0.0.1:1933 --timeout 3s

ov status
ov ls viking://resources/
```

如果你部署在 `8088`，把 base-url/ovcli url 改为 `http://127.0.0.1:8088`。

## 6. 导入与检索

```powershell
# 导入资源（Git 仓库或本地目录）
ov add-resource https://github.com/volcengine/OpenViking --wait
ov add-resource D:/project/ai-workflow --wait

# 浏览
ov ls viking://resources/
ov tree viking://resources/ -L 2

# 检索
ov find "OpenViking 是什么"
ov search "ai-workflow secretary 记忆策略"
```

检索建议：

1. 先缩范围再搜：优先 `target_uri` 指向项目路径
2. 精确问题先用 `find`，复杂意图再用 `search`

## 7. 故障排查

1. `connect refused`  
   服务没启动或端口不一致，先查服务监听地址。
2. `ov` 配置错误  
   检查 `OPENVIKING_CONFIG_FILE` / `OPENVIKING_CLI_CONFIG_FILE` 路径与 JSON 合法性。
3. 搜索无结果  
   先确认导入已完成，再检查模型参数与范围是否过窄。
4. 结果噪声大  
   收窄 `target_uri`，避免全局搜索。

## 8. 真实联调参数清单

做联调前至少确认：

1. 服务地址（例如 `http://127.0.0.1:1933`）
2. 鉴权方式（`none` 或 `api_key`）
3. embedding 参数（provider/base/model/dimension）
4. vlm 参数（provider/base/model）
5. workspace 路径（可选）

联调通过标准：

1. `probe` 至少一个端点 2xx
2. `ov status` 正常
3. `ov add-resource --wait` 完成
4. `ov find` 有有效命中
