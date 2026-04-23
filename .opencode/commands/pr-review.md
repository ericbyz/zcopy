---
name: pr-review
description: "动态 Code Review。根据改动内容自动组装专家团队，按风险等级分派审查。"
---

# 动态 Code Review

## 概述

> 当代码全部由 AI 生成、变更速度远超传统开发时，人工审查跟不上节奏。
> 但完全放弃审查又不行——尤其在文件同步系统中，一个看起来无害的改动可能导致数据不一致。

## 核心设计：动态 Agent Template

根据 PR 内容**自动组装专家团队**，不同领域的改动交给不同的专家。

## 流程

```
Step 1: 分析改动
  - 哪些文件被改了？
  - 改动的性质（新功能 / Bug 修复 / 重构 / 配置）
  - 涉及哪些领域

Step 2: 风险分级
  - CRITICAL：安全相关、同步引擎核心、File Provider 通信
  - HIGH：公共 API 变更、认证逻辑、数据存储
  - MEDIUM：内部实现改动、新测试、UI 调整
  - LOW：文档、注释、格式化

Step 3: 组装审查团队
  按改动领域选择对应的 expert：
  - Go 后端改动 → backend agent
  - Vue / Electron 改动 → frontend agent
  - sync/ 或 fileprovider/ 改动 → sync-engine rules
  - 构建脚本改动 → infra agent
  - CRITICAL/HIGH → 深度审查
  - LOW → 基本检查
  - 所有审查任务并行执行

Step 4: 汇总报告
  - 按风险等级排序
  - 明确的 APPROVE / REQUEST CHANGES / BLOCK 结论
```

## 改动领域 → Agent 映射

| 改动路径 | 审查 Agent | 额外规则 |
|---------|-----------|---------|
| client/backend/sync/ | backend + sync-engine rules | 快照一致性检查 |
| client/backend/fileprovider/ | backend + sync-engine rules | 多进程安全检查 |
| client/backend/handlers_*.go | backend | API 契约检查 |
| client/front/src/**/*.vue | frontend | 组件模式检查 |
| client/front-mac/electron/ | frontend + infra | IPC 安全检查 |
| client/front-mac/fileprovider/ | frontend + sync-engine rules | Extension 行为检查 |
| server/backend/handlers/ | backend | 认证 + 路径安全检查 |
| server/front/src/ | frontend | 组件模式检查 |
| scripts/ | infra | 构建流程检查 |
| .opencode/ | quality | 配置一致性检查 |

## 审查维度与检查项

### 安全审查（CRITICAL — 始终执行）

```
- [ ] 路径穿越防护：resolveUserPath() 校验
- [ ] JWT：token 生成/校验/过期完整
- [ ] 无硬编码密钥
- [ ] CORS 配置正确
- [ ] 输入校验完整
- [ ] File Provider 端点安全
```

### 正确性审查（HIGH — 默认执行）

```
- [ ] 同步引擎：快照读写一致性
- [ ] 实现了需求的所有场景
- [ ] 边界条件处理
- [ ] 错误传播正确（Go 中 error 返回、JS 中 catch 处理）
- [ ] 并发安全（goroutine、mutex）
```

### 性能审查（HIGH — 性能敏感代码执行）

```
- [ ] 无不必要的全量加载
- [ ] 大目录/大文件场景有分批处理
- [ ] 文件监听防抖有效
```

### 可维护性审查（MEDIUM — 可选）

```
- [ ] 命名清晰
- [ ] 函数不超过 50 行
- [ ] 无死代码
- [ ] 遵循现有代码模式
```

## 报告模板

```markdown
## Code Review Report

**改动范围**: N 文件, +M/-L 行
**风险等级**: [CRITICAL/HIGH/MEDIUM/LOW]

### CRITICAL 🔴
> （必须修复才能合并）

1. **[问题标题]** `file.go:行号`
   - **问题**: [描述]
   - **影响**: [可能导致什么后果]
   - **修复**: [建议]

### HIGH 🟠
> （必须修复）

### MEDIUM 🟡
> （建议修复）

### LOW ⚪
> （可选改进）

### Summary
- 🔴 CRITICAL: N
- 🟠 HIGH: N
- 🟡 MEDIUM: N
- ⚪ LOW: N

**Recommendation**: APPROVE / REQUEST CHANGES / BLOCK
```

## 参考文件

- 审查专家：`.opencode/agents/reviewer.md`
- 质量专家：`.opencode/agents/quality.md`
- 同步引擎规则：`.opencode/rules/sync-engine.md`
- 后端规则：`.opencode/rules/backend.md`
- 前端规则：`.opencode/rules/frontend.md`
