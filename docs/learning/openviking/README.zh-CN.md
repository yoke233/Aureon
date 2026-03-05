# OpenViking 学习目录（ai-workflow）

最后更新：2026-03-05

本目录已按“高频使用优先”压缩。旧分步文档已删除（保留 Git 历史追溯），
当前只保留 3 份主文档 + 1 份架构深潜。

## 快速入口（先读）

1. [00-start-here.zh-CN.md](./00-start-here.zh-CN.md)  
   5 分钟总览：先做什么、先看什么、最小闭环命令。

2. [ops-runbook.zh-CN.md](./ops-runbook.zh-CN.md)  
   运行手册：安装、配置、启动、探活、导入、搜索、排障、联调参数。

3. [integration-spec.zh-CN.md](./integration-spec.zh-CN.md)  
   集成规范：`ai-workflow` 对接 OpenViking 的 Store 接口、URI、权限与落地阶段。

4. [06-architecture-deep-dive.zh-CN.md](./06-architecture-deep-dive.zh-CN.md)  
   架构深潜（选读）：AGFS、L0/L1/L2、检索与会话记忆机制。

## 项目级配置约定

OpenViking 默认按项目私有配置运行：

- `D:/project/ai-workflow/.runtime/openviking/ov.conf`
- `D:/project/ai-workflow/.runtime/openviking/ovcli.conf`

模板保留在：

- `configs/openviking/ov.conf.example`
- `configs/openviking/ovcli.conf.example`
