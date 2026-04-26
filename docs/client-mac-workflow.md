# ZCopy macOS 客户端 + 后端工作流程

> 本文档梳理 ZCopy macOS 桌面客户端与 Go 后端的完整架构、启动流程、核心工作流及数据模型。
> 适用于新成员快速理解项目全貌。

---

## 目录

- [1. 整体架构](#1-整体架构)
- [2. 多进程拓扑](#2-多进程拓扑)
- [3. macOS Electron 客户端](#3-macos-electron-客户端)
- [4. Go 后端架构](#4-go-后端架构)
- [5. 数据模型](#5-数据模型)
- [6. HTTP 接口总览](#6-http-接口总览)
- [7. 同步引擎](#7-同步引擎)
- [8. 文件监听系统](#8-文件监听系统)
- [9. macOS File Provider 集成](#9-macos-file-provider-集成)
- [10. Vue 前端交互](#10-vue-前端交互)
- [11. 配置与环境变量](#11-配置与环境变量)
- [12. 构建流程](#12-构建流程)
- [13. 关键数据流](#13-关键数据流)
- [14. 已知问题与改进方向](#14-已知问题与改进方向)

---

## 1. 整体架构

ZCopy 是一个云文件同步工具，由 **Electron 桌面客户端 + Go 本地后端 + Go 远程服务端** 三层组成：

```
┌──────────────────────────────────────────────────────────────────┐
│                       ZCopy 整体架构                              │
├────────────────────────┬─────────────────────────────────────────┤
│      本地客户端         │               远程服务端                 │
│                        │                                         │
│  ┌──────────────────┐  │   HTTP/REST     ┌────────────────────┐  │
│  │  Electron Shell  │  │ ──────────────► │  Go 后端 (Gin)     │  │
│  │  (macOS/Win)     │  │   :8890         │  JWT 鉴权          │  │
│  │                  │  │ ◄────────────── │  JSON 文件数据库   │  │
│  └───────┬──────────┘  │                 └────────┬───────────┘  │
│          │             │                          │              │
│  ┌───────▼──────────┐  │                 ┌────────▼───────────┐  │
│  │   Vue 3 渲染层   │  │                 │   文件系统存储      │  │
│  │   (Composition)  │  │                 │   (按用户目录)      │  │
│  └───────┬──────────┘  │                 └────────────────────┘  │
│          │ HTTP :8090   │                                        │
│  ┌───────▼──────────┐  │                 ┌────────────────────┐  │
│  │   Go 本地后端    │  │                 │  Vue 3 Web 前端    │  │
│  │   同步引擎       │──┼───────────────► │  :5176 (开发)      │  │
│  │   文件监听       │  │                 └────────────────────┘  │
│  │   WebDAV 服务    │  │                                        │
│  └──────────────────┘  │                                        │
└────────────────────────┴─────────────────────────────────────────┘
```

### 核心职责划分

| 组件 | 职责 |
|------|------|
| **Electron Shell** | 进程管理（启动/停止 Go 后端和 Swift Host）、系统对话框、窗口管理 |
| **Vue 3 渲染层** | 用户界面：认证、任务管理、同步控制、日志查看 |
| **Go 本地后端** | REST API 服务、同步引擎、文件监听（fsnotify）、WebDAV 代理、任务持久化 |
| **Swift File Provider Host** | macOS Finder 集成：NSFileProviderDomain 注册与通信桥接 |
| **Go 远程服务端** | 用户认证（JWT）、文件 CRUD、文件存储 |

---

## 2. 多进程拓扑

macOS 客户端运行时由 **4 个进程** 协同工作：

```
┌─────────────────────────────────────────────────────────────┐
│                   macOS 客户端进程图                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  进程 1: Electron Main Process (Node.js)                    │
│  ├─ electron/main.js                                        │
│  ├─ 管理 BrowserWindow                                      │
│  ├─ IPC: dialog:pick-folder, shell:open-path                │
│  ├─ 启动进程 2 和进程 3                                     │
│  └─ will-quit 时清理所有子进程                               │
│       │                                                     │
│       ├── spawns ──► 进程 2: Go Backend (:8090)             │
│       │               ├─ REST API（Vue 调用）               │
│       │               ├─ 同步引擎 + fsnotify 监听           │
│       │               ├─ WebDAV 服务（随机端口）             │
│       │               └─ 通过 env vars 连接 File Provider   │
│       │                                                     │
│       └── spawns ──► 进程 3: ZCopyFileProviderHost.app      │
│                       (Swift 原生应用)                       │
│                       ├─ NWListener TCP 服务（随机端口）     │
│                       ├─ 启动时写入 bridge.json              │
│                       ├─ 处理: POST /register（注册域）      │
│                       ├─ 处理: GET /status（查询状态）       │
│                       └─ 包含: EleFileProvider.appex         │
│                                                             │
│  进程 4: Electron Renderer (Chromium)                       │
│  ├─ 加载 client/front/dist/index.html                       │
│  ├─ Vue 3 SPA                                              │
│  ├─ 通过 window.desktopApi 调用 IPC                         │
│  └─ HTTP 请求 → Go Backend :8090                            │
│                                                             │
│  macOS 系统:                                                │
│  ├─ Finder ←→ EleFileProvider.appex ←→ Swift Host          │
│  └─ NSFileProviderDomain 由 Swift Host 注册                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 进程间通信方式

| 通信路径 | 协议 | 端口/通道 |
|----------|------|-----------|
| Renderer → Electron Main | IPC (contextBridge) | `dialog:pick-folder`, `shell:open-path` |
| Renderer → Go Backend | HTTP REST | `:8090/api/v1/*` |
| Go Backend → Remote Server | HTTP REST | `:8890/api/v1/*` |
| Go Backend → Swift Host | HTTP + Bearer Token | `127.0.0.1:{随机}/register` |
| Finder → Go Backend | WebDAV | `127.0.0.1:{随机}/webdav/{taskID}/*` |
| Finder → Swift Host | NSFileProvider 框架 | 系统内部调用 |

---

## 3. macOS Electron 客户端

源码位置：`client/front-mac/electron/`

### 3.1 启动流程（main.js，277 行）

```
app.whenReady()
  │
  ├── 1. startFileProviderBridge()     [异步，失败不致命]
  │     ├── ensureFileProviderHostInstalled()
  │     │   └── 解压 ZCopyFileProviderHost.zip → ~/Applications/
  │     ├── 生成 UUID token
  │     ├── 创建状态目录 ~/Library/Application Support/{app}/fileprovider-host/
  │     ├── spawn(ZCopyFileProviderHost 可执行文件)
  │     │   env: ZCOPY_FILE_PROVIDER_STATE_DIR = 状态目录
  │     │   env: ZCOPY_FILE_PROVIDER_BRIDGE_TOKEN = UUID
  │     └── waitForBridgeInfo(bridge.json, token, 15s 超时)
  │         └── 每 250ms 轮询，直到文件出现且 token 匹配
  │
  ├── 2. startBackend()                [同步，失败会崩溃]
  │     ├── 校验 Go 二进制文件和 config.yaml 存在
  │     ├── 构建环境变量:
  │     │   ZCOPY_CLIENT_CONFIG = config.yaml 路径
  │     │   ZCOPY_CLIENT_DATA_DIR = userData/backend-data/
  │     │   ZCOPY_CLIENT_FP_BRIDGE_URL = bridgeState.url
  │     │   ZCOPY_CLIENT_FP_BRIDGE_TOKEN = bridgeState.token
  │     ├── 创建日志目录 userData/logs/
  │     └── spawn(Go 后端，stdout+stderr → backend.log)
  │
  ├── 3. createWindow()
  │     ├── BrowserWindow (1360×860, min 1100×700)
  │     ├── contextIsolation: true, nodeIntegration: false
  │     ├── preload.js 暴露 window.desktopApi
  │     └── 加载 client/front/dist/index.html
  │
  └── 4. 注册 F12 快捷键（切换 DevTools）
```

### 3.2 关键设计点

**File Provider Bridge 必须先于 Go 后端启动**，因为 Go 后端需要 bridge URL 和 token 作为环境变量。Bridge 启动失败是非致命的——应用仍可正常运行，只是没有 Finder 集成。

**Go 后端启动无健康检查**——启动后立即创建窗口，存在竞态条件（已知问题）。

### 3.3 IPC 桥接（preload.js，6 行）

```javascript
contextBridge.exposeInMainWorld('desktopApi', {
  pickFolder: () => ipcRenderer.invoke('dialog:pick-folder'),
  openPath: (targetPath) => ipcRenderer.invoke('shell:open-path', targetPath)
})
```

| 方法 | IPC 通道 | 功能 | 返回值 |
|------|---------|------|--------|
| `pickFolder()` | `dialog:pick-folder` | 系统目录选择器 | `string`（路径或空串） |
| `openPath(path)` | `shell:open-path` | 在 Finder 中打开 | `string`（错误信息或空串） |

### 3.4 生命周期管理

| 事件 | 行为 |
|------|------|
| `window-all-closed` | macOS 不退出（标准行为）；其他平台 `app.quit()` |
| `activate` | macOS dock 点击，无窗口时重新创建 |
| `will-quit` | 杀死 Go 后端进程和 Swift Host 进程，注销全局快捷键 |

### 3.5 运行时文件系统布局

```
~/Library/Application Support/zcopy-client-front-mac/
├── backend-data/              ← Go 后端数据
│   ├── tasks.json             ← 任务列表持久化
│   └── snapshots/             ← 按任务 ID 保存的文件指纹
│       └── {taskID}.json
├── fileprovider-host/         ← Swift Host 状态
│   ├── bridge.json            ← {url, token}
│   └── host.log
├── logs/
│   └── backend.log            ← Go 后端日志
└── webdav-cache/              ← WebDAV 临时缓存（瞬时，Close 后删除）
    └── {taskID}/

~/Applications/
└── ZCopyFileProviderHost.app/ ← 已安装的 File Provider Host
    └── Contents/
        ├── MacOS/ZCopyFileProviderHost
        └── PlugIns/EleFileProvider.appex
```

---

## 4. Go 后端架构

源码位置：`client/backend/`

### 4.1 包结构（重构后）

原来的单体 `main.go`（~1500 行）已拆分为专注的包：

```
client/backend/
├── main.go                     (113 行)  入口 + AppState 组装
├── routes.go                   (69 行)   路由注册
├── sync_task.go                (5 行)    同步适配器
├── fileprovider.go             (20 行)   File Provider 适配器
│
├── handlers_auth.go            (84 行)   认证代理
├── handlers_task.go            (131 行)  任务 CRUD
├── handlers_sync.go            (59 行)   同步触发
├── handlers_ondemand.go        (60 行)   按需同步
├── handlers_remote.go          (85 行)   远程浏览
├── handlers_log.go             (113 行)  日志查询 + 导出
├── handlers_platform.go        (194 行)  CFAPI + 系统能力
├── handlers_fileprovider.go    (31 行)   File Provider HTTP 适配
│
├── models/models.go            (86 行)   全部数据结构
├── config/
│   ├── config.go               (68 行)   配置加载 + 环境变量覆盖
│   └── config.yaml             (9 行)    默认配置
├── store/task_store.go         (105 行)  任务 JSON 持久化
├── auth/token.go               (31 行)   JWT 内存管理
├── proxy/client.go             (159 行)  远程服务 HTTP 客户端
├── sync/
│   ├── engine.go               (287 行)  同步引擎核心
│   ├── snapshot.go             (54 行)   快照加载/保存
│   └── ondemand.go             (170 行)  空间释放 + 云端回填
├── watcher/watcher.go          (116 行)  fsnotify 文件监听
├── log/
│   ├── transfer_log.go         (497 行)  环形缓冲 + 文件日志
│   └── log_test.go             (200 行)  FileLogStore 测试
├── middleware/cors.go          (25 行)   CORS 中间件
├── utils/utils.go              (71 行)   工具函数
├── fileprovider/
│   ├── service.go              (497 行)  FP 服务 + WebDAV + Bridge
│   ├── handlers.go             (274 行)  FP HTTP 处理器
│   └── webdav.go               (319 行)  WebDAV 文件系统实现
└── platform/syncroot.go        (25 行)   SyncRoot ID 生成
```

### 4.2 AppState（顶层编排器）

`AppState` 是所有依赖的组装点，通过组合接口实现松耦合：

```go
type AppState struct {
    cfg     models.AppConfig                      // 配置
    httpc   *http.Client                          // HTTP 客户端（60s 超时）
    store   *store.TaskStore                      // 任务持久化（JSON 文件）
    tokens  *auth.InMemoryTokenManager            // JWT 管理（内存）
    remote  *proxy.HTTPRemoteClient               // 远程服务代理
    logs    *logpkg.RingBufferLogStore            // 传输日志（2000 条环形缓冲）
    watcher *watcher.FSNotifyWatchManager         // 文件监听
    syncer  syncpkg.SyncEngine                    // 同步引擎
    fp      *fileproviderpkg.Service              // File Provider 服务
}
```

### 4.3 核心接口抽象

```go
// 任务持久化
type TaskRepository interface {
    List() []BackupTask
    Get(id string) (BackupTask, bool)
    Upsert(task BackupTask) error
    Remove(id string) error
}

// JWT 令牌管理
type TokenManager interface {
    GetToken() string
    SetToken(token string)
}

// 远程服务通信
type RemoteClient interface {
    RawRequest(method, path string, body io.Reader, contentType, token string) ([]byte, int, error)
    UploadFile(localFile, remoteDir, token string) error
    DownloadRemoteFile(remotePath, localPath, token string) error
    EnsureRemotePath(path, token string) error
}

// 同步引擎
type SyncEngine interface {
    SyncTask(taskID string) error
    ReleaseLocalSpace(taskID string) (ReleaseResult, error)
    HydrateFromCloud(taskID string) (BackupTask, error)
}

// 文件监听
type WatchManager interface {
    StartWatcher(taskID string, task BackupTask) error
    StopWatcher(taskID string)
    RestoreAutoWatchers()
}

// 传输日志
type LogStore interface {
    Push(level string, task BackupTask, filePath, message string)
    List(taskID, level string, limit int) []TransferLog
    Query(q ListQuery) (*ListResult, error)
    Export(q ListQuery) (io.Reader, error)
}
```

### 4.4 启动序列（main()）

```
1. slog.SetDefault()              → 配置结构化日志
2. config.LoadConfig()            → 加载 YAML + 环境变量覆盖
3. os.MkdirAll(dataDir, 0755)     → 创建数据目录
4. store.New(tasks.json 路径)     → 创建 TaskStore
5. store.Load()                   → 加载已有任务
6. &http.Client{Timeout: 60s}    → 创建 HTTP 客户端
7. auth.NewInMemoryTokenManager() → JWT 管理器
8. proxy.NewHTTPRemoteClient()    → 远程服务客户端
9. logpkg.NewRingBufferLogStore() → 日志存储（2000 条上限）
10. syncpkg.NewEngine(...)        → 同步引擎（含快照目录）
11. fileproviderpkg.New(...)      → File Provider 服务
12. 构建 AppState                → 组装所有依赖
13. watcher.NewFSNotifyWatchManager(store, app.syncTask) → 文件监听
14. app.fp.StartWebDAVServer()    → macOS: 启动随机端口 WebDAV 服务
15. app.watcher.RestoreAutoWatchers() → 恢复所有 autoBackup 任务的监听
16. gin.SetMode(cfg.Mode)         → 设置 Gin 模式
17. gin.Default() + middleware.CORS() → 创建路由器
18. registerRoutes(router, app)   → 注册全部 23 条路由
19. router.Run(":" + cfg.Port)    → 监听 :8090
```

---

## 5. 数据模型

### 5.1 AppConfig（配置）

```go
type AppConfig struct {
    Server struct {
        Port string  // 默认 "8090"
        Mode string  // 默认 "debug"
    }
    FileServer struct {
        BaseURL string  // 默认 "http://localhost:8890/api/v1"
    }
    Storage struct {
        DataDir string  // 默认 "./data"
    }
}
```

### 5.2 BackupTask（备份任务）

```go
type BackupTask struct {
    ID           string      // "task-YYYYMMDDHHmmss.nnnnnnnnn"
    Name         string      // 用户指定的任务名称
    LocalPath    string      // 本地目录绝对路径
    RemotePath   string      // 远程服务端路径
    AutoBackup   bool        // 是否启用自动备份（fsnotify 监听）
    OnDemandSync bool        // 是否启用按需同步（快照增量）
    Status       string      // "idle" | "syncing" | "error"
    LastError    string      // 最近一次错误信息
    LastSyncAt   *time.Time  // 最近同步时间
    SyncReport   *SyncReport // 当前/最近同步报告
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### 5.3 SyncReport（同步报告）

```go
type SyncReport struct {
    State            string     // "syncing" | "idle" | "failed"
    Mode             string     // "full" | "on_demand" | "on_demand_hydrate"
    Message          string
    TotalFiles       int
    UploadedFiles    int
    FailedFiles      int
    TotalBytes       int64
    TransferredBytes int64
    SpeedBytesPerSec int64
    FailedFilePaths  []string
    StartedAt        time.Time
    FinishedAt       *time.Time
}
```

### 5.4 TaskSnapshot（任务快照）

用于增量同步的文件指纹映射：

```go
type TaskSnapshot struct {
    Files map[string]FileFingerprint  // key = 相对路径
}

type FileFingerprint struct {
    Size    int64  // 文件大小（字节）
    ModUnix int64  // 修改时间 Unix 时间戳
}
```

快照存储在 `data/snapshots/{taskID}.json`，每次同步成功后更新。

### 5.5 TransferLog（传输日志）

```go
type TransferLog struct {
    ID        string    // "log-YYYYMMDDHHmmss.nnnnnnnnn"
    TaskID    string
    TaskName  string
    Level     string    // "debug" | "info" | "warn" | "error"
    FilePath  string
    Message   string
    CreatedAt time.Time
}
```

### 5.6 WatchController（监听控制器）

```go
type WatchController struct {
    StopCh chan struct{}  // 通知 watcher 停止
    DoneCh chan struct{}  // watcher 已完全停止
}
```

---

## 6. HTTP 接口总览

Go 后端在 `:8090` 提供 23 条路由：

### 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 返回 `{"status":"ok"}` |

### 认证代理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 代理注册到远程服务端 |
| POST | `/api/v1/auth/login` | 代理登录，本地保存 JWT |
| POST | `/api/v1/auth/logout` | 清空本地 JWT |
| GET | `/api/v1/auth/me` | 代理查询当前用户 |

### 任务 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/tasks` | 列出所有备份任务 |
| POST | `/api/v1/tasks` | 创建任务，校验本地目录，启动 watcher |
| PUT | `/api/v1/tasks/:id` | 更新任务，重启 watcher |
| DELETE | `/api/v1/tasks/:id` | 删除任务，停止 watcher |

### 同步操作

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/tasks/:id/sync` | 触发立即全量同步 |
| POST | `/api/v1/tasks/:id/auto/start` | 启用自动备份 + 启动 watcher |
| POST | `/api/v1/tasks/:id/auto/stop` | 停止自动备份 + 关闭 watcher |

### 按需同步

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/tasks/:id/on-demand/release` | 释放本地空间（删除与快照一致的文件） |
| POST | `/api/v1/tasks/:id/on-demand/hydrate` | 从云端下载全部文件 |
| POST | `/api/v1/tasks/:id/on-demand/cfapi/init` | 注册操作系统云文件集成 |
| GET | `/api/v1/tasks/:id/on-demand/cfapi/status` | 查询云文件集成状态 |

### 远程浏览 + 日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/remote/folders` | 列出远程目录（仅目录） |
| GET | `/api/v1/logs` | 分页查询传输日志 |
| GET | `/api/v1/logs/export` | 导出日志为 JSONL 文件 |

### File Provider

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/file-provider/tasks/:id/item` | 获取远程项目元数据 |
| GET | `/api/v1/file-provider/tasks/:id/children` | 列出目录子项 |
| GET | `/api/v1/file-provider/tasks/:id/content` | 下载文件内容（优先本地） |
| PUT | `/api/v1/file-provider/tasks/:id/content` | 上传文件内容（远程 + 本地镜像） |
| PUT | `/api/v1/file-provider/tasks/:id/rename` | 重命名（远程 + 本地） |
| POST | `/api/v1/file-provider/tasks/:id/folder` | 创建目录（远程 + 本地） |
| DELETE | `/api/v1/file-provider/tasks/:id/item` | 删除项目（远程 + 本地） |

### 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/system/capabilities` | 返回 OS、onDemandSupport、onDemandMode |

---

## 7. 同步引擎

源码位置：`client/backend/sync/`

### 7.1 SyncTask 完整流程

```
syncTask(taskID) 流程：
│
├── 1. loadTask(taskID)             → 从 store 获取任务，不存在则返回 ErrTaskNotFound
├── 2. tokens.GetToken()            → 获取 JWT，为空则返回 ErrUnauthorized
├── 3. acquire(taskID, task)        → 获取任务级互斥锁，已在同步则跳过
├── 4. [defer release(taskID)]      → 函数退出时释放锁
│
├── 5. 记录日志 "同步开始..."
├── 6. 判断同步模式：
│       OnDemandSync=true → "on_demand"（增量）
│       OnDemandSync=false → "full"（全量）
├── 7. 设置 task.Status = "syncing"，创建 SyncReport，持久化
│
├── 8. collectPendingFiles(task, report):
│       ├── utils.CollectLocalFiles(LocalPath)  → 遍历整个本地目录
│       ├── 如果增量模式: LoadSnapshot 从磁盘
│       ├── 逐文件对比 Size + ModUnix 与快照
│       ├── 跳过未变化文件（增量优化）
│       └── 构建待上传列表 + 统计总数
│
├── 9. 如果增量模式且无变更:
│       └── finishNoopSync: 状态="idle"，报告 "按需同步：无变更"
│
├── 10. EnsureRemotePath(remotePath) → 逐级创建远程目录
│
├── 11. uploadPendingFiles(...):
│       对每个待上传文件:
│       ├── 规范化远程目录路径
│       ├── UploadFile(absPath, remoteDir, token) → multipart POST
│       ├── 更新报告: UploadedFiles, TransferredBytes, Speed
│       ├── 更新 snapshot.Files[relPath] 为新指纹
│       ├── store.Upsert(task) → 持久化进度
│       └── 每 50 个文件记录一次进度百分比
│       全部完成后:
│       ├── 用完整文件列表填充快照（不仅是已上传的）
│       ├── 如果 OnDemandSync: SaveSnapshot 到磁盘
│       └── 如有失败: failSync 记录第一个错误
│
└── 12. finishSuccessfulSync → 状态="idle"，记录耗时和计数
```

### 7.2 ReleaseLocalSpace（释放本地空间）

```
releaseLocalSpace(taskID):
│
├── loadTask → ErrTaskNotFound
├── LoadSnapshot → ErrSnapshotMissing（无快照则无法对比）
├── CollectLocalFiles → 扫描当前本地状态
│
└── 遍历快照中的每个文件:
    ├── 本地不存在 → 跳过
    ├── Size 或 ModUnix 与快照不同（已修改）→ 跳过
    └── 与快照完全一致 → os.Remove(absPath)
```

返回结果包含: `ReleasedFiles`, `ReleasedBytes`, `SkippedFiles`

### 7.3 HydrateFromCloud（云端回填）

```
hydrateFromCloud(taskID):
│
├── loadTask → ErrTaskNotFound
├── GetToken → ErrUnauthorized
├── LoadSnapshot → ErrSnapshotMissing
│
├── 按字母序排序快照键
├── CollectLocalFiles → 构建当前本地文件映射
│
└── 遍历快照中的每个文件:
    ├── 本地文件指纹匹配 → 跳过（已存在）
    └── 不匹配或不存在 → DownloadRemoteFile 到本地
```

### 7.4 哨兵错误

```go
var ErrTaskNotFound    = errors.New("任务不存在")
var ErrUnauthorized    = errors.New("请先登录客户端")
var ErrSnapshotMissing = errors.New("同步快照不存在")
```

---

## 8. 文件监听系统

源码位置：`client/backend/watcher/watcher.go`

### 8.1 FSNotifyWatchManager

**字段：**
- `store` — TaskRepository，用于加载任务
- `watchers` — `map[string]*WatchController`，按 taskID 索引
- `onSync` — 回调函数 `func(taskID string) error`

### 8.2 StartWatcher 流程

```
StartWatcher(taskID, task):
│
├── StopWatcher(taskID)           → 先停止已有监听（幂等）
├── 创建 WatchController{StopCh, DoneCh}
├── 存入 watchers map
├── 启动 goroutine: runWatcher(task, ctrl)  → 文件监听循环
└── 启动 goroutine: onSync(taskID)          → 立即执行一次同步
```

### 8.3 核心监听循环（runWatcher）

```
runWatcher(task, ctrl):
│
├── defer close(DoneCh)
├── 创建 fsnotify.Watcher
├── filepath.Walk(LocalPath) → 添加所有子目录到 watcher
├── 创建 timer（初始停止，24h 虚拟时长）
│
└── SELECT 循环:
    ├── case <-StopCh:
    │     └── return（关闭信号）
    │
    ├── case evt := <-watcher.Events:
    │     ├── 如果是 Create 且为目录 → watcher.Add(newDir)
    │     ├── 设置 pending = true
    │     └── timer.Reset(2 秒)  ← 防抖重置
    │
    ├── case <-timer.C:
    │     ├── pending = false
    │     └── onSync(taskID)  ← 安静 2 秒后触发同步
    │
    └── case err := <-watcher.Errors:
          └── 记录警告日志
```

**防抖机制**：每次 fsnotify 事件重置 2 秒定时器。只有文件活动完全静默 2 秒后才触发同步。这防止了批量操作时的密集同步请求。

### 8.4 RestoreAutoWatchers（启动恢复）

启动时遍历所有任务，对 `AutoBackup == true` 的任务调用 `StartWatcher`。

---

## 9. macOS File Provider 集成

源码位置：`client/backend/fileprovider/`

### 9.1 架构总览

File Provider 是一个 **多协议桥接系统**，让 macOS Finder 能像操作本地文件一样操作远程文件：

```
macOS Finder
    │
    ▼ NSFileProvider 框架
┌─────────────────────────────────────┐
│ ZCopyFileProviderHost.app (Swift)   │
│  - 注册 NSFileProviderDomain        │
│  - 代理 Finder 请求到 Go 后端       │
└──────────────┬──────────────────────┘
               │ HTTP + Bearer Token
               ▼
┌─────────────────────────────────────┐
│ Go 后端                             │
│  ┌──────────────┐ ┌──────────────┐ │
│  │ REST API     │ │ WebDAV 服务  │ │
│  │ (7 个端点)   │ │ (随机端口)   │ │
│  └──────────────┘ └──────────────┘ │
└──────────────┬──────────────────────┘
               │ HTTP + JWT
               ▼
┌─────────────────────────────────────┐
│ 远程服务端 (:8890)                  │
└─────────────────────────────────────┘
```

### 9.2 Service 结构

```go
type Service struct {
    cfg           AppConfig
    httpc         *http.Client
    store         TaskRepository
    tokens        TokenManager
    remote        RemoteClient
    fpBridgeURL   string     // Swift Host HTTP 地址
    fpBridgeToken string     // Bridge 认证 token（UUID）
    webdavBaseURL  string    // WebDAV 服务 URL
    webdavUsername string    // WebDAV 用户名（默认 "zcopy"）
    webdavPassword string    // WebDAV 密码（随机生成或环境变量）
}
```

### 9.3 WebDAV 服务启动

```
StartWebDAVServer():
│
├── 平台检查: runtime.GOOS != "darwin" → 直接返回
├── 获取凭据: 用户名/密码（环境变量或随机生成）
├── 监听 127.0.0.1:0（随机端口）
├── 启动 http.Server（goroutine）
└── 保存 URL + 凭据到 Service 字段
```

### 9.4 remoteWebDAVFS（WebDAV 文件系统实现）

实现 `webdav.FileSystem` 接口，将远程服务器映射为本地文件系统：

| 方法 | 功能 |
|------|------|
| `Stat(ctx, name)` | 先查本地，再查远程，返回 FileInfo |
| `Mkdir(ctx, name, perm)` | 远程建目录 + 本地建镜像 |
| `OpenFile(ctx, name, flag, perm)` | **最复杂** — 读/写分流（见下文） |
| `RemoveAll(ctx, name)` | 远程删除 + 本地删除 |
| `Rename(ctx, old, new)` | 远程重命名 + 本地重命名 |

### 9.5 OpenFile 读路径（Finder 读取文件）

```
OpenFile(name, O_RDONLY, ...):
│
├── 1. 尝试本地文件 (localInfoForPath)
│   ├── 是目录 → 读取本地目录条目 → 返回 remoteWebDAVDirFile
│   ├── 是文件 → os.Open() → 返回 remoteWebDAVReadFile
│   └── 不存在 → 继续远程获取
│
├── 2. 远程获取 (remoteInfoForPath):
│   ├── 是目录 → 远程列出子项 → 返回 remoteWebDAVDirFile
│   └── 是文件:
│       ├── 下载到临时缓存: dataDir/webdav-cache/{taskID}/{base64(relPath)}
│       ├── os.Open(缓存文件)
│       └── 返回 remoteWebDAVReadFile{File, cleanupPath: 缓存路径}
│
└── 注意: Close() 时自动删除临时缓存文件（瞬时缓存）
```

**设计要点**：本地文件优先于远程，已同步/回填的文件无需网络请求。

### 9.6 OpenFile 写路径（Finder 写入文件）

```
OpenFile(name, O_WRONLY|O_CREATE|O_TRUNC, ...):
│
├── 1. 计算临时文件路径: dataDir/webdav-cache/{taskID}/{base64(relPath)}
├── 2. 创建临时文件
├── 3. 预填充内容:
│   ├── 非 O_TRUNC: 尝试从本地镜像复制，本地无则从远程下载
│   └── O_TRUNC: 空文件
└── 4. 返回 remoteWebDAVWriteFile → 实际上传在 Close() 中触发
```

### 9.7 写入关闭触发上传（remoteWebDAVWriteFile.Close）

```
Close():
│
├── sync.Once 确保只执行一次:
│   ├── Seek 到临时文件开头
│   ├── 获取 JWT token
│   ├── EnsureRemotePath(父目录)
│   ├── deleteRemotePath(已有文件)     → 覆盖旧版本
│   ├── uploadFileReader(临时文件)      → multipart POST 上传
│   ├── Seek 到开头
│   ├── writeLocalMirrorFile(...)       → 写入本地镜像
│   ├── Close 临时文件
│   └── Remove 临时文件
```

### 9.8 Bridge 通信协议

Go 后端与 Swift Host 之间通过 HTTP + Bearer Token 通信：

**注册 File Provider 域：**

```
POST {fpBridgeURL}/register
Authorization: Bearer {fpBridgeToken}

请求体:
{
    "id": "ZCopy.task-20260418120000.123456789",   // SyncRootID
    "name": "ZCopy 我的文档",                        // 显示名称
    "url": "http://127.0.0.1:52341/webdav/task-xxx", // WebDAV URL
    "user": "zcopy",                                // WebDAV 用户名
    "password": "<random-base64>"                   // WebDAV 密码
}

响应体:
{
    "registered": true,
    "reason": "...",
    "mountPath": "/Users/user/Library/...",
    "logPath": "/Users/user/Library/..."
}
```

**查询注册状态：**

```
GET {fpBridgeURL}/status?id=ZCopy.task-xxx&name=ZCopy+我的文档
Authorization: Bearer {fpBridgeToken}
```

### 9.9 认证层次

| 层次 | 机制 | 凭据来源 |
|------|------|---------|
| WebDAV → Go 后端 | HTTP Basic Auth | 随机生成或环境变量 |
| Go 后端 → Swift Host | Bearer Token | Electron 启动时生成的 UUID |
| Go 后端 → 远程服务端 | Bearer JWT | 用户登录后获取 |

WebDAV 的 Basic Auth 凭据在注册 File Provider 域时传递给 Swift Host，使其能认证到 WebDAV 服务。

---

## 10. Vue 前端交互

源码位置：`client/front/src/`

### 10.1 项目结构

```
client/front/src/
├── main.js                (6 行)   Vue 应用启动入口
├── App.vue                (45 行)  根组件 + 导航栏
├── router/index.js        (23 行)  Hash 路由，2 个路由
├── views/
│   ├── DashboardView.vue  (678 行) 全部业务逻辑（单体组件）
│   └── LogViewer.vue      (154 行) 日志页面（占位，未接入后端）
└── style.css              (408 行) 深色主题 + 玻璃态设计系统
```

### 10.2 状态管理

不使用 Vuex/Pinia，完全基于 Vue 3 Composition API 的 `ref()` 管理组件内状态。

**核心状态：**

| 变量 | 类型 | 用途 |
|------|------|------|
| `token` | `string` | JWT，持久化在 `localStorage('zcopy_token')` |
| `currentUser` | `object/null` | 当前用户信息 |
| `tasks` | `array` | 备份任务列表 |
| `logs` | `array` | 最近传输日志 |
| `capabilities` | `object/null` | 操作系统平台能力 |
| `authForm` | `object` | 登录/注册表单字段 |
| `taskForm` | `object` | 任务创建/编辑表单 |

**自动刷新**：`setInterval` 每 2.5 秒调用 `refreshDashboard()`，获取任务、日志、能力、按需同步状态。

### 10.3 API 调用（16 个端点）

| 方法 | 端点 | 触发函数 | 说明 |
|------|------|---------|------|
| POST | `/auth/register` | `submitAuth()` | 注册 |
| POST | `/auth/login` | `submitAuth()` | 登录 |
| GET | `/auth/me` | `fetchCurrentUser()` | 获取当前用户 |
| POST | `/auth/logout` | `logout()` | 登出 |
| GET | `/tasks` | `fetchTasks()` | 获取任务列表 |
| POST | `/tasks` | `saveTask()` | 创建任务 |
| PUT | `/tasks/:id` | `saveTask()` | 更新任务 |
| DELETE | `/tasks/:id` | `deleteTask()` | 删除任务 |
| POST | `/tasks/:id/sync` | `syncTask()` | 手动同步 |
| POST | `/tasks/:id/auto/start` | `toggleAutoBackup()` | 启用自动备份 |
| POST | `/tasks/:id/auto/stop` | `toggleAutoBackup()` | 停止自动备份 |
| GET | `/system/capabilities` | `fetchCapabilities()` | 系统能力 |
| GET | `/remote/folders` | `loadRemoteFolders()` | 远程目录浏览 |
| GET | `/logs` | `fetchLogs()` | 获取日志 |
| POST | `/tasks/:id/on-demand/cfapi/init` | `initTaskFileProvider()` | 初始化云集成 |
| GET | `/tasks/:id/on-demand/cfapi/status` | `fetchOnDemandStatuses()` | 云集成状态 |

### 10.4 用户交互流程

#### 认证流程
```
用户填写表单 → POST /auth/login 或 /auth/register
  → 成功: 保存 token 到 localStorage，开始轮询 dashboard
  → 失败: 显示错误消息
```

#### 任务创建
```
填写任务名 → 选择本地目录（Electron IPC / webkitdirectory）
  → 选择远程目录（远程目录浏览器模态框）→ POST /tasks
  → 如果启用按需同步: 自动调用 POST /tasks/:id/on-demand/cfapi/init
```

#### 同步触发
```
点击 "手动备份" → POST /tasks/:id/sync
  → 2.5s 轮询自动刷新 SyncReport（进度条、已上传/总文件数、速度）
```

---

## 11. 配置与环境变量

### 11.1 默认配置（config.yaml）

```yaml
server:
  port: "8090"
  mode: "debug"
file_server:
  base_url: "http://localhost:8890/api/v1"
storage:
  data_dir: "./data"
```

### 11.2 环境变量覆盖

| 变量 | 覆盖内容 | 设置者 |
|------|---------|--------|
| `ZCOPY_CLIENT_CONFIG` | 自定义配置文件路径 | Electron main.js |
| `ZCOPY_CLIENT_DATA_DIR` | 数据存储目录 | Electron main.js |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | File Provider Bridge 地址 | Electron main.js |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | Bridge 认证 Token | Electron main.js |
| `ZCOPY_CLIENT_WEBDAV_USER` | WebDAV 用户名 | 可选 |
| `ZCOPY_CLIENT_WEBDAV_PASSWORD` | WebDAV 密码 | 可选 |

### 11.3 配置文件搜索顺序

```
1. ZCOPY_CLIENT_CONFIG 环境变量指定路径
2. config/config.yaml（相对路径）
3. ./config/config.yaml
4. {可执行文件目录}/config/config.yaml
5. {可执行文件父目录}/config/config.yaml
6. {lookpath 目录}/config/config.yaml
7. 兜底: config/config.yaml
```

---

## 12. 构建流程

### 12.1 macOS 客户端构建

```bash
cd client/front-mac && npm run dist
```

完整构建流水线：

```
npm run dist
  │
  ├── 1. build:backend
  │     └── go build -C ../backend -o electron/bin/zcopy-client-backend
  │
  ├── 2. build:renderer
  │     └── node scripts/build-renderer.mjs
  │           └── 构建 client/front/ Vue 应用 → client/front/dist/
  │
  ├── 3. build:fileprovider
  │     └── node scripts/build-fileprovider.mjs
  │           └── Xcode 编译 EleFileProvider.xcodeproj → .appex
  │
  ├── 4. build:fileprovider-host
  │     └── node scripts/build-fileprovider-host.mjs
  │           └── 编译 Swift Host，嵌入 .appex，codesign，打包为 ZIP
  │
  └── 5. electron-builder --mac dmg
        └── 打包为 .dmg 安装镜像
```

### 12.2 electron-builder 资源打包

| 源 | 打包目标位置 | 用途 |
|----|-------------|------|
| `electron/bin/zcopy-client-backend` | `Resources/backend/` | Go 二进制（asarUnpack） |
| `../backend/config/config.yaml` | `Resources/backend/config/` | Go 后端配置 |
| `../front/dist/` | `Resources/renderer/` | Vue 前端构建产物 |
| `electron/fileprovider-host/ZCopyFileProviderHost.zip` | `Resources/fileprovider-host/` | Swift Host 压缩包 |

### 12.3 代码签名

`npm run sign:app` 执行 4 步签名：
1. Go 二进制签名
2. 原生模块签名（efphelper.node）
3. File Provider Extension 签名（.appex）
4. 主 App Bundle 签名

---

## 13. 关键数据流

### 13.1 完整同步流程（端到端）

```
用户点击 "手动备份"
  │
  ▼
Vue 前端 POST /api/v1/tasks/:id/sync
  │
  ▼
Go 后端 syncTask(taskID)
  ├── 获取任务锁（互斥）
  ├── 遍历本地目录，收集文件列表
  ├── 如果增量模式: 加载快照，对比指纹，过滤未变更文件
  ├── 逐级创建远程目录: POST /files/folder
  ├── 逐文件上传: POST /files/upload (multipart)
  ├── 更新快照指纹，持久化到磁盘
  └── 更新任务状态和 SyncReport
  │
  ▼
前端 2.5s 轮询自动获取最新 SyncReport
  └── 显示进度条、文件计数、传输速度
```

### 13.2 自动同步流程（fsnotify 触发）

```
本地文件变化（创建/修改/删除）
  │
  ▼
fsnotify 事件触发
  │
  ▼
2 秒防抖定时器重置
  │
  ▼ [文件活动静默 2 秒后]
  │
  ▼
onSync(taskID) → syncTask(taskID) → 与手动同步相同的流程
```

### 13.3 Finder 读取文件流程

```
用户在 Finder 中打开文件
  │
  ▼
macOS NSFileProvider 框架
  │
  ▼
ZCopyFileProviderHost.app (Swift)
  │
  ▼ GET /api/v1/file-provider/tasks/:id/content?path=...
  │
  ▼
Go 后端 Content() 处理器
  ├── 先查本地镜像: os.Stat(task.LocalPath/...)
  │   ├── 存在 → 直接返回文件 (c.File)
  │   └── 不存在 → 继续远程获取
  └── GET {remoteServer}/files/download?path=...
      → 流式返回文件内容
```

### 13.4 Finder 写入文件流程

```
用户在 Finder 中保存文件
  │
  ▼
macOS NSFileProvider 框架
  │
  ▼
ZCopyFileProviderHost.app (Swift)
  │
  ▼ WebDAV PUT /webdav/{taskID}/path/to/file
  │
  ▼
Go 后端 WebDAV 处理器
  ├── 创建临时文件
  ├── 接收写入数据
  ├── Close() 触发:
  │   ├── 上传到远程服务端 (multipart POST)
  │   └── 写入本地镜像文件
  └── 清理临时文件
```

### 13.5 认证流程

```
用户登录 (Vue 前端)
  │
  ▼ POST /api/v1/auth/login
  │
  ▼
Go 本地后端 proxyLogin()
  │
  ▼ POST http://localhost:8890/api/v1/auth/login
  │
  ▼
远程服务端验证密码 (bcrypt)
  │
  ▼ 签发 JWT (HS256, 72h 过期)
  │
  ▼
Go 本地后端提取响应中的 token
  │
  ▼ setToken() → InMemoryTokenManager 保存
  │
  ▼
后续所有远程请求自动附加 Authorization: Bearer <token>
```

---

## 14. 已知问题与改进方向

### 已知问题

| 问题 | 影响 | 位置 |
|------|------|------|
| Go 后端启动无健康检查 | Electron 窗口可能在后端就绪前加载 | `main.js` |
| Go 后端崩溃无恢复机制 | 无 `backendProcess.on('exit')` 处理 | `main.js` |
| JWT 仅存内存 | 重启后需重新登录 | `auth/token.go` |
| Bridge 失败静默处理 | 用户无感知 Finder 集成是否正常 | `main.js` |
| 日志文件无轮转 | `backend.log` 无限增长 | `main.js` |
| Vue 组件过大 | `DashboardView.vue` 678 行包含所有业务逻辑 | `client/front/src/` |
| LogViewer 未接入后端 | 仅有 UI 骨架，无实际功能 | `client/front/src/` |
| 测试覆盖不足 | 仅 `log/` 包有测试 | 全局 |

### 改进方向

1. **后端健康检查**：Go 启动后轮询 `GET /health`，100ms 间隔，10s 超时
2. **Token 持久化**：JWT 加密存储到本地文件，重启后恢复
3. **Graceful Shutdown**：SIGTERM 时等待进行中同步完成
4. **组件拆分**：将 `DashboardView.vue` 拆分为 AuthForm、TaskForm、TaskList、SyncProgress、LogPanel
5. **日志系统**：考虑使用 `FileLogStore`（已实现+测试）替代 `RingBufferLogStore`
6. **测试覆盖**：优先补充路径安全、同步引擎、TaskStore 的测试

---

## 附录：Go 后端依赖

| 包 | 版本 | 用途 |
|---|------|------|
| `gin-gonic/gin` | v1.12.0 | HTTP 框架 |
| `fsnotify/fsnotify` | v1.9.0 | 文件系统变更通知 |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML 配置解析 |
| `golang.org/x/net` | v0.51.0 | WebDAV 协议支持（间接） |
