---
title: 测试规范
scope: "**/*_test.go, **/__tests__/**, **/*.test.js, **/*.spec.js"
priority: HIGH
---

# 测试规范

**所有测试文件的编写规范。**

> 参见 instructions/testing-and-api.md（必须测试的模块清单、OpenAPI 规范）

---

## 核心原则

> 测试是和 AI 之间的合同条款。你的测试质量 = AI 输出质量的天花板。

### 测试三定律

1. **先写测试**（TDD）：红 → 绿 → 重构
2. **每个测试独立**：不依赖执行顺序，不依赖外部状态
3. **测试文件镜像源码结构**：`sync/engine.go` → `sync/engine_test.go`

---

## Go 测试模式

### 表驱动测试（优先）

```go
func TestResolveUserPath(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"normal path", "docs/file.txt", "docs/file.txt", false},
        {"traversal", "../../etc/passwd", "", true},
        {"empty", "", "", true},
        {"null byte", "file\x00.txt", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := resolveUserPath(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("unexpected error: %v", err)
            }
            if got != tt.want {
                t.Fatalf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### 测试工具

```
必须使用：
- t.TempDir() 创建临时目录（不硬编码路径）
- t.Parallel() 标记可并行测试
- t.Cleanup() 清理资源
- t.Fatal / t.Errorf 做断言（不引入 testify 等第三方库）

Mock 方式：
- 通过接口注入依赖（不 mock 具体实现）
- 参见 instructions/project-architecture.md（接口抽象）
```

### 测试分类

| 分类 | 位置 | 用途 | 运行频率 |
|------|------|------|---------|
| 单元测试 | `*_test.go`（同目录） | 测试单个函数/方法 | 每次改动 |
| 集成测试 | `*_integration_test.go` | 测试模块间交互 | 每次提交 |
| 端到端测试 | `tests/e2e/` | 测试完整流程 | 合入主分支前 |

---

## Vitest 测试模式（前端）

### 组件测试

```javascript
// TaskCard.spec.js
import { mount } from '@vue/test-utils'
import TaskCard from './TaskCard.vue'

describe('TaskCard', () => {
  it('should emit sync event when button clicked', async () => {
    const wrapper = mount(TaskCard, {
      props: { task: { id: '1', name: 'Test', status: 'idle' } }
    })
    await wrapper.find('.sync-btn').trigger('click')
    expect(wrapper.emitted('sync')).toBeTruthy()
  })
})
```

### 表驱动测试

```javascript
test.each([
  ['idle', '开始同步'],
  ['syncing', '同步中...'],
  ['error', '重试'],
])('should show correct action for status %s', (status, expectedText) => {
  const wrapper = mount(TaskCard, {
    props: { task: { id: '1', name: 'Test', status } }
  })
  expect(wrapper.find('.action-btn').text()).toBe(expectedText)
})
```

---

## 测试命名

```go
// ✅ 正确：描述具体行为和预期
func TestSyncTask_WithUnchangedFiles_ShouldSkipUpload(t *testing.T) { ... }
func TestCreateTask_WithInvalidLocalPath_ShouldReturn400(t *testing.T) { ... }

// ❌ 错误：模糊不清
func TestSync(t *testing.T) { ... }
func TestWorks(t *testing.T) { ... }
```

---

## 必须覆盖的场景

每个功能至少覆盖：

```
1. Happy path（正常流程）
2. 边界条件（空输入、最大值、零值、nil/undefined）
3. 错误路径（无效输入、资源不存在、网络超时）
4. 并发场景（如果适用，如 TaskStore 并发读写）
```

### ZCopy 必须测试的模块

```
参见 instructions/testing-and-api.md 获取完整清单，核心包括：
- resolveUserPath() / CleanRelativePath() — 路径穿越
- 认证流程 — 注册/登录/token 过期
- 同步引擎 — 快照比对、增量跳过
- TaskStore — CRUD + 并发 + JSON 持久化
- 文件上传 — 大小限制、冲突重命名
```

---

## 回归测试规则

> 每次发现 Bug，先写回归测试再修复。

```go
// Bug: 同步空目录导致 panic
// Step 1: 写回归测试（应该失败）
func TestSyncTask_EmptyDirectory_ShouldNotPanic(t *testing.T) {
    dir := t.TempDir() // 空目录
    _, err := syncTask(dir, "/remote/path")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

// Step 2: 修复 Bug
// Step 3: 确认测试通过
// Step 4: 永远保留这个回归测试
```

---

## Mock 规范

```
可以 Mock：
- 远程服务（HTTP 客户端，通过 RemoteStorage 接口）
- 文件系统（通过 t.TempDir() 隔离）
- 时间相关函数

不要 Mock：
- 被测函数本身（这等于没测）
- 简单的数据结构（BackupTask、SyncReport）
- 你正在实现的逻辑
```

---

## 测试覆盖率标准

| 模块类型 | 最低覆盖率 | 理想覆盖率 |
|---------|-----------|-----------|
| 同步引擎（sync/） | 80% | 95% |
| API Handler（handlers_*.go） | 70% | 90% |
| 工具函数（utils/） | 90% | 100% |
| 数据存储（store/） | 80% | 95% |
| 文件提供器（fileprovider/） | 60% | 80% |
| Vue 组件 | 60% | 80% |

---

## AI 辅助测试的场景

| 场景 | AI 擅长度 | 说明 |
|------|----------|------|
| 补充边界测试 | ⭐⭐⭐ | 给 AI 函数签名和已有测试，补充遗漏的边界 |
| 生成 Mock 数据 | ⭐⭐⭐ | 固定模式，AI 很擅长 |
| 写回归测试 | ⭐⭐ | 给 AI 最小可复现脚本，让它写测试 |
| 写集成测试 | ⭐ | 需要人明确组件间的交互契约 |
