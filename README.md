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

```go
package main

import (
    "context"
    "time"

    nikologs "github.com/M1z23r/nikologs-go"
)

func main() {
    client := nikologs.New("nk_your_api_key",
        nikologs.WithSource("my-service"),
        nikologs.WithFlushInterval(3 * time.Second),
        nikologs.WithBatchSize(200),
    )
    defer client.Shutdown(context.Background())

    client.Info("service started", nikologs.Fields{"version": "1.0.0"})
    client.Error("something broke", nikologs.Fields{"code": 500})
}
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `WithBaseURL(url)` | `https://nikologs.dimitrije.dev` | API base URL |
| `WithSource(s)` | `""` | Default source for all log entries |
| `WithFlushInterval(d)` | `5s` | How often the buffer is flushed |
| `WithBatchSize(n)` | `100` | Max entries per flush (API max: 1000) |
| `WithHTTPClient(c)` | `http.DefaultClient` | Custom HTTP client |
| `WithUploadKey(k)` | `""` | Upload API key (`nku_` prefix) |
| `WithOnError(fn)` | no-op | Callback for flush/send failures |

## Log Levels

```go
client.Success("done", nil)
client.Trace("detailed", nil)
client.Debug("debug info", nil)
client.Info("informational", nil)
client.Warn("warning", nil)
client.Error("error occurred", nil)
client.Fatal("fatal error", nil)

// Or use the generic method:
client.Log(nikologs.LevelInfo, "msg", fields)
```

### Per-Entry Options

```go
client.Info("deployed", nil,
    nikologs.WithTimestamp(time.Now()),
    nikologs.WithImageID("uuid"),
    nikologs.WithFileID("uuid"),
)
```

## slog Integration

```go
handler := nikologs.NewSlogHandler(client, &nikologs.SlogHandlerOptions{
    Level: slog.LevelInfo,
})
logger := slog.New(handler)
logger.Info("request handled", "method", "GET", "status", 200)
```

slog levels map to nikologs levels: Debug → debug, Info → info, Warn → warn, Error → error. Levels below Debug map to trace.

## Attachment Uploads

Upload images or files and reference them in log entries:

```go
client := nikologs.New("nk_your_api_key",
    nikologs.WithUploadKey("nku_your_upload_key"),
)

file, _ := os.Open("screenshot.png")
defer file.Close()

resp, err := client.UploadAttachment(context.Background(), file, "screenshot.png")
if err != nil {
    log.Fatal(err)
}

client.Error("UI bug", nil, nikologs.WithImageID(resp.ID))
```

## License

MIT
