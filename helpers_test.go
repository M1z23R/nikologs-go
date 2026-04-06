package nikologs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// captured is a thread-safe collector of log entries received by a test server.
type captured struct {
	mu     sync.Mutex
	logs   []map[string]any
	server *httptest.Server
}

// all returns a copy of all received log entries.
func (c *captured) all() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[string]any, len(c.logs))
	copy(out, c.logs)
	return out
}

// waitFor waits up to d for at least n entries to arrive, returning them.
func (c *captured) waitFor(t *testing.T, n int, d time.Duration) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		got := len(c.logs)
		c.mu.Unlock()
		if got >= n {
			return c.all()
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t.Fatalf("timed out waiting for %d entries, got %d", n, len(c.logs))
	return nil
}

// newCapture creates a Client whose logs are captured by an httptest server.
// Each log call triggers an immediate flush (BatchSize=1).
func newCapture(t *testing.T, opts ...Option) (*Client, *captured) {
	t.Helper()
	cap := &captured{}
	cap.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Logs []map[string]any `json:"logs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		cap.mu.Lock()
		cap.logs = append(cap.logs, body.Logs...)
		cap.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	t.Cleanup(cap.server.Close)

	allOpts := append([]Option{
		WithBaseURL(cap.server.URL),
		WithBatchSize(1),
		WithFlushInterval(time.Hour),
	}, opts...)
	c := New("nk_test", allOpts...)
	return c, cap
}
