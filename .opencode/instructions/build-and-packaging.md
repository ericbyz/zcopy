# 构建与打包规范

> 本文件定义构建命令和打包规则。
> 架构详解（启动流程、Electron 时序、多进程协作）→ 参见 `instructions/architecture.md`
> 构建相关领域专家 → 参见 `agents/infra.md`

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

## ⚠️ 打包必须使用脚本命令

**禁止手动 `go build` + `electron-builder` 拆步执行**。所有打包必须通过 `npm run` 脚本完成，脚本内部按正确顺序执行：Go 后端编译 → Vue 前端构建 → Swift 组件（macOS）→ Electron 打包。手动拆步会导致产物缺失或顺序错误。

### Windows 客户端

```bash
cd client/front
npm run dist            # 完整打包：Go 编译 + Vue 构建 + Electron（目录输出，--win dir）
npm run dist:portable   # 完整打包：Go 编译 + Vue 构建 + Electron（便携 exe）
```

### macOS 客户端

```bash
cd client/front-mac
npm run dist            # 完整打包：Go 编译 + Vue 渲染层 + FileProvider.appex + Host.app + Electron DMG
npm run dist:dir        # 完整打包：同上，输出目录而非 DMG（调试用）
npm run sign:app        # 签名：codesign 已构建的 .app（需设置 $CSC_NAME）
```

> macOS `dist` / `dist:dir` 内部自动调用 `build:backend` → `build:renderer` → `build:fileprovider` → `build:fileprovider-host` → `electron-builder`，无需先手动 `npm run build` 共享前端。

### 快捷打包 + 启动服务端

```bash
node scripts/quick-start.mjs
```

## 其他构建命令（仅限开发调试）

| 目标 | 命令 | 输出 |
|------|------|------|
| 服务端启动 | `cd server/backend && go run main.go` | `:8890` |
| 服务端 Web | `cd server/front && npm run dev` | `:5173` |
| 客户端开发 | `cd client/front && npm run dev` | Electron + Go `:8090` |
| 仅编译 Go 后端 | `cd client/front && npm run build:backend` | `electron/bin/zcopy-client-backend` |

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
- 客户端退出时停止所有 watcher、关闭 File Provider bridge 相关进程
- 使用 `os.Signal` + `context.WithCancel` 实现
