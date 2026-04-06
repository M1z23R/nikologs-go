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

func TestLogMethodsBufferEntries(t *testing.T) {
	c, cap := newCapture(t)
	defer c.Shutdown(t.Context())

	c.Info("hello", Fields{"k": "v"})
	c.Error("oops", nil)
	c.Debug("dbg", nil, WithImageID("img-1"))

	logs := cap.waitFor(t, 3, 2*time.Second)
	if logs[0]["level"] != "info" || logs[0]["message"] != "hello" {
		t.Errorf("entry 0: %v", logs[0])
	}
	meta, _ := logs[0]["metadata"].(map[string]any)
	if meta["k"] != "v" {
		t.Errorf("entry 0 meta: %v", logs[0]["metadata"])
	}
	if logs[1]["level"] != "error" || logs[1]["message"] != "oops" {
		t.Errorf("entry 1: %v", logs[1])
	}
	if logs[2]["level"] != "debug" || logs[2]["image_id"] != "img-1" {
		t.Errorf("entry 2: %v", logs[2])
	}
}

func TestLogAllLevels(t *testing.T) {
	c, cap := newCapture(t, WithSource("test-svc"))
	defer c.Shutdown(t.Context())

	methods := []struct {
		call func(string, Fields, ...LogOption)
		name string
	}{
		{c.Success, "success"},
		{c.Trace, "trace"},
		{c.Debug, "debug"},
		{c.Info, "info"},
		{c.Warn, "warn"},
		{c.Error, "error"},
		{c.Fatal, "fatal"},
	}

	for _, m := range methods {
		m.call("msg", nil)
	}

	logs := cap.waitFor(t, 7, 2*time.Second)
	for i, m := range methods {
		if logs[i]["level"] != m.name {
			t.Errorf("entry %d level = %v, want %v", i, logs[i]["level"], m.name)
		}
		if logs[i]["source"] != "test-svc" {
			t.Errorf("entry %d source = %v, want test-svc", i, logs[i]["source"])
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
