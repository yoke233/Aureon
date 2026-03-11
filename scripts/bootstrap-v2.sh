#!/usr/bin/env bash
# bootstrap-v2.sh — 自动化 V2 环境自举脚本
#
# 一键完成: 依赖检查 → 配置初始化 → 后端编译 → 前端构建 → 冒烟测试 → 启动
#
# 用法:
#   bash scripts/bootstrap-v2.sh              # 完整自举 (检查+构建+验证)
#   bash scripts/bootstrap-v2.sh --check      # 仅检查依赖和配置
#   bash scripts/bootstrap-v2.sh --start      # 自举后启动服务
#   bash scripts/bootstrap-v2.sh --ci         # CI 模式: 无交互, 跳过启动
#
# 环境变量:
#   PORT                  后端端口 (默认 8080)
#   VITE_UI_VERSION       前端版本 (强制设为 v2)
#   AI_WORKFLOW_DATA_DIR  数据目录 (默认 .ai-workflow)
#   SKIP_FRONTEND         跳过前端构建 (设为 1)
#   SKIP_TESTS            跳过冒烟测试 (设为 1)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

PORT="${PORT:-8080}"
DATA_DIR="${AI_WORKFLOW_DATA_DIR:-$PROJECT_ROOT/.ai-workflow}"
SKIP_FRONTEND="${SKIP_FRONTEND:-0}"
SKIP_TESTS="${SKIP_TESTS:-0}"

# 强制 V2 UI
export VITE_UI_VERSION="v2"

MODE="full"
START_AFTER=false
CI_MODE=false

for arg in "$@"; do
    case "$arg" in
        --check)  MODE="check" ;;
        --start)  START_AFTER=true ;;
        --ci)     CI_MODE=true ;;
        *) echo "Unknown arg: $arg"; exit 1 ;;
    esac
done

# ── 颜色输出 ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

step() { echo -e "${BLUE}==> $1${NC}"; }
ok()   { echo -e "${GREEN}  ✓ $1${NC}"; }
warn() { echo -e "${YELLOW}  ⚠ $1${NC}"; }
fail() { echo -e "${RED}  ✗ $1${NC}"; }

ERRORS=0
record_error() {
    fail "$1"
    ERRORS=$((ERRORS + 1))
}

# ═══════════════════════════════════════════════════════════════════
# 阶段 1: 依赖检查
# ═══════════════════════════════════════════════════════════════════
step "阶段 1/5: 依赖检查"

# Go
if command -v go &>/dev/null; then
    GO_VERSION=$(go version | grep -oP 'go\d+\.\d+' | head -1)
    ok "Go: $GO_VERSION"
else
    record_error "Go 未安装 (需要 1.25+)"
fi

# Node
if command -v node &>/dev/null; then
    NODE_VERSION=$(node --version)
    ok "Node: $NODE_VERSION"
else
    if [ "$SKIP_FRONTEND" = "0" ]; then
        record_error "Node 未安装 (需要 22+)"
    else
        warn "Node 未安装 (已跳过前端)"
    fi
fi

# npm
if command -v npm &>/dev/null; then
    NPM_VERSION=$(npm --version)
    ok "npm: v$NPM_VERSION"
fi

# Git
if command -v git &>/dev/null; then
    ok "Git: $(git --version | cut -d' ' -f3)"
else
    record_error "Git 未安装"
fi

# PowerShell (optional, for smoke tests)
if command -v pwsh &>/dev/null; then
    ok "PowerShell: $(pwsh --version 2>/dev/null | head -1)"
else
    warn "PowerShell 未安装 (冒烟测试将使用 Go test 直接运行)"
fi

if [ "$ERRORS" -gt 0 ] && [ "$MODE" = "check" ]; then
    echo ""
    fail "依赖检查失败 ($ERRORS 个错误)"
    exit 1
fi

# ═══════════════════════════════════════════════════════════════════
# 阶段 2: 配置初始化
# ═══════════════════════════════════════════════════════════════════
step "阶段 2/5: V2 配置初始化"

mkdir -p "$DATA_DIR"

CONFIG_FILE="$DATA_DIR/config.toml"
SECRETS_FILE="$DATA_DIR/secrets.toml"

if [ -f "$CONFIG_FILE" ]; then
    ok "配置文件已存在: $CONFIG_FILE"
else
    step "  生成默认配置..."
    if go run ./cmd/ai-flow config init 2>/dev/null; then
        ok "配置已生成: $CONFIG_FILE"
    else
        # 如果 config init 失败，创建最小 v2 配置
        cat > "$CONFIG_FILE" <<'TOML'
# AI Workflow V2 配置 — 自动生成

