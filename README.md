<h1 align="center">ZCopy</h1>

<p align="center">
  <strong>云文件同步工具 / Cloud File Sync Tool</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?logo=vue.js" alt="Vue" />
  <img src="https://img.shields.io/badge/Electron-28-47848F?logo=electron" alt="Electron" />
  <img src="https://img.shields.io/badge/Swift-5-F05138?logo=swift" alt="Swift" />
  <img src="https://img.shields.io/badge/License-MIT-blue" alt="License" />
</p>

<p align="center">
  <a href="#english">English</a> | <a href="#中文">中文</a>
</p>

---

<a id="english"></a>

## Overview

ZCopy is a cross-platform cloud file synchronization tool consisting of a **Go file server** and an **Electron desktop client**. It supports macOS Finder and Windows Cloud Files native integration via on-demand sync — files appear in your file explorer without taking up local disk space until you actually open them.

### Key Features

- **Incremental Sync** — Snapshot-based change detection (size + modification time) with automatic deduplication
- **Native OS Integration** — macOS File Provider (Finder) and Windows CFAPI (Cloud Files API) for transparent on-demand sync
- **Real-time Backup** — File system watcher with 2-second debounce for automatic backup on change
- **Multi-Server Support** — Connect to multiple file servers with SSDP LAN discovery and automatic server identification (UUID)
- **On-Demand Sync** — Free local space by releasing synced files; re-download (hydrate) on demand
- **Client Heartbeat** — Online monitoring with 10s heartbeat interval and session management
- **Docker Deployment** — Production-ready Docker Compose configuration with health checks

### Architecture

```
┌──────────────────────────────┐         ┌──────────────────────────────┐
│         Desktop Client       │  HTTP   │        File Server           │
│                              │ ◄═════► │                              │
│  ┌─────────┐  ┌───────────┐ │  :8890  │  ┌────────────────────────┐  │
│  │Electron │  │  Go Local │ │         │  │  Go API (Gin + JWT)    │  │
│  │ + Vue 3 │  │  Backend  │ │         │  │  JSON File Database    │  │
│  └─────────┘  │  :8090     │ │         │  └────────────────────────┘  │
│               └───────────┘ │         │  ┌────────────────────────┐  │
│  ┌───────────────────────┐  │         │  │  Vue 3 SPA Frontend   │  │
│  │macOS: File Provider   │  │         │  └────────────────────────┘  │
│  │Windows: CFAPI         │  │         │                              │
│  └───────────────────────┘  │         │                              │
└──────────────────────────────┘         └──────────────────────────────┘
```

### Tech Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Server Backend | Go 1.21 + Gin 1.9 | REST API, JWT (HS256) auth |
| Server Frontend | Vue 3 + Vite | SPA with dark theme |
| Client Backend | Go 1.25 + Gin 1.12 | Local API, sync engine, file watcher |
| Client Frontend | Electron 28 + Vue 3 | Desktop app (Windows & macOS) |
| macOS Integration | Swift Extension + Host App | NSFileProviderReplicatedExtension |
| Windows Integration | PowerShell + CFAPI | StorageProviderSyncRootManager |
| Database | JSON files + mutex | No SQL dependency |
| Authentication | JWT (HS256) + bcrypt | Token-based, 72h expiry |

---

## Getting Started

### Prerequisites

- **Go** 1.21+ (server) / 1.25+ (client backend)
- **Node.js** 18+ (for Vue / Electron builds)
- **macOS**: Xcode + Swift toolchain (for File Provider)
- **Windows**: PowerShell 5.1+ (for CFAPI)

### Quick Start

The easiest way to start both server and client:

```bash
node scripts/quick-start.mjs
```

### Start Server

```bash
# Backend API (port 8890)
cd server/backend
go run main.go

# Web frontend (port 5176, dev mode)
cd server/front
npm install
npm run dev
```

### Start Client

**Windows:**

```bash
cd client/front
npm install
npm run dev          # Dev mode: Electron + Go backend on :8090
```

