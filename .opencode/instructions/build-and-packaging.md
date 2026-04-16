# 构建与打包规范

## Git 卫生

### .gitignore 必须包含

```
*.exe
*.dll
*.so
*.dylib
data/
*.db
.env
.env.*
storage/
node_modules/
dist/
release/
```

### 禁止提交

- 二进制产物（`*.exe`、Go 编译输出）
- 密钥 / secret（JWT secret、数据库密码）
- 数据目录（`data/`、`storage/`）
- IDE 配置（`.vscode/`、`.idea/`）

已提交的二进制需 `git filter-branch` 清理历史。

## Go 版本

当前服务端 `go 1.21`、客户端 `go 1.25` 不一致。统一为项目支持的最低版本，在 `go.mod` 中声明。

## 构建命令

| 目标 | 命令 | 输出 |
|------|------|------|
| 服务端启动 | `cd server/backend && go run main.go` | `:8890` |
| 服务端 Web | `cd server/front && npm run dev` | `:5173` |
| 客户端开发 | `cd client/front && npm run dev` | Electron + Go `:8090` |
| Windows 打包 | `cd client/front && npm run dist:portable` | `.exe` |
| macOS 打包 | `cd client/front-mac && npm run dist` | `.dmg` |

## Electron 启动时序

当前 Go 后端启动后无健康检查，存在竞态。正确流程：

```
1. 启动 Go 后端进程
2. 轮询 GET /health（间隔 100ms，超时 10s）
3. 就绪后创建 BrowserWindow
4. Go 后端崩溃 → 监听 exit 事件 → 提示用户 / 自动重启
```

## Graceful Shutdown

- 服务端收到 SIGTERM 时等待进行中的同步完成
- 客户端退出时停止所有 watcher、关闭 WebDAV 服务
- 使用 `os.Signal` + `context.WithCancel` 实现
