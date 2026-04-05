# Nikologs Go Client — Design Spec

## Overview

A production-ready, zero-dependency Go package (`github.com/M1z23r/nikologs-go`, package `nikologs`) that provides a client for the Nikologs log aggregation API. Focused on log ingestion and attachment uploads.

## API Surface

### Base URL

`https://nikologs.dimitrije.dev`

### Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/v1/ingest/logs` | `nk_` / `nkp_` key | Ingest log batch (max 1000) |
| POST | `/api/v1/ingest/attachments` | `nku_` key | Upload image/file attachment |

### API Key Types

| Prefix | Type | Rate Limit | Use |
|--------|------|------------|-----|
| `nk_` | Private | 10,000/min | Backend ingestion |
| `nkp_` | Public | 1,000/min | Frontend (origin-checked) |
| `nku_` | Upload | 1,000/min | Attachment upload only |

### Log Levels

| Int | Name |
|-----|------|
| 0 | success |
| 1 | trace |
| 2 | debug |
| 3 | info |
| 4 | warn |
| 5 | error |
| 6 | fatal |

### Log Entry Fields

- `level` (required): string name or int 0-6
- `message` (required): string
- `source` (optional): string, can be set as client default
- `metadata` (optional): `map[string]any`
- `timestamp` (optional): RFC3339, defaults to server time
- `image_id` (optional): UUID string
- `file_id` (optional): UUID string

### Ingest Response (202)

```json
{"accepted": 3, "errors": [{"index": 1, "error": "invalid level"}]}
```

### Attachment Response (200)

```json
{"id": "uuid", "kind": "image", "token": "hex"}
```

Attachment constraints:
- Images (`image/*`): max 10MB
- Files (`text/html`, `text/plain`, `text/csv`, `application/json`): max 5MB
- Multipart form field name: `file`

## Architecture

### Single Client, Two Keys

One `Client` struct handles both logging and uploads. The ingestion API key is required. The upload key is optional, provided via `WithUploadKey()`. Calling `UploadAttachment()` without an upload key returns an error.

### Client Construction

```go
client := nikologs.New("nk_your_key",
    nikologs.WithSource("my-service"),
    nikologs.WithFlushInterval(3 * time.Second),
    nikologs.WithBatchSize(200),
    nikologs.WithHTTPClient(customClient),
    nikologs.WithUploadKey("nku_your_key"),
    nikologs.WithOnError(func(err error) { log.Println(err) }),
)
defer client.Shutdown(ctx)
```

Base URL defaults to `https://nikologs.dimitrije.dev`.

### Defaults

| Option | Default |
|--------|---------|
| Flush interval | 5 seconds |
| Batch size | 100 |
| HTTP client | `http.DefaultClient` |
| Source | empty string (omitted from payload) |
| OnError | no-op |
| Upload key | empty (uploads disabled) |

### Logging Methods

```go
client.Success(msg, nikologs.Fields{"key": "value"})
client.Trace(msg, fields)
client.Debug(msg, fields)
client.Info(msg, fields)
client.Warn(msg, fields)
client.Error(msg, fields)
client.Fatal(msg, fields)
client.Log(level, msg, fields)
```

All methods are non-blocking. Each accepts variadic `LogOption` for per-log overrides:

```go
client.Info("deployed", nil, nikologs.WithTimestamp(t), nikologs.WithImageID(id))
```

`Fields` is `type Fields map[string]any`.

### Buffering

- Buffered channel with capacity = 10 * batch size
- Background flush goroutine drains the channel into a batch slice
- Flush triggers: batch reaches configured size OR flush interval timer fires
- If the channel is full, the log entry is dropped and `OnError` is called (never block callers)
- The flush goroutine owns the batch slice exclusively — no mutex needed for the hot path

### Retry on Flush Failure

- Exponential backoff: 1s, 2s, 4s (3 attempts max)
- On persistent failure: call `OnError` with the error, drop the batch
- Buffer never grows unboundedly

### Shutdown

`Shutdown(ctx context.Context)`:
1. Signal the flush goroutine to stop accepting new entries
2. Drain remaining entries from the channel
3. Perform a final flush
4. Respect context deadline/cancellation — if context expires, return `ctx.Err()`

### Attachment Upload

```go
resp, err := client.UploadAttachment(ctx, reader, "screenshot.png")
// resp.ID, resp.Kind, resp.Token
```

- Synchronous (not buffered)
- Uses the upload key (`nku_`)
- Returns `AttachmentResponse` with ID for use in `WithImageID`/`WithFileID`

### slog Integration

```go
handler := nikologs.NewSlogHandler(client, &nikologs.SlogHandlerOptions{
    Level: slog.LevelInfo,
})
logger := slog.New(handler)
```

Level mapping:
| slog Level | nikologs Level |
|------------|---------------|
| < Debug | trace |
| Debug | debug |
| Info | info |
| Warn | warn |
| Error+ | error |

- Attributes are converted to `Fields` metadata
- Groups become dot-separated key prefixes (e.g., group `"http"` + key `"status"` = `"http.status"`)
- Respects the handler's minimum level filter
- `WithAttrs` and `WithGroup` return new handler instances (immutable)

## Response Types

```go
type IngestResponse struct {
    Accepted int           `json:"accepted"`
    Errors   []IngestError `json:"errors"`
}

type IngestError struct {
    Index int    `json:"index"`
    Error string `json:"error"`
}

type AttachmentResponse struct {
    ID    string `json:"id"`
    Kind  string `json:"kind"`
    Token string `json:"token"`
}
```

## File Structure

```
nikologs-go/
├── client.go          # Client struct, New(), options, Shutdown
├── log.go             # Log(), level methods, entry types, Fields, LogOption
├── buffer.go          # internal buffering, flush loop, retry logic
├── ingest.go          # HTTP calls to /ingest/logs and /ingest/attachments
├── slog_handler.go    # slog.Handler implementation
├── errors.go          # error types
├── example_test.go    # runnable examples
├── client_test.go     # unit tests
├── go.mod
└── README.md
```

## Non-Functional Requirements

- Zero dependencies beyond stdlib
- Go 1.21+ (uses `log/slog`)
- Godoc comments on all exported symbols
- Unit tests >80% coverage (mock HTTP layer)
- Runnable examples in `example_test.go`
- README with install, quick start, config, slog, and attachment sections
