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
