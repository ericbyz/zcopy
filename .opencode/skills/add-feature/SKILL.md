---
name: add-feature
description: "添加新功能时的标准化流程。从需求到实现到验证，确保在正确的约束下工作。"
---

# 添加新功能

## 概述

标准化"添加新功能"的完整流程。核心思想：**先想清楚怎么验证，再写代码**。

## 流程

```
理解需求 → 确认验证方案 → 写设计文档 → 写失败测试 → 实现功能 → 测试通过？ → 回归测试 → Code Review → 合并
```

## Step 1: 理解需求

在写任何代码之前，回答这些问题：

```markdown
## 需求理解

**功能描述**：[一句话描述这个功能做什么]
**影响范围**：
  - [ ] 客户端 Go 后端（client/backend/）
  - [ ] 客户端前端（client/front/src/ 或 client/front-mac/）
  - [ ] 服务端 Go 后端（server/backend/）
  - [ ] 服务端前端（server/front/src/）
  - [ ] Swift File Provider（client/front-mac/fileprovider/）
**输入**：[从哪里拿什么数据]
**输出**：[产生什么结果]
**边界条件**：[什么情况下行为不同]
**非功能需求**：[性能、安全、跨平台等约束]
```

如果以上任何一项不确定 → **停下来问用户**，不要猜。

## Step 2: 确认验证方案

> 这是最关键的一步。验证方案定义了你和 AI 之间的合同。

```markdown
## 验证方案

**单元测试**：
  - 测试用例 1：[描述] → 输入 [x]，预期 [y]
  - 测试用例 2：[边界条件] → 输入 [x]，预期 [y]
  - 测试用例 3：[错误场景] → 输入 [x]，预期 [y]

**集成测试**：[如果需要多模块协作]

**手动验证**：[如果自动化测试无法覆盖，列出验证步骤]

**回归检查**：跑全量测试，确保没有破坏旧功能
```

## Step 3: 写设计文档

```markdown
## 设计文档

**涉及文件**：
  - 新建：`client/backend/handlers_xxx.go`
  - 修改：`client/backend/routes.go`（路由注册）
  - 测试：`client/backend/handlers_xxx_test.go`

**接口设计**：
  - API 路径和方法
  - 请求/响应格式

**依赖关系**：
  - 依赖哪些已有模块
  - 对外暴露什么
```

**Ask First**: 如果设计涉及修改公共 API、引入新依赖、跨模块改动 → 先和用户确认。

## Step 4: TDD（红-绿-重构）

```
1. 写失败测试 → 跑测试确认它失败
2. 最小实现让测试通过 → 跑测试确认通过
3. 重构（如果需要）→ 跑测试确认仍然通过
4. 补充边界测试 → 确认全部通过
```

## Step 5: 回归验证

```
1. 跑全量测试（Go: go test ./... / 前端: npm run test）
2. 如果有失败 → 修复 → 重新跑
3. 检查测试覆盖率（新代码是否有测试）
```

## Step 6: Code Review

使用 `/pr-review` 命令触发动态审查。

## 关键原则

1. **验证方案在代码之前**——测试是你的合同条款
2. **一次只做一个功能**——不要顺带"改进"其他东西
3. **小 commit**——每个逻辑步骤一个 commit
4. **不确定就问**——不要在假设上堆代码
5. **遵循现有模式**——参见 rules/backend.md 或 rules/frontend.md

## 参考文件

- 核心规则：`.opencode/rules/core.md`
- 后端约束：`.opencode/rules/backend.md`
- 前端约束：`.opencode/rules/frontend.md`
- 测试规范：`.opencode/rules/testing.md`
- 项目架构：`.opencode/instructions/project-architecture.md`
- 测试要求：`.opencode/instructions/testing-and-api.md`
