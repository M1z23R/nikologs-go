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
