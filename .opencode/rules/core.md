---
title: 核心开发规则
scope: always
priority: CRITICAL
---

# 核心开发规则

**这些规则在所有开发活动中始终生效。**

---

## Evidence 驱动（最高优先级）

> 你的测试质量 = AI 输出质量的天花板。

### 规则 1：先有验证方案再写代码

任何功能实现之前，必须先明确：
1. 怎么证明这段代码是对的？（测试用例）
2. 怎么证明改动没破坏旧代码？（回归测试）
3. 验证命令是什么？（具体命令 + 预期输出）

**反面案例**："优化一下同步模块" → AI 无所适从
**正面案例**："把 syncTask 的增量模式改为基于哈希对比，输入不变，输出新增 `HashBased bool` 字段，用 `sync/engine_test.go` 的 `TestSyncTaskIncremental` 验证"

### 规则 2：最小可复现 demo 优先

遇到 Bug 时：
1. ❌ 不要把整个项目扔给 AI
2. ❌ 不要用自然语言描述 Bug 现象
3. ✅ 先提炼最小可复现脚本
4. ✅ 用最小脚本问 AI

> 写最小 demo 花的时间，远远小于在大代码库上反复让 AI 猜的时间。

### 规则 3：验收标准是合同条款

> AI 会像 RL 中的 reward hacking 一样钻测试漏洞。

- 测试有漏洞 → AI 会找到并钻过去
- 明确的验收标准 = 最好的 prompt engineering
- 如果发现 AI 的输出"看起来对但不是你想要的"，说明你的验收标准有漏洞

---

## 代码质量底线

### 类型安全

```
禁止：
- as any（TypeScript/Vue）
- @ts-ignore、@ts-expect-error（除非带 TODO issue 链接）
- panic 在库代码中使用（Go）
- log.Fatalf 在工具函数中使用（Go）

正确做法：
- 修类型定义，不要压制错误
- Go 中返回 error，让调用方决定如何处理
- 参见 instructions/project-architecture.md（Go 风格规范）
```

### 错误处理

```
禁止：
- catch(e) {} 空块（JS）
- if err != nil { log.Println(err) } 只 log 不处理（Go）
- 忽略 error 返回值（Go）

正确做法：
- catch 特定异常
- 在合适的层级处理错误
- Go 中错误原样返回，上下文在 HTTP 层加
- 参见 instructions/project-architecture.md（哨兵错误模式）
```

### 测试规范

```
禁止：
- 删除失败测试来"通过"
- 只测 happy path
- 测试之间有依赖顺序

正确做法：
- 每个测试独立运行
- 覆盖 happy path + 边界 + 错误路径
- 新 Bug 先写回归测试再修
- 参见 instructions/testing-and-api.md（必须测试的模块）
```

---

## 任务拆解规范

### 子任务三要素（缺一不可）

```markdown
### Task N: [描述]

**输入**：[从哪里拿什么数据]
**输出**：[产生什么结果，放在哪里]
**验证**：[具体命令 + 预期输出]

**实现步骤**：
1. ...
2. ...

**Commit**：`type(scope): 简短描述`
```

### 分批读写原则

```
1. 先读 5 个相关文件 → 理解模式
2. 做改动 → 跑验证
3. 再读下一批 → 继续改动
4. 不要一次读 20 个文件再一口气改完
```

---

## Commit 规范

### 格式

```
<type>(<scope>): <description>

[可选 body]
[可选 footer]
```

### Type 列表

| Type | 说明 |
|------|------|
| feat | 新功能 |
| fix | Bug 修复 |
| refactor | 重构（不改行为） |
| test | 添加或修改测试 |
| docs | 文档 |
| chore | 构建/工具/依赖 |
| perf | 性能优化 |

### Scope 列表（ZCopy 项目专用）

| Scope | 覆盖范围 |
|-------|---------|
| sync | 同步引擎（client/backend/sync/） |
| auth | 认证相关（handlers_auth, auth/） |
| task | 任务管理（handlers_task, store/） |
| fp | File Provider（fileprovider/, front-mac/fileprovider/） |
| ui | 前端界面（client/front/src/, server/front/src/） |
| server | 服务端（server/backend/） |
| build | 构建打包（scripts/, electron-builder 配置） |

### 粒度

- 一个逻辑改动 = 一个 commit
- Fix PR 里不要混入重构
- 大改动拆成多个小 commit，方便 bisect 和 revert

---

## 运行时 Evidence 外化工具箱

当 AI 需要调试运行时问题时，使用以下工具把状态外化为文本：

### Go 后端调试

| 工具/方法 | 用途 | 命令 |
|-----------|------|------|
| pprof CPU | 性能瓶颈定位 | `go tool pprof http://localhost:8090/debug/pprof/profile` |
| pprof 内存 | 内存泄漏排查 | `go tool pprof http://localhost:8090/debug/pprof/heap` |
| delve attach | 看进程卡在哪里 | `dlv attach <PID>` |
| 端口占用 | 确认服务是否在监听 | `lsof -i :8090`（客户端）/ `lsof -i :8890`（服务端） |
| 健康检查 | 服务是否就绪 | `curl http://localhost:8090/health` |
| 日志追踪 | 传输日志查看 | `GET /logs` API 端点 |

### 前端调试

| 工具/方法 | 用途 | 命令 |
|-----------|------|------|
| DevTools Console | 渲染层错误 | Electron 中 Ctrl+Shift+I |
| 网络面板 | API 请求追踪 | DevTools → Network |
| Vue DevTools | 组件状态检查 | 浏览器扩展 |

### 多进程协作调试

| 工具/方法 | 用途 | 命令 |
|-----------|------|------|
| bridge.json 检查 | macOS FP 桥接状态 | `cat ~/Library/Application\ Support/ZCopy/bridge.json` |
| 进程树 | 确认子进程启动 | `pstree -p <Electron_PID>` |
| curl 端到端 | API 链路验证 | `curl -v http://localhost:8090/auth/me` |

> 原则：AI 看得到的是静态代码，看不到的是动态运行时。人的职责是把运行时信息喂给 AI。
