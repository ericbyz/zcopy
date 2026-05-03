---
title: 同步引擎与 File Provider 规则
scope: "client/backend/sync/**, client/backend/fileprovider/**, client/backend/watcher/**, client/front-mac/fileprovider/**, client/backend/platform/**"
priority: CRITICAL
---

# 同步引擎与 File Provider 规则

**改动同步引擎或 File Provider 相关代码时自动生效。**

> 这是最容易让 AI 挖坑的领域。同步引擎涉及文件系统状态、多进程协作、平台特定 API，AI 很难凭静态代码推断运行时行为。

---

## 同步引擎硬性规则

### 规则 1：Snapshot 一致性

```
快照是增量同步的唯一事实来源：
- 同步开始时读取快照 → 同步结束时写入快照
- 快照写入必须在所有文件上传成功后
- 快照写入失败 → 同步状态标记为 error，不更新快照
- 快照格式：map[relativePath]FileFingerprint{Size, ModUnix}

违反 → 文件重复上传（浪费带宽）或遗漏变更（数据不一致）
```

### 规则 2：上传幂等性

```
同一个文件多次上传必须安全：
- 服务端使用 UUID 后缀处理冲突（自动重命名）
- 客户端不应假设服务端文件名 == 本地文件名
- 上传失败后重试不会导致数据损坏
```

### 规则 3：错误恢复

```
同步失败时的行为：
- 单个文件上传失败 → 记录到 FailedFilePaths，继续处理其他文件
- 全部失败 → 状态标记为 error，LastError 记录根因
- 同步被中断（进程退出）→ 下次启动重新全量同步
- 不允许在 error 状态下静默忽略失败
```

### 规则 4：文件监听防抖

```
fsnotify 事件处理：
- 2 秒防抖窗口，期间每次事件重置计时器
- 安静下来后才触发同步（不是每次事件都同步）
- 新建的子目录要动态添加 watcher
- 不要监听 .DS_Store、._ 等系统文件
```

---

## File Provider 多进程规则

### 进程链路

```
Finder  ←→  EleFileProvider.appex（Swift Extension）
                    │
                    │ HTTP → 127.0.0.1:8090
                    ▼
             Go Backend（Gin :8090）
                    │
                    │ HTTP → :8890
                    ▼
             ZCopy Server

+ ZCopyFileProviderHost.app（域名注册，独立进程）
```

### 规则 1：Bridge 通信竞态

```
bridge.json 是 Electron 和 Host App 之间的通信桥梁：
- Electron 写入 token → 启动 Host App
- Host App 写入 URL + token → bridge.json
- Electron 轮询 bridge.json（250ms 间隔，15s 超时）

风险：
- Host App 尚未写入 → Electron 读到空文件
- Host App 崩溃 → Electron 永远等不到

排查：检查 bridge.json 内容、Host App 进程状态
```

### 规则 2：REST 端点无认证

```
⚠️ 当前 File Provider 的 7 个 REST 端点无认证：
- GET/PUT /file-provider/tasks/:id/item
- GET /file-provider/tasks/:id/children
- GET/PUT /file-provider/tasks/:id/content
- PUT /file-provider/tasks/:id/rename
- POST /file-provider/tasks/:id/folder
- DELETE /file-provider/tasks/:id/item

安全建议（尚未实现）：
- 添加 Bearer token 认证
- 限制只监听 loopback 接口
```

### 规则 3：系统文件过滤

```
File Provider 枚举时必须过滤：
- .DS_Store
- AppleDouble 文件（._ 前缀）
- Icon\r（自定义图标文件）
- Thumbs.db（Windows）

违反 → 系统文件同步到服务端，浪费空间且可能触发异常
```

---

## 平台集成约束

### macOS（File Provider）

```
- Extension 运行在沙盒中，权限由 entitlements 控制
- App Group entitlements 当前为空数组 → Extension 无法通过共享容器通信
- domain 标识符格式：ZCopy.{sanitized-task-id}
- content policy: downloadLazily（按需下载）
- enumerateChanges() 当前为空实现 → 远程变更不自动刷新 Finder
```

### Windows（CFAPI）

```
- 使用 PowerShell 调用 StorageProviderSyncRootManager.Register()
- Sync Root ID 格式：ZCopy.{sanitized-task-id}
- 支持渐进式 hydration、自动 dehydration、完整填充策略
```

---

## 故障对照表

| 症状 | 最可能的原因 | 排查工具 |
|------|-------------|---------|
| **同步卡在 syncing** | 单个文件上传阻塞 / goroutine 泄漏 | `curl :8090/tasks` 查状态、`lsof -i :8890` 查连接 |
| **文件未同步** | 快照与实际不一致 / watcher 未启动 | 检查 `data/snapshots/{taskID}.json`、确认 watcher 状态 |
| **Finder 不显示文件** | Extension 未注册 / bridge.json 损坏 | 检查 bridge.json、`pluginkit -m` 查 Extension 状态 |
| **上传 401 错误** | JWT 过期 / 本地 token 丢失 | 检查 `curl :8090/auth/me`、确认 token 持久化 |
| **同步重复上传** | 快照写入失败 / modTime 精度问题 | 对比快照和实际文件的 Size + ModUnix |
| **端口冲突** | 多个 Go 后端实例 | `lsof -i :8090`、`lsof -i :8890` |
| **Host App 注册失败** | entitlements 问题 / 沙盒限制 | 检查 Host.entitlements、查看 Host App 日志 |
| **内存持续增长** | fsnotify watcher 泄漏 / 快照无限增长 | `go tool pprof heap`、检查 watcher 数量 |

---

## Evidence 收集清单（同步 Bug 调试）

```bash
# 1. 查看任务状态
curl http://localhost:8090/tasks | jq '.'

# 2. 查看同步日志
curl http://localhost:8090/logs | jq '.'

# 3. 检查快照文件
cat client/backend/data/snapshots/{taskID}.json | jq '.'

# 4. 查看文件系统实际状态
ls -la {task.LocalPath}/

# 5. 端口状态
lsof -i :8090   # 客户端 Go 后端
lsof -i :8890   # 服务端

# 6. macOS File Provider 状态
cat ~/Library/Application\ Support/ZCopy/bridge.json
pluginkit -mDp com.apple.FileProvider-NonProviding

# 7. Go 性能分析
go tool pprof http://localhost:8090/debug/pprof/heap
go tool pprof http://localhost:8090/debug/pprof/profile?seconds=30
```

---

## 性能优化原则

```
1. 先 profile 再优化（不要凭直觉）
2. 一次只改一个变量
3. 改完立刻跑 benchmark 对比
4. 优化结果记录在 docs/ 下

性能敏感路径：
- collectLocalFiles()：大目录遍历
- snapshot 对比：大量文件时的 diff 性能
- 上传并发控制：避免同时打开太多连接
```
