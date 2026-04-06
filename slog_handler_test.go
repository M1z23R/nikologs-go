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
		{slog.LevelDebug - 1, LevelTrace},
		{slog.LevelDebug, LevelDebug},
		{slog.LevelInfo, LevelInfo},
		{slog.LevelWarn, LevelWarn},
		{slog.LevelError, LevelError},
		{slog.LevelError + 4, LevelError},
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

func metaOf(t *testing.T, log map[string]any) map[string]any {
	t.Helper()
	m, _ := log["metadata"].(map[string]any)
	return m
}

func TestSlogHandlerHandle(t *testing.T) {
	c, cap := newCapture(t, WithSource("slog-test"))
	defer c.Shutdown(t.Context())

	logger := slog.New(NewSlogHandler(c, nil))
	logger.Info("hello", "method", "GET", "status", 200)

	logs := cap.waitFor(t, 1, 2*time.Second)
	if logs[0]["level"] != "info" {
		t.Errorf("level = %v", logs[0]["level"])
	}
	if logs[0]["message"] != "hello" {
		t.Errorf("message = %v", logs[0]["message"])
	}
	meta := metaOf(t, logs[0])
	if meta["method"] != "GET" {
		t.Errorf("meta[method] = %v", meta["method"])
	}
	// JSON unmarshals numbers as float64
	if meta["status"] != float64(200) {
		t.Errorf("meta[status] = %v (%T)", meta["status"], meta["status"])
	}
}

func TestSlogHandlerWithGroup(t *testing.T) {
	c, cap := newCapture(t)
	defer c.Shutdown(t.Context())

	logger := slog.New(NewSlogHandler(c, nil)).WithGroup("http")
	logger.Info("request", "status", 200)

	logs := cap.waitFor(t, 1, 2*time.Second)
	meta := metaOf(t, logs[0])
	if meta["http.status"] != float64(200) {
		t.Errorf("expected http.status=200, got meta=%v", meta)
	}
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	c, cap := newCapture(t)
	defer c.Shutdown(t.Context())

	h := NewSlogHandler(c, nil)
	logger := slog.New(h.WithAttrs([]slog.Attr{slog.String("service", "api")}))
	logger.Info("req")

	logs := cap.waitFor(t, 1, 2*time.Second)
	meta := metaOf(t, logs[0])
	if meta["service"] != "api" {
		t.Errorf("expected service=api, got meta=%v", meta)
	}
}

func TestSlogHandlerNestedGroups(t *testing.T) {
	c, cap := newCapture(t)
	defer c.Shutdown(t.Context())

	logger := slog.New(NewSlogHandler(c, nil)).WithGroup("a").WithGroup("b")
	logger.Info("nested", "key", "val")

	logs := cap.waitFor(t, 1, 2*time.Second)
	meta := metaOf(t, logs[0])
	if meta["a.b.key"] != "val" {
		t.Errorf("expected a.b.key=val, got meta=%v", meta)
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
