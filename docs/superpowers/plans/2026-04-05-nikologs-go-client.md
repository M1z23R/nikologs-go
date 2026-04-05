# Nikologs Go Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a zero-dependency Go client package for the Nikologs log aggregation API with buffered ingestion, attachment uploads, and slog integration.

**Architecture:** Single `Client` struct with functional options, a buffered channel for non-blocking log calls, a background flush goroutine with exponential-backoff retry, and an `slog.Handler` adapter. Attachment uploads are synchronous via a separate upload key.

**Tech Stack:** Go 1.21+ stdlib only (`net/http`, `encoding/json`, `log/slog`, `sync`, `time`, `mime/multipart`)

---

### Task 1: Project Scaffolding — go.mod and Error Types

**Files:**
- Create: `go.mod`
- Create: `errors.go`
- Create: `errors_test.go`

- [ ] **Step 1: Initialize go.mod**

```bash
cd /home/dimitrije/projects/dimitrije/nikologs-client
go mod init github.com/M1z23r/nikologs-go
```

Expected: `go.mod` created with `module github.com/M1z23r/nikologs-go` and `go 1.21`.

- [ ] **Step 2: Write the error types test**

Create `errors_test.go`:

```go
package nikologs

import (
	"errors"
	"testing"
)

func TestIngestErrorImplementsError(t *testing.T) {
	e := IngestError{Index: 1, Error: "invalid level"}
	want := "log entry 1: invalid level"
	if got := e.Err().Error(); got != want {
		t.Errorf("IngestError.Err() = %q, want %q", got, want)
	}
}

func TestIngestResponseErrorNil(t *testing.T) {
	r := IngestResponse{Accepted: 3, Errors: nil}
	if r.HasErrors() {
		t.Error("expected HasErrors() == false when Errors is nil")
	}
}

func TestIngestResponseErrorPresent(t *testing.T) {
	r := IngestResponse{
		Accepted: 2,
		Errors:   []IngestError{{Index: 0, Error: "bad"}},
	}
	if !r.HasErrors() {
		t.Error("expected HasErrors() == true when Errors is non-empty")
	}
}

func TestErrNoUploadKeyMessage(t *testing.T) {
	err := ErrNoUploadKey
	want := "nikologs: upload key not configured"
	if err.Error() != want {
		t.Errorf("ErrNoUploadKey = %q, want %q", err.Error(), want)
	}
}

func TestErrShutdownMessage(t *testing.T) {
	err := ErrShutdown
	want := "nikologs: client is shut down"
	if err.Error() != want {
		t.Errorf("ErrShutdown = %q, want %q", err.Error(), want)
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrNoUploadKey, ErrShutdown) {
		t.Error("ErrNoUploadKey and ErrShutdown should not be equal")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test -run TestIngestError -v ./...
```

Expected: compilation error — types not defined yet.

- [ ] **Step 4: Write errors.go**

Create `errors.go`:

```go
package nikologs

import (
	"errors"
	"fmt"
)

// ErrNoUploadKey is returned when UploadAttachment is called without an upload key.
var ErrNoUploadKey = errors.New("nikologs: upload key not configured")

// ErrShutdown is returned when logging is attempted on a shut-down client.
var ErrShutdown = errors.New("nikologs: client is shut down")

// IngestError represents a single error from the ingest API for a specific log entry.
type IngestError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// Err returns the IngestError as a Go error.
func (e IngestError) Err() error {
	return fmt.Errorf("log entry %d: %s", e.Index, e.Error)
}

// IngestResponse is the response from the log ingest API.
type IngestResponse struct {
	Accepted int           `json:"accepted"`
	Errors   []IngestError `json:"errors"`
}

// HasErrors reports whether the ingest response contains any errors.
func (r IngestResponse) HasErrors() bool {
	return len(r.Errors) > 0
}

// AttachmentResponse is the response from the attachment upload API.
type AttachmentResponse struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Token string `json:"token"`
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -run "TestIngestError|TestIngestResponse|TestErrNo|TestErrShutdown|TestSentinel" -v ./...
```

Expected: all 6 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod errors.go errors_test.go
git commit -m "feat: add go.mod and error/response types"
```

---

### Task 2: Log Entry Types, Levels, and Fields

**Files:**
- Create: `log.go`
- Create: `log_test.go`

- [ ] **Step 1: Write the log types test**

Create `log_test.go`:

```go
package nikologs

import (
	"testing"
	"time"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelSuccess, "success"},
		{LevelTrace, "trace"},
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{LevelFatal, "fatal"},
		{Level(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", int(tt.level), got, tt.want)
		}
	}
}

func TestLogOptionTimestamp(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := &entry{}
	WithTimestamp(ts)(e)
	if e.Timestamp == nil || !e.Timestamp.Equal(ts) {
		t.Errorf("WithTimestamp did not set timestamp, got %v", e.Timestamp)
	}
}

