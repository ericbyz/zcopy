# Go 项目规范

## 架构纪律

**单文件不超过 300 行。** 超过必须按职责拆包。当前 `client/backend/main.go`（1506 行）是已确认的技术债务，拆分方向：

```
client/backend/
├── main.go           ← 入口 + 路由注册（< 200 行）
├── config/           ← 配置加载
├── handlers/         ← HTTP handler（按任务/认证/同步/远程分组）
├── sync/             ← 同步引擎（核心逻辑 + 快照比对）
├── watcher/          ← fsnotify 文件监听
├── store/            ← TaskStore 持久化
├── proxy/            ← 认证代理
├── cfapi/            ← Windows Cloud Files API
└── fileprovider.go   ← macOS WebDAV + File Provider（可独立为包）
```

**函数不超过 50 行。** `syncTask()`（164 行）、`hydrateFromCloud()`（94 行）需要拆分为可独立测试的子函数。

**一个结构体不超过 3 个职责。** `AppState` 当前 6 种职责（认证代理 + 任务管理 + 同步引擎 + 文件监听 + WebDAV + CFAPI），需拆分。目标：每个领域独立结构体，通过接口解耦。

## 接口抽象（解除硬依赖）

引入以下接口，使核心逻辑可脱离 HTTP / 文件系统 / 远程服务进行测试：

```go
// 远程存储操作
type RemoteStorage interface {
    ListFiles(remotePath string) ([]RemoteFileItem, error)
    EnsureDir(remotePath string) error
    UploadFile(localPath, remotePath string) error
    DownloadFile(remotePath string) ([]byte, error)
    DeleteFile(remotePath string) error
}

// 任务持久化
type TaskRepository interface {
    List() []BackupTask
    Get(id string) (BackupTask, bool)
    Upsert(task BackupTask) error
    Delete(id string) error
}

// 同步引擎
type SyncEngine interface {
    Sync(taskID string) (*SyncReport, error)
}
```

Handler / Watcher / CLI 层依赖接口，不依赖具体实现。

## Go 风格

- Import 三组（stdlib / 第三方 / 本项目），组间空行，组内字母序
- 错误原样返回，不 `fmt.Errorf` 包装——上下文在 HTTP 层加
- 哨兵错误 `var ErrXxx = errors.New(...)` 放包级别
- JSON tag 用 camelCase；JSON 文件持久化用 `json.MarshalIndent("", "  ")`
- 只用标准库 `"log"`，不引入第三方日志库
- 时间操作强制 `Asia/Shanghai`
- 文件权限：目录 `0755`，文件 `0644`

## 安全底线

- **JWT Secret 禁止明文提交**：从环境变量读取，`config.yaml` 中不放默认值
- **`log.Fatalf` 禁止出现在工具函数中**——`utils.HashPassword()` 是已知反例，必须返回 error
- 路径安全：所有用户输入的文件路径必须过 `resolveUserPath()` 校验（`filepath.Clean` + 前缀检查）
- CORS：当前回显任意 Origin，生产环境必须改为白名单
- Token 不存内存——持久化到本地加密文件，重启后恢复
