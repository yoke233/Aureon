---
dimension: testability
weight: 45
threshold:
  pass: 90
  warn: 80
metrics:
  - name: backend_unit_pass
    command: pwsh -NoProfile -File .\scripts\test\backend-unit.ps1
    hard_gate: true
  - name: frontend_unit_pass
    command: pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1
    hard_gate: true
  - name: frontend_build_pass
    command: pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
    hard_gate: true
---

# Testability 证据

> 本文件记录当前项目最小测试闭环，优先复用仓库现有 PowerShell 测试脚本。

## 适用范围

- `internal/**`
- `web/**`
- `src-tauri/**` 关联到前端构建输出时，也受本维度间接约束

## 规则

- 后端主流程至少通过 `backend-unit.ps1`
- 前端主流程至少通过 `frontend-unit.ps1`
- 前端发布基础至少通过 `frontend-build.ps1`
- 任何一个硬门禁失败，都视为当前批次不可合格

## 当前证据来源

- [backend-unit.ps1](D:/project/ai-workflow/scripts/test/backend-unit.ps1)
- [frontend-unit.ps1](D:/project/ai-workflow/scripts/test/frontend-unit.ps1)
- [frontend-build.ps1](D:/project/ai-workflow/scripts/test/frontend-build.ps1)

## 扩展建议

后续可继续加入：

- `backend-integration.ps1`
- `backend-e2e.ps1`
- `frontend-e2e.ps1`
- `suite-p3.ps1`
