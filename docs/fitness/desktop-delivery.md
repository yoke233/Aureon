---
dimension: desktop_delivery
weight: 20
threshold:
  pass: 100
  warn: 80
metrics:
  - name: tauri_nsis_target
    command: Select-String -Path .\src-tauri\tauri.conf.json -Pattern '"targets": \["nsis"\]'
    hard_gate: false
  - name: tauri_windows_workflow_present
    command: Select-String -Path .\.github\workflows\tauri-windows.yml -Pattern 'desktop-v\*|Build Tauri Windows bundle|upload-artifact'
    hard_gate: false
---

# Desktop Delivery 证据

> 本文件先校验桌面 Windows 打包链路的配置闭环，后续再逐步升级为真实构建门禁。

## 当前目标

- Tauri Windows 安装包目标已经切到 `nsis`
- GitHub Actions 已具备独立 Windows 打包工作流
- sidecar 构建仍复用根目录 `tauri:build` 脚本

## 当前证据来源

- [tauri.conf.json](D:/project/ai-workflow/src-tauri/tauri.conf.json)
- [tauri-windows.yml](D:/project/ai-workflow/.github/workflows/tauri-windows.yml)
- [package.json](D:/project/ai-workflow/package.json)

## 为什么当前先不做硬门禁

- 本地与 CI 环境都需要 Rust/Tauri/NSIS 依赖齐备
- 当前项目刚建立桌面发布链路，先校验配置存在性更稳
- 等桌面构建在 CI 连续稳定后，再把真实 `npm run tauri:build` 提升为硬门禁
