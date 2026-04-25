# ZCopy - 项目入口

> 云文件同步工具：Electron 桌面客户端 + Go 服务端。支持 macOS Finder / Windows Cloud Files 的按需同步集成。
> 核心原则：**AI 是规划放大器，不是编码替代品。人的工作是拆对需求、做实验证、外化 evidence。**

## 技术栈

| 层 | 技术 | 版本 |
|----|------|------|
| 客户端后端 | Go + Gin | Go 1.25，Gin 1.12 |
| 客户端前端 | Electron + Vue 3 | Electron 28，Vue 3.4 |
| 服务端后端 | Go + Gin | Go 1.21，Gin 1.9 |
| 服务端前端 | Vue 3 + Vite | Vue 3.4，Vite 4.5 |
| 鉴权 | JWT（HS256） | golang-jwt/jwt/v5 |
| 数据库 | JSON 文件 | 内存切片 + mutex 保护 |
| macOS File Provider | Swift Extension + Host App + REST API | — |
| Windows CFAPI | PowerShell + StorageProviderSyncRootManager | — |

## 目录结构

```
zcopy/
├── AGENTS.md                # ← 你在这里（项目入口 & 路由）
├── client/                  # 桌面客户端（Electron + Go）
│   ├── backend/             # Go 后端（本地 API :8090 + 同步引擎）
│   ├── front/               # Windows Electron 客户端（Vue 3）
│   └── front-mac/           # macOS Electron 客户端（+ Swift File Provider）
├── server/                  # 远程文件服务
│   ├── backend/             # Go API 服务（:8890）
│   └── front/               # Web 前端（Vue 3 SPA）
├── scripts/                 # 构建脚本
└── .opencode/               # AI 辅助开发配置（四层结构）
    ├── instructions/         # 项目特定技术约束
    ├── rules/                # 方法论/流程规则（按路径自动匹配）
    ├── agents/               # 领域专家（按需激活）
    ├── skills/               # 可复用工作流
    └── commands/             # 用户可直接调用的动作
```

---

## 底线约束（Always / Ask First / Never）

### Always Do

1. **先设计验证方案再写代码** —— 测试质量 = AI 输出质量的天花板
2. **每个子任务有明确的输入、输出、验证标准** —— 模糊描述 = AI 迷路
3. **修改代码后立即跑验证** —— 诊断、lint、测试，一个不能少
4. **分批读写** —— 先读 5 个文件理解模式 → 改动 → 再读下一批
5. **遵循现有代码风格** —— 不要引入新的模式，除非有明确的设计决策
6. **打包必须用 npm run 脚本** —— 禁止手动 go build + electron-builder 拆步

### Ask First

1. 引入新依赖或新工具
2. 修改公共 API 或接口签名
3. 跨模块的大范围重构
4. 任何你不确定是否符合项目意图的改动

### Never Do

1. **禁止 `as any`、`@ts-ignore`** —— 类型错误要修，不要压
2. **禁止 `panic` 在库代码、`log.Fatalf` 在工具函数** —— 返回 error
3. **禁止空 catch 块 / 只 log 不处理** —— 错误必须妥善处理
4. **禁止删除失败的测试来"通过"** —— 测试失败说明代码有问题
5. **禁止 commit 未经验证的代码** —— 跑过测试再提交
6. **禁止在 fix PR 里混入重构** —— 修 bug 就修 bug

---

## 路由表：按需查找深层信息

