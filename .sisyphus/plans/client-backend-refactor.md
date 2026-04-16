# Refactor ZCopy Client Backend: 2-File Monolith → Well-Structured Go Project

## TL;DR

> **Quick Summary**: Break the 2575-line Go client backend (main.go 1507 lines + fileprovider.go 1068 lines) into ~20 focused files across 11 packages, following the server backend pattern. Each step is incremental and keeps the code compilable. TDD-oriented with tests alongside each package.
>
> **Deliverables**:
> - 11 Go packages: models, config, utils, middleware, store, auth, proxy, sync, watcher, handlers, fileprovider, platform
> - 7 interfaces: TaskRepository, TokenManager, RemoteClient, SyncEngine, WatchManager, LogStore, FileProviderBridge
> - Test files for every package
> - Graceful shutdown with signal handling
> - All existing API endpoints preserved identically
> - syncTask() decomposed from 164 lines into ≤50-line functions
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 8 waves
> **Critical Path**: Wave 1 → Wave 2 → Wave 3 → Wave 4 → Wave 5 → Wave 6 → Wave 7 → Wave 8 → Final Verification

---

## Context

### Original Request
Refactor `client/backend/` from the current 2-file monolith into a well-structured Go project. Define exact target package structure, interfaces, incremental steps, function mapping, new abstractions, test files, and commit strategy. Keep the system working at every step.

### Architecture Review Findings
- **God object**: `AppState` has 12 fields, 4 mutexes, 6 responsibilities (auth proxy + task management + sync engine + file watcher + WebDAV + CFAPI)
- **Monolithic file**: `main.go` 1507 lines mixing data structures, persistence, HTTP handlers, sync engine, CFAPI
- **Long functions**: `syncTask()` 164 lines flat logic, `hydrateFromCloud()` 94 lines
- **Concurrent state**: `map[string]bool` + manual mutex for sync status tracking
- **Global token**: JWT in memory only, lost on restart
- **No error recovery**: fsnotify watcher errors silently ignored
- **PowerShell hardcoded**: 65-line CFAPI script embedded in Go string
- **Zero tests**: No test coverage anywhere

### Research Findings
- **Server backend pattern** (well-structured reference): main.go 95 lines, handlers split by domain, database in own package, models separated
- **Current function count**: ~55 functions/methods in main.go, ~40 in fileprovider.go
- **Current API routes**: 24 HTTP endpoints across auth, tasks, sync, on-demand, file-provider, system
- **Go module**: `zcopy-client-backend`, Go 1.25, dependencies: gin, fsnotify, yaml.v3, golang.org/x/net/webdav

---

## Work Objectives

### Core Objective
Decompose the client backend into focused packages following Go best practices, introducing interfaces for testability, while preserving every existing API contract and behavior.

### Concrete Deliverables

**Target Package Structure** (with file names and estimated line counts):

