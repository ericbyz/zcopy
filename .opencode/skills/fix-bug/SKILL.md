---
name: fix-bug
description: "系统性修复 Bug 的流程。先提炼最小 demo，再定位根因，最后最小修复。"
---

# 修复 Bug

## 概述

> 最小可复现 demo 是被严重低估的实践。

系统性的 Bug 修复流程。核心原则：**先有 evidence，再有修复**。

## 铁律

```
没有根因分析，不允许提出修复方案。
症状修复 = 失败。
```

## 流程

```
复现 Bug → 提炼最小 demo → 定位根因 → 写回归测试 → 最小修复 → 测试通过 → 回归测试 → Code Review → 合并
```

## Step 1: 复现 Bug

```
必须确认：
1. 能可靠复现吗？
2. 复现步骤是什么？
3. 每次都出现还是偶发？
4. 影响范围（哪个平台、哪些文件类型、多大的文件）
```

如果无法复现 → 收集更多信息（日志、环境、时间模式），不要猜。

## Step 2: 提炼最小可复现 Demo

> 这是整个流程中最关键的一步。

```
目标：把 Bug 从大代码库中剥离出来，变成一个独立的最小测试用例。

为什么：
- 人：在剥离过程中排除干扰变量，理清问题本质
- AI：在干净上下文中直接分析核心逻辑，不用猜"哪部分相关"

实际效果：
- ❌ 把整个项目扔给 AI → AI 兜圈子，提不靠谱假设
- ✅ 给 AI 最小测试用例 → 迅速定位根因，给出精准修复
```

### 最小 demo 的标准

```go
// 最小 demo 应该：
// 1. 不依赖远程服务（Mock 掉 RemoteStorage）
// 2. 不依赖文件系统状态（用 t.TempDir()）
// 3. 能在几秒内运行
// 4. 清晰展示错误行为

// Example: Bug — 同步空目录导致 panic
func TestSyncTask_EmptyDirectory_ShouldNotPanic(t *testing.T) {
    dir := t.TempDir() // 空目录
    store := NewMockRemoteStorage()
    _, err := SyncTask(dir, "/remote", store)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

## Step 3: 定位根因

```
从最小 demo 出发：
1. 错误发生在哪一行？
2. 为什么会发生？（条件不满足？状态错误？）
3. 根因是什么？（不是"这行代码写错了"，而是"为什么在这里用了错误的逻辑"）

ZCopy 常见根因类别：
- 快照与文件系统不一致
- 并发竞态（mutex 保护不足）
- 路径处理错误（路径穿越、编码问题）
- HTTP 超时或重试逻辑缺失
```

## Step 4: 写回归测试

```
1. 先写一个会因为 Bug 而失败的测试
2. 确认测试确实失败（红）
3. 这个测试将永远保留在代码库中
```

## Step 5: 最小修复

```
原则：
- 只修 Bug，不顺带重构
- 只改必要的代码
- 修复后回归测试必须通过
- 全量测试必须通过
```

## Step 6: 验证

```
1. 回归测试通过（红 → 绿）
2. 全量测试通过
3. 最小 demo 不再复现 Bug
4. 边界场景也正确
```

## 修复尝试失败时

```
如果修复失败：
1. 撤销修改（git checkout / undo）
2. 回到 Step 3，重新分析根因
3. 不要在失败的修复上堆更多修复

如果连续 3 次修复失败：
1. 停止所有修改
2. 回退到干净状态
3. 重新提炼最小 demo
4. 请架构专家评估（可能是架构问题，不是代码 Bug）
```

## 参考文件

- 同步引擎规则：`.opencode/rules/sync-engine.md`
- 测试规范：`.opencode/rules/testing.md`
- 核心方法论：`.opencode/rules/core.md`（Evidence 外化工具箱）
- 故障对照表：`.opencode/rules/sync-engine.md`
