# ZCopy - 项目知识库

> 云文件同步工具：Electron 桌面客户端 + Go 服务端。支持 macOS Finder / Windows Cloud Files 的按需同步集成。

## 架构总览

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
│ └───────────┘ │                 │                        │
│               │                 │ ┌──────────────────┐  │
│ ┌───────────┐ │                 │ │  Vue 3 SPA       │  │
│ │ Web 前端  │ │                 │ │  :5173（开发）   │  │
│ │（服务端） │ │                 │ └──────────────────┘  │
│ └───────────┘ │                 │                        │
└───────────────┴─────────────────┴───────────────────────┘
```

## Monorepo 目录结构

```
zcopy/
├── client/                    # 桌面客户端（Electron + Go）
│   ├── backend/               # Go 后端（本地 API + 同步引擎）
│   │   ├── main.go            # Gin 服务、同步引擎、文件监听、任务 CRUD（约 1500 行）
│   │   ├── fileprovider.go    # macOS WebDAV + File Provider 桥接（约 600 行）
│   │   ├── config/
│   │   │   └── config.yaml    # 本地服务配置
│   │   ├── data/              # 持久化状态
│   │   │   ├── tasks.json     # 任务存储（JSON 数组）
│   │   │   └── snapshots/     # 按任务保存的文件指纹，用于增量同步
│   │   ├── go.mod             # 模块：zcopy-client-backend（Go 1.25）
│   │   └── go.sum
│   ├── front/                 # Electron Windows 客户端（Vue 3 + Vite）
│   │   ├── electron/
│   │   │   ├── main.js        # Electron 主进程：拉起 Go 后端，加载 Vue 渲染层
│   │   │   └── preload.js     # IPC 桥：dialog:pick-folder
│   │   ├── index.html         # 渲染层入口（zh-CN）
│   │   ├── package.json       # 模块：zcopy-client-front（Electron 28 + Vue 3.4）
│   │   └── dist/              # 前端构建产物
│   └── front-mac/             # Electron macOS 客户端（File Provider 版本）
│       ├── electron/
│       │   ├── main.js        # macOS 主进程：File Provider 桥接 + 后端拉起
│       │   └── preload.js     # IPC 桥：dialog:pick-folder
│       ├── package.json       # 模块：zcopy-client-front-mac
│       └── release/           # macOS 构建输出
│
├── server/                    # 远程文件服务
│   ├── backend/               # Go API 服务
│   │   ├── main.go            # 路由注册、CORS、服务启动
│   │   ├── handlers/
│   │   │   ├── auth.go        # 注册、登录、当前用户 —— JWT 发放
│   │   │   └── file.go        # 列表、建目录、上传、下载、删除
│   │   ├── middleware/
│   │   │   └── auth.go        # AuthRequired()、CurrentUser()、JWT 校验
│   │   ├── models/
│   │   │   └── models.go      # User、File、FileVersion、Share 结构体
│   │   ├── database/
│   │   │   └── database.go    # 基于 JSON 文件的存储（虽然叫 .db，但不是 SQL）
│   │   ├── storage/           # （预留）
│   │   ├── utils/
│   │   │   └── utils.go       # 密码哈希、文件哈希、路径清洗、MIME 类型
│   │   ├── config/
│   │   │   ├── config.go      # 基于 Viper 的配置加载
│   │   │   └── config.yaml    # 默认配置
│   │   ├── data/
│   │   │   └── zcopy.db       # JSON 数据文件（users 数组）
│   │   ├── go.mod             # 模块：zcopy-server-backend（Go 1.21）
│   │   └── go.sum
│   └── front/                 # Web 前端（Vue 3 SPA）
│       ├── src/
│       │   ├── main.js        # Vue 启动入口
│       │   ├── App.vue        # 主组件：认证 + 文件浏览 + CRUD
│       │   └── style.css      # 深色主题、玻璃态面板、响应式布局
│       ├── index.html         # 入口 HTML（zh-CN）
│       ├── vite.config.js     # 端口 5173，host 0.0.0.0
│       └── package.json       # 模块：zcopy-front（Vue 3.4 + Axios）
│
└── README.md                  # （当前为空）
```

## 技术栈

| 层 | 技术 | 版本 |
|----|------|------|
| 客户端后端 | Go + Gin | Go 1.25，Gin 1.12 |
| 客户端前端 | Electron + Vue 3 | Electron 28，Vue 3.4 |
| 服务端后端 | Go + Gin | Go 1.21，Gin 1.9 |
| 服务端前端 | Vue 3 + Vite | Vue 3.4，Vite 4.5 |
| 鉴权 | JWT（HS256） | golang-jwt/jwt/v5 |
| 数据库 | JSON 文件 | 不是 SQL，内存切片 + mutex 保护 |
| 配置 | YAML + Viper | — |
| 文件监听 | fsnotify | v1.9.0 |
| macOS File Provider | electron-macos-file-provider + WebDAV | golang.org/x/net/webdav |
| Windows CFAPI | PowerShell + StorageProviderSyncRootManager | — |

## 服务端后端（`server/backend/`）

### 启动流程（`main.go`）

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

### API 接口

**认证**（`/api/v1/auth`）—— 公开接口：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/auth/register` | `Register` | 创建用户（username、email、password、nickname） |
| POST | `/auth/login` | `Login` | 使用账号（用户名或邮箱）+ 密码登录 |
| GET | `/auth/me` | `Me` | 获取当前用户（需鉴权） |

