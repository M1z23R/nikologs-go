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
