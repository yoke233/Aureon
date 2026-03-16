---
dimension: security
weight: 10
threshold:
  pass: 90
  warn: 80
metrics:
  - name: audit_redaction_tests
    command: go test ./internal/audit -run 'TestLogger' -count=1
    hard_gate: false
  - name: audit_http_routes_under_admin_scope
    command: Select-String -Path .\internal\adapters\http\handler.go -Pattern 'RequireScope\(httpx\.ScopeAdmin\)|/tool-calls/\{auditID\}|/runs/\{runID\}/audit-timeline'
    hard_gate: false
  - name: audit_http_behavior_tests
    command: go test ./internal/adapters/http -run 'TestAPI_(ToolCallAuditRoutes|RunAuditTimelineRoute)' -count=1
    hard_gate: false
  - name: known_security_gaps_are_tracked
    command: Select-String -Path .\docs\system-gaps-review.md -Pattern 'CORS 默认允许所有来源|无 Go 静态分析配置|无 Prometheus/Grafana 指标'
    hard_gate: false
---

# Security 证据

> 当前项目还没有完整的 `semgrep/trivy/golangci-lint` 安全扫描链，所以本文件先把已经存在的安全性证据收口起来，避免这些能力回退。

## 当前安全策略

### 1. 先保住已有能力

当前仓库已经有价值的安全/审计能力包括：

- tool call audit
- audit timeline
- redaction 预览脱敏
- admin scope 保护的审计读取接口

相比空白起步，这些更值得先纳入 fitness。

### 2. 把已知缺口显式记录，而不是假装已解决

当前还没完整落地的安全基础设施，已经在：

- [system-gaps-review.md](D:/project/ai-workflow/docs/system-gaps-review.md)

中明确记录，例如：

- CORS 默认过宽
- 缺少静态分析配置
- 缺少 metrics / readiness / 更完整运维基线

## 证据来源

- 审计实现与测试：
  - [internal/audit](D:/project/ai-workflow/internal/audit)
  - [logger_test.go](D:/project/ai-workflow/internal/audit/logger_test.go)
- 审计路由与测试：
  - [handler.go](D:/project/ai-workflow/internal/adapters/http/handler.go)
  - [handler_test.go](D:/project/ai-workflow/internal/adapters/http/handler_test.go)
- 缺口审查：
  - [system-gaps-review.md](D:/project/ai-workflow/docs/system-gaps-review.md)

## 后续演进建议

下一批适合升级成真正安全门禁的项：

- `golangci-lint`
- `npm audit`
- `govulncheck`
- `trivy`
- `semgrep`