**文件**（`/api/v1/files`）—— 全部需要鉴权：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/files?path=` | `ListFiles` | 列目录（目录优先，再按名称排序） |
| POST | `/files/folder` | `CreateFolder` | 在指定路径创建目录 |
| POST | `/files/upload` | `UploadFile` | multipart 上传，冲突时自动重命名（UUID 后缀） |
| GET | `/files/download?path=` | `DownloadFile` | 下载文件，设置 Content-Type / Content-Disposition |
| DELETE | `/files?path=` | `DeleteFile` | 删除文件或目录（不可删除用户根目录） |

**客户端能力**（`/api/v1/client`）—— 需要鉴权：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/client/capabilities` | `ClientCapabilities` | 返回已实现与规划中的功能列表 |

**健康检查**：
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 返回 `{"status": "ok"}` |

### 认证流程

1. 用户注册或登录后，服务端签发 JWT（HS256，过期时间可配置，默认 72 小时）
2. Token claims：`{ user_id, exp, iat, sub: "zcopy-user" }`
3. 客户端访问受保护接口时在请求头携带 `Authorization: Bearer <token>`
4. `middleware.AuthRequired()` 校验 token，加载用户，并执行 `c.Set("user", user)`
5. 处理函数通过 `middleware.CurrentUser(c)` 读取用户
6. 密码使用 bcrypt（DefaultCost=10）哈希

### 数据模型（`models/models.go`）

**User**（当前实际使用，并持久化在 JSON 数据中）：
| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 自增主键 |
| Username | string | 唯一，最大 100 字符 |
| Email | string | 唯一，最大 255 字符，统一转小写 |
| Password | string | bcrypt 哈希值 |
| Nickname | string | 展示名，最大 100 字符 |
| Avatar | string | URL/路径（当前未使用） |
| CreatedAt / UpdatedAt / DeletedAt | time | 支持软删除 |

**File、FileVersion、Share**：目前仅定义结构，尚未真正接入逻辑（属于规划中的功能骨架）。

### 数据库层（`database/database.go`）

⚠️ **不是 SQL**，虽然文件扩展名叫 `.db`，结构体也用了类似 GORM 的标签。

- 使用 JSON 文件（`data/zcopy.db`）存储 `[]models.User`
- 通过 `sync.RWMutex` 保证线程安全
- 以内存切片为主，每次写入调用 `flushLocked()`
- 使用自增的 `nextID` 计数器
- 主要方法：`InitDB()`、`UserExists()`、`CreateUser()`、`FindUserByAccount()`、`FindUserByID()`

### 文件存储

- 每个用户使用独立目录：`{storage.root_dir}/user-{id}/`
- 文件系统本身就是唯一事实来源，不在数据库里存文件元数据
- `resolveUserPath()` 会校验最终路径仍位于用户根目录内，防止路径穿越
- multipart 最大内存限制：64 MB

### 配置（`config/config.yaml`）

```yaml
server:
  port: "8890"
  mode: "debug"          # gin 模式：debug/release/test
database:
  path: "./data/zcopy.db"
auth:
  secret_key: "zcopy-secret-key-change-in-production"
  token_expire_hours: 72
storage:
  root_dir: "./storage"
```

---

## 客户端后端（`client/backend/`）

### 启动流程（`main.go`）

```
1. loadConfig()                 → 加载 YAML 配置，并支持环境变量覆盖
2. 创建数据目录
3. 初始化 TaskStore             → 读取 data/tasks.json
4. 创建 AppState 单例
5. startWebDAVServer()          → 仅 macOS：启动随机端口 WebDAV，供 File Provider 使用
6. restoreAutoWatchers()        → 恢复所有 autoBackup 任务的 fsnotify 监听
7. 在 :8090 启动 Gin 服务
```

### 本地 API 接口

