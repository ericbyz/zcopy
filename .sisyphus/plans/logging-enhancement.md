# 日志增强系统

## TL;DR

> **Quick Summary**: 为 ZCopy 全栈构建统一的结构化日志系统，基于 Go `log/slog` 实现文件持久化（JSONL 格式、按天轮转、大小上限、自动清理），服务端和客户端后端所有关键操作全覆盖，两端前端均提供日志查看页面（分页查询、多维度筛选、关键词搜索、导出）。
> 
> **Deliverables**:
> - 服务端后端 `logger/` 包：slog 结构化日志 + JSONL 文件输出 + 按天轮转 + 大小上限 + 保留策略
> - 客户端后端 `log/` 包重写：替换内存环形缓冲为 slog 文件持久化，保持 `LogStore` 接口兼容
> - 两端后端 GET /logs API 增强：分页、级别过滤、任务过滤、关键词搜索、时间范围、导出
> - 服务端后端：请求日志中间件 + auth/file handler 审计日志
> - 客户端后端：sync/watcher/fileprovider/proxy/handler 全面日志覆盖
> - 客户端前端：vue-router + 独立日志页面（完整筛选面板 + 分页 + 搜索 + 导出）
> - 服务端前端：日志查看区域（筛选 + 分页 + 搜索 + 导出，内嵌于 App.vue）
> - Windows Electron 修复：stdio 捕获 + macOS 日志文件统一
> - 完整 TDD 测试覆盖（log/logger 包的轮转、并发、持久化）
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Task 1 → Task 7 → Task 12 → Task 15 → F1-F4

---

## Context

### Original Request
日志增强系统，务必做到日志持久化，每一步关键操作都需要记录日志，不管是服务端还是客户端，不仅要有文件记录还要有前台显示，客户端也需要显示。

### Interview Summary
**Key Discussions**:
- 前端刷新方式：用户选择「新增页面查询，用户分页点击查询，结构化全面一些」（非实时推送）
- 筛选能力：级别过滤 + 按任务过滤 + 关键词搜索 + 按时间范围 + 导出日志
- 日志轮转：按时间轮转（每天一个文件）
- 前端覆盖：服务端前端和客户端前端都需要日志显示
- Go 日志库：选择 `log/slog`（Go 标准库）
- 前端架构：客户端前端引入 vue-router 实现独立日志页面
- 测试策略：TDD

**Research Findings**:
- 服务端后端：零业务日志，仅 10 处启动 `log.*` 调用，无级别、无结构化、无持久化
- 客户端后端：TransferLog 环形缓冲（2000 条/内存），仅 8 处使用，60+ 函数完全静默
- 客户端前端：已有基本日志卡片（18 条硬编码限制），无筛选/时间戳/分页
- 服务端前端：无日志功能
- Windows Electron `stdio:'ignore'` 导致后端输出完全丢失
- fileprovider 800+ 行零日志

### Metis Review
**Identified Gaps** (addressed):
- Go 1.21 slog 可用性 → 确认可用（1.21.0 引入 slog）
- 包名冲突 → 服务端用 `logger/`，客户端保留 `log/`
- TransferLog 向后兼容 → 保持 `LogStore` 接口 + API 响应格式
- 日志轮转实现 → 在 `io.Writer` 层实现，而非完整 `slog.Handler`
- 按天轮转不够 → 增加单文件大小上限（100MB）
- 需要保留策略 → 30 天 + 最大总大小 500MB
- 异步写入 → buffered channel，sync 引擎不阻塞
- 服务端前端不加 vue-router → 用折叠/标签区域
- 日志中禁止记录密码/JWT/文件内容
- 同步操作不逐文件记录 → 仅摘要 + 错误逐条

---

## Work Objectives

### Core Objective
构建统一的结构化日志基础设施，覆盖 ZCopy 全栈 4 个组件（服务端后端/前端、客户端后端/前端），实现日志持久化（JSONL 文件）、关键操作全覆盖、前端分页查询 UI。

### Concrete Deliverables
- `server/backend/logger/` — slog 结构化日志包（JSONL + 按天轮转 + 大小上限 + 保留策略）
- `client/backend/log/` — 重写为 slog 文件持久化（保持 LogStore 接口）
- `server/backend/handlers/log.go` — 日志查询 API（分页 + 搜索 + 导出）
- `client/backend/handlers_log.go` — 增强日志查询 API
- `server/backend/middleware/logging.go` — 请求/响应日志中间件
- 两端后端所有 handler/sync/watcher/fileprovider/proxy 添加日志调用
- `client/front/src/views/LogViewer.vue` — 客户端日志页面（vue-router）
- `server/front/src/components/LogViewer.vue` — 服务端日志查看区域
- 完整单元测试（logger/log 包的轮转、并发、持久化、保留策略）

### Definition of Done
- [ ] `go test ./server/backend/logger/ -v` → 全部 PASS
- [ ] `go test ./client/backend/log/ -v` → 全部 PASS
- [ ] 服务端后端启动后 `data/logs/` 下生成 JSONL 文件
- [ ] 客户端后端启动后 `data/logs/` 下生成 JSONL 文件
- [ ] `curl http://localhost:8890/api/v1/logs?limit=10` 返回分页日志
- [ ] `curl http://localhost:8090/api/v1/logs?limit=10` 返回分页日志
- [ ] 客户端前端 `/logs` 路由显示完整日志页面
- [ ] 服务端前端日志区域可展开、筛选、分页

### Must Have
- Go `log/slog` 结构化 JSON 日志（JSONL 格式）
- 按天轮转 + 单文件 100MB 上限
- 保留策略：30 天 / 最大 500MB
- 异步写入（buffered channel），不阻塞业务逻辑
- 日志级别：DEBUG / INFO / WARN / ERROR
- GET /logs API：分页（page/pageSize）、级别过滤、任务过滤（客户端）、关键词搜索、时间范围、导出
- 客户端前端 vue-router 独立日志页面
- 服务端前端内嵌日志查看区域
- 所有 auth 操作记录（注册/登录/登出/JWT 校验失败）
- 所有文件操作记录（上传/下载/删除/重命名/建目录）
- 所有同步操作记录（开始/进度摘要/完成/失败）
- 所有 watcher 操作记录（启动/停止/防抖触发/错误）
- Windows Electron stdio 捕获修复

### Must NOT Have (Guardrails)
- ❌ 不引入第三方日志轮转库（lumberjack 等）
- ❌ 不在日志中记录密码、JWT token、文件内容
- ❌ 同步操作不逐文件记录 INFO 级别（仅摘要 + 错误逐条）
- ❌ 不给服务端前端加 vue-router（用折叠区域/标签）
- ❌ 不实现实时日志推送（WebSocket/SSE）— 明确排除
- ❌ 不做日志远程传输/远程聚合
- ❌ 不在此次任务中重构 App.vue（仅提取 LogViewer 组件）
- ❌ 不在此次任务中重构 handler 函数结构（仅添加日志调用）
- ❌ 不做日志告警/桌面通知
- ❌ 日志消息长度上限 4096 字符
- ❌ 日志文件权限 0644（目录 0755）
- ❌ 单条日志 JSON 最大 8KB

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO（项目零测试覆盖）
- **Automated tests**: YES (TDD) — 仅 logger/log 包采用 TDD，handler 日志集成用 Agent QA
- **Framework**: Go 标准 `testing` 包
- **TDD scope**: 轮转逻辑、并发安全、持久化往返、保留策略、大小上限

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **API 端点**: Use Bash (curl) — Send requests, assert status + response fields
- **Go 包**: Use Bash (go test) — Run tests, verify PASS/FAIL
- **Frontend UI**: Use Playwright (playwright skill) — Navigate, interact, assert DOM, screenshot
- **文件验证**: Use Bash — Check file existence, content, permissions

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — 6 tasks, ALL parallel):
├── T1:  Server backend logger/ 包 (TDD) [deep]
├── T2:  Client backend log/ 包重写 (TDD) [deep]
├── T3:  Client backend GET /logs API 增强 [unspecified-high]
├── T4:  Server backend GET /logs API 创建 [unspecified-high]
├── T5:  Client frontend vue-router + LogViewer 骨架 [visual-engineering]
└── T6:  Server frontend LogViewer 骨架 [visual-engineering]

Wave 2 (Logging integration — 8 tasks, ALL parallel after Wave 1):
├── T7:  Server backend 请求日志中间件 (depends: 1) [unspecified-high]
├── T8:  Server backend auth handler 审计日志 (depends: 1) [unspecified-high]
├── T9:  Server backend file handler 审计日志 (depends: 1) [unspecified-high]
├── T10: Client backend sync engine 日志 (depends: 2) [deep]
├── T11: Client backend watcher/fileprovider/proxy 日志 (depends: 2) [deep]
├── T12: Client backend HTTP handler 日志 (depends: 2) [unspecified-high]
├── T13: Electron 修复：Windows stdio + macOS 日志统一 (depends: none) [quick]
└── T14: Server backend utils 反模式修复 (depends: 1) [quick]

Wave 3 (Frontend full build — 2 tasks, parallel after Wave 2):
├── T15: Client frontend LogViewer 完整实现 (depends: 3, 5) [visual-engineering]
└── T16: Server frontend LogViewer 完整实现 (depends: 4, 6) [visual-engineering]

