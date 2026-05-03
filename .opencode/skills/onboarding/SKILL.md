---
name: onboarding
description: "新人（或新 session）快速理解 ZCopy 项目的引导流程。"
---

# 新人上手

## 概述

新 session 启动时的引导流程。帮助 AI（或新成员）快速理解项目上下文，减少无效探索。

## 引导流程

```
读 AGENTS.md → 了解目录结构 → 看核心代码 → 跑一遍测试 → 了解工具链 → Ready
```

## Step 1: 读项目入口

```
必读文件：
1. AGENTS.md — 项目全景：架构、技术栈、API 文档、关键约定
2. .opencode/instructions/project-architecture.md — Go 规范、安全底线

关键问题：
- 项目做什么？→ 云文件同步工具（Electron + Go）
- 技术栈？→ Go 1.21/1.25 + Gin + Vue 3.4 + Electron 28 + Swift
- 目录怎么组织？→ Monorepo：client/ + server/ + scripts/
- 底线约束？→ 参见 AGENTS.md 和 instructions/
```

## Step 2: 了解目录结构

```
核心目录：
client/backend/   → Go 本地后端（:8090）
client/front/     → Windows Electron 客户端
client/front-mac/ → macOS Electron + File Provider
server/backend/   → Go 服务端（:8890）
server/front/     → Web SPA
scripts/          → 构建脚本

探索方式：
- 分批读：先看每个目录的入口文件（main.go / main.js / package.json）
- 不要一次读所有文件
- 理解数据流：本地文件 → Go 后端 → 服务端 → 远程存储
```

## Step 3: 看核心代码

```
从入口出发，理解数据流：

客户端后端（:8090）：
1. main.go → 启动流程（配置加载 + TaskStore + AppState + Gin）
2. routes.go → 路由注册（7 组）
3. handlers_task.go → 任务 CRUD
4. sync/engine.go → 同步引擎核心

服务端后端（:8890）：
1. main.go → 路由注册 + CORS
2. handlers/auth.go → 注册/登录/JWT
3. handlers/file.go → 文件操作

macOS File Provider：
1. front-mac/fileprovider/EleFileProvider/Extension.swift
2. backend/fileprovider/service.go
3. front-mac/fileprovider-host/Sources/main.swift
```

## Step 4: 跑一遍测试

```
# Go 后端测试
cd client/backend && go test ./...
cd server/backend && go test ./...

# 前端测试（如果有）
cd client/front && npm run test
cd server/front && npm run test

目的：
- 确认开发环境可用
- 了解测试覆盖范围
- 为后续改动提供回归基线

⚠️ 当前项目零测试覆盖，这是已确认的严重问题
```

## Step 5: 了解工具链

```
需要知道的命令：

# 开发
cd server/backend && go run main.go          # 服务端 :8890
cd server/front && npm run dev               # Web UI :5173
cd client/front && npm run dev               # Windows 客户端
cd client/front-mac && npm run start         # macOS 客户端

# 构建
cd client/front && npm run dist              # Windows 打包
cd client/front-mac && npm run dist          # macOS 打包

# 快捷
node scripts/quick-start.mjs                 # 一键启动 + 打包

端口：
- 8890：服务端 API
- 5173：开发服务器
- 8090：客户端 Go 后端
```

## 快速上下文模板

复制并填写以下模板，快速为新的开发 session 建立上下文：

```markdown
## 项目上下文

**项目**：ZCopy
**做什么**：云文件同步工具，支持 macOS Finder / Windows Cloud Files 按需同步
**技术栈**：Go 1.25 + Gin + Vue 3.4 + Electron 28 + Swift File Provider
**架构**：Electron + Go 本地后端（:8090）+ Go 远程服务端（:8890）
**目录**：client/（桌面客户端）+ server/（远程服务）+ scripts/（构建脚本）
**测试命令**：go test ./...（Go）、npm run test（前端）
**当前任务**：[要做什么]

**特殊约束**：
- 单文件不超过 300 行，函数不超过 50 行
- 零测试覆盖（严重问题，新代码必须带测试）
- JSON 文件数据库（不是 SQL）
- 打包必须用 npm run 脚本（不手动拆步）
```
