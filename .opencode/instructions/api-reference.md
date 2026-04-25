# API 参考文档

> ZCopy 的完整 API 接口定义，包括服务端和客户端本地 API。

---

## 服务端 API（`:8890`）

### 认证（`/api/v1/auth`）—— 公开接口

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/auth/register` | `Register` | 创建用户（username、email、password、nickname） |
| POST | `/auth/login` | `Login` | 使用账号（用户名或邮箱）+ 密码登录 |
| GET | `/auth/me` | `Me` | 获取当前用户（需鉴权） |

### 文件（`/api/v1/files`）—— 需要鉴权

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/files?path=` | `ListFiles` | 列目录（目录优先，再按名称排序） |
| POST | `/files/folder` | `CreateFolder` | 在指定路径创建目录 |
| POST | `/files/upload` | `UploadFile` | multipart 上传，冲突时自动重命名（UUID 后缀） |
| GET | `/files/download?path=` | `DownloadFile` | 下载文件，设置 Content-Type / Content-Disposition |
| DELETE | `/files?path=` | `DeleteFile` | 删除文件或目录（不可删除用户根目录） |

### 客户端能力（`/api/v1/client`）—— 需要鉴权

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/client/capabilities` | `ClientCapabilities` | 返回已实现与规划中的功能列表 |

### 健康检查

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

### 响应格式

```
成功：{ "message": "操作描述", "data": {...} }
列表：{ "items": [...], "total": 100 }
错误：{ "message": "中文错误描述" }
```

状态码：200 / 201 / 204 / 400 / 401 / 404 / 409 / 500 / 502

---

## 客户端本地 API（`:8090`）

### 认证代理（转发到服务端）

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/auth/register` | `proxyRegister` | 转发注册 |
| POST | `/auth/login` | `proxyLogin` | 转发登录（本地保存 JWT） |
| POST | `/auth/logout` | `logout` | 清空本地 JWT |
| GET | `/auth/me` | `proxyMe` | 转发获取当前用户 |

### 任务 CRUD

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/tasks` | `listTasks` | 列出所有备份任务 |
| POST | `/tasks` | `createTask` | 创建任务（校验本地目录、启动 watcher） |
| PUT | `/tasks/:id` | `updateTask` | 更新任务属性 |
| DELETE | `/tasks/:id` | `deleteTask` | 删除任务（停止 watcher） |

### 同步操作

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/tasks/:id/sync` | `syncTaskNow` | 手动立即全量同步 |
| POST | `/tasks/:id/auto/start` | `startAutoTask` | 启用自动备份并启动 watcher |
| POST | `/tasks/:id/auto/stop` | `stopAutoTask` | 停止自动备份并关闭 watcher |

### 按需同步（空间管理）

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| POST | `/tasks/:id/on-demand/release` | `releaseLocalSpace` | 删除与快照一致的本地文件 |
| POST | `/tasks/:id/on-demand/hydrate` | `hydrateFromCloud` | 从服务端下载全部文件 |
| POST | `/tasks/:id/on-demand/cfapi/init` | `initTaskCFAPI` | 注册操作系统云文件集成 |
| GET | `/tasks/:id/on-demand/cfapi/status` | `taskCFAPIStatus` | 查询云文件集成状态 |

### 远程浏览 + 系统信息

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/remote/folders` | `listRemoteFolders` | 仅列远程目录 |
| GET | `/logs` | `listLogs` | 查看传输日志（可筛选） |
| GET | `/logs/export` | `exportLogs` | 导出传输日志 |
| GET | `/system/capabilities` | `systemCapabilities` | 返回操作系统与按需同步能力 |
| GET | `/health` | 内联处理 | `{"status":"ok"}` |

### File Provider（供 macOS Swift Extension 调用）

⚠️ 当前无认证，任何本地进程可访问。

| 方法 | 路径 | 处理函数 | 说明 |
|------|------|----------|------|
| GET | `/file-provider/tasks/:id/item` | `fileProviderItem` | 获取文件/目录元数据 |
| GET | `/file-provider/tasks/:id/children` | `fileProviderChildren` | 列出目录子项 |
| GET | `/file-provider/tasks/:id/content` | `fileProviderContent` | 下载文件内容（流式） |
| PUT | `/file-provider/tasks/:id/content` | `fileProviderPutContent` | 上传文件（远程 + 本地镜像） |
| PUT | `/file-provider/tasks/:id/rename` | `fileProviderRenameItem` | 重命名文件/目录 |
| POST | `/file-provider/tasks/:id/folder` | `fileProviderCreateFolder` | 创建目录（远程 + 本地） |
| DELETE | `/file-provider/tasks/:id/item` | `fileProviderDeleteItem` | 删除文件/目录 |

---

## 数据模型

### 服务端模型（`server/backend/models/models.go`）

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

### 客户端模型（`client/backend/models/models.go`）

**BackupTask**：ID、Name、LocalPath、RemotePath、AutoBackup、OnDemandSync、Status、LastError、LastSyncAt、SyncReport、CreatedAt、UpdatedAt

**SyncReport**：State、Mode、Message、TotalFiles、UploadedFiles、FailedFiles、TotalBytes、TransferredBytes、SpeedBytesPerSec、FailedFilePaths、StartedAt、FinishedAt

**TaskSnapshot**：`map[relativePath]FileFingerprint{Size, ModUnix}`，用于增量变化检测

**TransferLog**：内存环形缓冲区（最多 2000 条）

**TaskStore**：线程安全的 JSON 文件存储（`data/tasks.json`）

### 服务端数据库层（`server/backend/database/database.go`）

⚠️ **不是 SQL**，虽然文件扩展名叫 `.db`，结构体也用了类似 GORM 的标签。

- 使用 JSON 文件（`data/zcopy.db`）存储 `[]models.User`
- 通过 `sync.RWMutex` 保证线程安全
- 以内存切片为主，每次写入调用 `flushLocked()`
- 使用自增的 `nextID` 计数器
- 主要方法：`InitDB()`、`UserExists()`、`CreateUser()`、`FindUserByAccount()`、`FindUserByID()`

### 服务端文件存储

- 每个用户使用独立目录：`{storage.root_dir}/user-{id}/`
- 文件系统本身就是唯一事实来源，不在数据库里存文件元数据
- `resolveUserPath()` 会校验最终路径仍位于用户根目录内，防止路径穿越
- multipart 最大内存限制：64 MB

### 服务端配置（`server/backend/config/config.yaml`）

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
