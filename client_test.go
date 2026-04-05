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