```
client/backend/
├── main.go                          (~80 lines)   startup, routing, wiring, graceful shutdown
├── config/
│   ├── config.go                    (~70 lines)   AppConfig struct, LoadConfig(), resolveConfigPath()
│   └── config.yaml                  (unchanged)
├── models/
│   └── models.go                    (~80 lines)   BackupTask, SyncReport, TransferLog, TaskSnapshot, FileFingerprint, LocalFileItem, WatchController, SyncStatus
├── store/
│   ├── task_store.go                (~110 lines)  TaskStore struct + CRUD methods
│   └── task_store_test.go           (~120 lines)  unit tests
├── auth/
│   ├── token.go                     (~65 lines)   TokenManager interface + InMemoryTokenManager
│   └── token_test.go                (~60 lines)   unit tests
├── proxy/
│   ├── client.go                    (~70 lines)   RemoteClient interface + HTTPRemoteClient
│   └── client_test.go               (~80 lines)   unit tests with httptest server
├── sync/
│   ├── engine.go                    (~130 lines)  SyncEngine interface + implementation (syncTask decomposed)
│   ├── snapshot.go                  (~55 lines)   snapshot load/save/path
│   ├── ondemand.go                  (~100 lines)  releaseLocalSpace + hydrateFromCloud
│   ├── engine_test.go               (~100 lines)  unit tests
│   └── snapshot_test.go             (~60 lines)   unit tests
├── watcher/
│   ├── watcher.go                   (~90 lines)   WatchManager interface + fsnotify implementation
│   └── watcher_test.go              (~60 lines)   unit tests
├── log/
│   ├── transfer_log.go              (~50 lines)   LogStore interface + RingBufferLogStore
│   └── transfer_log_test.go         (~60 lines)   unit tests
├── handlers/
│   ├── auth.go                      (~90 lines)   proxyRegister, proxyLogin, proxyMe, logout
│   ├── task.go                      (~160 lines)  listTasks, createTask, updateTask, deleteTask
│   ├── sync.go                      (~70 lines)   syncTaskNow, startAutoTask, stopAutoTask
│   ├── ondemand.go                  (~40 lines)   releaseLocalSpace, hydrateFromCloud (delegate to sync)
│   ├── remote.go                    (~80 lines)   listRemoteFolders
│   ├── log.go                       (~50 lines)   listLogs
│   ├── cfapi.go                     (~100 lines)  initTaskCFAPI, taskCFAPIStatus
│   ├── system.go                    (~30 lines)   systemCapabilities
│   └── fileprovider.go              (~250 lines)  7 file-provider HTTP handlers
├── fileprovider/
│   ├── webdav.go                    (~80 lines)   startWebDAVServer, serveWebDAV
│   ├── bridge.go                    (~100 lines)  FileProviderBridge + callFileProviderBridge
│   ├── remote_ops.go                (~110 lines)  listRemoteItems, deleteRemotePath, renameRemotePath, uploadFileReader, remoteInfoForPath, remoteDirEntries
│   ├── webdav_fs.go                 (~200 lines)  remoteWebDAVFS (Mkdir, OpenFile, RemoveAll, Rename, Stat) + FileInfo types
│   ├── mirror.go                    (~70 lines)   taskLocalPath, localInfoForPath, writeLocalMirrorFile, renameLocalMirrorPath
│   ├── helpers.go                   (~50 lines)   normalizeWebDAVPath, joinRemotePath, randomBridgeSecret, normalizeFileProviderTaskID, fileProviderIdentifier
│   └── webdav_fs_test.go            (~80 lines)   unit tests
├── platform/
│   ├── windows.go                   (~100 lines)  registerWindowsSyncRoot, isSyncRootRegistered, syncRootID
│   ├── windows_test.go              (~40 lines)   unit tests (build-tagged)
│   └── syncroot.go                  (~20 lines)   syncRootID (shared, non-platform-specific)
├── utils/
│   └── utils.go                     (~40 lines)   parseJSONMessage, normalizeRemote, calcSpeed, collectLocalFiles
├── middleware/
│   └── cors.go                      (~25 lines)   cors() middleware
├── data/
│   ├── tasks.json                   (unchanged)
│   └── snapshots/                   (unchanged)
├── go.mod
└── go.sum
```

### Definition of Done
- [ ] `go build ./...` succeeds with zero errors
- [ ] `go vet ./...` passes clean
- [ ] `go test ./...` passes all tests
- [ ] All 24 API endpoints respond identically to pre-refactor behavior
- [ ] No file exceeds 300 lines
- [ ] No function exceeds 50 lines
- [ ] No struct has more than 3 responsibilities
- [ ] 7 interfaces defined and used for dependency injection
- [ ] Test coverage exists for every package

### Must Have
- Every existing API endpoint preserved with identical request/response contracts
- Incremental refactoring: `go build` passes after every commit
- Interface abstractions for all major components (RemoteClient, TaskRepository, etc.)
- Unit tests for store, auth, proxy, sync, watcher, log packages
- Graceful shutdown with OS signal handling
- syncTask() decomposed into ≤50-line functions
- fileprovider.go split into one `fileprovider/` package with multiple files
- PowerShell script extracted to a separate file or loaded at runtime

### Must NOT Have (Guardrails)
- **No behavioral changes**: Do NOT fix bugs during refactoring — preserve existing behavior exactly
- **No new features**: Do NOT add retry logic, conflict detection, etc. — only restructure
- **No API contract changes**: Request/response JSON shapes must remain identical
- **No framework changes**: Keep Gin, fsnotify, webdav — don't swap libraries
- **No config format changes**: config.yaml remains identical
- **No data format changes**: tasks.json and snapshot files remain backward-compatible
- **AI slop to avoid**:
  - Do NOT over-abstract with generic interfaces when a concrete type suffices
  - Do NOT add excessive comments explaining obvious Go patterns
  - Do NOT create wrapper types that add no value (e.g., `type TaskID string`)
  - Do NOT rename existing exported JSON fields (breaks API contract)
  - Do NOT add logging framework — keep `log` package

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO
- **Automated tests**: YES (TDD-oriented — write interface + test skeleton before implementation)
- **Framework**: Go standard `testing` package (already available in Go 1.25)
- **TDD approach**: For each package, define the interface first, write test cases, then move implementation

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go packages**: Use `go test`, `go build`, `go vet` — verify compilation, run tests, check for race conditions
- **API endpoints**: Use Bash (curl) — start server, hit each endpoint, assert status + response fields
- **Integration**: Use Bash — build binary, start, exercise full workflow, verify responses

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - pure extraction, no logic changes):
├── Task 1:  Extract models to models/ package [quick]
├── Task 2:  Extract config to config/ package [quick]
├── Task 3:  Extract utils to utils/ package [quick]
└── Task 4:  Extract CORS middleware to middleware/ package [quick]

