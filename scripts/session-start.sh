#!/usr/bin/env bash
# session-start.sh — Claude Code 会话启动钩子
#
# 在每次 Claude Code 会话开始时自动运行，确保 V2 环境就绪。
# 快速幂等检查，仅在需要时执行修复操作。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

export VITE_UI_VERSION="v2"
DATA_DIR="${AI_WORKFLOW_DATA_DIR:-$PROJECT_ROOT/.ai-workflow}"

ISSUES=()

# ── 快速依赖检查 ──
command -v go &>/dev/null || ISSUES+=("Go not found")
command -v node &>/dev/null || ISSUES+=("Node not found")

# ── 数据目录 ──
mkdir -p "$DATA_DIR"

# ── 配置文件 ──
if [ ! -f "$DATA_DIR/config.toml" ]; then
    go run ./cmd/ai-flow config init 2>/dev/null || true
fi

# ── Go 模块 ──
if [ ! -d vendor ] && [ -f go.sum ]; then
    go mod download 2>/dev/null &
fi

# ── 前端依赖 ──
if [ ! -d web/node_modules ] && command -v npm &>/dev/null; then
    npm --prefix web install --prefer-offline 2>/dev/null &
fi

# 等待后台任务完成
wait 2>/dev/null || true

# ── 编译检查 (快速, 不生成二进制) ──
go build ./cmd/ai-flow 2>/dev/null || ISSUES+=("Go build failed")

# ── 报告 ──
if [ ${#ISSUES[@]} -gt 0 ]; then
    echo "V2 session-start warnings:"
    for issue in "${ISSUES[@]}"; do
        echo "  - $issue"
    done
fi

echo "V2 environment ready (data: $DATA_DIR)"
