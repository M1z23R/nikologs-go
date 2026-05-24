# nikologs-go

A Go client for the [Nikologs](https://nikologs.dimitrije.dev) log aggregation API.

- Buffered, non-blocking log ingestion
- Automatic batching with configurable flush interval and batch size
- Exponential backoff retry on failure
- Attachment uploads (images and files)
- `slog.Handler` integration for Go's standard structured logging
- Zero dependencies beyond the Go standard library

## Install

```bash
go get github.com/M1z23r/nikologs-go
```

Requires Go 1.21+.

## Quick Start

Declare the client once as `nlog` and pass it into your services. `nlog` is the
recommended name throughout your codebase — short, unambiguous, and reads
naturally at call sites like `nlog.Info(...)` and `nlog.Error(...)`.

> **Fire-and-forget.** `nlog.Info`, `nlog.Warn`, `nlog.Error`, etc. are
> non-blocking: they allocate an entry and do a non-blocking send onto an
> in-memory buffer, then return. All batching, HTTP, and retries happen on a
> background goroutine started by `nikologs.New`. You can call them directly
> from request hot paths — **no need to wrap them in `go func() { ... }()`**.
> If the buffer fills (service down, sustained burst), logs are dropped and
> the `WithOnError` callback fires; your app never blocks.

```go
package main

import (
    "context"
    "time"

    nikologs "github.com/M1z23r/nikologs-go"
)

func main() {
    nlog := nikologs.New("nk_your_api_key",
        nikologs.WithSource("my-service"),
        nikologs.WithFlushInterval(3*time.Second),
        nikologs.WithBatchSize(200),
    )
    defer nlog.Shutdown(context.Background())

    nlog.Info("service started", nikologs.Fields{"version": "1.0.0"})

    svc := NewUserService(nlog)
    svc.SignIn("u-123")
}
```

## Passing `nlog` to Services

Services and handlers accept `*nikologs.Client` as a dependency. Store it on the
struct as `nlog` and call `nlog.Warn`, `nlog.Error`, etc. directly.

```go
type UserService struct {
    nlog *nikologs.Client
    // ...other dependencies
}

func NewUserService(nlog *nikologs.Client) *UserService {
    return &UserService{nlog: nlog}
}

func (s *UserService) SignIn(userID string) error {
    s.nlog.Info("sign-in attempt", nikologs.Fields{"user_id": userID})

    if err := s.verify(userID); err != nil {
        s.nlog.Error("sign-in failed", nikologs.Fields{
            "user_id": userID,
            "error":   err.Error(),
        })
        return err
    }

    s.nlog.Success("sign-in ok", nikologs.Fields{"user_id": userID})
    return nil
}
```

The same pattern works for HTTP handlers, background workers, or any other
component — construct `nlog` once in `main`, then inject it wherever logging
is needed.

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `WithBaseURL(url)` | `https://nikologs.dimitrije.dev` | API base URL |
| `WithSource(s)` | `""` | Default source for all log entries |
| `WithDefaultTags(tags...)` | `nil` | Tags applied to every log entry |
| `WithFlushInterval(d)` | `5s` | How often the buffer is flushed |
| `WithBatchSize(n)` | `100` | Max entries per flush (API max: 1000) |
| `WithHTTPClient(c)` | `http.DefaultClient` | Custom HTTP client |
| `WithUploadKey(k)` | `""` | Upload API key (`nku_` prefix) |
| `WithOnError(fn)` | no-op | Callback for flush/send failures |

## Log Levels

```go
nlog.Success("done", nil)
nlog.Trace("detailed", nil)
nlog.Debug("debug info", nil)
nlog.Info("informational", nil)
nlog.Warn("warning", nil)
nlog.Error("error occurred", nil)
nlog.Fatal("fatal error", nil)

// Or use the generic method:
nlog.Log(nikologs.LevelInfo, "msg", fields)
```

### Per-Entry Options

```go
nlog.Info("deployed", nil,
    nikologs.WithTimestamp(time.Now()),
    nikologs.WithImageID("uuid"),
    nikologs.WithFileID("uuid"),
    nikologs.WithTags("deploy", "release"),
)
```

## Tags

Tags are short labels attached to a log entry. They're useful for filtering and
grouping logs in the read API (`?tags=prod,api`, OR semantics).

Set tags that apply to every entry with `WithDefaultTags`, and add per-entry tags
with the `WithTags` log option. Both are combined; the server lowercases, trims,
and dedupes them, allowing up to 20 unique tags per entry (128 chars each).

```go
nlog := nikologs.New("nk_your_api_key",
    nikologs.WithSource("my-service"),
    nikologs.WithDefaultTags("env:prod", "region:eu"),
)

// Sent with tags: ["env:prod", "region:eu", "payment", "stripe"]
nlog.Error("charge failed", nil, nikologs.WithTags("payment", "stripe"))
```

## slog Integration

If you prefer Go's standard `log/slog`, wrap `nlog` in a `slog.Handler`:

```go
nlog := nikologs.New("nk_your_api_key",
    nikologs.WithSource("my-service"),
)
defer nlog.Shutdown(context.Background())

handler := nikologs.NewSlogHandler(nlog, &nikologs.SlogHandlerOptions{
    Level: slog.LevelInfo,
})
logger := slog.New(handler)
logger.Info("request handled", "method", "GET", "status", 200)
```

slog levels map to nikologs levels: Debug → debug, Info → info, Warn → warn, Error → error. Levels below Debug map to trace.

## Attachment Uploads

Upload images or files and reference them in log entries:

```go
nlog := nikologs.New("nk_your_api_key",
    nikologs.WithUploadKey("nku_your_upload_key"),
)

file, _ := os.Open("screenshot.png")
defer file.Close()

resp, err := nlog.UploadAttachment(context.Background(), file, "screenshot.png")
if err != nil {
    log.Fatal(err)
}

nlog.Error("UI bug", nil, nikologs.WithImageID(resp.ID))
```

## License

MIT
