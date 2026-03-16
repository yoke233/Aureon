---
dimension: code_quality
weight: 20
threshold:
  pass: 90
  warn: 80
metrics:
  - name: gofmt_clean
    command: $files = git ls-files '*.go'; if (-not $files) { Write-Output 'gofmt clean'; exit 0 }; $bad = gofmt -l $files; if ([string]::IsNullOrWhiteSpace(($bad | Out-String))) { Write-Output 'gofmt clean'; exit 0 } else { $bad; exit 1 }
    pattern: gofmt clean
    hard_gate: true
  - name: go_vet_pass
    command: go vet ./...
    hard_gate: true
  - name: frontend_lint_pass
    command: npm --prefix web run lint
    hard_gate: true
---

# Code Quality 证据

> 本文件收口当前仓库已经稳定存在的最小代码质量约束，不引入额外工具链。

## 规则目标

- Go 代码格式不能回退
- Go 基础静态检查不能回退
- 前端 ESLint 不能回退

## 当前选择

当前不直接引入新工具，而是先复用项目里已有且成本最低的三项：

- `gofmt`
- `go vet`
- `npm --prefix web run lint`

## 当前证据来源

- [ci.yml](D:/project/ai-workflow/.github/workflows/ci.yml)
- [web/package.json](D:/project/ai-workflow/web/package.json)

## 后续可扩展项

- `golangci-lint`
- 前端 `prettier`
- 重复代码检查
- Tauri/Rust `cargo clippy`