**macOS:**

```bash
cd client/front-mac
npm install
npm run start        # Dev mode: Electron + Go backend + File Provider
```

### Docker Deployment

```bash
docker compose up -d
```

This starts two server instances (ports 8890, 8891) with their corresponding frontends (ports 18176, 28176), all with health checks and persistent volumes.

### Build for Distribution

```bash
# Windows client (directory output)
cd client/front
npm run dist

# Windows client (portable .exe)
cd client/front
npm run dist:portable

# macOS client (.dmg)
cd client/front-mac
npm run dist

# macOS client (directory output, for debugging)
cd client/front-mac
npm run dist:dir
```

> ⚠️ Always use `npm run dist` scripts. Do NOT manually run `go build` + `electron-builder` separately — the scripts handle the correct build order.

---

## Configuration

### Server Configuration (`server/backend/config/config.yaml`)

```yaml
server:
  port: "8890"
  mode: "debug"          # debug / release / test
database:
  path: "./data/zcopy.db"
auth:
  secret_key: "change-me-in-production"
  token_expire_hours: 72
storage:
  root_dir: "./storage"
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ZCOPY_CLIENT_CONFIG` | Custom config file path (client) |
| `ZCOPY_CLIENT_DATA_DIR` | Data storage directory (client) |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | macOS File Provider bridge URL |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | macOS File Provider bridge auth token |
| `ZCOPY_CORS_ORIGINS` | Allowed CORS origins (server, Docker) |
| `ZCOPY_SERVER_CONFIG` | Custom server config path (Docker) |

### Ports

| Service | Port | Purpose |
|---------|------|---------|
| Server API | 8890 | REST API |
| Server Web UI | 5176 | Vue SPA (dev) |
| Client Backend | 8090 | Local REST API |
| Client Vite | 5173 | Vue dev server |
| FP Host App | random | macOS File Provider bridge |

---

## API Overview

### Server API (`:8890`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register new user |
| POST | `/api/v1/auth/login` | Login, returns JWT |
| GET | `/api/v1/auth/me` | Get current user (auth required) |
| GET | `/api/v1/files?path=` | List directory |
| POST | `/api/v1/files/upload` | Upload file (multipart) |
| GET | `/api/v1/files/download?path=` | Download file |
| POST | `/api/v1/files/folder` | Create directory |
| DELETE | `/api/v1/files?path=` | Delete file or directory |
| GET | `/api/v1/server/info` | Server UUID & info (public) |
| POST | `/api/v1/client/heartbeat` | Client heartbeat (auth required) |
| GET | `/health` | Health check |

### Client Local API (`:8090`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET / POST / PUT / DELETE | `/tasks` | Backup task CRUD |
| POST | `/tasks/:id/sync` | Manual full sync |
| POST | `/tasks/:id/auto/start` | Enable auto-backup + file watcher |
| POST | `/tasks/:id/on-demand/release` | Free local space |
| POST | `/tasks/:id/on-demand/hydrate` | Re-download all files |
| GET / POST | `/servers` | Server management + SSDP scan |

---

## Project Structure

```
zcopy/
├── server/                     # File server
│   ├── backend/                #   Go API (Gin + JWT)
│   └── front/                  #   Vue 3 SPA
├── client/                     # Desktop client
│   ├── backend/                #   Go local API + sync engine
│   ├── front/                  #   Windows Electron app (Vue 3)
│   └── front-mac/              #   macOS Electron app + Swift File Provider
├── scripts/                    # Build & quick-start scripts
├── docker-compose.yaml         # Multi-server Docker deployment
└── docs/                       # Documentation
```

---

## Roadmap

