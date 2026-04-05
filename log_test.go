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