Wave 2 (Persistence + Auth - introduce interfaces + tests):
├── Task 5:  Extract TaskStore to store/ with TaskRepository interface + tests [unspecified-high]
└── Task 6:  Extract token management to auth/ with TokenManager interface + tests [quick]

Wave 3 (Remote communication layer):
└── Task 7:  Extract remote client to proxy/ with RemoteClient interface + tests [unspecified-high]

Wave 4 (Core engines - parallel extraction):
├── Task 8:  Extract sync engine to sync/ with SyncEngine interface + tests [deep]
├── Task 9:  Extract watcher to watcher/ with WatchManager interface + tests [unspecified-high]
├── Task 10: Extract Windows CFAPI to platform/ + tests [quick]
└── Task 11: Extract transfer log to log/ with LogStore interface + tests [quick]

Wave 5 (HTTP handlers - split by domain):
├── Task 12: Extract auth handlers (proxyRegister, proxyLogin, proxyMe, logout) [quick]
├── Task 13: Extract task CRUD handlers [unspecified-high]
├── Task 14: Extract sync + on-demand handlers [unspecified-high]
├── Task 15: Extract remote + log handlers [quick]
├── Task 16: Extract CFAPI + system handlers [quick]
└── Task 17: Extract file-provider HTTP handlers [unspecified-high]

Wave 6 (File Provider package):
├── Task 18: Extract WebDAV server + bridge + remote_ops to fileprovider/ [deep]
├── Task 19: Extract WebDAV FS implementation + mirror to fileprovider/ [deep]
└── Task 20: Extract fileprovider helpers + wire fileprovider handlers [quick]

