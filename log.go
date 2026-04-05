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
