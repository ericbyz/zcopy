# ZCopy - Project Knowledge Base

> Cloud file sync tool: Electron desktop client + Go server. Supports macOS Finder / Windows Cloud Files integration for on-demand sync.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    ZCopy Architecture                     │
├───────────────┬─────────────────┬───────────────────────┤
│   Client      │                 │       Server           │
│               │                 │                        │
│ ┌───────────┐ │   HTTP/REST     │ ┌──────────────────┐  │
│ │ Electron  │ │ ───────────────►│ │  Go Backend      │  │
│ │ + Vue 3   │ │   :8890         │ │  (Gin + JWT)     │  │
│ │ Renderer  │ │◄─────────────── │ │  JSON-file DB    │  │
│ └─────┬─────┘ │                 │ └────────┬─────────┘  │
│       │       │                 │          │            │
│ ┌─────▼─────┐ │                 │ ┌────────▼─────────┐  │
│ │ Go Client │ │                 │ │  Filesystem      │  │
│ │ Backend   │ │                 │ │  Storage         │  │
│ │ :8090     │ │                 │ │  (per-user dir)  │  │
│ └───────────┘ │                 │ └──────────────────┘  │
│               │                 │                        │
│ ┌───────────┐ │                 │ ┌──────────────────┐  │
│ │ Web Front │ │                 │ │  Vue 3 SPA       │  │
│ │ (Server)  │ │                 │ │  :5173 (dev)     │  │
│ └───────────┘ │                 │ └──────────────────┘  │
└───────────────┴─────────────────┴───────────────────────┘
```

## Monorepo Structure

```
zcopy/
├── client/                    # Desktop client (Electron + Go)
│   ├── backend/               # Go backend (local API + sync engine)
│   │   ├── main.go            # Gin server, sync engine, file watching, task CRUD (~1500 lines)
│   │   ├── fileprovider.go    # macOS WebDAV + File Provider bridge (~600 lines)
│   │   ├── config/
│   │   │   └── config.yaml    # Local server config
│   │   ├── data/              # Persisted state
│   │   │   ├── tasks.json     # Task store (JSON array)
│   │   │   └── snapshots/     # Per-task file fingerprints for incremental sync
│   │   ├── go.mod             # Module: zcopy-client-backend (Go 1.25)
│   │   └── go.sum
│   ├── front/                 # Electron Windows client (Vue 3 + Vite)
│   │   ├── electron/
│   │   │   ├── main.js        # Electron main: spawns Go backend, loads Vue renderer
│   │   │   └── preload.js     # IPC bridge: dialog:pick-folder
│   │   ├── index.html         # Renderer entry (zh-CN)
│   │   ├── package.json       # Module: zcopy-client-front (Electron 28 + Vue 3.4)
│   │   └── dist/              # Built frontend assets
│   └── front-mac/             # Electron macOS client (file provider variant)
│       ├── electron/
│       │   ├── main.js        # macOS main: file provider bridge + backend spawn
│       │   └── preload.js     # IPC bridge: dialog:pick-folder
│       ├── package.json       # Module: zcopy-client-front-mac (electron-macos-file-provider)
│       └── release/           # macOS build output
│
├── server/                    # Remote file server
│   ├── backend/               # Go API server
│   │   ├── main.go            # Router setup, CORS, server bootstrap
│   │   ├── handlers/
│   │   │   ├── auth.go        # Register, Login, Me — JWT token issuance
│   │   │   └── file.go        # ListFiles, CreateFolder, Upload, Download, Delete
│   │   ├── middleware/
│   │   │   └── auth.go        # AuthRequired(), CurrentUser(), JWT validation
│   │   ├── models/
│   │   │   └── models.go      # User, File, FileVersion, Share structs
│   │   ├── database/
│   │   │   └── database.go    # JSON-file-backed Store (NOT SQL despite .db extension)
│   │   ├── storage/           # (future use)
│   │   ├── utils/
│   │   │   └── utils.go       # Password hashing, file hash, path sanitization, MIME types
│   │   ├── config/
│   │   │   ├── config.go      # Viper-based config loader
│   │   │   └── config.yaml    # Default config
│   │   ├── data/
│   │   │   └── zcopy.db       # JSON database file (users array)
│   │   ├── go.mod             # Module: zcopy-server-backend (Go 1.21)
│   │   └── go.sum
│   └── front/                 # Web frontend (Vue 3 SPA)
│       ├── src/
│       │   ├── main.js        # Vue app bootstrap
│       │   ├── App.vue        # Main component: auth + file browser + CRUD operations
│       │   └── style.css      # Dark theme, glassy panels, responsive grid
│       ├── index.html         # Entry HTML (zh-CN)
│       ├── vite.config.js     # Port 5173, host 0.0.0.0
│       └── package.json       # Module: zcopy-front (Vue 3.4 + Axios)
│
└── README.md                  # (empty)
```

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Client Backend | Go + Gin | Go 1.25, Gin 1.12 |
| Client Frontend | Electron + Vue 3 | Electron 28, Vue 3.4 |
| Server Backend | Go + Gin | Go 1.21, Gin 1.9 |
| Server Frontend | Vue 3 + Vite | Vue 3.4, Vite 4.5 |
| Auth | JWT (HS256) | golang-jwt/jwt/v5 |
| DB | JSON file | (NOT SQL — mutex-protected in-memory slice) |
| Config | YAML + Viper | — |
| File Watch | fsnotify | v1.9.0 |
| macOS FP | electron-macos-file-provider + WebDAV | golang.org/x/net/webdav |
| Windows CFAPI | PowerShell + StorageProviderSyncRootManager | — |

## Server Backend (`server/backend/`)

### Startup Sequence (`main.go`)

```
1. config.LoadConfig()              → loads config.yaml via Viper
2. utils.EnsureDirectoryExists()    → creates storage root dir
3. database.InitDB()                → loads/creates JSON database
4. gin.SetMode()                    → debug/release mode from config
5. router := gin.Default()          → Logger + Recovery middleware
6. corsMiddleware()                 → custom CORS (mirrors Origin)
7. Register routes                  → /api/v1/auth, /api/v1/files, /api/v1/client
8. router.Run(":" + port)           → default :8890
```

### API Endpoints

**Auth** (`/api/v1/auth`) — public:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/auth/register` | `Register` | Create user (username, email, password, nickname) |
| POST | `/auth/login` | `Login` | Login with account (username or email) + password |
| GET | `/auth/me` | `Me` | Get current user (auth required) |

