# 架构详解

> ZCopy 的系统架构、启动流程、核心子系统详细文档。

---

## 架构图

```
┌─────────────────────────────────────────────────────────┐
│                    ZCopy 架构图                          │
├───────────────┬─────────────────┬───────────────────────┤
│   客户端       │                 │        服务端          │
│               │                 │                        │
│ ┌───────────┐ │   HTTP/REST     │ ┌──────────────────┐  │
│ │ Electron  │ │ ───────────────►│ │  Go 后端         │  │
│ │ + Vue 3   │ │   :8890         │ │  (Gin + JWT)     │  │
│ │ 渲染层     │ │◄─────────────── │ │  JSON 文件数据库 │  │
│ └─────┬─────┘ │                 │ └────────┬─────────┘  │
│       │       │                 │          │            │
│ ┌─────▼─────┐ │                 │ ┌────────▼─────────┐  │
│ │ Go 客户端 │ │                 │ │  文件系统存储    │  │
│ │ 后端      │ │                 │ │  （按用户目录）  │  │
│ │ :8090     │ │                 │ └──────────────────┘  │
│ └─────┬─────┘ │                 │                        │
│       │       │                 │ ┌──────────────────┐  │
│ ┌─────▼──────┐│                 │ │  Vue 3 SPA       │  │
│ │Swift FP    ││                 │ │  :5173（开发）   │  │
│ │Extension   ││                 │ └──────────────────┘  │
│ │+ Host App  ││                 │                        │
│ └────────────┘│                 │                        │
└───────────────┴─────────────────┴───────────────────────┘
```

---

## 启动流程

### 服务端（`server/backend/main.go`）

```
1. config.LoadConfig()              → 通过 Viper 加载 config.yaml
2. utils.EnsureDirectoryExists()    → 创建存储根目录
3. database.InitDB()                → 加载/创建 JSON 数据库
4. gin.SetMode()                    → 根据配置设置 debug/release 模式
5. router := gin.Default()          → Logger + Recovery 中间件
6. corsMiddleware()                 → 自定义 CORS（回显 Origin）
7. 注册路由                         → /api/v1/auth、/api/v1/files、/api/v1/client
8. router.Run(":" + port)           → 默认监听 :8890
```

### 客户端（`client/backend/main.go`）

```
1. loadConfig()                 → 加载 YAML 配置，并支持环境变量覆盖
2. 创建数据目录
3. 初始化 TaskStore             → 读取 data/tasks.json
4. 创建 AppState 单例
5. restoreAutoWatchers()        → 恢复所有 autoBackup 任务的 fsnotify 监听
6. 在 :8090 启动 Gin 服务       → 7 组路由（auth/task/sync/ondemand/remote/file-provider/system）
```

### Electron 主进程 — Windows（`client/front/electron/main.js`）

1. 判断当前是开发模式还是打包模式
2. 解析后端可执行文件路径（`electron/bin/zcopy-client-backend.exe`）
3. 设置环境变量：`ZCOPY_CLIENT_CONFIG`、`ZCOPY_CLIENT_DATA_DIR`
4. 拉起 Go 后端进程
5. 创建 `BrowserWindow`，开发时加载 Vite，打包后加载 `dist/`
6. 绑定 DevTools 快捷键

### Electron 主进程 — macOS（`client/front-mac/electron/main.js`）

macOS 额外逻辑：
1. 加载 `electron-macos-file-provider`（可选依赖）
2. 启动动态端口的桥接 HTTP 服务
3. 注入 `ZCOPY_CLIENT_FP_BRIDGE_URL` + `ZCOPY_CLIENT_FP_BRIDGE_TOKEN`
4. 通过桥接完成 File Provider 域注册
5. 渲染层使用 `client/front/dist` 这套共享 Vue 构建产物

---

## 同步引擎

### 同步流程

```
syncTask() 流程：
1. 获取任务级互斥锁
2. 设置状态 → "syncing"
3. collectLocalFiles() → 遍历本地目录 → []LocalFileItem
4. 如果是增量模式：读取 snapshot → 用 size + modTime 对比 → 跳过未变化文件
5. ensureRemotePath() → 逐级创建远程目录（循环调用 POST /files/folder）
6. 对每个文件执行 uploadFile() → multipart POST /files/upload
7. 更新 snapshot → 持久化到 data/snapshots/{taskID}.json
8. 设置状态 → "idle"（成功）或 "error"（带失败详情）
```

### 文件监听系统

- 使用 `fsnotify` 进行实时目录监听
- **防抖**：2 秒定时器，期间每次事件都会重置，只有安静下来后才触发同步
- 动态为新建的子目录添加 watcher
- `restoreAutoWatchers()` 会在启动时恢复所有 `autoBackup` 任务

---

## macOS File Provider（四进程架构）

```
Finder  ←→  EleFileProvider.appex（Swift，NSFileProviderReplicatedExtension）
                   │
                   │ HTTP → 127.0.0.1:8090
                   ▼
            Go Backend（Gin :8090，7 个 REST 端点）
                   │
                   │ HTTP → :8890
                   ▼
            ZCopy Server（远程文件存储）

            ZCopyFileProviderHost.app（Swift，独立进程）
              • NWListener HTTP 服务（随机端口）
              • POST /register → NSFileProviderManager.add(domain)
              • GET /status → 查询域名注册状态
              • 写 bridge.json 供 Electron 读取
```

### Go 层（`client/backend/fileprovider/`）

- `service.go`：Bridge HTTP 通信 + 远程文件操作
- `handlers.go`：7 个 REST 端点，供 Swift Extension 调用（item/children/content/put/rename/folder/delete）
- `fileprovider.go`：薄适配层（20 行），委托给 `fileprovider.Service`