func TestLogOptionImageID(t *testing.T) {
	e := &entry{}
	WithImageID("img-123")(e)
	if e.ImageID != "img-123" {
		t.Errorf("WithImageID = %q, want %q", e.ImageID, "img-123")
	}
}

func TestLogOptionFileID(t *testing.T) {
	e := &entry{}
	WithFileID("file-456")(e)
	if e.FileID != "file-456" {
		t.Errorf("WithFileID = %q, want %q", e.FileID, "file-456")
	}
}

func TestEntryMarshal(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := &entry{
		Level:   LevelInfo,
		Message: "hello",
		Source:  "svc",
		Meta:    Fields{"k": "v"},
	}
	WithTimestamp(ts)(e)
	WithImageID("img-1")(e)

	m := e.toPayload()
	if m["level"] != "info" {
		t.Errorf("level = %v, want info", m["level"])
	}
	if m["message"] != "hello" {
		t.Errorf("message = %v, want hello", m["message"])
	}
	if m["source"] != "svc" {
		t.Errorf("source = %v, want svc", m["source"])
	}
	if m["timestamp"] != "2026-01-01T00:00:00Z" {
		t.Errorf("timestamp = %v", m["timestamp"])
	}
	if m["image_id"] != "img-1" {
		t.Errorf("image_id = %v", m["image_id"])
	}
	if _, ok := m["file_id"]; ok {
		t.Error("file_id should be omitted when empty")
	}
	meta, ok := m["metadata"].(Fields)
	if !ok || meta["k"] != "v" {
		t.Errorf("metadata = %v", m["metadata"])
	}
}