**Files** (`/api/v1/files`) — all require auth:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/files?path=` | `ListFiles` | List directory (dirs first, then alpha sort) |
| POST | `/files/folder` | `CreateFolder` | Create folder at path |
| POST | `/files/upload` | `UploadFile` | Multipart upload, auto-rename on conflict (UUID suffix) |
| GET | `/files/download?path=` | `DownloadFile` | Download with Content-Type/Content-Disposition |
| DELETE | `/files?path=` | `DeleteFile` | Delete file or directory (cannot delete user root) |

**Client** (`/api/v1/client`) — auth required:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/client/capabilities` | `ClientCapabilities` | Feature list (implemented + planned) |

**Health**:
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Returns `{"status": "ok"}` |

### Authentication Flow

1. User registers or logs in → server issues JWT (HS256, configurable expiry, default 72h)
2. Token claims: `{ user_id, exp, iat, sub: "zcopy-user" }`
3. Client sends `Authorization: Bearer <token>` on protected routes
4. `middleware.AuthRequired()` validates token → loads user → `c.Set("user", user)`
5. Handlers retrieve user via `middleware.CurrentUser(c)`
6. Passwords hashed with bcrypt (DefaultCost=10)

### Data Models (`models/models.go`)

**User** (active — persisted in JSON DB):
| Field | Type | Notes |
|-------|------|-------|
| ID | uint | Auto-increment PK |
| Username | string | Unique, max 100 |
| Email | string | Unique, max 255, stored lowercase |
| Password | string | bcrypt hash |
| Nickname | string | Display name, max 100 |
| Avatar | string | URL/path (unused) |
| CreatedAt / UpdatedAt / DeletedAt | time | Soft delete support |