- [x] Token authentication (JWT)
- [x] File CRUD (list / upload / download / delete)
- [x] macOS File Provider integration
- [x] Windows CFAPI integration
- [x] Snapshot-based incremental sync
- [x] On-demand sync (release / hydrate)
- [x] Auto-backup with file watcher
- [x] Transfer logs (ring buffer)
- [x] Multi-server support
- [x] SSDP LAN discovery
- [x] Client heartbeat + online monitoring
- [ ] Refresh token rotation
- [ ] Hash-based incremental sync
- [ ] Conflict detection & resolution
- [ ] Resumable upload / download
- [ ] Device binding
- [ ] Sync task history
- [ ] Multi-server cluster backup

---

<a id="中文"></a>

## 概述

ZCopy 是一个跨平台云文件同步工具，由 **Go 文件服务器** 和 **Electron 桌面客户端** 组成。支持 macOS Finder 和 Windows 云文件的原生集成，实现按需同步——文件在文件管理器中可见但不占用本地空间，打开时才从服务器下载。

### 核心特性

- **增量同步** — 基于快照的变化检测（文件大小 + 修改时间），自动跳过未变化的文件
- **原生系统集成** — macOS File Provider（Finder 集成）和 Windows CFAPI（云文件 API）实现透明按需同步
- **实时备份** — 文件系统监听器，2 秒防抖，文件变动自动触发同步
- **多服务器支持** — 连接多个文件服务器，SSDP 局域网自动发现，UUID 服务器标识
- **按需同步** — 释放已同步文件的本地空间；需要时从服务器重新下载（水合）
- **客户端心跳** — 10 秒间隔心跳，在线状态监控与会话管理
- **Docker 部署** — 开箱即用的 Docker Compose 配置，含健康检查

### 技术栈

| 层 | 技术 | 说明 |
|---|------|------|
| 服务端后端 | Go 1.21 + Gin 1.9 | REST API，JWT (HS256) 鉴权 |
| 服务端前端 | Vue 3 + Vite | 单页应用，暗色主题 |
| 客户端后端 | Go 1.25 + Gin 1.12 | 本地 API、同步引擎、文件监听 |
| 客户端前端 | Electron 28 + Vue 3 | 桌面应用（Windows & macOS） |
| macOS 集成 | Swift Extension + Host App | NSFileProviderReplicatedExtension |
| Windows 集成 | PowerShell + CFAPI | StorageProviderSyncRootManager |
| 数据库 | JSON 文件 + mutex | 无 SQL 依赖 |
| 鉴权 | JWT (HS256) + bcrypt | Token 鉴权，72 小时有效期 |

---

## 快速开始

### 前置要求

- **Go** 1.21+（服务端）/ 1.25+（客户端后端）
- **Node.js** 18+（Vue / Electron 构建）
- **macOS**：Xcode + Swift 工具链（File Provider）
- **Windows**：PowerShell 5.1+（CFAPI）

### 一键启动

```bash
node scripts/quick-start.mjs
```

### 启动服务端

```bash
# 后端 API（端口 8890）
cd server/backend
go run main.go

# Web 前端（端口 5176，开发模式）
cd server/front
npm install
npm run dev
```

### 启动客户端

**Windows：**

```bash
cd client/front
npm install
npm run dev          # 开发模式：Electron + Go 后端 :8090
```

**macOS：**

```bash
cd client/front-mac
npm install
npm run start        # 开发模式：Electron + Go 后端 + File Provider
```

### Docker 部署

```bash
docker compose up -d
```

启动两个服务器实例（端口 8890、8891）及对应前端（端口 18176、28176），含健康检查和持久化卷。

### 打包分发

```bash
# Windows 客户端（目录输出）
cd client/front
npm run dist

# Windows 客户端（便携版 .exe）
cd client/front
npm run dist:portable

# macOS 客户端（.dmg）
cd client/front-mac
npm run dist

# macOS 客户端（目录输出，调试用）
cd client/front-mac
npm run dist:dir
```

> ⚠️ 必须使用 `npm run dist` 脚本打包，不要手动拆步执行 `go build` + `electron-builder`，脚本内部确保了正确的构建顺序。

---

## 配置

### 服务端配置 (`server/backend/config/config.yaml`)