Wave FINAL (Verification — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T7 → T15 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 8 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | - | T7, T8, T9, T14 | 1 |
| T2 | - | T10, T11, T12 | 1 |
| T3 | T2 | T15 | 1 |
| T4 | T1 | T16 | 1 |
| T5 | - | T15 | 1 |
| T6 | - | T16 | 1 |
| T7 | T1 | T15 | 2 |
| T8 | T1 | - | 2 |
| T9 | T1 | - | 2 |
| T10 | T2 | - | 2 |
| T11 | T2 | - | 2 |
| T12 | T2 | - | 2 |
| T13 | - | - | 2 |
| T14 | T1 | - | 2 |
| T15 | T3, T5 | F1-F4 | 3 |
| T16 | T4, T6 | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 1**: **6** — T1→`deep`, T2→`deep`, T3→`unspecified-high`, T4→`unspecified-high`, T5→`visual-engineering`, T6→`visual-engineering`
- **Wave 2**: **8** — T7→`unspecified-high`, T8→`unspecified-high`, T9→`unspecified-high`, T10→`deep`, T11→`deep`, T12→`unspecified-high`, T13→`quick`, T14→`quick`
- **Wave 3**: **2** — T15→`visual-engineering`, T16→`visual-engineering`
- **FINAL**: **4** — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

- [x] 1. 服务端后端 `logger/` 包 (TDD)

  **What to do**:
  - 创建 `server/backend/logger/` 包（注意包名不能用 `log/`，会遮蔽标准库）
  - 基于 `log/slog` 实现 JSON 结构化日志，输出 JSONL 格式
  - 实现自定义 `io.Writer` 处理日志轮转：
    - 按天轮转：文件名格式 `data/logs/YYYY-MM-DD.jsonl`
    - 大小上限：单文件超过 100MB 时触发轮转（文件名加序号后缀 `.1`, `.2`）
    - 保留策略：自动清理超过 30 天的文件，总大小超 500MB 时清理最旧文件
  - 实现异步写入：buffered channel（容量 4096），后台 goroutine 消费
  - 提供 `Init(defaultLevel, logDir string)` 和 `L() *slog.Logger` 全局访问
  - 提供辅助函数：`Info(msg, fields)`, `Warn(msg, fields)`, `Error(msg, fields)`, `Debug(msg, fields)`
  - 日志字段命名约定：`time`, `level`, `msg`, `user_id`, `operation`, `path`, `duration_ms`, `status_code`, `error`, `request_id`
  - 在 `main.go` 启动时调用 `logger.Init()`
  - **TDD**：先写测试再实现
    - `logger_test.go`: TestDailyRotation（跨午夜生成文件）
    - `logger_test.go`: TestSizeCapRotation（超 100MB 轮转）
    - `logger_test.go`: TestRetentionCleanup（30 天清理 + 总大小清理）
    - `logger_test.go`: TestConcurrentWrites（并发 goroutine 写入无丢失）
    - `logger_test.go`: TestJSONLFormat（每行可 jq 解析，含必填字段）
    - `logger_test.go`: TestAsyncFlush（关闭时所有日志已落盘）
    - `logger_test.go`: TestDiskFullGraceful（写只读目录不 panic）

  **Must NOT do**:
  - 不引入 lumberjack 等第三方轮转库
  - 不实现完整 `slog.Handler` 接口，用 `io.Writer` 包装
  - 不在日志中记录密码/token

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: TDD + slog 自定义 Writer + 并发安全 + 文件轮转逻辑，需要深度理解和严谨实现
  - **Skills**: [`/ai-slop-remover`]
    - `/ai-slop-remover`: 日志包需要精简无冗余，AI slop 在日志基础设施中尤其有害
  - **Skills Evaluated but Omitted**:
    - `frontend-design`: 纯后端 Go 包，无前端

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5, T6)
  - **Blocks**: T7, T8, T9, T14
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `server/backend/config/config.go` — Viper 配置加载模式，logger 配置可参考此结构
  - `server/backend/main.go:20-70` — 服务端启动流程，logger.Init() 应在此插入
  - `server/backend/config/config.yaml` — 当前配置结构，需新增 `log` 配置段

  **API/Type References**:
  - Go 标准库 `log/slog` — JSONHandler, HandlerOptions, Level 等核心类型
  - Go 标准库 `log/slog` — Handler interface 的 6 个方法，理解 Writer 包装方案

  **External References**:
  - `https://pkg.go.dev/log/slog` — Go 官方 slog 文档
  - `https://pkg.go.dev/log/slog#JSONHandler` — JSON 输出 handler

  **WHY Each Reference Matters**:
  - config.go: 提供配置结构体和加载模式，logger 配置应保持一致
  - main.go: 确定初始化时机（config 之后、路由之前）
  - config.yaml: 需要新增 `log.level`, `log.dir`, `log.max_age_days`, `log.max_size_mb` 等配置项
  - slog 官方文档: Writer 包装方案的关键是 `slog.New(slog.NewJSONHandler(writer, opts))`，Writer 负责轮转

  **Acceptance Criteria**:

  **TDD Tests**:
  - [ ] Test file created: `server/backend/logger/logger_test.go`
  - [ ] `go test ./server/backend/logger/ -v -count=1` → ALL PASS (7 tests)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: JSONL 文件生成与格式验证
    Tool: Bash
    Preconditions: 服务端后端已编译，logger 包可用
    Steps:
      1. cd server/backend && go test ./logger/ -run TestJSONLFormat -v
      2. 检查生成的临时日志文件：head -1 {logfile} | jq '.time, .level, .msg'
      3. 验证 JSONL 每行均为合法 JSON，含 time/level/msg 字段
    Expected Result: jq 成功解析，输出 time 级别和消息内容
    Failure Indicators: jq 解析失败，缺少必填字段
    Evidence: .sisyphus/evidence/task-1-jsonl-format.txt

  Scenario: 按天轮转验证
    Tool: Bash
    Preconditions: 无
    Steps:
      1. cd server/backend && go test ./logger/ -run TestDailyRotation -v
      2. 测试内部创建两个不同日期的日志，验证生成两个文件
    Expected Result: 测试 PASS，确认按日期分割文件
    Failure Indicators: 测试 FAIL 或仅生成一个文件
    Evidence: .sisyphus/evidence/task-1-daily-rotation.txt

  Scenario: 磁盘容错 — 写入只读目录不崩溃
    Tool: Bash
    Preconditions: 无
    Steps:
      1. cd server/backend && go test ./logger/ -run TestDiskFullGraceful -v
      2. 测试内部尝试写入只读目录，验证 logger 不 panic
    Expected Result: 测试 PASS，logger 静默降级（写 stderr fallback）
    Failure Indicators: 测试 panic 或 FAIL
    Evidence: .sisyphus/evidence/task-1-disk-full.txt
  ```

  **Commit**: YES (groups with T2)
  - Message: `feat(logger): add slog-based structured logging with daily rotation`
  - Files: `server/backend/logger/*.go`, `server/backend/config/config.yaml`
  - Pre-commit: `cd server/backend && go test ./logger/ -v`

- [x] 2. 客户端后端 `log/` 包重写 (TDD)

  **What to do**:
  - 重写 `client/backend/log/transfer_log.go`：替换 `RingBufferLogStore` 为 slog 文件持久化
  - 保持 `LogStore` 接口不变（`Push` 和 `List` 方法签名保留）
  - 新增 `FileLogStore` 实现类：
    - 内部使用 `log/slog` + JSONL 文件输出
    - 日志目录：`data/logs/`，文件名格式 `YYYY-MM-DD.jsonl`
    - 按天轮转 + 单文件 100MB 上限 + 30 天保留
    - 异步写入（buffered channel）
  - `Push()` 方法：写入 slog 同时写入文件
  - `List()` 方法增强签名：支持分页、关键词搜索、时间范围过滤
    - 新增 `ListQuery` 结构体：`TaskID, Level, Keyword, StartTime, EndTime, Page, PageSize`
    - 返回 `ListResult{Items []TransferLog, Total int, Page int, PageSize int}`
  - `Export()` 新方法：导出指定条件的日志为 JSONL 文件
  - 更新 `client/backend/main.go` 中的初始化：替换 `NewRingBufferLogStore()` → `NewFileLogStore()`
  - **TDD**：
    - `log_test.go`: TestFilePersistence（重启后日志仍在）
    - `log_test.go`: TestListPagination（分页查询）
    - `log_test.go`: TestKeywordSearch（关键词搜索）
    - `log_test.go`: TestTimeRangeFilter（时间范围过滤）
    - `log_test.go`: TestConcurrentPush（并发写入安全）
    - `log_test.go`: TestRotationAndRetention（轮转+清理）

  **Must NOT do**:
  - 不改变 `LogStore` 接口的 `Push` 方法签名（保持向后兼容）
  - 不删除 `TransferLog` 模型定义
  - 不引入第三方库

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 需要在保持接口兼容的前提下替换实现，涉及文件 I/O + 搜索 + 分页 + 并发
  - **Skills**: [`/ai-slop-remover`]
    - `/ai-slop-remover`: 核心包需要精简

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5, T6)
  - **Blocks**: T10, T11, T12
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `client/backend/log/transfer_log.go:1-60` — 现有 `LogStore` 接口和 `RingBufferLogStore` 实现，新实现必须保持接口兼容
  - `client/backend/models/models.go` — `TransferLog` 结构体定义，保持不变
  - `client/backend/main.go:43-45` — `AppState.pushLog()` 包装模式，保持此模式
  - `client/backend/main.go:55-60` — 初始化位置，替换 RingBufferLogStore

  **API/Type References**:
  - `client/backend/log/transfer_log.go:11-13` — `LogStore` 接口定义（Push + List）
  - `client/backend/models/models.go:TransferLog` — 日志条目模型

  **Test References**:
  - 项目测试规范：表驱动测试 + `t.TempDir()` + 标准 `testing` 包

  **WHY Each Reference Matters**:
  - transfer_log.go: 这是要替换的核心文件，必须保持接口签名
  - models.go: TransferLog 模型是 API 契约，不能修改字段
  - main.go pushLog(): 所有 handler 通过此包装调用，接口不变则上层无需改动

  **Acceptance Criteria**:

  **TDD Tests**:
  - [ ] Test file created: `client/backend/log/log_test.go`
  - [ ] `go test ./client/backend/log/ -v -count=1` → ALL PASS (6 tests)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 日志持久化验证 — 写入后重启可读
    Tool: Bash
    Preconditions: 无
    Steps:
      1. cd client/backend && go test ./log/ -run TestFilePersistence -v
      2. 验证 Push 后日志写入 JSONL 文件
      3. 模拟重启（重新加载），验证 List 返回之前的日志
    Expected Result: 重启后日志完整可读
    Failure Indicators: 重启后 List 返回空或缺失条目
    Evidence: .sisyphus/evidence/task-2-persistence.txt

  Scenario: 分页与搜索功能
    Tool: Bash
    Preconditions: 无
    Steps:
      1. cd client/backend && go test ./log/ -run TestListPagination -v
      2. cd client/backend && go test ./log/ -run TestKeywordSearch -v
      3. 验证 pageSize 限制、page 偏移、keyword 匹配
    Expected Result: 分页返回正确子集，keyword 匹配 message/filePath 字段
    Failure Indicators: 分页越界或搜索无匹配
    Evidence: .sisyphus/evidence/task-2-pagination-search.txt
  ```

  **Commit**: YES (groups with T1)
  - Message: `feat(logger): replace ring buffer with slog-based file persistence`
  - Files: `client/backend/log/*.go`
  - Pre-commit: `cd client/backend && go test ./log/ -v`

- [x] 3. 客户端后端 GET /logs API 增强

  **What to do**:
  - 重写 `client/backend/handlers_log.go` 的日志查询 handler
  - 新增查询参数支持：
    - `page` (默认 1), `pageSize` (默认 20, 最大 100)
    - `taskId` — 按任务 ID 过滤（已有）
    - `level` — 按级别过滤：debug/info/warn/error（已有，扩展支持更多级别）
    - `keyword` — 关键词搜索（匹配 message 和 filePath 字段）
    - `startTime` — ISO 8601 时间戳，过滤 `createdAt >= startTime`
    - `endTime` — ISO 8601 时间戳，过滤 `createdAt <= endTime`
  - 响应格式从 `{"items": [...]}` 改为 `{"items": [...], "total": N, "page": N, "pageSize": N}`
  - 新增 `GET /api/v1/logs/export` 端点：
    - 支持与查询相同的过滤参数
    - 返回 `Content-Type: application/x-jsonlines`
    - 返回 `Content-Disposition: attachment; filename=zcopy-logs-{timestamp}.jsonl`
  - 调用 `logStore.List()` 新签名（使用 `ListQuery`）
  - 添加新路由到 `main.go`

  **Must NOT do**:
  - 不改变 `items` 中单个日志条目的字段名（保持 id/taskId/taskName/level/filePath/message/createdAt）
  - 不删除现有 `limit` 参数支持（向后兼容：`limit` 作为 `pageSize` 的别名）

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: API 端点重写，涉及查询参数解析、分页逻辑、文件导出，需要仔细处理边界
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (但依赖 T2 的 ListQuery 定义，可先 mock)
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5, T6)
  - **Blocks**: T15
  - **Blocked By**: T2 (ListQuery 结构体定义)

  **References**:

  **Pattern References**:
  - `client/backend/handlers_log.go` — 当前 GET /logs 实现，需重写
  - `client/backend/main.go` — 路由注册位置，需添加 `/logs/export` 路由
  - `client/backend/handlers_task.go:listTasks()` — 参考现有 handler 模式

  **API/Type References**:
  - `client/backend/log/transfer_log.go:LogStore` — 接口方法签名（将被 T2 修改为支持 ListQuery）
  - `client/backend/models/models.go:TransferLog` — 响应条目模型

  **WHY Each Reference Matters**:
  - handlers_log.go: 这是需要重写的文件
  - main.go: 需要注册新路由
  - LogStore: handler 调用 logStore.List()，需适配新签名

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 分页查询日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行在 :8090，有测试日志数据
    Steps:
      1. curl -s 'http://localhost:8090/api/v1/logs?page=1&pageSize=5' | jq '.page, .pageSize, .total'
      2. curl -s 'http://localhost:8090/api/v1/logs?page=2&pageSize=5' | jq '.items | length'
    Expected Result: page=1, pageSize=5, total 正确；第二页 items <= 5
    Failure Indicators: 缺少 total/page 字段或分页不正确
    Evidence: .sisyphus/evidence/task-3-pagination.json

  Scenario: 关键词搜索
    Tool: Bash (curl)
    Preconditions: 有包含 "同步完成" 的日志
    Steps:
      1. curl -s 'http://localhost:8090/api/v1/logs?keyword=同步完成' | jq '.items[].message'
    Expected Result: 所有返回条目的 message 或 filePath 包含关键词
    Failure Indicators: 返回不匹配的条目或漏匹配
    Evidence: .sisyphus/evidence/task-3-keyword-search.json

  Scenario: 时间范围过滤
    Tool: Bash (curl)
    Preconditions: 有不同时间的日志
    Steps:
      1. curl -s 'http://localhost:8090/api/v1/logs?startTime=2025-01-01T00:00:00Z&endTime=2026-12-31T23:59:59Z' | jq '.total'
      2. curl -s 'http://localhost:8090/api/v1/logs?startTime=2099-01-01T00:00:00Z' | jq '.total'
    Expected Result: 第一个查询 total > 0，第二个 total = 0
    Failure Indicators: 时间过滤不生效
    Evidence: .sisyphus/evidence/task-3-time-range.json

  Scenario: 日志导出
    Tool: Bash (curl)
    Preconditions: 有日志数据
    Steps:
      1. curl -s -o /tmp/zcopy-export.jsonl 'http://localhost:8090/api/v1/logs/export?level=info'
      2. head -1 /tmp/zcopy-export.jsonl | jq '.level'
      3. wc -l /tmp/zcopy-export.jsonl
    Expected Result: 文件下载成功，每行为合法 JSON，所有 level 为 "info"
    Failure Indicators: 文件为空或 JSON 解析失败
    Evidence: .sisyphus/evidence/task-3-export.jsonl
  ```

  **Commit**: YES (groups with T4)
  - Message: `feat(logger): enhance log API with pagination, search, time range, and export`
  - Files: `client/backend/handlers_log.go`, `client/backend/main.go`

- [x] 4. 服务端后端 GET /logs API 创建

  **What to do**:
  - 创建 `server/backend/handlers/log.go` — 日志查询 handler
  - 服务端日志结构不同于客户端（无 taskId/taskName）：
    - 结构化字段：`user_id`, `operation`, `path`, `method`, `status_code`, `duration_ms`, `error`, `request_id`
  - 查询参数：`page`, `pageSize`, `level`, `keyword`, `startTime`, `endTime`, `userId`（管理员过滤）
  - 响应格式：`{"items": [...], "total": N, "page": N, "pageSize": N}`
  - 每个日志条目字段：`id`, `time`, `level`, `msg`, `userId`, `operation`, `path`, `method`, `statusCode`, `durationMs`, `requestId`
  - 新增 `GET /api/v1/logs/export` 端点（同客户端逻辑）
  - 添加路由到 `server/backend/main.go`
  - 需要 `AuthRequired` 中间件保护（日志含敏感路径信息）
  - 实现：读取 JSONL 文件 → 解析 → 过滤 → 分页 → 返回

  **Must NOT do**:
  - 不暴露 JWT token 内容到日志条目
  - 不提供无认证的日志访问
  - 不用 SQL 数据库存储日志（从文件读取）

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 新建 API handler + JSONL 文件读取解析 + 分页搜索逻辑
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5, T6)
  - **Blocks**: T16
  - **Blocked By**: T1 (logger 包提供 JSONL 文件格式)

  **References**:

  **Pattern References**:
  - `server/backend/handlers/auth.go:Register()` — handler 模式参考（Gin context + JSON 响应）
  - `server/backend/main.go:60-80` — 路由注册位置
  - `server/backend/middleware/auth.go:AuthRequired()` — 日志 API 需要此中间件

  **API/Type References**:
  - `client/backend/handlers_log.go` — 客户端日志 handler 参考（分页逻辑可复用）
  - `server/backend/config/config.yaml` — 需新增 log 相关配置

  **WHY Each Reference Matters**:
  - auth.go: handler 编写模式（Gin 框架、错误响应格式）
  - main.go: 路由注册位置
  - middleware/auth.go: 日志 API 必须受认证保护

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 认证保护 — 未认证请求被拒绝
    Tool: Bash (curl)
    Preconditions: 服务端后端运行在 :8890
    Steps:
      1. curl -s -w '\n%{http_code}' 'http://localhost:8890/api/v1/logs'
    Expected Result: HTTP 401
    Failure Indicators: HTTP 200（未认证可访问）
    Evidence: .sisyphus/evidence/task-4-auth-check.txt

  Scenario: 认证后分页查询
    Tool: Bash (curl)
    Preconditions: 已注册用户，有 JWT token
    Steps:
      1. TOKEN=$(curl -s -X POST http://localhost:8890/api/v1/auth/login -H 'Content-Type: application/json' -d '{"account":"test","password":"test123"}' | jq -r '.data.token')
      2. curl -s -H "Authorization: Bearer $TOKEN" 'http://localhost:8890/api/v1/logs?page=1&pageSize=5' | jq '.total, .page, .pageSize'
    Expected Result: 返回合法分页结构，total/page/pageSize 字段存在
    Failure Indicators: 缺少分页字段或无认证
    Evidence: .sisyphus/evidence/task-4-pagination.json

  Scenario: 级别过滤
    Tool: Bash (curl)
    Preconditions: 有不同级别的日志
    Steps:
      1. curl -s -H "Authorization: Bearer $TOKEN" 'http://localhost:8890/api/v1/logs?level=error' | jq '.items[].level'
    Expected Result: 所有条目 level 为 "error"
    Failure Indicators: 包含其他级别的条目
    Evidence: .sisyphus/evidence/task-4-level-filter.json

  Scenario: 日志导出
    Tool: Bash (curl)
    Preconditions: 有日志数据
    Steps:
      1. curl -s -o /tmp/server-export.jsonl -H "Authorization: Bearer $TOKEN" 'http://localhost:8890/api/v1/logs/export'
      2. head -1 /tmp/server-export.jsonl | jq '.time, .level, .msg'
    Expected Result: 下载成功，每行为合法 JSON
    Evidence: .sisyphus/evidence/task-4-export.jsonl
  ```

  **Commit**: YES (groups with T3)
  - Message: `feat(logger): add server-side log query API with auth protection`
  - Files: `server/backend/handlers/log.go`, `server/backend/main.go`

- [x] 5. 客户端前端 vue-router + LogViewer 骨架

  **What to do**:
  - 安装 `vue-router`：`cd client/front && npm install vue-router@4`
  - 创建 `client/front/src/router/index.js` — 路由配置：
    - `/` → Dashboard（现有 App.vue 内容提取为 DashboardView.vue）
    - `/logs` → LogViewer.vue
  - 创建 `client/front/src/views/DashboardView.vue` — 提取现有 App.vue 的所有功能（auth、task、sync 等）
  - 创建 `client/front/src/views/LogViewer.vue` — 日志页面骨架：
    - 顶部筛选栏：级别下拉框、任务选择框、关键词输入、时间范围选择器
    - 日志表格：时间、级别、任务名、文件路径、消息
    - 底部分页：上一页/下一页 + 页码显示
    - 导出按钮
  - 创建 `client/front/src/App.vue` — 简化为路由容器 + 顶部导航（Dashboard / 日志 两个 tab）
  - 更新 `client/front/src/main.js` — 注册 router
  - 保持现有桌面 API 兼容（`window.desktopApi`）
  - **重要**：此任务仅搭建骨架结构和 UI 布局，数据对接在 Wave 3 T15 完成

  **Must NOT do**:
  - 不重构现有功能逻辑，仅提取为 DashboardView.vue
  - 不修改任何业务逻辑（auth、task、sync 代码原封不动移入 DashboardView）
  - 不添加 Pinia/Vuex 状态管理
  - 不实现数据对接（仅 UI 骨架 + mock 数据）

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 前端组件拆分 + 路由搭建 + UI 布局设计
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 创建生产级 UI 组件，确保日志页面设计质量
  - **Skills Evaluated but Omitted**:
    - `ui-ux-pro-max`: 可选但 frontend-design 更适合此场景

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4, T6)
  - **Blocks**: T15
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `client/front/src/App.vue` — 现有 678 行单文件，需要提取为 DashboardView + App shell + LogViewer
  - `client/front/src/main.js` — Vue 入口，需注册 router
  - `client/front/src/style.css` — 现有深色主题样式，LogViewer 应复用

  **API/Type References**:
  - `client/front/package.json` — 当前依赖：vue 3.4 + axios，需新增 vue-router@4
  - `client/front/vite.config.js` — Vite 配置

  **External References**:
  - Vue Router 4 文档: `https://router.vuejs.org/` — 安装和基本配置

  **WHY Each Reference Matters**:
  - App.vue: 拆分源文件，理解现有状态变量和函数分布
  - style.css: 复用现有深色主题和卡片样式
  - package.json: 确认 vue 版本兼容性

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 路由切换正常
    Tool: Playwright
    Preconditions: 客户端前端 Vite dev server 运行
    Steps:
      1. Navigate to 'http://localhost:5173/'
      2. Assert page contains 'ZCopy Desktop' heading
      3. Click navigation link to '/logs'
      4. Assert URL changed to '/logs'
      5. Assert page contains '日志' heading or log-related content
    Expected Result: 路由切换正常，两个页面均渲染
    Failure Indicators: 404、空白页、或路由不切换
    Evidence: .sisyphus/evidence/task-5-routing.png (screenshot)

  Scenario: Dashboard 功能保留
    Tool: Playwright
    Preconditions: 客户端前端运行
    Steps:
      1. Navigate to 'http://localhost:5173/'
      2. Assert login form is visible (username/password fields)
      3. Assert '创建备份任务' or task-related content exists
    Expected Result: 原有 Dashboard 功能完整保留
    Failure Indicators: 功能缺失或 UI 损坏
    Evidence: .sisyphus/evidence/task-5-dashboard.png (screenshot)
  ```

  **Commit**: NO (groups with T15 in Wave 3)

- [x] 6. 服务端前端 LogViewer 骨架

  **What to do**:
  - 在 `server/front/src/App.vue` 中添加「日志」标签页/折叠区域
  - 创建 `server/front/src/components/LogViewer.vue` — 日志查看组件：
    - 顶部筛选栏：级别下拉框、关键词输入、时间范围选择器
    - 日志表格：时间、级别、用户、操作、路径、消息
    - 底部分页：上一页/下一页 + 页码
    - 导出按钮
  - 在 App.vue 登录后添加 tab 切换（文件浏览 / 日志查看）
  - **重要**：此任务仅搭建 UI 骨架，数据对接在 Wave 3 T16 完成
  - 不引入 vue-router，用简单的 tab 状态切换

  **Must NOT do**:
  - 不引入 vue-router
  - 不重构现有文件浏览器功能
  - 不实现数据对接（仅 UI 骨架 + mock 数据）

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 前端组件创建 + UI 布局设计
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 确保与现有深色玻璃态 UI 风格一致

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4, T5)
  - **Blocks**: T16
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `server/front/src/App.vue` — 现有 359 行文件浏览器，需添加 tab 切换和 LogViewer 区域
  - `server/front/src/style.css` — 现有深色玻璃态主题样式

  **API/Type References**:
  - `server/front/package.json` — 当前依赖：vue 3.4 + axios

  **WHY Each Reference Matters**:
  - App.vue: 理解现有 tab 切换模式（已有 auth form 的 login/register tab 可参考）
  - style.css: 保持视觉一致性

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 日志 tab 切换
    Tool: Playwright
    Preconditions: 服务端前端 Vite dev server 运行，已登录
    Steps:
      1. Navigate to 'http://localhost:5173/'
      2. Login with test credentials
      3. Assert file browser is visible
      4. Click '日志' tab/button
      5. Assert LogViewer component renders (filter bar + table + pagination)
    Expected Result: tab 切换正常，日志区域显示筛选栏和表格骨架
    Failure Indicators: tab 不切换或组件不渲染
    Evidence: .sisyphus/evidence/task-6-log-tab.png (screenshot)
  ```

  **Commit**: NO (groups with T16 in Wave 3)

- [x] 7. 服务端后端请求日志中间件

  **What to do**:
  - 创建 `server/backend/middleware/logging.go`
  - 实现 `RequestLogger()` Gin 中间件：
    - 为每个请求生成唯一 `request_id`（UUID 或 nanoid）
    - 记录：method, path, query, status_code, duration_ms, client_ip, user_id（如已认证）
    - 使用 `logger.L()` 的 `Info` 级别
    - 将 `request_id` 注入 `c.Set("request_id", id)` 供 handler 使用
  - 在 `main.go` 路由注册中添加此中间件（在 CORS 之后、AuthRequired 之前）
  - 对 `/health` 端点跳过日志记录（避免噪音）

  **Must NOT do**:
  - 不记录请求 body（可能含密码/文件内容）
  - 不记录响应 body
  - 不阻塞请求处理（日志写入异步）

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Gin 中间件实现 + slog 集成 + request ID 生成
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T8-T14)
  - **Blocks**: T15
  - **Blocked By**: T1 (logger 包)

  **References**:

  **Pattern References**:
  - `server/backend/middleware/auth.go` — 现有中间件模式（Gin handler func + c.Set/c.Get）
  - `server/backend/main.go:55-65` — 中间件注册位置

  **API/Type References**:
  - `server/backend/logger/` — T1 创建的 logger 包，使用 `logger.L()` 获取全局 logger

  **WHY Each Reference Matters**:
  - auth.go: 理解 Gin 中间件写法（c.Next(), c.AbortWithStatusJSON 等）
  - main.go: 中间件注册顺序（CORS → logging → auth → handlers）

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 请求日志记录
    Tool: Bash (curl)
    Preconditions: 服务端后端运行在 :8890
    Steps:
      1. curl -s http://localhost:8890/health
      2. 查看最新日志文件：tail -5 server/backend/data/logs/*.jsonl | grep health || echo "health skipped (good)"
      3. curl -s http://localhost:8890/api/v1/auth/login -X POST -H 'Content-Type: application/json' -d '{"account":"x","password":"y"}'
      4. tail -3 server/backend/data/logs/*.jsonl | jq 'select(.msg | contains("request")) | .method, .path, .status_code'
    Expected Result: health 端点无日志；login 请求有日志记录 method/path/status
    Failure Indicators: health 端点被记录或 login 无日志
    Evidence: .sisyphus/evidence/task-7-request-logging.txt

  Scenario: request_id 存在且唯一
    Tool: Bash
    Preconditions: 服务端后端运行
    Steps:
      1. 连续发 3 个请求
      2. 从日志中提取 request_id 字段：cat server/backend/data/logs/*.jsonl | jq -r '.request_id' | sort | uniq | wc -l
    Expected Result: 3 个不同的 request_id
    Failure Indicators: request_id 重复或缺失
    Evidence: .sisyphus/evidence/task-7-request-id.txt
  ```

  **Commit**: YES (groups with T8, T9)
  - Message: `feat(logger): add request logging middleware with request ID`
  - Files: `server/backend/middleware/logging.go`, `server/backend/main.go`

- [x] 8. 服务端后端 auth handler 审计日志

  **What to do**:
  - 在 `server/backend/handlers/auth.go` 的每个 handler 中添加日志：
    - `Register()`: 记录注册尝试（INFO 级别，含 username）、注册成功（INFO）、注册失败（WARN，含原因如用户名已存在）
    - `Login()`: 记录登录尝试（INFO，含 account）、登录成功（INFO）、登录失败（WARN，含原因如密码错误）
    - `Me()`: DEBUG 级别记录（静默，不敏感）
    - `issueToken()`: 不单独记录（由 Register/Login 覆盖）
  - 在 `server/backend/middleware/auth.go` 的 `AuthRequired()` 中添加：
    - 缺少 Authorization header → WARN
    - JWT 过期 → INFO（常见，非异常）
    - JWT 无效/格式错误 → WARN
    - 用户不存在 → WARN

  **Must NOT do**:
  - 不在日志中记录密码明文
  - 不记录完整 JWT token
  - 不改变 handler 返回值或错误消息

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 在现有 handler 中添加 slog 调用，需理解认证流程
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T9-T14)
  - **Blocks**: None
  - **Blocked By**: T1 (logger 包)

  **References**:

  **Pattern References**:
  - `server/backend/handlers/auth.go` — Register, Login, Me handler 实现
  - `server/backend/middleware/auth.go` — AuthRequired 中间件

  **API/Type References**:
  - `server/backend/logger/` — `logger.Info()`, `logger.Warn()`, `logger.Error()` 等

  **WHY Each Reference Matters**:
  - auth.go: 需要理解每个分支（成功/失败）才能添加正确的日志级别
  - middleware/auth.go: JWT 校验的每个失败路径都需要对应的日志

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 注册审计日志
    Tool: Bash (curl)
    Preconditions: 服务端运行
    Steps:
      1. curl -s -X POST http://localhost:8890/api/v1/auth/register -H 'Content-Type: application/json' -d '{"username":"logtest","email":"log@test.com","password":"test123","nickname":"Test"}'
      2. grep 'register' server/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level, .username'
    Expected Result: INFO 级别日志含 username 和 "register" 关键词
    Failure Indicators: 无注册相关日志或含密码
    Evidence: .sisyphus/evidence/task-8-auth-register.txt

  Scenario: 登录失败审计
    Tool: Bash (curl)
    Preconditions: 服务端运行
    Steps:
      1. curl -s -X POST http://localhost:8890/api/v1/auth/login -H 'Content-Type: application/json' -d '{"account":"nobody","password":"wrong"}'
      2. grep 'login' server/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level'
    Expected Result: WARN 级别日志含 login 失败原因
    Failure Indicators: 无失败日志或级别为 INFO
    Evidence: .sisyphus/evidence/task-8-auth-login-fail.txt

  Scenario: JWT 校验失败日志
    Tool: Bash (curl)
    Preconditions: 服务端运行
    Steps:
      1. curl -s -H 'Authorization: Bearer invalid-token' http://localhost:8890/api/v1/auth/me
      2. grep -i 'jwt\|token\|auth' server/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level'
    Expected Result: WARN 级别日志记录 JWT 校验失败
    Evidence: .sisyphus/evidence/task-8-jwt-fail.txt
  ```

  **Commit**: YES (groups with T7, T9)
  - Message: `feat(logger): add audit logging to auth handlers and middleware`
  - Files: `server/backend/handlers/auth.go`, `server/backend/middleware/auth.go`

- [x] 9. 服务端后端 file handler + database 审计日志

  **What to do**:
  - 在 `server/backend/handlers/file.go` 每个 handler 中添加日志：
    - `ListFiles()`: DEBUG 级别记录路径 + 返回条目数
    - `CreateFolder()`: INFO 级别记录用户 + 路径
    - `UploadFile()`: INFO 级别记录用户 + 文件名 + 大小；WARN 记录冲突重命名；ERROR 记录写入失败
    - `DownloadFile()`: INFO 级别记录用户 + 文件名 + 大小
    - `DeleteFile()`: WARN 级别记录用户 + 路径（删除是敏感操作）
    - `RenameFile()`: INFO 级别记录用户 + 旧路径 + 新路径
  - 在 `server/backend/database/database.go` 添加日志：
    - `CreateUser()`: INFO 级别记录新用户 ID
    - 错误路径：ERROR 级别记录 JSON 写入失败
  - 在 `server/backend/utils/utils.go` 修复反模式：
    - `HashPassword()` 和 `GenerateRandomString()` 的 `log.Fatalf` → 返回 error

  **Must NOT do**:
  - 不记录文件内容
  - 不改变 handler 返回值或 HTTP 状态码
  - 不修改 `resolveUserPath()` 的安全逻辑

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 在现有 handler 中添加 slog 调用，需理解文件操作流程
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T8, T10-T14)
  - **Blocks**: None
  - **Blocked By**: T1 (logger 包)

  **References**:

  **Pattern References**:
  - `server/backend/handlers/file.go` — ListFiles, CreateFolder, UploadFile, DownloadFile, DeleteFile, RenameFile
  - `server/backend/database/database.go` — CreateUser, flushLocked

  **WHY Each Reference Matters**:
  - file.go: 每个文件操作 handler 都需要日志，理解成功/失败分支
  - database.go: 数据库操作也需要审计

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 文件上传审计
    Tool: Bash (curl)
    Preconditions: 已登录，有 JWT token
    Steps:
      1. echo "test content" > /tmp/test-upload.txt
      2. curl -s -X POST -H "Authorization: Bearer $TOKEN" -F "file=@/tmp/test-upload.txt" 'http://localhost:8890/api/v1/files/upload?path=/'
      3. grep 'upload' server/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level, .path'
    Expected Result: INFO 日志含文件名和路径
    Failure Indicators: 无上传日志
    Evidence: .sisyphus/evidence/task-9-file-upload.txt

  Scenario: 文件删除审计
    Tool: Bash (curl)
    Preconditions: 已上传的测试文件
    Steps:
      1. curl -s -X DELETE -H "Authorization: Bearer $TOKEN" 'http://localhost:8890/api/v1/files?path=/test-upload.txt'
      2. grep 'delete' server/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level, .path'
    Expected Result: WARN 级别日志（删除是敏感操作）
    Evidence: .sisyphus/evidence/task-9-file-delete.txt
  ```

  **Commit**: YES (groups with T7, T8)
  - Message: `feat(logger): add audit logging to file handlers and database operations`
  - Files: `server/backend/handlers/file.go`, `server/backend/database/database.go`, `server/backend/utils/utils.go`

- [x] 10. 客户端后端 sync engine 日志

  **What to do**:
  - 在 `client/backend/sync/engine.go` 添加日志：
    - `SyncTask()`: INFO 级别记录同步开始（含 taskID, taskName, localPath, remotePath）
    - `collectPendingFiles()`: INFO 级别记录总文件数 + 待上传数 + 跳过数（快照命中）
    - `uploadPendingFiles()`: ERROR 级别记录单文件上传失败（已有）；INFO 级别记录进度摘要（每 50 个文件一次）
    - `finishSuccessfulSync()`: INFO 级别增强为含摘要（上传文件数 + 总字节数 + 耗时秒数）
    - `failSync()`: ERROR 级别记录失败原因 + 已上传文件数
    - `acquire()`: WARN 级别记录锁竞争（同步已在进行）
    - `release()`: DEBUG 级别
  - 在 `client/backend/sync/snapshot.go` 添加日志：
    - `LoadSnapshot()`: DEBUG 级别记录加载结果（文件数或文件不存在）
    - `SaveSnapshot()`: INFO 级别记录保存成功，ERROR 级别记录失败（替换 `_ =` 静默忽略）
  - 在 `client/backend/sync/ondemand.go` 增强现有日志：
    - `ReleaseLocalSpace()`: 增加摘要（扫描文件数/释放文件数/跳过文件数）
    - `HydrateFromCloud()`: 增加摘要（下载文件数/跳过文件数/总字节数）

  **Must NOT do**:
  - 不逐文件记录 INFO 级别日志（避免 10K 文件产生 10K 条日志）
  - 不修改同步逻辑本身

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 同步引擎是核心逻辑，需要理解同步流程才能添加正确的日志
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T9, T11-T14)
  - **Blocks**: None
  - **Blocked By**: T2 (log 包重写)

  **References**:

  **Pattern References**:
  - `client/backend/sync/engine.go` — SyncTask, collectPendingFiles, uploadPendingFiles, finishSuccessfulSync, failSync, acquire, release
  - `client/backend/sync/snapshot.go` — LoadSnapshot, SaveSnapshot
  - `client/backend/sync/ondemand.go` — ReleaseLocalSpace, HydrateFromCloud

  **API/Type References**:
  - `client/backend/log/transfer_log.go:LogStore` — 通过 `pushLog()` 或直接 slog 调用

  **WHY Each Reference Matters**:
  - engine.go: 核心同步逻辑，每个函数都需要评估日志需求
  - snapshot.go: `_ = SaveSnapshot()` 是已知错误吞没点，必须修复
  - ondemand.go: 已有基础日志，需增强为摘要式

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 同步开始和完成日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行，有配置好的同步任务
    Steps:
      1. curl -s -X POST 'http://localhost:8090/api/v1/tasks/{taskId}/sync'
      2. grep 'sync.*start\|sync.*complete\|同步' client/backend/data/logs/*.jsonl | tail -5 | jq '.msg, .level'
    Expected Result: INFO 日志含同步开始和完成，完成日志含文件数/字节数/耗时
    Failure Indicators: 无同步开始日志或完成日志缺摘要
    Evidence: .sisyphus/evidence/task-10-sync-lifecycle.txt

  Scenario: Snapshot 保存错误不再被忽略
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. grep -n '_ =.*SaveSnapshot\|_ =.*snapshot' client/backend/sync/engine.go
    Expected Result: 无匹配（错误不再被 _ = 忽略）
    Failure Indicators: 仍有 `_ =` 忽略 SaveSnapshot 错误
    Evidence: .sisyphus/evidence/task-10-snapshot-fix.txt

  Scenario: 锁竞争日志
    Tool: Bash (curl)
    Preconditions: 任务正在同步中
    Steps:
      1. 连续发 2 次同步请求：curl -s -X POST 'http://localhost:8090/api/v1/tasks/{taskId}/sync' & curl -s -X POST 'http://localhost:8090/api/v1/tasks/{taskId}/sync'
      2. grep 'lock\|already\|竞争' client/backend/data/logs/*.jsonl | jq '.msg, .level'
    Expected Result: WARN 级别日志记录锁竞争
    Evidence: .sisyphus/evidence/task-10-lock-contention.txt
  ```

  **Commit**: YES (groups with T11, T12)
  - Message: `feat(logger): add structured logging to sync engine, snapshot, and on-demand modules`
  - Files: `client/backend/sync/engine.go`, `client/backend/sync/snapshot.go`, `client/backend/sync/ondemand.go`

- [x] 11. 客户端后端 watcher/fileprovider/proxy 日志

  **What to do**:
  - **watcher/watcher.go**:
    - `StartWatcher()`: INFO 级别记录 watcher 启动（taskID, localPath）
    - `StopWatcher()`: INFO 级别记录 watcher 停止
    - `runWatcher()`: INFO 级别记录防抖触发同步；WARN 记录 fsnotify Errors channel 事件；DEBUG 记录新增子目录监控
    - `RestoreAutoWatchers()`: INFO 级别记录恢复的 watcher 数量
  - **fileprovider/service.go**:
    - `StartWebDAVServer()`: INFO 级别记录 WebDAV 端口
    - `callFileProviderBridge()`: DEBUG 级别记录请求 URL + 响应状态码；ERROR 记录请求失败
    - `InitTask()`: INFO 级别记录 FP 域注册
    - `listRemoteItems()/deleteRemotePath()/renameRemotePath()/uploadFileReader()`: DEBUG 级别记录操作
  - **fileprovider/webdav.go**:
    - `OpenFile()` read: INFO 级别记录缓存未命中 → 远程下载
    - `OpenFile()` write: INFO 级别记录写入操作
    - `remoteWebDAVWriteFile.Close()`: INFO 级别记录上传完成 + 文件名；ERROR 记录上传失败
  - **fileprovider/handlers.go**:
    - 每个 handler: DEBUG 级别记录请求方法 + 路径
  - **proxy/client.go**:
    - `RawRequest()`: DEBUG 级别记录 method + URL + status_code + duration_ms；ERROR 记录请求失败
    - `UploadFile()`: INFO 级别记录文件名 + 大小 + 耗时
    - `DownloadRemoteFile()`: INFO 级别记录文件名 + 大小 + 耗时
    - `EnsureRemotePath()`: DEBUG 级别记录路径创建

  **Must NOT do**:
  - 不修改任何业务逻辑，仅添加日志调用
  - 不在 WebDAV 热路径上添加 INFO 以上级别（避免大量日志）

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 跨 3 个包添加日志，需要理解 fileprovider/proxy/watcher 的完整调用链
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T10, T12-T14)
  - **Blocks**: None
  - **Blocked By**: T2 (log 包重写)

  **References**:

  **Pattern References**:
  - `client/backend/watcher/watcher.go` — StartWatcher, StopWatcher, runWatcher, RestoreAutoWatchers
  - `client/backend/fileprovider/service.go` — StartWebDAVServer, callFileProviderBridge, InitTask, listRemoteItems, uploadFileReader
  - `client/backend/fileprovider/webdav.go` — OpenFile, remoteWebDAVWriteFile.Close, serveWebDAV
  - `client/backend/fileprovider/handlers.go` — Item, Children, Content, PutContent, RenameItem, CreateFolder, DeleteItem
  - `client/backend/proxy/client.go` — RawRequest, UploadFile, DownloadRemoteFile, EnsureRemotePath

  **WHY Each Reference Matters**:
  - watcher.go: fsnotify Errors channel 当前静默吞掉错误，这是 bug
  - fileprovider/*: 800+ 行零日志，是最关键的静默区域
  - proxy/client.go: 所有远程通信无日志，debug 时完全不可见

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Watcher 生命周期日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行，有同步任务
    Steps:
      1. curl -s -X POST 'http://localhost:8090/api/v1/tasks/{taskId}/auto/start'
      2. grep 'watcher' client/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level'
      3. curl -s -X POST 'http://localhost:8090/api/v1/tasks/{taskId}/auto/stop'
      4. grep 'watcher' client/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level'
    Expected Result: INFO 日志记录 watcher start 和 stop
    Evidence: .sisyphus/evidence/task-11-watcher-lifecycle.txt

  Scenario: Proxy 请求日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行，已登录
    Steps:
      1. curl -s 'http://localhost:8090/api/v1/remote/folders'
      2. grep 'proxy\|remote\|request' client/backend/data/logs/*.jsonl | tail -5 | jq '.msg, .level, .method, .status_code'
    Expected Result: DEBUG 日志记录远程 HTTP 请求 method/url/status
    Evidence: .sisyphus/evidence/task-11-proxy-logging.txt

  Scenario: fileprovider 不再完全静默
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. 统计 fileprovider 包中的日志调用数：grep -rn 'log\.\|logger\.\|slog\.' client/backend/fileprovider/ | grep -v '_test.go' | wc -l
    Expected Result: 至少 10 处日志调用（之前为 0）
    Evidence: .sisyphus/evidence/task-11-fp-logging-count.txt
  ```

  **Commit**: YES (groups with T10, T12)
  - Message: `feat(logger): add logging to watcher, fileprovider, and proxy modules`
  - Files: `client/backend/watcher/watcher.go`, `client/backend/fileprovider/*.go`, `client/backend/proxy/client.go`

- [x] 12. 客户端后端 HTTP handler 日志

  **What to do**:
  - 在所有 HTTP handler 文件中添加日志：
  - **handlers_auth.go**:
    - `proxyRegister()`: INFO 记录注册代理请求；ERROR 记录服务端错误
    - `proxyLogin()`: INFO 记录登录成功；WARN 记录登录失败；INFO 记录 token 保存
    - `logout()`: INFO 记录登出
    - `proxyMe()`: DEBUG 级别
  - **handlers_task.go**:
    - `createTask()`: INFO 记录任务创建（含名称、本地路径、远程路径）
    - `updateTask()`: INFO 记录任务更新（含变更字段）
    - `deleteTask()`: WARN 记录任务删除（含任务名）
    - `listTasks()`: DEBUG 级别
  - **handlers_sync.go**:
    - `syncTaskNow()`: INFO 记录手动同步触发
    - `startAutoTask()`: INFO 记录自动备份启动
    - `stopAutoTask()`: INFO 记录自动备份停止
  - **handlers_ondemand.go**:
    - `releaseLocalSpace()`: INFO 记录空间释放请求
    - `hydrateFromCloud()`: INFO 记录云文件下载请求
  - **handlers_platform.go**: 已有部分日志（增强现有 pushLog 调用）
  - **handlers_remote.go**:
    - `listRemoteFolders()`: DEBUG 级别记录远程浏览

  **Must NOT do**:
  - 不修改 handler 返回值或错误处理逻辑
  - 不在 handler 中添加 slog 全局调用（应通过 pushLog 或 logStore）

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 批量在 handler 中添加日志调用，工作量大但模式统一
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T11, T13, T14)
  - **Blocks**: None
  - **Blocked By**: T2 (log 包重写)

  **References**:

  **Pattern References**:
  - `client/backend/handlers_auth.go` — 所有认证代理 handler
  - `client/backend/handlers_task.go` — 任务 CRUD handler
  - `client/backend/handlers_sync.go` — 同步控制 handler
  - `client/backend/handlers_ondemand.go` — 按需同步 handler
  - `client/backend/handlers_platform.go` — 平台 handler（已有部分 pushLog）
  - `client/backend/handlers_remote.go` — 远程浏览 handler
  - `client/backend/main.go:43-45` — AppState.pushLog() 包装模式

  **WHY Each Reference Matters**:
  - 所有 handler 文件：逐一添加日志，需要理解每个 handler 的成功/失败分支
  - pushLog() 模式：保持一致的日志调用方式

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 任务创建审计日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行，已登录
    Steps:
      1. curl -s -X POST 'http://localhost:8090/api/v1/tasks' -H 'Content-Type: application/json' -d '{"name":"日志测试任务","localPath":"/tmp/test","remotePath":"/test","autoBackup":false}'
      2. grep 'task.*create\|创建' client/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level, .taskName'
    Expected Result: INFO 日志含任务名和路径
    Evidence: .sisyphus/evidence/task-12-task-create.txt

  Scenario: 登录代理日志
    Tool: Bash (curl)
    Preconditions: 客户端后端运行
    Steps:
      1. curl -s -X POST 'http://localhost:8090/api/v1/auth/login' -H 'Content-Type: application/json' -d '{"account":"test","password":"wrong"}'
      2. grep 'login\|登录' client/backend/data/logs/*.jsonl | tail -3 | jq '.msg, .level'
    Expected Result: WARN 日志记录登录失败
    Evidence: .sisyphus/evidence/task-12-auth-proxy.txt

  Scenario: 所有 handler 文件均有日志调用
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. for f in handlers_auth handlers_task handlers_sync handlers_ondemand handlers_remote; do echo "=== $f ==="; grep -c 'pushLog\|logStore\|slog\.' client/backend/${f}.go || echo "0"; done
    Expected Result: 每个 handler 文件至少 1 处日志调用
    Evidence: .sisyphus/evidence/task-12-handler-coverage.txt
  ```

  **Commit**: YES (groups with T10, T11)
  - Message: `feat(logger): add logging to all client HTTP handlers`
  - Files: `client/backend/handlers_auth.go`, `client/backend/handlers_task.go`, `client/backend/handlers_sync.go`, `client/backend/handlers_ondemand.go`, `client/backend/handlers_platform.go`, `client/backend/handlers_remote.go`

- [x] 13. Electron 修复：Windows stdio + macOS 日志统一

  **What to do**:
  - **Windows (`client/front/electron/main.js`)**:
    - 将 `stdio: 'ignore'` 改为文件重定向：创建 `userData/logs/` 目录，将 stdout/stderr 重定向到 `backend.log`
    - 启动后轮询 `/health` 确认后端就绪（解决竞态问题，符合构建规范）
  - **macOS (`client/front-mac/electron/main.js`)**:
    - 保留现有 `backend.log` 重定向（向后兼容）
    - 在关于对话框或设置中说明新日志位置（`data/logs/*.jsonl`）
    - 无需改变现有重定向逻辑（slog 文件输出是独立的）

  **Must NOT do**:
  - 不删除 macOS 现有的 backend.log 重定向（避免破坏现有调试习惯）
  - 不引入新的 npm 依赖

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 修改两处 Electron main.js 的 stdio 配置，改动小且明确
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T12, T14)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `client/front/electron/main.js:45` — Windows stdio:'ignore' 需要修复
  - `client/front-mac/electron/main.js:154-162` — macOS 现有 backend.log 重定向模式

  **WHY Each Reference Matters**:
  - Windows main.js: 当前 stdio:'ignore' 丢失所有 Go 输出
  - macOS main.js: 参考其文件重定向模式用于 Windows 修复

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Windows Electron 不再使用 stdio:'ignore'
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. grep "stdio" client/front/electron/main.js
    Expected Result: 无 'ignore' 字样，改为文件路径或 pipe
    Failure Indicators: 仍有 stdio: 'ignore'
    Evidence: .sisyphus/evidence/task-13-windows-stdio.txt

  Scenario: macOS Electron 保持 backend.log 重定向
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. grep "backendLogFd\|backend.log\|logs/" client/front-mac/electron/main.js | wc -l
    Expected Result: 至少 2 处匹配（创建 + 重定向）
    Evidence: .sisyphus/evidence/task-13-macos-preserve.txt
  ```

  **Commit**: YES
  - Message: `fix(electron): capture Windows backend output and preserve macOS log redirection`
  - Files: `client/front/electron/main.js`, `client/front-mac/electron/main.js`

- [x] 14. 服务端后端 utils 反模式修复

  **What to do**:
  - 修复 `server/backend/utils/utils.go` 中的 `log.Fatalf` 反模式：
    - `HashPassword()`: 将 `log.Fatalf` 改为返回 `error`
    - `GenerateRandomString()`: 将 `log.Fatalf` 改为返回 `error`
  - 更新所有调用点：
    - `server/backend/handlers/auth.go:Register()` — 处理 `HashPassword` 返回的 error
    - 其他可能的调用点

  **Must NOT do**:
  - 不修改函数签名以外的行为
  - 不在此次修复中重构其他 utils 函数

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 两个函数签名修改 + 调用点适配
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T13)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `server/backend/utils/utils.go:19,33` — 两个 `log.Fatalf` 反模式位置
  - `server/backend/handlers/auth.go:Register()` — HashPassword 调用点

  **WHY Each Reference Matters**:
  - utils.go: 反模式位置，需改为返回 error
  - auth.go: 需要适配新的函数签名

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: utils 中不再有 log.Fatalf
    Tool: Bash (grep)
    Preconditions: 无
    Steps:
      1. grep -n 'log.Fatalf' server/backend/utils/utils.go
    Expected Result: 无匹配
    Failure Indicators: 仍有 log.Fatalf
    Evidence: .sisyphus/evidence/task-14-no-fatal.txt

  Scenario: 服务端仍可正常编译
    Tool: Bash
    Preconditions: 无
    Steps:
      1. cd server/backend && go build ./...
    Expected Result: 编译成功，无错误
    Failure Indicators: 编译失败（调用点未适配）
    Evidence: .sisyphus/evidence/task-14-build.txt
  ```

  **Commit**: YES
  - Message: `fix(utils): replace log.Fatalf with error returns in utility functions`
  - Files: `server/backend/utils/utils.go`, `server/backend/handlers/auth.go`

- [x] 15. 客户端前端 LogViewer 完整实现

  **What to do**:
  - 完善 `client/front/src/views/LogViewer.vue`（T5 创建的骨架）：
    - **筛选面板**：
      - 级别下拉框：全部 / DEBUG / INFO / WARN / ERROR
      - 任务选择框：动态加载任务列表，按 taskId 过滤
      - 关键词输入框：搜索 message 和 filePath
      - 时间范围：开始时间 + 结束时间（`<input type="datetime-local">`）
      - 查询按钮 + 重置按钮
    - **日志表格**：
      - 列：时间（格式化显示）、级别（彩色标签）、任务名、文件路径、消息
      - 级别颜色：DEBUG→灰, INFO→蓝, WARN→橙, ERROR→红
      - 空状态提示：「暂无日志」
    - **分页**：
      - 底部分页栏：上一页 / 页码 / 下一页 / 总条数显示
      - pageSize 默认 20
    - **导出**：
      - 导出按钮 → 调用 `/logs/export` → 下载 JSONL 文件
    - **数据对接**：
      - `fetchLogs()` 调用 `GET /api/v1/logs` 传递所有筛选参数
      - 删除现有 Dashboard 中的旧日志卡片（`log-card` 区域）
    - **导航高亮**：
      - 在 App.vue 导航中高亮当前路由

  **Must NOT do**:
  - 不实现实时推送（明确排除）
  - 不添加 Pinia/Vuex
  - 不重构 DashboardView 中的其他功能
  - 不自动刷新（仅用户点击查询按钮手动触发）

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 完整前端页面实现，包含筛选、表格、分页、导出等交互
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 确保日志页面与现有深色主题风格一致，设计质量高

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T16)
  - **Blocks**: F1-F4
  - **Blocked By**: T3 (API 增强), T5 (LogViewer 骨架)

  **References**:

  **Pattern References**:
  - `client/front/src/views/LogViewer.vue` — T5 创建的骨架
  - `client/front/src/views/DashboardView.vue` — 现有 fetchLogs 模式可参考（api.get + ref）
  - `client/front/src/style.css` — 深色主题样式

  **API/Type References**:
  - `GET /api/v1/logs` — 增强后的 API（T3），参数：page, pageSize, taskId, level, keyword, startTime, endTime
  - `GET /api/v1/logs/export` — 日志导出 API（T3）
  - `GET /api/v1/tasks` — 任务列表（用于任务选择框）

  **WHY Each Reference Matters**:
  - LogViewer.vue: T5 的骨架需要填充实际功能
  - DashboardView.vue: 理解现有的 API 调用模式（axios 配置）
  - style.css: 保持视觉一致性

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 日志页面加载并显示日志
    Tool: Playwright
    Preconditions: 客户端后端运行，有日志数据
    Steps:
      1. Navigate to 'http://localhost:5173/logs'
      2. Click '查询' button
      3. Assert table rows exist (selector: 'table tbody tr' or '.log-table tr')
      4. Assert first row has: timestamp cell, level badge, message cell
    Expected Result: 表格渲染日志条目，含时间戳和级别标签
    Failure Indicators: 表格为空或缺少列
    Evidence: .sisyphus/evidence/task-15-log-page-loaded.png

  Scenario: 级别筛选功能
    Tool: Playwright
    Preconditions: 有不同级别的日志
    Steps:
      1. Navigate to 'http://localhost:5173/logs'
      2. Select level 'ERROR' from dropdown
      3. Click '查询'
      4. Assert all level badges contain 'error'
    Expected Result: 仅显示 error 级别日志
    Failure Indicators: 显示非 error 级别日志
    Evidence: .sisyphus/evidence/task-15-level-filter.png

  Scenario: 分页功能
    Tool: Playwright
    Preconditions: 有超过 20 条日志
    Steps:
      1. Navigate to 'http://localhost:5173/logs'
      2. Click '查询'
      3. Assert pagination shows '第 1 页' and total count
      4. Click '下一页'
      5. Assert page number changes to 2
    Expected Result: 分页切换正常
    Failure Indicators: 分页不切换或数据不变化
    Evidence: .sisyphus/evidence/task-15-pagination.png

  Scenario: 关键词搜索
    Tool: Playwright
    Preconditions: 有包含"同步"的日志
    Steps:
      1. Navigate to 'http://localhost:5173/logs'
      2. Type '同步' in keyword input
      3. Click '查询'
      4. Assert table rows contain '同步' in message cell
    Expected Result: 仅显示匹配关键词的日志
    Evidence: .sisyphus/evidence/task-15-keyword-search.png

  Scenario: 日志导出
    Tool: Playwright
    Preconditions: 有日志数据
    Steps:
      1. Navigate to 'http://localhost:5173/logs'
      2. Click '导出' button
      3. Assert download initiated (check for download event or file)
    Expected Result: JSONL 文件下载触发
    Evidence: .sisyphus/evidence/task-15-export.txt
  ```

  **Commit**: YES (groups with T5)
  - Message: `feat(logger): implement full log viewer page with filters, pagination, and export`
  - Files: `client/front/src/views/LogViewer.vue`, `client/front/src/App.vue`, `client/front/src/views/DashboardView.vue`
  - Pre-commit: `cd client/front && npm run build`

- [x] 16. 服务端前端 LogViewer 完整实现

  **What to do**:
  - 完善 `server/front/src/components/LogViewer.vue`（T6 创建的骨架）：
    - **筛选面板**：
      - 级别下拉框：全部 / DEBUG / INFO / WARN / ERROR
      - 关键词输入框：搜索 msg 和 path
      - 时间范围：开始时间 + 结束时间
      - 查询按钮 + 重置按钮
    - **日志表格**：
      - 列：时间、级别、用户 ID、操作、路径、消息
      - 级别颜色：同客户端
    - **分页**：同客户端
    - **导出**：同客户端
    - **数据对接**：
      - 调用 `GET /api/v1/logs` + `Authorization` header
      - 调用 `GET /api/v1/logs/export`
  - 在 `server/front/src/App.vue` 中：
    - 登录后添加 tab 切换（文件浏览 ↔ 日志查看）
    - 文件浏览为默认 tab

  **Must NOT do**:
  - 不引入 vue-router
  - 不重构现有文件浏览器功能
  - 不自动刷新

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 完整前端组件实现
  - **Skills**: [`frontend-design`]
    - `frontend-design`: 保持与现有深色玻璃态 UI 一致

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T15)
  - **Blocks**: F1-F4
  - **Blocked By**: T4 (服务端日志 API), T6 (LogViewer 骨架)

  **References**:

  **Pattern References**:
  - `server/front/src/components/LogViewer.vue` — T6 创建的骨架
  - `server/front/src/App.vue` — 现有 App 结构，需添加 tab 切换
  - `server/front/src/style.css` — 深色玻璃态主题

  **API/Type References**:
  - `GET /api/v1/logs` — 服务端日志 API（T4）
  - `GET /api/v1/logs/export` — 服务端日志导出（T4）

  **WHY Each Reference Matters**:
  - LogViewer.vue: T6 骨架需填充功能
  - App.vue: 理解现有 auth tab 切换模式

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 日志 tab 显示日志
    Tool: Playwright
    Preconditions: 服务端前后端运行，已登录
    Steps:
      1. Login at 'http://localhost:5173/'
      2. Click '日志' tab
      3. Click '查询' button
      4. Assert log table renders with rows
    Expected Result: 日志表格显示条目
    Failure Indicators: 表格为空或组件不渲染
    Evidence: .sisyphus/evidence/task-16-log-viewer.png

  Scenario: 级别筛选和分页
    Tool: Playwright
    Preconditions: 有日志数据
    Steps:
      1. Select level 'ERROR' from dropdown
      2. Click '查询'
      3. Assert all rows show 'error' level badge
      4. If total > pageSize, click '下一页' and verify page change
    Expected Result: 级别过滤和分页工作正常
    Evidence: .sisyphus/evidence/task-16-filter-pagination.png

  Scenario: 日志导出
    Tool: Bash (curl)
    Preconditions: 服务端运行，有 JWT
    Steps:
      1. curl -s -o /tmp/server-log-export.jsonl -H "Authorization: Bearer $TOKEN" 'http://localhost:8890/api/v1/logs/export'
      2. head -1 /tmp/server-log-export.jsonl | jq '.time, .level, .msg'
    Expected Result: 文件下载成功，每行合法 JSON
    Evidence: .sisyphus/evidence/task-16-export.jsonl
  ```

  **Commit**: YES (groups with T6)
  - Message: `feat(logger): implement server-side log viewer with filters and pagination`
  - Files: `server/front/src/components/LogViewer.vue`, `server/front/src/App.vue`
  - Pre-commit: `cd server/front && npm run build`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./server/backend/logger/ -v` + `go test ./client/backend/log/ -v` + `go vet ./...`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify no passwords/tokens in log output.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill for UI)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: empty logs, large log volume, invalid filter params. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1 批次**: `feat(logger): add structured logging infrastructure` - server/backend/logger/..., client/backend/log/..., handlers_log.go, client/front/src/router/, server/front/src/components/LogViewer.vue
  - Pre-commit: `go test ./server/backend/logger/ ./client/backend/log/ -v`
- **Wave 2 批次**: `feat(logger): add audit logging to all handlers and core modules` - server/backend/middleware/, server/backend/handlers/, client/backend/sync/, client/backend/watcher/, client/backend/fileprovider/, client/backend/proxy/, client/backend/handlers_*.go, electron/main.js
  - Pre-commit: `go vet ./...`
- **Wave 3 批次**: `feat(logger): implement frontend log viewer with filters and pagination` - client/front/src/views/LogViewer.vue, client/front/src/router/, server/front/src/components/LogViewer.vue, server/front/src/App.vue
  - Pre-commit: `cd client/front && npm run build && cd ../../server/front && npm run build`
- **最终**: `feat(logger): complete logging enhancement system` - all files
  - Pre-commit: full test suite + build

---

## Success Criteria

### Verification Commands
```bash
# Go 包测试
cd server/backend && go test ./logger/ -v          # Expected: ALL PASS
cd client/backend && go test ./log/ -v              # Expected: ALL PASS

# 服务端日志文件
ls server/backend/data/logs/*.jsonl                  # Expected: 至少 1 个文件
head -1 server/backend/data/logs/*.jsonl | jq .      # Expected: 有效 JSON 含 level/time/msg 字段

# 客户端日志文件
ls client/backend/data/logs/*.jsonl                  # Expected: 至少 1 个文件

# 服务端日志 API
curl -s http://localhost:8890/api/v1/logs?limit=5    # Expected: {"items":[...],"total":N,"page":1,"pageSize":5}
curl -s http://localhost:8890/api/v1/logs?level=error # Expected: 所有 items level=error
curl -s "http://localhost:8890/api/v1/logs?keyword=login" # Expected: 匹配结果
curl -s http://localhost:8890/api/v1/logs/export > /tmp/zcopy-logs.jsonl # Expected: 文件下载

# 客户端日志 API
curl -s http://localhost:8090/api/v1/logs?limit=5     # Expected: {"items":[...],"total":N,"page":1,"pageSize":5}
curl -s http://localhost:8090/api/v1/logs?taskId=xxx  # Expected: 按任务过滤
curl -s http://localhost:8090/api/v1/logs?level=error # Expected: 所有 items level=error
curl -s http://localhost:8090/api/v1/logs/export > /tmp/zcopy-client-logs.jsonl

# 日志轮转验证
ls -la server/backend/data/logs/                      # Expected: 按 YYYY-MM-DD 命名
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (`go test ./server/backend/logger/ ./client/backend/log/ -v`)
- [ ] Server backend JSONL log files exist and are valid
- [ ] Client backend JSONL log files exist and are valid
- [ ] Server GET /logs API returns paginated results with all filter params
- [ ] Client GET /logs API returns paginated results with all filter params
- [ ] Client frontend `/logs` route shows log viewer
- [ ] Server frontend log section renders and filters work
- [ ] No passwords/tokens/JWT appear in any log file
- [ ] Windows Electron captures backend output to file