**File, FileVersion, Share** — defined but NOT yet wired up (planned features scaffold).

### Database Layer (`database/database.go`)

⚠️ **NOT SQL** — despite `.db` extension and GORM-style struct tags.

- JSON file (`data/zcopy.db`) storing `[]models.User`
- `sync.RWMutex` for thread safety
- In-memory slice with `flushLocked()` on every write
- Auto-incrementing `nextID` counter
- Methods: `InitDB()`, `UserExists()`, `CreateUser()`, `FindUserByAccount()`, `FindUserByID()`

### File Storage

- Per-user isolated directories: `{storage.root_dir}/user-{id}/`
- Filesystem IS the source of truth — no file metadata in database
- Path traversal protection in `resolveUserPath()` verifies resolved path stays within user root
- Max multipart memory: 64 MB

### Configuration (`config/config.yaml`)

```yaml
server:
  port: "8890"
  mode: "debug"          # gin mode: debug/release/test
database:
  path: "./data/zcopy.db"
auth:
  secret_key: "zcopy-secret-key-change-in-production"
  token_expire_hours: 72
storage:
  root_dir: "./storage"
```

---

## Client Backend (`client/backend/`)

### Startup Sequence (`main.go`)

```
1. loadConfig()                 → YAML config + env var overrides
2. Create data directory
3. Init TaskStore               → reads data/tasks.json
4. Create AppState singleton
5. startWebDAVServer()          → macOS only: random port WebDAV for File Provider
6. restoreAutoWatchers()        → restart fsnotify for all autoBackup tasks
7. Start Gin server on :8090
```

### Local API Endpoints

**Auth proxy** (forwards to server):
| Method | Path | Handler |
|--------|------|---------|
| POST | `/auth/register` | `proxyRegister` |
| POST | `/auth/login` | `proxyLogin` (stores JWT locally) |
| POST | `/auth/logout` | `logout` (clears local JWT) |
| GET | `/auth/me` | `proxyMe` |

**Task CRUD**:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/tasks` | `listTasks` | List all backup tasks |
| POST | `/tasks` | `createTask` | Create task (validates local dir, starts watcher) |
| PUT | `/tasks/:id` | `updateTask` | Update task properties |
| DELETE | `/tasks/:id` | `deleteTask` | Delete task (stops watcher) |

**Sync Operations**:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/tasks/:id/sync` | `syncTaskNow` | Trigger immediate full sync |
| POST | `/tasks/:id/auto/start` | `startAutoTask` | Enable auto-backup + start watcher |
| POST | `/tasks/:id/auto/stop` | `stopAutoTask` | Disable auto-backup + stop watcher |

**On-Demand Sync** (space management):
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/tasks/:id/on-demand/release` | `releaseLocalSpace` | Delete local files matching snapshot |
| POST | `/tasks/:id/on-demand/hydrate` | `hydrateFromCloud` | Download all files from server |
| POST | `/tasks/:id/on-demand/cfapi/init` | `initTaskCFAPI` | Register OS cloud file integration |
| GET | `/tasks/:id/on-demand/cfapi/status` | `taskCFAPIStatus` | Check cloud integration status |

**Remote Browsing + System**:
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/remote/folders` | `listRemoteFolders` | List remote directories only |
| GET | `/logs` | `listLogs` | Transfer logs (filterable) |
| GET | `/system/capabilities` | `systemCapabilities` | OS info, onDemand support |
| GET | `/health` | inline | `{"status":"ok"}` |

### Key Data Models

**BackupTask**: ID, Name, LocalPath, RemotePath, AutoBackup, OnDemandSync, Status, LastError, LastSyncAt, SyncReport, CreatedAt, UpdatedAt

