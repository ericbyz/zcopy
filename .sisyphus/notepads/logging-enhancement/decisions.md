# Decisions

## 2026-04-18 Planning Phase
- Go `log/slog` for structured logging (stdlib, no third-party)
- Package naming: server uses `logger/`, client keeps `log/`
- Rotation at `io.Writer` layer, not full `slog.Handler`
- Daily rotation + 100MB size cap + 30-day retention + 500MB max total
- Async writes via buffered channel (cap 4096)
- Client frontend: vue-router for dedicated log page
- Server frontend: tab/section (no vue-router)
- Frontend: paginated query (NOT real-time), user clicks to refresh
- TDD for logger/log packages only; handler logging via Agent QA