| 你要做的事 | 去哪里看 |
|-----------|---------|
| 添加新功能 | → `.opencode/skills/add-feature/SKILL.md` |
| 修复 Bug | → `.opencode/skills/fix-bug/SKILL.md` |
| 重构代码 | → `.opencode/skills/refactor/SKILL.md` |
| 新人上手 | → `.opencode/skills/onboarding/SKILL.md` |
| 做 Code Review | → `.opencode/commands/pr-review.md` |
| 生成提交信息 | → `.opencode/commands/gen-commit.md` |
| 检查项目健康度 | → `.opencode/commands/health-check.md` |
| 后端开发约束 | → `.opencode/rules/backend.md` |
| 前端开发约束 | → `.opencode/rules/frontend.md` |
| 同步引擎约束 | → `.opencode/rules/sync-engine.md` |
| 测试规范 | → `.opencode/rules/testing.md` |
| 需要架构专家 | → `.opencode/agents/architect.md` |
| 需要后端专家 | → `.opencode/agents/backend.md` |
| 需要前端专家 | → `.opencode/agents/frontend.md` |
| 需要质量审查 | → `.opencode/agents/quality.md` |
| 需要基础设施专家 | → `.opencode/agents/infra.md` |
| 需要 Code Review | → `.opencode/agents/reviewer.md` |
| Go 风格 / 接口抽象 / 安全 | → `.opencode/instructions/project-architecture.md` |
| 测试要求 / API 文档规范 | → `.opencode/instructions/testing-and-api.md` |
| 构建命令 / 打包脚本 | → `.opencode/instructions/build-and-packaging.md` |
| API 接口文档（全部端点） | → `.opencode/instructions/api-reference.md` |
| 架构详解 / 启动流程 / File Provider | → `.opencode/instructions/architecture.md` |

---

## 核心方法论速查

### 1. Evidence 驱动开发

```
写代码之前先问：怎么证明这段代码是对的？
→ 有测试 → 让 AI 在测试约束下工作
→ 没有 → 先写测试，再写实现
→ Bug → 先提炼最小可复现 demo，再问 AI
```

### 2. 嵌套式任务拆解

```
大任务 → 拆成阶段 → 每个阶段有子计划
每个子任务必须包含：
  - 输入：从哪里拿数据
  - 输出：产生什么结果
  - 验证：怎么证明做对了
```

### 3. 多层验证防线

```
第一层：LSP 诊断 + go vet —— 自动拦低级问题
第二层：领域专家 agent 动态审查 —— 比通用 lint 深入
第三层：回归测试 —— 每次发现新问题就补测试，滚雪球
```

### 4. AI 能力边界认知

```
✅ AI 擅长：模式复制、代码审查、多文件协同改动、文档生成
⚠️ 需人工外化 evidence：同步引擎调试、File Provider 行为验证
❌ AI 不擅长：长时间自主运行、理解隐含先验
```

---

## 开发速查

### 前置要求

- Go 1.21+（服务端），Go 1.25+（客户端后端）
- Node.js（Vue / Electron）
- macOS 需要 Xcode；Windows 需要 PowerShell

### 启动命令

```bash
# 服务端
cd server/backend && go run main.go          # :8890
cd server/front && npm run dev               # :5173

# 客户端
cd client/front && npm run dev               # Windows Electron + Go :8090
cd client/front-mac && npm run start         # macOS Electron

# 一键
node scripts/quick-start.mjs
```

### 端口

| 服务 | 端口 | 用途 |
|------|------|------|
| 服务端 API | 8890 | REST API |
| 服务端 Web UI | 5173 | Vue SPA（开发） |
| 客户端后端 | 8090 | Electron 本地 API |
| 客户端 Vite | 5173 | Vue 开发服务器 |
| WebDAV | 随机 | macOS File Provider |
| FP Host App | 随机 | macOS FP 域名注册 |

### 关键约定

- **语言**：UI 默认中文（zh-CN），时区 `Asia/Shanghai`
- **数据库**：JSON 文件 + mutex，不是 SQL
- **鉴权**：JWT Bearer token
- **同步策略**：基于快照增量同步（size + modTime）+ 文件监听防抖
- **路径安全**：`resolveUserPath()` 防路径穿越

---

## 配置工程说明

本项目的 `.opencode/` 目录是需要持续维护的工程产物：

- **instructions/** 存放项目特定技术约束（API 文档、架构详解、Go 规范）
- **rules/** 随踩坑经验积累逐步精细化
- **agents/** 随项目复杂度增长逐步分化领域专家
- **skills/** 随重复性工作浮现逐步标准化流程
- **commands/** 随日常工作提炼逐步自动化动作