**SyncReport**: State, Mode, Message, TotalFiles, UploadedFiles, FailedFiles, TotalBytes, TransferredBytes, SpeedBytesPerSec, FailedFilePaths, StartedAt, FinishedAt

**TaskSnapshot**: `map[relativePath]FileFingerprint{Size, ModUnix}` — incremental change detection

**TransferLog**: In-memory ring buffer (max 2000 entries)

**TaskStore**: Thread-safe JSON file store (`data/tasks.json`)

### Sync Engine

```
syncTask() flow:
1. Acquire per-task mutex
2. Set status → "syncing"
3. collectLocalFiles() → walk local dir → []LocalFileItem
4. If incremental mode: load snapshot → compare size+modTime → skip unchanged
5. ensureRemotePath() → create remote dir hierarchy (POST /files/folder per segment)
6. For each file: uploadFile() → multipart POST /files/upload
7. Update snapshot → persist to data/snapshots/{taskID}.json
8. Set status → "idle" (success) or "error" (with details)
```

### File Watching System

- Uses `fsnotify` for real-time directory monitoring
- **Debounce**: 2-second timer resets on each event → sync triggers only after quiet period
- Dynamically adds new subdirectories to watcher
- `restoreAutoWatchers()` resumes all autoBackup tasks on startup

### macOS File Provider (`fileprovider.go`)

- Local **WebDAV server** on `127.0.0.1:{random_port}` for macOS Finder integration
- `remoteWebDAVFS` implements `webdav.FileSystem` — proxies remote server files
  - Read: downloads to temp cache → serves to Finder
  - Write: captures file → uploads to server on Close()
- **File Provider Bridge**: HTTP communication with external Swift process
  - `POST /register` → registers File Provider domain for task
  - `GET /status` → checks registration status
- Bridge configured via `ZCOPY_CLIENT_FP_BRIDGE_URL` + `ZCOPY_CLIENT_FP_BRIDGE_TOKEN`

### Windows Cloud Files API

- Uses PowerShell to call `StorageProviderSyncRootManager.Register()`
- Progressive hydration, auto-dehydration, full population policy
- Sync root ID format: `ZCopy.{sanitized-task-id}`

### Environment Variable Overrides

| Variable | Overrides |
|----------|-----------|----------|
| `ZCOPY_CLIENT_CONFIG` | Custom config file path |
| `ZCOPY_CLIENT_DATA_DIR` | Storage data directory |
| `ZCOPY_CLIENT_FP_BRIDGE_URL` | macOS FP bridge URL |
| `ZCOPY_CLIENT_FP_BRIDGE_TOKEN` | macOS FP bridge auth token |
| `ZCOPY_CLIENT_WEBDAV_USER` | WebDAV credentials (macOS) |
| `ZCOPY_CLIENT_WEBDAV_PASSWORD` | WebDAV credentials (macOS) |

---

## Client Frontend (`client/front/` + `client/front-mac/`)

### Electron IPC Pattern

```
Renderer (Vue 3)  ←→  preload.js  ←→  main.js (Electron main)
     │                                    │
     │  window.desktopApi.pickFolder()    │
     │  ─────────────────────────────────►│
     │       ipcRenderer.invoke()         │
     │                                    │  dialog.showOpenDialog()
     │              selected path         │
     │  ◄─────────────────────────────────│
```

- `preload.js` exposes `window.desktopApi.pickFolder()` to renderer
- `main.js` handles `dialog:pick-folder` IPC → opens native directory picker

### Electron Main Process (Windows — `client/front/electron/main.js`)

1. Determines dev vs packaged mode
2. Resolves backend binary path (`electron/bin/zcopy-client-backend.exe`)
3. Sets env vars: `ZCOPY_CLIENT_CONFIG`, `ZCOPY_CLIENT_DATA_DIR`
4. Spawns Go backend process
5. Creates `BrowserWindow` → loads Vite dev server (dev) or `dist/` (packaged)
6. DevTools shortcut wired

### Electron Main Process (macOS — `client/front-mac/electron/main.js`)