func TestEntryMarshalOmitsEmptyOptionals(t *testing.T) {
	e := &entry{
		Level:   LevelDebug,
		Message: "bare",
	}
	m := e.toPayload()
	for _, key := range []string{"source", "metadata", "timestamp", "image_id", "file_id"} {
		if _, ok := m[key]; ok {
			t.Errorf("expected %q to be omitted for bare entry", key)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run "TestLevel|TestLogOption|TestEntry" -v ./...
```

Expected: compilation error — types not defined.

- [ ] **Step 3: Write log.go**

Create `log.go`:

```go
package nikologs

import "time"

// Level represents a log severity level.
type Level int

const (
	// LevelSuccess is severity 0.
	LevelSuccess Level = iota
	// LevelTrace is severity 1.
	LevelTrace
	// LevelDebug is severity 2.
	LevelDebug
	// LevelInfo is severity 3.
	LevelInfo
	// LevelWarn is severity 4.
	LevelWarn
	// LevelError is severity 5.
	LevelError
	// LevelFatal is severity 6.
	LevelFatal
)

var levelNames = [...]string{"success", "trace", "debug", "info", "warn", "error", "fatal"}

// String returns the lowercase name of the level.
func (l Level) String() string {
	if l >= 0 && int(l) < len(levelNames) {
		return levelNames[l]
	}
	return "unknown"
}

// Fields is a map of metadata key-value pairs attached to a log entry.
type Fields map[string]any

// entry is an internal log entry buffered before sending.
type entry struct {
	Level     Level
	Message   string
	Source    string
	Meta      Fields
	Timestamp *time.Time
	ImageID   string
	FileID    string
}

// toPayload converts the entry to the JSON-serializable map for the API.
func (e *entry) toPayload() map[string]any {
	m := map[string]any{
		"level":   e.Level.String(),
		"message": e.Message,
	}
	if e.Source != "" {
		m["source"] = e.Source
	}
	if e.Meta != nil && len(e.Meta) > 0 {
		m["metadata"] = e.Meta
	}
	if e.Timestamp != nil {
		m["timestamp"] = e.Timestamp.UTC().Format(time.RFC3339)
	}
	if e.ImageID != "" {
		m["image_id"] = e.ImageID
	}
	if e.FileID != "" {
		m["file_id"] = e.FileID
	}
	return m
}

// LogOption configures optional fields on a single log entry.
type LogOption func(*entry)

// WithTimestamp sets the timestamp on a log entry.
func WithTimestamp(t time.Time) LogOption {
	return func(e *entry) {
		e.Timestamp = &t
	}
}

// WithImageID attaches an image attachment ID to a log entry.
func WithImageID(id string) LogOption {
	return func(e *entry) {
		e.ImageID = id
	}
}

// WithFileID attaches a file attachment ID to a log entry.
func WithFileID(id string) LogOption {
	return func(e *entry) {
		e.FileID = id
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run "TestLevel|TestLogOption|TestEntry" -v ./...
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add log.go log_test.go
git commit -m "feat: add log levels, entry type, Fields, and LogOption"
```

---

### Task 3: Client Struct and Functional Options

**Files:**
- Create: `client.go`
- Create: `client_test.go`

- [ ] **Step 1: Write the client options test**

Create `client_test.go`:

```go
package nikologs

import (
	"net/http"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	c := New("nk_test")
	defer c.Shutdown(t.Context())

	if c.apiKey != "nk_test" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "nk_test")
	}
	if c.baseURL != "https://nikologs.dimitrije.dev" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.batchSize != 100 {
		t.Errorf("batchSize = %d, want 100", c.batchSize)
	}
	if c.flushInterval != 5*time.Second {
		t.Errorf("flushInterval = %v, want 5s", c.flushInterval)
	}
	if c.httpClient != http.DefaultClient {
		t.Error("expected http.DefaultClient")
	}
	if c.source != "" {
		t.Errorf("source = %q, want empty", c.source)
	}
	if c.uploadKey != "" {
		t.Errorf("uploadKey = %q, want empty", c.uploadKey)
	}
}

func TestNewWithOptions(t *testing.T) {
	custom := &http.Client{Timeout: 10 * time.Second}
	var gotErr error
	onErr := func(err error) { gotErr = err }

	c := New("nk_test",
		WithBaseURL("https://custom.example.com"),
		WithSource("my-svc"),
		WithFlushInterval(2*time.Second),
		WithBatchSize(50),
		WithHTTPClient(custom),
		WithUploadKey("nku_upload"),
		WithOnError(onErr),
	)
	defer c.Shutdown(t.Context())

	if c.baseURL != "https://custom.example.com" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.source != "my-svc" {
		t.Errorf("source = %q", c.source)
	}
	if c.flushInterval != 2*time.Second {
		t.Errorf("flushInterval = %v", c.flushInterval)
	}
	if c.batchSize != 50 {
		t.Errorf("batchSize = %d", c.batchSize)
	}
	if c.httpClient != custom {
		t.Error("httpClient not set")
	}
	if c.uploadKey != "nku_upload" {
		t.Errorf("uploadKey = %q", c.uploadKey)
	}

	// Verify onError was wired up
	c.onError(errTest)
	if gotErr != errTest {
		t.Error("onError callback not wired")
	}
}

var errTest = ErrShutdown // reuse a sentinel for testing
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run "TestNew" -v ./...
```

Expected: compilation error — `Client`, `New`, options not defined.

- [ ] **Step 3: Write client.go**

Create `client.go`:

```go
package nikologs

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultBaseURL       = "https://nikologs.dimitrije.dev"
	defaultFlushInterval = 5 * time.Second
	defaultBatchSize     = 100
)

// Client is a buffered client for the Nikologs log ingestion API.
type Client struct {
	baseURL       string
	apiKey        string
	uploadKey     string
	source        string
	flushInterval time.Duration
	batchSize     int
	httpClient    *http.Client
	onError       func(error)

	entries  chan *entry
	quit     chan struct{}
	done     chan struct{}
	shutdown bool
}

// Option configures the Client.
type Option func(*Client)

// New creates a new Client with the given API key and options.
// The client starts a background goroutine that flushes buffered logs.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:       defaultBaseURL,
		apiKey:        apiKey,
		flushInterval: defaultFlushInterval,
		batchSize:     defaultBatchSize,
		httpClient:    http.DefaultClient,
		onError:       func(error) {},
		quit:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	c.entries = make(chan *entry, c.batchSize*10)
	go c.flushLoop()
	return c
}

// Shutdown flushes remaining buffered logs and stops the background goroutine.
// It blocks until all pending logs are flushed or the context expires.
func (c *Client) Shutdown(ctx context.Context) error {
	c.shutdown = true
	close(c.quit)
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WithBaseURL sets the base URL for the Nikologs API.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithSource sets the default source field for all log entries.
func WithSource(source string) Option {
	return func(c *Client) {
		c.source = source
	}
}

// WithFlushInterval sets how often the buffer is flushed.
func WithFlushInterval(d time.Duration) Option {
	return func(c *Client) {
		c.flushInterval = d
	}
}

// WithBatchSize sets the maximum number of log entries per flush.
func WithBatchSize(size int) Option {
	return func(c *Client) {
		c.batchSize = size
	}
}

// WithHTTPClient sets a custom HTTP client for API calls.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithUploadKey sets the upload API key (nku_ prefix) for attachment uploads.
func WithUploadKey(key string) Option {
	return func(c *Client) {
		c.uploadKey = key
	}
}

// WithOnError sets a callback invoked when a flush or send fails after retries.
func WithOnError(fn func(error)) Option {
	return func(c *Client) {
		c.onError = fn
	}
}
```

- [ ] **Step 4: Create a stub flushLoop so the code compiles**

We need a minimal `buffer.go` so `c.flushLoop()` compiles. Create `buffer.go`:

```go
package nikologs

// flushLoop runs in a background goroutine, batching and flushing log entries.
func (c *Client) flushLoop() {
	defer close(c.done)
	// Full implementation in Task 5.
	for {
		select {
		case <-c.quit:
			return
		case <-c.entries:
			// drain, will be replaced
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -run "TestNew" -v ./...
```

Expected: both tests PASS.

- [ ] **Step 6: Commit**

```bash
git add client.go client_test.go buffer.go
git commit -m "feat: add Client struct, New(), and functional options"
```

---

### Task 4: Logging Methods (Log, Info, Error, etc.)

**Files:**
- Modify: `log.go` (add methods)
- Modify: `client_test.go` (add logging tests)

- [ ] **Step 1: Write the logging methods test**

Append to `client_test.go`:

```go
func TestLogMethodsBufferEntries(t *testing.T) {
	c := New("nk_test", WithBatchSize(100), WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	c.Info("hello", Fields{"k": "v"})
	c.Error("oops", nil)
	c.Debug("dbg", nil, WithImageID("img-1"))

	// Drain 3 entries from the channel
	for i := 0; i < 3; i++ {
		select {
		case e := <-c.entries:
			switch i {
			case 0:
				if e.Level != LevelInfo || e.Message != "hello" {
					t.Errorf("entry 0: level=%v msg=%q", e.Level, e.Message)
				}
				if e.Meta["k"] != "v" {
					t.Errorf("entry 0: meta=%v", e.Meta)
				}
			case 1:
				if e.Level != LevelError || e.Message != "oops" {
					t.Errorf("entry 1: level=%v msg=%q", e.Level, e.Message)
				}
			case 2:
				if e.Level != LevelDebug || e.ImageID != "img-1" {
					t.Errorf("entry 2: level=%v imageID=%q", e.Level, e.ImageID)
				}
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for entries")
		}
	}
}

func TestLogAllLevels(t *testing.T) {
	c := New("nk_test", WithSource("test-svc"), WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	methods := []struct {
		call  func(string, Fields, ...LogOption)
		level Level
	}{
		{c.Success, LevelSuccess},
		{c.Trace, LevelTrace},
		{c.Debug, LevelDebug},
		{c.Info, LevelInfo},
		{c.Warn, LevelWarn},
		{c.Error, LevelError},
		{c.Fatal, LevelFatal},
	}

	for _, m := range methods {
		m.call("msg", nil)
		select {
		case e := <-c.entries:
			if e.Level != m.level {
				t.Errorf("expected level %v, got %v", m.level, e.Level)
			}
			if e.Source != "test-svc" {
				t.Errorf("expected source test-svc, got %q", e.Source)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for level %v", m.level)
		}
	}
}

func TestLogOnShutdownClient(t *testing.T) {
	var gotErr error
	c := New("nk_test", WithOnError(func(err error) { gotErr = err }), WithFlushInterval(time.Hour))
	c.Shutdown(t.Context())

	c.Info("after shutdown", nil)
	if gotErr != ErrShutdown {
		t.Errorf("expected ErrShutdown, got %v", gotErr)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run "TestLogMethods|TestLogAllLevels|TestLogOnShutdown" -v ./...
```

Expected: compilation error — logging methods not defined.

- [ ] **Step 3: Add logging methods to log.go**

Append to `log.go`:

```go
// Log buffers a log entry at the given level. It is non-blocking.
// If the client is shut down or the buffer is full, the entry is dropped
// and the OnError callback is invoked.
func (c *Client) Log(level Level, msg string, fields Fields, opts ...LogOption) {
	if c.shutdown {
		c.onError(ErrShutdown)
		return
	}
	e := &entry{
		Level:   level,
		Message: msg,
		Source:  c.source,
		Meta:    fields,
	}
	for _, opt := range opts {
		opt(e)
	}
	select {
	case c.entries <- e:
	default:
		c.onError(fmt.Errorf("nikologs: buffer full, dropping log: %s", msg))
	}
}

// Success logs at LevelSuccess.
func (c *Client) Success(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelSuccess, msg, fields, opts...)
}

// Trace logs at LevelTrace.
func (c *Client) Trace(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelTrace, msg, fields, opts...)
}

// Debug logs at LevelDebug.
func (c *Client) Debug(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelDebug, msg, fields, opts...)
}

// Info logs at LevelInfo.
func (c *Client) Info(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelInfo, msg, fields, opts...)
}

// Warn logs at LevelWarn.
func (c *Client) Warn(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelWarn, msg, fields, opts...)
}

// Error logs at LevelError.
func (c *Client) Error(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelError, msg, fields, opts...)
}

// Fatal logs at LevelFatal.
func (c *Client) Fatal(msg string, fields Fields, opts ...LogOption) {
	c.Log(LevelFatal, msg, fields, opts...)
}
```

Also add the `"fmt"` import to the existing import block in `log.go`:

```go
import (
	"fmt"
	"time"
)
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run "TestLogMethods|TestLogAllLevels|TestLogOnShutdown" -v ./...
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add log.go client_test.go
git commit -m "feat: add Log() and per-level convenience methods"
```

---

### Task 5: HTTP Ingest Layer

**Files:**
- Create: `ingest.go`
- Create: `ingest_test.go`

- [ ] **Step 1: Write the ingest HTTP test**

Create `ingest_test.go`:

```go
package nikologs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendBatchSuccess(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: 2})
	}))
	defer srv.Close()

	c := New("nk_test123", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	entries := []*entry{
		{Level: LevelInfo, Message: "one", Source: "svc"},
		{Level: LevelError, Message: "two"},
	}

	resp, err := c.sendBatch(entries)
	if err != nil {
		t.Fatalf("sendBatch error: %v", err)
	}
	if resp.Accepted != 2 {
		t.Errorf("accepted = %d, want 2", resp.Accepted)
	}
	if gotAuth != "Bearer nk_test123" {
		t.Errorf("auth = %q", gotAuth)
	}
	logs, ok := gotBody["logs"].([]any)
	if !ok || len(logs) != 2 {
		t.Fatalf("expected 2 logs in body, got %v", gotBody)
	}
}

func TestSendBatchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.sendBatch([]*entry{{Level: LevelInfo, Message: "x"}})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendBatchWithPartialErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{
			Accepted: 1,
			Errors:   []IngestError{{Index: 0, Error: "invalid level"}},
		})
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	resp, err := c.sendBatch([]*entry{{Level: LevelInfo, Message: "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasErrors() {
		t.Error("expected HasErrors() == true")
	}
	if resp.Errors[0].Error != "invalid level" {
		t.Errorf("error = %q", resp.Errors[0].Error)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run "TestSendBatch" -v ./...
```

Expected: compilation error — `sendBatch` not defined.

- [ ] **Step 3: Write ingest.go**

Create `ingest.go`:

```go
package nikologs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	ingestPath     = "/api/v1/ingest/logs"
	attachmentPath = "/api/v1/ingest/attachments"
)

// sendBatch sends a batch of entries to the ingest API.
func (c *Client) sendBatch(entries []*entry) (*IngestResponse, error) {
	logs := make([]map[string]any, len(entries))
	for i, e := range entries {
		logs[i] = e.toPayload()
	}
	body, err := json.Marshal(map[string]any{"logs": logs})
	if err != nil {
		return nil, fmt.Errorf("nikologs: marshal error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+ingestPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("nikologs: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nikologs: send error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nikologs: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result IngestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("nikologs: decode error: %w", err)
	}
	return &result, nil
}

// UploadAttachment uploads a file to the attachment API.
// It requires an upload key (nku_ prefix) to be configured via WithUploadKey.
func (c *Client) UploadAttachment(ctx context.Context, file io.Reader, filename string) (*AttachmentResponse, error) {
	if c.uploadKey == "" {
		return nil, ErrNoUploadKey
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("nikologs: multipart error: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("nikologs: copy error: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+attachmentPath, &buf)
	if err != nil {
		return nil, fmt.Errorf("nikologs: request error: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.uploadKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nikologs: upload error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nikologs: upload status %d: %s", resp.StatusCode, string(respBody))
	}

	var result AttachmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("nikologs: decode error: %w", err)
	}
	return &result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run "TestSendBatch" -v ./...
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ingest.go ingest_test.go
git commit -m "feat: add HTTP ingest and attachment upload"
```

---

### Task 6: Attachment Upload Tests

**Files:**
- Modify: `ingest_test.go` (add upload tests)

- [ ] **Step 1: Write attachment upload tests**

Append to `ingest_test.go`:

```go
func TestUploadAttachmentSuccess(t *testing.T) {
	var gotContentType string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		// Verify the multipart form contains a "file" field
		r.ParseMultipartForm(1 << 20)
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile error: %v", err)
			return
		}
		defer f.Close()
		if header.Filename != "test.png" {
			t.Errorf("filename = %q, want test.png", header.Filename)
		}
		data, _ := io.ReadAll(f)
		if string(data) != "image-data" {
			t.Errorf("file content = %q", string(data))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AttachmentResponse{ID: "uuid-1", Kind: "image", Token: "abc"})
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithUploadKey("nku_upload"), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	resp, err := c.UploadAttachment(t.Context(), strings.NewReader("image-data"), "test.png")
	if err != nil {
		t.Fatalf("UploadAttachment error: %v", err)
	}
	if resp.ID != "uuid-1" || resp.Kind != "image" || resp.Token != "abc" {
		t.Errorf("resp = %+v", resp)
	}
	if gotAuth != "Bearer nku_upload" {
		t.Errorf("auth = %q", gotAuth)
	}
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("content-type = %q", gotContentType)
	}
}

func TestUploadAttachmentNoKey(t *testing.T) {
	c := New("nk_test", WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.UploadAttachment(t.Context(), strings.NewReader("data"), "f.txt")
	if err != ErrNoUploadKey {
		t.Errorf("expected ErrNoUploadKey, got %v", err)
	}
}

func TestUploadAttachmentServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("too large"))
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithUploadKey("nku_key"), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.UploadAttachment(t.Context(), strings.NewReader("data"), "f.txt")
	if err == nil {
		t.Fatal("expected error for 413")
	}
}
```

Also add `"strings"` and `"time"` to the import block in `ingest_test.go`:

```go
import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test -run "TestUploadAttachment" -v ./...
```

Expected: all 3 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add ingest_test.go
git commit -m "test: add attachment upload tests"
```

---

### Task 7: Buffer and Flush Loop with Retry

**Files:**
- Modify: `buffer.go` (full implementation)
- Create: `buffer_test.go`

- [ ] **Step 1: Write the buffer flush tests**

Create `buffer_test.go`:

```go
package nikologs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlushOnBatchSize(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(5),
		WithFlushInterval(time.Hour), // don't let timer trigger
	)

	for i := 0; i < 5; i++ {
		c.Info("msg", nil)
	}

	// Wait for the flush to happen
	deadline := time.After(2 * time.Second)
	for received.Load() < 5 {
		select {
		case <-deadline:
			t.Fatalf("timed out, received %d logs", received.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestFlushOnTimer(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(100),              // won't fill
		WithFlushInterval(50*time.Millisecond),
	)

	c.Info("timer-flush", nil)

	deadline := time.After(2 * time.Second)
	for received.Load() < 1 {
		select {
		case <-deadline:
			t.Fatalf("timed out, received %d", received.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestShutdownFlushesRemaining(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(100),
		WithFlushInterval(time.Hour),
	)

	c.Info("one", nil)
	c.Info("two", nil)

	// Give entries time to be buffered
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := c.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	if got := received.Load(); got != 2 {
		t.Errorf("received = %d, want 2", got)
	}
}

func TestRetryOnFailure(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: 1})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(1),
		WithFlushInterval(time.Hour),
	)
	c.Info("retry-me", nil)

	deadline := time.After(15 * time.Second)
	for attempts.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for retries, got %d attempts", attempts.Load())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
	if got := attempts.Load(); got < 3 {
		t.Errorf("attempts = %d, want >= 3", got)
	}
}

func TestRetryExhaustedCallsOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var mu sync.Mutex
	var errors []error
	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(1),
		WithFlushInterval(time.Hour),
		WithOnError(func(err error) {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}),
	)
	c.Info("fail-me", nil)

	// Wait for retries to exhaust (1s + 2s + 4s = 7s worst case, but test server is fast)
	deadline := time.After(15 * time.Second)
	for {
		mu.Lock()
		n := len(errors)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for onError")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestConcurrentLogging(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(10),
		WithFlushInterval(50*time.Millisecond),
	)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Info("concurrent", nil)
		}()
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	c.Shutdown(ctx)

	if got := received.Load(); got != 50 {
		t.Errorf("received = %d, want 50", got)
	}
}
```

Also add `"context"` to the import block:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run "TestFlush|TestShutdown|TestRetry|TestConcurrent" -v ./...
```

Expected: tests fail — buffer.go is a stub.

- [ ] **Step 3: Write the full buffer.go implementation**

Replace `buffer.go` with:

```go
package nikologs

import (
	"fmt"
	"time"
)

const maxRetries = 3

// flushLoop runs in a background goroutine, batching and flushing log entries.
// It flushes when the batch reaches batchSize or the flush interval timer fires.
func (c *Client) flushLoop() {
	defer close(c.done)

	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	batch := make([]*entry, 0, c.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toSend := batch
		batch = make([]*entry, 0, c.batchSize)
		c.sendWithRetry(toSend)
	}

	for {
		select {
		case e := <-c.entries:
			batch = append(batch, e)
			if len(batch) >= c.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-c.quit:
			// Drain remaining entries from channel
			for {
				select {
				case e := <-c.entries:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

// sendWithRetry sends a batch with exponential backoff retry.
func (c *Client) sendWithRetry(entries []*entry) {
	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := c.sendBatch(entries)
		if err == nil {
			return
		}
		if attempt < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		} else {
			c.onError(fmt.Errorf("nikologs: flush failed after %d retries: %w", maxRetries, err))
		}
	}
}

// Flush manually flushes any buffered log entries.
func (c *Client) Flush() {
	// Send a signal by temporarily draining entries into a batch and sending
	done := make(chan struct{})
	go func() {
		defer close(done)
		batch := make([]*entry, 0, c.batchSize)
		for {
			select {
			case e := <-c.entries:
				batch = append(batch, e)
			default:
				if len(batch) > 0 {
					c.sendWithRetry(batch)
				}
				return
			}
		}
	}()
	<-done
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run "TestFlush|TestShutdown|TestRetry|TestConcurrent" -v -timeout 30s ./...
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add buffer.go buffer_test.go
git commit -m "feat: implement buffered flush loop with exponential backoff retry"
```

---

### Task 8: slog Handler

**Files:**
- Create: `slog_handler.go`
- Create: `slog_handler_test.go`

- [ ] **Step 1: Write the slog handler tests**

Create `slog_handler_test.go`:

```go
package nikologs

import (
	"log/slog"
	"testing"
	"time"
)

func TestSlogHandlerLevelMapping(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)

	tests := []struct {
		slogLevel slog.Level
		want      Level
	}{
		{slog.LevelDebug - 1, LevelTrace},  // below debug -> trace
		{slog.LevelDebug, LevelDebug},
		{slog.LevelInfo, LevelInfo},
		{slog.LevelWarn, LevelWarn},
		{slog.LevelError, LevelError},
		{slog.LevelError + 4, LevelError},  // above error -> still error
	}

	for _, tt := range tests {
		got := h.(*slogHandler).mapLevel(tt.slogLevel)
		if got != tt.want {
			t.Errorf("mapLevel(%v) = %v, want %v", tt.slogLevel, got, tt.want)
		}
	}
}

func TestSlogHandlerEnabled(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, &SlogHandlerOptions{Level: slog.LevelWarn})

	if h.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("Info should not be enabled when min is Warn")
	}
	if !h.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("Warn should be enabled when min is Warn")
	}
	if !h.Enabled(t.Context(), slog.LevelError) {
		t.Error("Error should be enabled when min is Warn")
	}
}

func TestSlogHandlerHandle(t *testing.T) {
	c := New("nk_test", WithSource("slog-test"), WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)
	logger := slog.New(h)

	logger.Info("hello", "method", "GET", "status", 200)

	select {
	case e := <-c.entries:
		if e.Level != LevelInfo {
			t.Errorf("level = %v, want Info", e.Level)
		}
		if e.Message != "hello" {
			t.Errorf("message = %q", e.Message)
		}
		if e.Meta["method"] != "GET" {
			t.Errorf("meta[method] = %v", e.Meta["method"])
		}
		if e.Meta["status"] != 200 {
			t.Errorf("meta[status] = %v", e.Meta["status"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for entry")
	}
}

func TestSlogHandlerWithGroup(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)
	logger := slog.New(h).WithGroup("http")

	logger.Info("request", "status", 200)

	select {
	case e := <-c.entries:
		if e.Meta["http.status"] != 200 {
			t.Errorf("expected http.status=200, got meta=%v", e.Meta)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)
	logger := slog.New(h.WithAttrs([]slog.Attr{slog.String("service", "api")}))

	logger.Info("req")

	select {
	case e := <-c.entries:
		if e.Meta["service"] != "api" {
			t.Errorf("expected service=api, got meta=%v", e.Meta)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestSlogHandlerNestedGroups(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)
	logger := slog.New(h).WithGroup("a").WithGroup("b")

	logger.Info("nested", "key", "val")

	select {
	case e := <-c.entries:
		if e.Meta["a.b.key"] != "val" {
			t.Errorf("expected a.b.key=val, got meta=%v", e.Meta)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestSlogHandlerFiltersBelowLevel(t *testing.T) {
	c := New("nk_test", WithFlushInterval(time.Hour))
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, &SlogHandlerOptions{Level: slog.LevelError})
	logger := slog.New(h)

	logger.Info("should be dropped")

	select {
	case <-c.entries:
		t.Error("entry should not have been buffered")
	case <-time.After(100 * time.Millisecond):
		// expected: nothing received
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run "TestSlogHandler" -v ./...
```

Expected: compilation error — `NewSlogHandler` not defined.

- [ ] **Step 3: Write slog_handler.go**

Create `slog_handler.go`:

```go
package nikologs

import (
	"context"
	"log/slog"
)

// SlogHandlerOptions configures the slog handler.
type SlogHandlerOptions struct {
	// Level is the minimum slog level that will be forwarded to nikologs.
	// Defaults to slog.LevelInfo if nil/zero.
	Level slog.Leveler
}

// slogHandler implements slog.Handler, forwarding records to a nikologs Client.
type slogHandler struct {
	client *Client
	level  slog.Leveler
	group  string // dot-separated group prefix
	attrs  []slog.Attr
}

// NewSlogHandler returns an slog.Handler that forwards log records to the given Client.
func NewSlogHandler(client *Client, opts *SlogHandlerOptions) slog.Handler {
	h := &slogHandler{
		client: client,
		level:  slog.LevelInfo,
	}
	if opts != nil && opts.Level != nil {
		h.level = opts.Level
	}
	return h
}

// Enabled reports whether the handler is configured to handle records at the given level.
func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle converts the slog.Record to a nikologs entry and buffers it.
func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(Fields)

	// Add pre-set attrs
	for _, a := range h.attrs {
		h.addAttr(fields, h.group, a)
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		h.addAttr(fields, h.group, a)
		return true
	})

	var meta Fields
	if len(fields) > 0 {
		meta = fields
	}

	h.client.Log(h.mapLevel(r.Level), r.Message, meta)
	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &slogHandler{
		client: h.client,
		level:  h.level,
		group:  h.group,
		attrs:  newAttrs,
	}
}

// WithGroup returns a new handler with the given group prefix.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &slogHandler{
		client: h.client,
		level:  h.level,
		group:  newGroup,
		attrs:  h.attrs,
	}
}

// mapLevel converts a slog.Level to a nikologs Level.
func (h *slogHandler) mapLevel(l slog.Level) Level {
	switch {
	case l < slog.LevelDebug:
		return LevelTrace
	case l < slog.LevelInfo:
		return LevelDebug
	case l < slog.LevelWarn:
		return LevelInfo
	case l < slog.LevelError:
		return LevelWarn
	default:
		return LevelError
	}
}

// addAttr adds a single slog.Attr to the fields map with the given group prefix.
func (h *slogHandler) addAttr(fields Fields, prefix string, a slog.Attr) {
	a = a.Resolve()
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			h.addAttr(fields, key, ga)
		}
		return
	}

	fields[key] = a.Value.Any()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run "TestSlogHandler" -v ./...
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add slog_handler.go slog_handler_test.go
git commit -m "feat: add slog.Handler integration"
```

---

### Task 9: Example Tests

**Files:**
- Create: `example_test.go`

- [ ] **Step 1: Write example_test.go**

Create `example_test.go`:

```go
package nikologs_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	nikologs "github.com/M1z23r/nikologs-go"
)

func ExampleNew() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
		nikologs.WithFlushInterval(3*time.Second),
		nikologs.WithBatchSize(200),
	)
	defer client.Shutdown(context.Background())

	client.Info("service started", nikologs.Fields{"version": "1.0.0"})
	// Output:
}

func ExampleClient_Info() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer client.Shutdown(context.Background())

	client.Info("user signed in", nikologs.Fields{
		"user_id": "u-123",
		"method":  "oauth",
	})
	// Output:
}

func ExampleClient_Log() {
	client := nikologs.New("nk_your_api_key")
	defer client.Shutdown(context.Background())

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	client.Log(nikologs.LevelWarn, "disk usage high", nikologs.Fields{
		"percent": 92,
	}, nikologs.WithTimestamp(ts))
	// Output:
}

func ExampleClient_UploadAttachment() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithUploadKey("nku_your_upload_key"),
	)
	defer client.Shutdown(context.Background())

	file := strings.NewReader(`{"error": "stack trace here"}`)
	resp, err := client.UploadAttachment(context.Background(), file, "error.json")
	if err != nil {
		fmt.Println("upload failed:", err)
		return
	}

	// Use the attachment ID in a log entry
	client.Error("crash report", nil, nikologs.WithFileID(resp.ID))
	// Output:
}