**认证代理**（转发到服务端）：
| 方法 | 路径 | 处理函数 |
|------|------|----------|
| POST | `/auth/register` | `proxyRegister` |
| POST | `/auth/login` | `proxyLogin`（本地保存 JWT） |
| POST | `/auth/logout` | `logout`（清空本地 JWT） |
| GET | `/auth/me` | `proxyMe` |

**任务 CRUD**：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/tasks` | `listTasks` | 列出所有备份任务 |
| POST | `/tasks` | `createTask` | 创建任务（校验本地目录、启动 watcher） |
| PUT | `/tasks/:id` | `updateTask` | 更新任务属性 |
| DELETE | `/tasks/:id` | `deleteTask` | 删除任务（停止 watcher） |

**同步操作**：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/tasks/:id/sync` | `syncTaskNow` | 手动立即全量同步 |
| POST | `/tasks/:id/auto/start` | `startAutoTask` | 启用自动备份并启动 watcher |
| POST | `/tasks/:id/auto/stop` | `stopAutoTask` | 停止自动备份并关闭 watcher |

**按需同步**（空间管理）：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/tasks/:id/on-demand/release` | `releaseLocalSpace` | 删除与快照一致的本地文件 |
| POST | `/tasks/:id/on-demand/hydrate` | `hydrateFromCloud` | 从服务端下载全部文件 |
| POST | `/tasks/:id/on-demand/cfapi/init` | `initTaskCFAPI` | 注册操作系统云文件集成 |
| GET | `/tasks/:id/on-demand/cfapi/status` | `taskCFAPIStatus` | 查询云文件集成状态 |

**远程浏览 + 系统信息**：
| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/remote/folders` | `listRemoteFolders` | 仅列远程目录 |
| GET | `/logs` | `listLogs` | 查看传输日志（可筛选） |
| GET | `/system/capabilities` | `systemCapabilities` | 返回操作系统与按需同步能力 |
| GET | `/health` | 内联处理 | `{"status":"ok"}` |

### 核心数据模型

**BackupTask**：ID、Name、LocalPath、RemotePath、AutoBackup、OnDemandSync、Status、LastError、LastSyncAt、SyncReport、CreatedAt、UpdatedAt

**SyncReport**：State、Mode、Message、TotalFiles、UploadedFiles、FailedFiles、TotalBytes、TransferredBytes、SpeedBytesPerSec、FailedFilePaths、StartedAt、FinishedAt

**TaskSnapshot**：`map[relativePath]FileFingerprint{Size, ModUnix}`，用于增量变化检测

**TransferLog**：内存环形缓冲区（最多 2000 条）

**TaskStore**：线程安全的 JSON 文件存储（`data/tasks.json`）

### 同步引擎

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

### macOS File Provider（`fileprovider.go`）

- 在 `127.0.0.1:{随机端口}` 启动本地 **WebDAV 服务**，供 macOS Finder 集成使用
- `remoteWebDAVFS` 实现了 `webdav.FileSystem`，本质是对远程文件进行代理
  - 读：先下载到临时缓存，再提供给 Finder
  - 写：先接收写入内容，关闭时上传到服务端
- **File Provider Bridge**：通过 HTTP 与外部 Swift 进程通信
  - `POST /register`：为任务注册 File Provider 域
  - `GET /status`：检查注册状态
- 桥接通过 `ZCOPY_CLIENT_FP_BRIDGE_URL` + `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` 配置

### Windows Cloud Files API

- 使用 PowerShell 调用 `StorageProviderSyncRootManager.Register()`
- 支持渐进式 hydration、自动 dehydration、完整填充策略
- Sync Root ID 格式：`ZCopy.{sanitized-task-id}`

### 环境变量覆盖

| 变量 | 覆盖内容 |
|------|----------|
| `ZCOPY_CLIENT_CONFIG` | 自定义配置文件路径 |
| `ZCOPY_CLIENT_DATA_DIR` | 数据存储目录 |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | macOS File Provider 桥接地址 |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | macOS File Provider 桥接鉴权 token |
| `ZCOPY_CLIENT_WEBDAV_USER` | WebDAV 凭据（macOS） |
| `ZCOPY_CLIENT_WEBDAV_PASSWORD` | WebDAV 凭据（macOS） |

---

## 客户端前端（`client/front/` + `client/front-mac/`）

### Electron IPC 模式

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

### Electron 主进程（Windows —— `client/front/electron/main.js`）

1. 判断当前是开发模式还是打包模式
2. 解析后端可执行文件路径（`electron/bin/zcopy-client-backend.exe`）
3. 设置环境变量：`ZCOPY_CLIENT_CONFIG`、`ZCOPY_CLIENT_DATA_DIR`
4. 拉起 Go 后端进程
5. 创建 `BrowserWindow`，开发时加载 Vite，打包后加载 `dist/`
6. 绑定 DevTools 快捷键