```yaml
server:
  port: "8890"
  mode: "debug"          # debug / release / test
database:
  path: "./data/zcopy.db"
auth:
  secret_key: "生产环境务必修改"
  token_expire_hours: 72
storage:
  root_dir: "./storage"
```

### 环境变量

| 变量 | 说明 |
|------|------|
| `ZCOPY_CLIENT_CONFIG` | 自定义配置文件路径（客户端） |
| `ZCOPY_CLIENT_DATA_DIR` | 数据存储目录（客户端） |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | macOS File Provider 桥接地址 |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | macOS File Provider 桥接鉴权 Token |
| `ZCOPY_CORS_ORIGINS` | 允许的 CORS 来源（服务端，Docker） |
| `ZCOPY_SERVER_CONFIG` | 自定义服务端配置路径（Docker） |

### 端口

| 服务 | 端口 | 用途 |
|------|------|------|
| 服务端 API | 8890 | REST API |
| 服务端 Web UI | 5176 | Vue SPA（开发） |
| 客户端后端 | 8090 | 本地 REST API |
| 客户端 Vite | 5173 | Vue 开发服务器 |
| FP Host App | 随机 | macOS File Provider 桥接 |

---

## API 概览

### 服务端 API（`:8890`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 注册用户 |
| POST | `/api/v1/auth/login` | 登录，返回 JWT |
| GET | `/api/v1/auth/me` | 获取当前用户（需鉴权） |
| GET | `/api/v1/files?path=` | 列出目录 |
| POST | `/api/v1/files/upload` | 上传文件（multipart） |
| GET | `/api/v1/files/download?path=` | 下载文件 |
| POST | `/api/v1/files/folder` | 创建目录 |
| DELETE | `/api/v1/files?path=` | 删除文件或目录 |
| GET | `/api/v1/server/info` | 服务器 UUID 及信息（公开） |
| POST | `/api/v1/client/heartbeat` | 客户端心跳（需鉴权） |
| GET | `/health` | 健康检查 |

### 客户端本地 API（`:8090`）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET / POST / PUT / DELETE | `/tasks` | 备份任务 CRUD |
| POST | `/tasks/:id/sync` | 手动全量同步 |
| POST | `/tasks/:id/auto/start` | 启用自动备份 + 文件监听 |
| POST | `/tasks/:id/on-demand/release` | 释放本地空间 |
| POST | `/tasks/:id/on-demand/hydrate` | 从服务器重新下载全部文件 |
| GET / POST | `/servers` | 服务器管理 + SSDP 扫描 |

---

## 项目结构

```
zcopy/
├── server/                     # 文件服务器
│   ├── backend/                #   Go API（Gin + JWT）
│   └── front/                  #   Vue 3 单页应用
├── client/                     # 桌面客户端
│   ├── backend/                #   Go 本地 API + 同步引擎
│   ├── front/                  #   Windows Electron 客户端（Vue 3）
│   └── front-mac/              #   macOS Electron 客户端 + Swift File Provider
├── scripts/                    # 构建与快速启动脚本
├── docker-compose.yaml         # 多服务器 Docker 部署
└── docs/                       # 文档
```

---

## 开发路线图

- [x] Token 鉴权（JWT）
- [x] 文件增删改查（列表 / 上传 / 下载 / 删除）
- [x] macOS File Provider 集成
- [x] Windows CFAPI 集成
- [x] 基于快照的增量同步
- [x] 按需同步（释放 / 水合）
- [x] 文件监听自动备份
- [x] 传输日志（环形缓冲区）
- [x] 多服务器支持
- [x] SSDP 局域网发现
- [x] 客户端心跳 + 在线监控
- [ ] Refresh Token 轮换
- [ ] 基于哈希的增量同步
- [ ] 冲突检测与处理
- [ ] 断点续传
- [ ] 设备绑定
- [ ] 同步任务历史
- [ ] 多服务器集群备份

---

## License

[MIT](LICENSE)
