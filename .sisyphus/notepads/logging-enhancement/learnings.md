# Learnings

## Logger Package Implementation
- Successfully created a slog-based structured logging package with JSONL output
- Implemented daily rotation, size-based rotation, and retention policies
- Used buffered channel + background goroutine for async writes
- Added graceful degradation to stderr on disk errors
- All tests passing using only the standard library (no third-party dependencies)
- Integrated with the existing config system (added LogConfig to config.go and config.yaml)

## FileLogStore Implementation (Transfer Logs)
- Implemented FileLogStore as a drop-in replacement for RingBufferLogStore while preserving LogStore interface
- Used raw JSONL (not slog) for full control over TransferLog structure
- Async writes via buffered channel (1000 capacity), non-blocking Push() with graceful drop on backpressure
- Added sync.Once for safe double Close() protection
- Added writeWg WaitGroup + Flush() method for testability (waiting for all async writes to complete)
- Log directory: configurable, default data/logs/; filename: YYYY-MM-DD.jsonl
- Rotation: daily (based on date), size-based (100MB default), retention (30 days default)
- Query supports: taskID, level, keyword, time range, pagination
- Export() returns io.Reader of JSONL for filtered logs
- All 6 required tests passing: TestFilePersistence, TestListPagination, TestKeywordSearch, TestTimeRangeFilter, TestConcurrentPush, TestRotationAndRetention