### Electron 主进程（macOS —— `client/front-mac/electron/main.js`）

macOS 额外逻辑：
1. 加载 `electron-macos-file-provider`（可选依赖）
2. 启动动态端口的桥接 HTTP 服务
3. 注入 `ZCOPY_CLIENT_FP_BRIDGE_URL` + `ZCOPY_CLIENT_FP_BRIDGE_TOKEN`
4. 通过桥接完成 File Provider 域注册
5. 渲染层使用 `client/front/dist` 这套共享 Vue 构建产物

### 构建命令

**Windows 客户端**：
```bash
cd client/front
npm run dist            # 构建后端 + Vue + Electron（目录输出）
npm run dist:portable   # 构建后端 + Vue + Electron（便携 exe）
```

**macOS 客户端**：
```bash
cd client/front         # 先构建共享 Vue 前端
npm run build
cd ../front-mac
npm run dist            # 构建后端 + Electron DMG
```

---

## 服务端前端（`server/front/`）

### 技术选型

- Vue 3 SPA，使用 Composition API
- Axios 做 HTTP，请求 token 存在 `localStorage('zcopy_token')`
- 基础地址：`VITE_API_BASE_URL`，默认 `http://localhost:8890/api/v1`
- 请求拦截器自动附加 `Authorization: Bearer <token>`
- 深色主题 UI，玻璃态面板，响应式布局

### UI 结构（`App.vue`）

- **认证区域**：登录 / 注册 Tab + 表单输入
- **文件浏览器**：面包屑导航 + 路径编辑
- **文件列表**：名称、大小、更新时间、操作（进入目录、下载、删除）
- **工具栏**：上传文件、创建目录

### 状态管理

- 组件内状态管理（不使用 Vuex / Pinia）
- 本地状态包括：token、currentUser、currentPath、fileItems、loading、message、authForm、uploadFile、folderName
- `breadcrumbList` 根据 `currentPath` 计算得出

---

## 规划中的功能（来自 ClientCapabilities 接口）

| 功能 | 状态 |
|------|------|
| Token 认证 | ✅ 已实现 |
| 文件列表 / 建目录 / 上传 / 下载 / 删除 | ✅ 已实现 |
| 客户端登录 + 刷新 token | 🔲 规划中 |
| 目录快照比对 | 🔲 规划中 |
| 基于哈希的增量同步 | 🔲 规划中 |
| 冲突检测与处理 | 🔲 规划中 |
| 断点续传 / 断点续下 | 🔲 规划中 |
| 设备绑定 | 🔲 规划中 |
| 同步任务历史 | 🔲 规划中 |

---

## 开发速查

### 前置要求

- Go 1.21+（服务端），Go 1.25+（客户端后端）
- Node.js（Vue / Electron）
- 平台依赖：macOS 需要 Xcode 才能构建 File Provider；Windows 需要 PowerShell 支持 CFAPI

### 启动服务端

```bash
cd server/backend && go run main.go          # API 监听 :8890
cd server/front && npm run dev               # Web UI 监听 :5173
```

### 启动客户端（开发模式）

```bash
cd client/front && npm run dev               # 启动 Vite + Electron + Go 后端
cd client/front-mac && npm run start         # macOS Electron（需先构建 Vue 前端）
```

### 构建客户端二进制

```bash
cd client/front && npm run build:backend     # 编译 Go → electron/bin/zcopy-client-backend.exe
```

### 端口一览

| 服务 | 端口 | 用途 |
|------|------|------|
| 服务端 API | 8890 | REST API |
| 服务端 Web UI | 5173 | Vue SPA（开发） |
| 客户端后端 | 8090 | Electron 本地 API |
| 客户端 Vite | 5173 | Vue 开发服务器 |
| WebDAV | 随机端口 | macOS File Provider |

---

## 关键约定

- **语言**：UI 默认使用中文（zh-CN）
- **时区**：服务端启动时强制设置为 `Asia/Shanghai`
- **数据库**：JSON 文件 + 内存切片 + mutex，不是 SQL，虽然扩展名叫 `.db`
- **文件存储**：文件系统是唯一事实来源，不在数据库中保存文件元数据
- **路径安全**：`resolveUserPath()` 与 `CleanRelativePath()` 防止路径穿越
- **鉴权**：JWT Bearer token；Web 前端保存在 localStorage；客户端后端保存在本地状态中
- **同步策略**：基于快照的增量同步（size + modTime 对比）+ 文件监听防抖
- **平台差异**：macOS 使用 WebDAV + File Provider bridge；Windows 使用 PowerShell 调用 CFAPI