Wave 7 (Main rewrite + graceful shutdown):
├── Task 21: Rewrite main.go as orchestrator with dependency injection [deep]
└── Task 22: Add graceful shutdown with OS signal handling [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews, then user okay):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA — exercise all 24 API endpoints (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Wave 1 → Wave 2 → Wave 3 → Wave 4 → Wave 5 → Wave 6 → Wave 7 → Final
Max Concurrent: 4 (Wave 1, Wave 4)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|------------|--------|------|
| 1 (models) | None | 5,6,7,8,9,10,11,12-17,18-20,21 | 1 |
| 2 (config) | None | 5,6,7,21 | 1 |
| 3 (utils) | None | 7,8,15,18,19 | 1 |
| 4 (middleware) | None | 21 | 1 |
| 5 (store) | 1,2 | 8,9,13,14,17,21 | 2 |
| 6 (auth) | 1 | 7,12,18,21 | 2 |
| 7 (proxy) | 1,3,6 | 8,18,19,21 | 3 |
| 8 (sync) | 1,5,6,7 | 14,21 | 4 |
| 9 (watcher) | 1,5,8 | 13,14,21 | 4 |
| 10 (platform) | 1 | 16,21 | 4 |
| 11 (log) | 1 | 15,21 | 4 |
| 12 (auth handlers) | 6,7 | 21 | 5 |
| 13 (task handlers) | 5,9 | 21 | 5 |
| 14 (sync handlers) | 5,8,9 | 21 | 5 |
| 15 (remote+log handlers) | 7,11 | 21 | 5 |
| 16 (cfapi+system handlers) | 10 | 21 | 5 |
| 17 (fileprovider handlers) | 7,5 | 20,21 | 5 |
| 18 (webdav+bridge) | 1,3,6,7 | 20,21 | 6 |
| 19 (webdav_fs+mirror) | 1,3,7 | 20,21 | 6 |
| 20 (helpers+wire handlers) | 17,18,19 | 21 | 6 |
| 21 (main rewrite) | 4,5,6,7,8,9,10,11,12,13,14,15,16,17,20 | 22 | 7 |
| 22 (graceful shutdown) | 21 | Final | 7 |

### Agent Dispatch Summary

- **Wave 1**: **4 tasks** — T1-T4 → `quick`
- **Wave 2**: **2 tasks** — T5 → `unspecified-high`, T6 → `quick`
- **Wave 3**: **1 task** — T7 → `unspecified-high`
- **Wave 4**: **4 tasks** — T8 → `deep`, T9 → `unspecified-high`, T10 → `quick`, T11 → `quick`
- **Wave 5**: **6 tasks** — T12 → `quick`, T13 → `unspecified-high`, T14 → `unspecified-high`, T15 → `quick`, T16 → `quick`, T17 → `unspecified-high`
- **Wave 6**: **3 tasks** — T18 → `deep`, T19 → `deep`, T20 → `quick`
- **Wave 7**: **2 tasks** — T21 → `deep`, T22 → `quick`
- **FINAL**: **4 tasks** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## Interface Definitions

These interfaces are introduced during extraction to decouple dependencies and enable testing.

### TaskRepository (store/ package)
```go
type TaskRepository interface {
    List() []models.BackupTask
    Get(id string) (models.BackupTask, bool)
    Upsert(task models.BackupTask) error
    Remove(id string) error
}
```

### TokenManager (auth/ package)
```go
type TokenManager interface {
    GetToken() string
    SetToken(token string)
}
```

### RemoteClient (proxy/ package)
```go
type RemoteClient interface {
    RawRequest(method, path string, body io.Reader, contentType, token string) ([]byte, int, error)
    UploadFile(localFile, remoteDir, token string) error
    DownloadRemoteFile(remotePath, localPath, token string) error
    EnsureRemotePath(path, token string) error
}
```

### SyncEngine (sync/ package)
```go
type SyncEngine interface {
    SyncTask(taskID string) error
}
```

### WatchManager (watcher/ package)
```go
type WatchManager interface {
    StartWatcher(taskID string) error
    StopWatcher(taskID string)
    RestoreAutoWatchers()
}
```

### LogStore (log/ package)
```go
type LogStore interface {
    Push(level string, task models.BackupTask, filePath, message string)
    List(taskID, level string, limit int) []models.TransferLog
}
```

### FileProviderBridge (fileprovider/ package)
```go
type FileProviderBridgeStatus struct {
    Registered bool   `json:"registered"`
    Reason     string `json:"reason"`
    MountPath  string `json:"mountPath"`
    LogPath    string `json:"logPath"`
}

type FileProviderBridge interface {
    Available() bool
    InitTask(task models.BackupTask) (FileProviderBridgeStatus, error)
    GetStatus(task models.BackupTask) (FileProviderBridgeStatus, error)
}
```

---

## TODOs

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go build ./...` + `go test ./...`. Review all changed files for: no file > 300 lines, no function > 50 lines, no struct > 3 responsibilities. Check AI slop: excessive comments, over-abstraction, wrapper types, renamed JSON fields. Verify all 7 interfaces are defined and used.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Build and start the server backend + client backend. Exercise ALL 24 API endpoints using curl. Verify each response matches expected shape. Test cross-concern integration (create task → sync → check logs). Test edge cases: sync without login, create task with bad path, delete non-existent task. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Endpoints [24/24 pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes. Verify no behavioral changes vs pre-refactor.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Commit | Message | Files Changed | Pre-commit Check |
|--------|---------|---------------|------------------|
| 1 | `refactor(client): extract models package` | models/models.go, main.go | `go build` |
| 2 | `refactor(client): extract config package` | config/config.go, main.go | `go build` |
| 3 | `refactor(client): extract utils and middleware` | utils/utils.go, middleware/cors.go, main.go | `go build` |
| 4 | `refactor(client): extract store with TaskRepository interface` | store/task_store.go, store/task_store_test.go, main.go | `go build && go test ./store/...` |
| 5 | `refactor(client): extract auth with TokenManager interface` | auth/token.go, auth/token_test.go, main.go | `go build && go test ./auth/...` |
| 6 | `refactor(client): extract proxy with RemoteClient interface` | proxy/client.go, proxy/client_test.go, main.go | `go build && go test ./proxy/...` |
| 7 | `refactor(client): extract sync engine, watcher, platform, log` | sync/, watcher/, platform/, log/, main.go | `go build && go test ./sync/... ./watcher/... ./log/...` |
| 8 | `refactor(client): extract all HTTP handlers` | handlers/, main.go | `go build` |
| 9 | `refactor(client): extract fileprovider package` | fileprovider/, main.go | `go build` |
| 10 | `refactor(client): rewrite main.go orchestrator + graceful shutdown` | main.go | `go build && go test ./...` |
| 11 | `test(client): add integration tests for all API endpoints` | tests/ or handler tests | `go test ./...` |

---

## Success Criteria

### Verification Commands
```bash
cd client/backend
go build ./...                    # Expected: success, no errors
go vet ./...                      # Expected: clean, no warnings
go test ./...                     # Expected: all tests pass
find . -name '*.go' ! -name '*_test.go' -exec wc -l {} + | sort -rn | head -20  # Expected: no file > 300 lines
grep -rn 'func ' --include='*.go' . | awk -F'{' '{print length}' | sort -rn | head -20  # Check function sizes
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (`go test ./...`)
- [ ] No file exceeds 300 lines
- [ ] No function exceeds 50 lines
- [ ] No struct has >3 responsibilities
- [ ] All 7 interfaces defined and used
- [ ] All 24 API endpoints preserved identically
- [ ] Graceful shutdown works (SIGINT/SIGTERM)
- [ ] `go vet` passes clean
