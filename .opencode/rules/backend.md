---
title: 后端开发规则
scope: "client/backend/**/*.go, server/backend/**/*.go"
priority: HIGH
---

# 后端开发规则

**改动 Go 后端代码时自动生效。**

> 参见 instructions/project-architecture.md（Go 风格规范、接口抽象、安全底线）

---

## Gin API 设计

### 接口契约

```
每个 API 端点必须定义：
1. 请求参数（类型 + 校验规则 + 必填/选填）
2. 响应格式（成功 + 每种错误场景）
3. HTTP 状态码（200/400/401/404/500）
4. 参见 instructions/testing-and-api.md（OpenAPI 规范）
```

### Handler 模式

```go
// ✅ 正确：薄 handler，业务逻辑在独立函数中
func (s *AppState) listTasks(c *gin.Context) {
    tasks := s.Store.List()
    c.JSON(http.StatusOK, gin.H{"items": tasks})
}

// ❌ 错误：在 handler 里写同步引擎逻辑
func (s *AppState) syncTaskNow(c *gin.Context) {
    // 100 行同步逻辑... → 应该调用 sync.SyncTask()
}
```

### 错误响应格式

```
统一结构：
- 成功：{ "message": "操作描述", "data": {...} }
- 列表：{ "items": [...], "total": 100 }
- 错误：{ "message": "中文错误描述" }

状态码严格限定：200 / 201 / 204 / 400 / 401 / 404 / 409 / 500 / 502
```

### 禁止

- 在 Handler 层写同步引擎或文件操作逻辑
- 在 Service 层直接操作 `gin.Context`
- 返回裸 `panic` 或 `log.Fatalf`（用 `c.JSON(500, ...)`）

---

## JSON 文件数据库

> ZCopy 使用 JSON 文件 + 内存切片 + mutex，不是 SQL。

### 查询规范

```
必须：
- 所有读写通过 mutex 保护（RWMutex：读用 RLock，写用 Lock）
- 写操作后调用 flushLocked() 持久化
- 使用自增 ID，不要用随机数
- 大量数据用分页，不要一次全加载到内存

禁止：
- 不加锁直接读写内存切片
- 在持锁状态下做网络 I/O（会阻塞其他操作）
- 修改数据结构后忘记调用 flushLocked()
```

### 数据安全

```
虽然不是 SQL，但仍有安全风险：
- 路径穿越：所有用户输入的路径必须过 resolveUserPath() 校验
- 数据完整性：写入时用 json.MarshalIndent 保持可读性
- 并发安全：TaskStore 的每个方法都要考虑并发场景
```

### 迁移（结构变更）

```
1. 新增字段用 `json:"field,omitempty"` 保持向后兼容
2. 不要删除已有字段的 JSON tag
3. 结构变更后手动更新 data/ 下的 JSON 文件
```

---

## 并发与 Goroutine

```
原则：
- 共享状态用 sync.RWMutex 或原子操作保护
- Goroutine 必须有退出机制（context cancellation 或 done channel）
- 不要假设执行顺序（除非有同步机制）
- 防止 goroutine 泄漏：每个 go func() 都要有退出路径

ZCopy 特有并发场景：
- syncTask() 持有任务级互斥锁 → 不允许同一任务并发同步
- fsnotify watcher + 防抖定时器 → 2 秒窗口内事件合并
- TaskStore 读写 → RWMutex 保护
- AppState 多 handler 并发访问 → 各字段独立加锁

死锁排查：
1. 画出锁的依赖图
2. 确保加锁顺序一致
3. 不要在持锁状态下请求另一个锁
```

---

## 日志规范

```go
// ✅ 正确：标准库 log，带上下文
log.Printf("[Sync] Task %s: uploaded %d/%d files, %d failed", taskID, uploaded, total, failed)

// ❌ 错误：fmt.Println 或无上下文日志
fmt.Println("upload failed")
```

### 日志级别

| 级别 | 用途 |
|------|------|
| log.Printf | 关键业务事件（同步完成/失败、任务创建、认证操作） |
| log.Debugf | 调试信息（文件对比详情、快照差异） |

> 项目约定只用标准库 `"log"`，不引入第三方日志库。
> 参见 instructions/project-architecture.md（Go 风格规范）

---

## 认证代理

```
客户端后端的认证 handler 是代理模式：
1. 接收前端请求 → 转发到服务端 :8890
2. 登录成功后本地保存 JWT
3. 后续请求自动附加 Authorization header

注意：
- Token 持久化到本地加密文件（当前仅存内存，需改进）
- 登出时清空本地 Token
- Token 过期后自动引导重新登录
```
