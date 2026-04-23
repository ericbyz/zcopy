---
name: refactor
description: "重构代码的标准化流程。确保重构不改变行为，小步前进，每步可验证。"
---

# 重构代码

## 概述

重构 = **在不改变外部行为的前提下改善内部结构**。

核心原则：
1. 小步前进，每步可验证
2. 先加测试保护网，再动手重构
3. 重构和功能开发永远不同时进行

## 前置条件

```
重构前必须确认：
- [ ] 有足够的测试覆盖（保护网）
- [ ] 测试全部通过（基线确认）
- [ ] 目标明确（改什么、为什么改）

如果测试不够 → 先补测试，再重构。
参见 instructions/testing-and-api.md
```

## 流程

### Phase 1: 分析现状

```
1. 理解当前代码结构
2. 识别要改的点：
   - AppState 上帝对象（6 种职责 → 需要拆分）
   - main.go 过长（1506 行 → 按职责拆包）
   - syncTask 函数过长（164 行 → 拆子函数）
   - 重复代码模式
3. 确定重构目标（不是"全面优化"，而是具体的改善）
4. 确认测试覆盖了会被改动的行为
```

### Phase 2: 制定重构计划

```markdown
## 重构计划

**目标**：[具体要改善什么]
**涉及文件**：[列出文件]
**重构步骤**：
  1. [最小步骤 1] → 验证：go test ./...
  2. [最小步骤 2] → 验证：go test ./...
  3. ...
**预期结果**：[改善后的结构]
**风险**：[可能影响什么]
**参考**：参见 instructions/project-architecture.md（拆分方向）
```

### Phase 3: 逐步重构

```
每一步：
1. 做最小的改动
2. 跑测试确认全部通过
3. 如果测试失败 → 撤销这一步，重新思考
4. 通过 → commit

commit message: "refactor(scope): 描述这一步做了什么"
```

### Phase 4: 验证

```
1. 全量测试通过
2. 公共 API 行为不变（集成测试）
3. 性能没有退化（如果涉及性能敏感路径）
4. Code Review
```

## 重构手法参考

| 手法 | 适用场景 | ZCopy 示例 |
|------|---------|-----------|
| 提取函数 | 函数超过 50 行 | syncTask() 164 行 → 拆为 collectChanges + uploadChanges + updateSnapshot |
| 提取结构体 | 单个结构体职责过多 | AppState → AuthProxy + TaskManager + SyncEngine |
| 移动函数 | 函数在错误的文件里 | handlers 中同步逻辑 → sync 包 |
| 引入接口 | 需要解耦或测试 | RemoteStorage、TaskRepository 接口 |
| 重命名 | 命名不清晰 | proxyRegister → Register（带上下文注释） |
| 内联函数 | 函数只被调用一次且名称没增加信息量 | 薄包装函数 |
| 分解条件表达式 | 复杂的条件判断 | 文件过滤逻辑 |

## 禁止

```
1. 重构的同时加新功能（分开做）
2. 重构的同时修 Bug（分开做）
3. 没有测试就重构
4. 一次改超过 5 个文件（拆成多步）
5. "顺便优化一下"的冲动
```

## Ask First

```
以下重构需要先确认：
- 改公共 API 签名（handler 路径、请求/响应格式）
- 跨模块的依赖调整（客户端 + 服务端同时改）
- 引入新的接口抽象
- 改目录结构
- 涉及 File Provider 通信协议
```

## 参考文件

- 架构专家：`.opencode/agents/architect.md`
- 测试规范：`.opencode/rules/testing.md`
- 项目架构：`.opencode/instructions/project-architecture.md`（拆分方向、接口抽象）