### Swift Extension（`client/front-mac/fileprovider/EleFileProvider/`）

- `Extension.swift`：完整实现 `NSFileProviderReplicatedExtension`（item/fetchContents/createItem/modifyItem/deleteItem）
- `FileProviderEnumerator.swift`：目录枚举器（含 working set 递归枚举）
- `FileProviderItem.swift`：`NSFileProviderItem` 协议，content policy 为 `downloadLazily`
- `FileProviderService.swift`：HTTP 客户端直连 Go 后端 :8090，含去重（8 秒窗口）和系统文件过滤
- 系统文件过滤：.DS_Store、AppleDouble（`._`）、`Icon\r`

### Swift Host App（`client/front-mac/fileprovider-host/`）

- 独立 `NSApplication`（`.prohibited` 激活策略，无 Dock 图标）
- `NWListener` 随机端口 HTTP 服务
- 域名注册：`NSFileProviderManager.add(domain)`
- 域名标识符：`ZCopy.{sanitized-task-id}`，显示名：`"ZCopy " + taskName`
- 写 `bridge.json`（`{url, token}`）供 Electron 主进程读取
- Bearer token 认证

### Electron 集成（`front-mac/electron/main.js`）

1. 解压 `ZCopyFileProviderHost.zip` → `~/Applications/ZCopyFileProviderHost.app`
2. 生成 UUID token，注入环境变量
3. 启动 Host 进程，轮询 `bridge.json`（250ms 间隔，15s 超时）
4. 将 bridge URL/token 通过环境变量传给 Go 后端
5. 退出时 kill Host + Go 进程

### Entitlements

| 文件 | 关键权限 |
|------|----------|
| App.entitlements | 沙盒、JIT、文件读写、网络客户端+服务端 |
| App-Inherit.entitlements | 沙盒 + 继承（子进程） |
| Provider.entitlements | 沙盒、application-groups（⚠️ 空数组）、网络客户端 |
| Host.entitlements | 无沙盒、网络客户端+服务端 |

---

## Windows Cloud Files API

- 使用 PowerShell 调用 `StorageProviderSyncRootManager.Register()`
- 支持渐进式 hydration、自动 dehydration、完整填充策略
- Sync Root ID 格式：`ZCopy.{sanitized-task-id}`

---

## Electron IPC 模式

```
Renderer（Vue 3）  ←→  preload.js  ←→  main.js（Electron 主进程）
       │                                      │
       │  window.desktopApi.pickFolder()      │
       │  ───────────────────────────────────►│
       │         ipcRenderer.invoke()         │
       │                                      │  dialog.showOpenDialog()
       │              返回选择路径            │
       │  ◄───────────────────────────────────│
```

- `preload.js` 向渲染层暴露 `window.desktopApi.pickFolder()`
- `main.js` 处理 `dialog:pick-folder` IPC，并弹出系统目录选择器

---

## 服务端前端（`server/front/`）

- Vue 3 SPA，使用 Composition API
- Axios 做 HTTP，请求 token 存在 `localStorage('zcopy_token')`
- 基础地址：`VITE_API_BASE_URL`，默认 `http://localhost:8890/api/v1`
- 请求拦截器自动附加 `Authorization: Bearer <token>`
- 深色主题 UI，玻璃态面板，响应式布局

### 组件

| 组件 | 用途 |
|------|------|
| `AuthCard.vue` | 登录/注册卡片 |
| `FileBrowser.vue` | 文件浏览器 |
| `LogViewer.vue` | 日志查看 |
| `ThemeToggle.vue` | 主题切换 |

### 状态管理

- 组件内状态管理（不使用 Vuex / Pinia）
- 本地状态：token、currentUser、currentPath、fileItems、loading、message、authForm、uploadFile、folderName
- `breadcrumbList` 根据 `currentPath` 计算得出

---

## 环境变量覆盖

| 变量 | 覆盖内容 |
|------|----------|
| `ZCOPY_CLIENT_CONFIG` | 自定义配置文件路径 |
| `ZCOPY_CLIENT_DATA_DIR` | 数据存储目录 |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | macOS File Provider 桥接地址 |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | macOS File Provider 桥接鉴权 token |

---

## 已知限制

- App Group entitlements 为空数组，Extension 无法通过共享容器与主 App 交换数据
- File Provider REST 端点无认证，任何本地进程可访问
- `enumerateChanges()` 为空实现，远程变更不自动刷新 Finder
- Working set 枚举递归获取整个文件树，大目录性能差

---

## 规划中的功能

| 功能 | 状态 |
|------|------|
| Token 认证 | ✅ 已实现 |
| 文件列表 / 建目录 / 上传 / 下载 / 删除 | ✅ 已实现 |
| macOS File Provider 集成 | ✅ 已实现 |
| Windows CFAPI 集成 | ✅ 已实现 |
| 目录快照比对（增量同步） | ✅ 已实现 |
| 按需同步（释放本地空间 / 水合） | ✅ 已实现 |
| 自动备份（文件监听 + 防抖） | ✅ 已实现 |
| 传输日志（环形缓冲区） | ✅ 已实现 |
| 客户端登录 + 刷新 token | 🔲 规划中 |
| 基于哈希的增量同步 | 🔲 规划中 |
| 冲突检测与处理 | 🔲 规划中 |
| 断点续传 / 断点续下 | 🔲 规划中 |
| 设备绑定 | 🔲 规划中 |
| 同步任务历史 | 🔲 规划中 |
