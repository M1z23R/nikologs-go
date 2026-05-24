# AGENTS.md — nikologs-go

Quick-reference for AI coding agents integrating `github.com/M1z23r/nikologs-go`
into a Go project. Read this before writing code that imports this package.

## What it is

A Go client for the Nikologs log aggregation API. Buffered, non-blocking,
batched ingestion with retries. Zero deps beyond stdlib. Requires Go 1.21+.

## The one thing to get right: fire-and-forget

`nlog.Info/Warn/Error/...` are **non-blocking**. They push onto an in-memory
buffer; a background goroutine does batching, HTTP, and retries.

- **DO** call them directly from request hot paths, handlers, middleware.
- **DO NOT** wrap calls in `go func() { ... }()` — already async.
- **DO NOT** `defer` log calls for "safety" — unnecessary.
- If the buffer fills, entries are dropped and `WithOnError` fires. The app
  never blocks.

## Canonical setup

Construct once in `main`, inject as `nlog *nikologs.Client` into services.
The idiomatic name is `nlog` (not `logger`, not `log`).

```go
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
    defer nlog.Shutdown(context.Background()) // flushes remaining entries
    // ...
}
```

**Always `defer nlog.Shutdown(ctx)`** — without it, buffered entries on exit
are lost.

## Log API

```go
nlog.Info("msg", nikologs.Fields{"user_id": id, "count": 3})
nlog.Error("failed", nikologs.Fields{"err": err.Error()})
nlog.Info("no metadata", nil) // pass nil, not empty Fields{}
```

Levels: `Success`, `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`
(`Fatal` does **not** exit the process — it's just a severity).

`Fields` is `map[string]any`. Values must be JSON-serializable.

Per-entry options: `WithTimestamp(t)`, `WithImageID(id)`, `WithFileID(id)`,
`WithTags(tags...)`.

## Tags

Tags are labels for filtering/grouping logs in the read API. Set cross-cutting
tags once via `WithDefaultTags(...)` on `New`; add per-entry tags via the
`WithTags(...)` log option. Both are combined and sent. The server lowercases,
trims, and dedupes; max 20 unique tags per entry, 128 chars each.

```go
nlog := nikologs.New("nk_...", nikologs.WithDefaultTags("env:prod"))
nlog.Error("charge failed", nil, nikologs.WithTags("payment", "stripe"))
// -> tags: ["env:prod", "payment", "stripe"]
```

## Client options (for `nikologs.New`)

| Option | Default | Notes |
|---|---|---|
| `WithBaseURL(url)` | `https://nikologs.dimitrije.dev` | Override for self-hosted |
| `WithSource(s)` | `""` | Tags every entry with a source name |
| `WithDefaultTags(tags...)` | `nil` | Tags applied to every entry; merged with per-call `WithTags` |
| `WithFlushInterval(d)` | `5s` | |
| `WithBatchSize(n)` | `100` | API max: 1000 |
| `WithHTTPClient(c)` | `http.DefaultClient` | |
| `WithUploadKey(k)` | `""` | `nku_`-prefixed key; required for attachments |
| `WithOnError(fn)` | no-op | Observe dropped/failed entries |

## slog integration

If the project already uses `log/slog`, wrap the client:

```go
handler := nikologs.NewSlogHandler(nlog, &nikologs.SlogHandlerOptions{
    Level: slog.LevelInfo,
})
logger := slog.New(handler)
```

Mapping: `slog.Debug→debug`, `Info→info`, `Warn→warn`, `Error→error`, below
Debug → `trace`.

A reserved top-level `tags` attr routes into the entry's tags instead of
metadata. Value may be `[]string`, `[]any` of strings, or a single string.
Override the key with `SlogHandlerOptions{TagsKey: "..."}`; a `tags` attr under
a `WithGroup` prefix stays in metadata.

```go
logger.Error("charge failed", slog.Any("tags", []string{"payment", "stripe"}))
```

## Attachments

Requires `WithUploadKey`. Upload, then reference the returned ID.

```go
resp, err := nlog.UploadAttachment(ctx, file, "screenshot.png")
// resp.ID
nlog.Error("UI bug", nil, nikologs.WithImageID(resp.ID))
```

`UploadAttachment` **is blocking** (unlike log calls) — it performs the HTTP
request synchronously.

## Common mistakes to avoid

- ❌ `go func() { nlog.Info(...) }()` — already non-blocking.
- ❌ Creating a new `Client` per request — construct once, share it.
- ❌ Forgetting `defer nlog.Shutdown(ctx)` — loses in-flight logs.
- ❌ Passing an empty `Fields{}` when you mean "no metadata" — pass `nil`.
- ❌ Treating `Fatal` like `log.Fatal` — it does not call `os.Exit`.
- ❌ Storing non-JSON-serializable values (channels, funcs) in `Fields`.

## Where to look in this repo

- `client.go` — `Client`, `New`, options, `Shutdown`
- `log.go` — levels, `Fields`, `Log`/`Info`/`Error`/... and `LogOption`
- `ingest.go` — batching/HTTP/retry internals
- `slog_handler.go` — `slog.Handler` adapter
- `example_test.go` — runnable usage examples
- `README.md` — user-facing docs