func ExampleNewSlogHandler() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer client.Shutdown(context.Background())

	handler := nikologs.NewSlogHandler(client, &nikologs.SlogHandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	logger.Info("request handled", "method", "GET", "status", 200)
	// Output:
}

func ExampleWithOnError() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithOnError(func(err error) {
			fmt.Println("nikologs error:", err)
		}),
	)
	defer client.Shutdown(context.Background())
	// Output:
}

func ExampleWithHTTPClient() {
	custom := &http.Client{Timeout: 30 * time.Second}
	client := nikologs.New("nk_your_api_key",
		nikologs.WithHTTPClient(custom),
	)
	defer client.Shutdown(context.Background())
	// Output:
}
```

- [ ] **Step 2: Run examples to verify they compile**

```bash
go test -run "Example" -v ./...
```

Expected: all examples PASS (they produce no output, matching `// Output:`).

- [ ] **Step 3: Commit**

```bash
git add example_test.go
git commit -m "docs: add runnable example tests"
```

---

### Task 10: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md**

Create `README.md`:

```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

### Task 11: Test Coverage Verification

- [ ] **Step 1: Run full test suite with coverage**

```bash
go test -v -race -coverprofile=coverage.out ./...
```

Expected: all tests pass, race detector clean.

- [ ] **Step 2: Check coverage percentage**

```bash
go tool cover -func=coverage.out | tail -1
```

Expected: total coverage > 80%.

- [ ] **Step 3: If coverage is below 80%, add tests for uncovered lines**

Identify uncovered lines:

```bash
go tool cover -func=coverage.out | grep -v "100.0%"
```

Add targeted tests for any file below 80% and re-run. Repeat until total is above 80%.

- [ ] **Step 4: Commit any new tests**

```bash
git add -A
git commit -m "test: improve coverage to >80%"
```

Plan complete and saved to `docs/superpowers/plans/2026-04-05-nikologs-go-client.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?