Additional macOS-specific logic:
1. Loads `electron-macos-file-provider` (optional dependency)
2. Starts bridge HTTP server on dynamic port
3. Populates `ZCOPY_CLIENT_FP_BRIDGE_URL` + `ZCOPY_CLIENT_FP_BRIDGE_TOKEN`
4. File provider domain registration via bridge
5. Renderer loaded from `client/front/dist` (shared Vue build)

### Build Commands

**Windows client**:
```bash
cd client/front
npm run dist            # build backend + Vue + Electron (dir output)
npm run dist:portable   # build backend + Vue + Electron (portable exe)
```

**macOS client**:
```bash
cd client/front         # first, build the Vue frontend
npm run build
cd ../front-mac
npm run dist            # build backend + Electron DMG
```

---

## Server Frontend (`server/front/`)

### Tech

- Vue 3 SPA with Composition API
- Axios for HTTP, token in `localStorage('zcopy_token')`
- Base URL: `VITE_API_BASE_URL` or default `http://localhost:8890/api/v1`
- Request interceptor auto-attaches `Authorization: Bearer <token>`
- Dark theme UI with glassy panels, responsive grid

### UI Structure (`App.vue`)

- **Auth area**: Login/Register tabs with form inputs
- **File browser**: Breadcrumb navigation + path editor
- **File list**: Name, size, updatedAt, actions (Enter folder, Download, Delete)
- **Toolbar**: Upload file, Create folder

### State Management

- Component-scoped (no Vuex/Pinia)
- Local state: token, currentUser, currentPath, fileItems, loading, message, authForm, uploadFile, folderName
- `breadcrumbList` computed from `currentPath`

---

## Planned Features (from ClientCapabilities endpoint)

| Feature | Status |
|---------|--------|
| Token auth | ✅ Implemented |
| File list / Create folder / Upload / Download / Delete | ✅ Implemented |
| Client login + refresh token | 🔲 Planned |
| Directory snapshot compare | 🔲 Planned |
| Incremental sync by hash | 🔲 Planned |
| Conflict detection & resolution | 🔲 Planned |
| Upload resume / Download resume | 🔲 Planned |
| Device binding | 🔲 Planned |
| Sync task history | 🔲 Planned |

---

## Development Quick Reference

### Prerequisites

- Go 1.21+ (server), Go 1.25+ (client backend)
- Node.js (for Vue/Electron)
- Platform-specific: macOS needs Xcode for file provider; Windows needs PowerShell for CFAPI

### Run Server

```bash
cd server/backend && go run main.go          # API on :8890
cd server/front && npm run dev                # Web UI on :5173
```

### Run Client (dev)

```bash
cd client/front && npm run dev                # Starts Vite + Electron with Go backend
cd client/front-mac && npm run start          # macOS Electron (needs built Vue frontend first)
```

### Build Client Binary

```bash
cd client/front && npm run build:backend      # Compile Go → electron/bin/zcopy-client-backend.exe
```

### Ports

| Service | Port | Purpose |
|---------|------|---------|
| Server API | 8890 | REST API |
| Server Web UI | 5173 | Vue SPA (dev) |
| Client Backend | 8090 | Local API for Electron |
| Client Vite | 5173 | Vue dev server |
| WebDAV | Random | macOS File Provider |

---

## Key Conventions

- **Language**: UI is in Chinese (zh-CN)
- **Timezone**: Server forces `Asia/Shanghai` at init
- **Database**: JSON file with mutex-protected in-memory slice — NOT SQL, despite `.db` extension
- **File storage**: Filesystem is source of truth, no file metadata in database
- **Path security**: `resolveUserPath()` and `CleanRelativePath()` prevent path traversal
- **Auth**: JWT Bearer tokens, stored in localStorage (web) or in-memory (client backend)
- **Sync strategy**: Snapshot-based incremental (size + modTime comparison), debounce file watcher events
- **Platform differences**: macOS uses WebDAV + File Provider bridge; Windows uses CFAPI via PowerShell