[store]
path = "store.db"

[server]
port = 8080

[v2]
mock_executor = false

[v2.sandbox]
enabled = false
TOML
        ok "最小配置已生成: $CONFIG_FILE"
    fi
fi

if [ ! -f "$SECRETS_FILE" ]; then
    cat > "$SECRETS_FILE" <<'TOML'
# AI Workflow Secrets — 自动生成
# 请根据需要填入实际的 API key

# [secrets]
# anthropic_api_key = ""
# openai_api_key = ""
# github_token = ""
TOML
    ok "secrets 模板已生成: $SECRETS_FILE"
else
    ok "secrets 文件已存在: $SECRETS_FILE"
fi

# 确保 .gitignore 包含数据目录敏感文件
GITIGNORE="$PROJECT_ROOT/.gitignore"
if [ -f "$GITIGNORE" ]; then
    for pattern in ".ai-workflow/secrets.toml" ".ai-workflow/*.db" ".ai-workflow/home/"; do
        if ! grep -qF "$pattern" "$GITIGNORE" 2>/dev/null; then
            echo "$pattern" >> "$GITIGNORE"
            ok "已添加 $pattern 到 .gitignore"
        fi
    done
fi

if [ "$MODE" = "check" ]; then
    echo ""
    ok "依赖和配置检查通过"
    exit 0
fi

# ═══════════════════════════════════════════════════════════════════
# 阶段 3: 后端编译
# ═══════════════════════════════════════════════════════════════════
step "阶段 3/5: 后端编译"

if go build -o ./ai-flow ./cmd/ai-flow; then
    ok "后端二进制编译成功: ./ai-flow"
    VERSION=$(./ai-flow version 2>/dev/null || echo "unknown")
    ok "版本: $VERSION"
else
    record_error "后端编译失败"
fi

# ═══════════════════════════════════════════════════════════════════
# 阶段 4: 前端构建
# ═══════════════════════════════════════════════════════════════════
if [ "$SKIP_FRONTEND" = "1" ]; then
    step "阶段 4/5: 前端构建 (已跳过)"
else
    step "阶段 4/5: 前端构建 (VITE_UI_VERSION=v2)"

    if [ ! -d web/node_modules ]; then
        step "  安装前端依赖..."
        npm --prefix web ci
        ok "前端依赖已安装"
    else
        ok "前端依赖已就绪"
    fi

    # 类型检查
    if npm --prefix web run typecheck 2>/dev/null; then
        ok "TypeScript 类型检查通过"
    else
        warn "TypeScript 类型检查有警告 (非致命)"
    fi

    # 构建
    if VITE_UI_VERSION=v2 npm --prefix web run build; then
        ok "前端构建成功 (V2 模式)"
    else
        record_error "前端构建失败"
    fi
fi

# ═══════════════════════════════════════════════════════════════════
# 阶段 5: 冒烟测试
# ═══════════════════════════════════════════════════════════════════
if [ "$SKIP_TESTS" = "1" ]; then
    step "阶段 5/5: 冒烟测试 (已跳过)"
else
    step "阶段 5/5: V2 冒烟测试"

    # 核心后端测试
    if go test ./internal/v2/... -count=1 -timeout 120s 2>&1 | tail -5; then
        ok "V2 引擎测试通过"
    else
        record_error "V2 引擎测试失败"
    fi

    # V2 API handler 测试
    if go test ./internal/v2/api/... -count=1 -timeout 60s 2>&1 | tail -3; then
        ok "V2 API 测试通过"
    else
        record_error "V2 API 测试失败"
    fi
fi

# ═══════════════════════════════════════════════════════════════════
# 结果汇总
# ═══════════════════════════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$ERRORS" -gt 0 ]; then
    fail "自举完成，但有 $ERRORS 个错误"
    echo ""
    echo "请修复上述错误后重新运行。"
    exit 1
else
    ok "V2 环境自举成功!"
    echo ""
    echo -e "  ${BLUE}启动服务:${NC}  ./ai-flow server --port $PORT"
    echo -e "  ${BLUE}开发模式:${NC}  VITE_UI_VERSION=v2 bash scripts/dev.sh"
    echo -e "  ${BLUE}仅后端:${NC}    go run ./cmd/ai-flow server --port $PORT"
    echo -e "  ${BLUE}运行测试:${NC}  go test ./internal/v2/..."
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ── 可选: 自举后启动 ──
if $START_AFTER && [ "$ERRORS" -eq 0 ]; then
    echo ""
    step "启动 V2 服务..."
    export VITE_UI_VERSION=v2
    exec ./ai-flow server --port "$PORT"
fi
