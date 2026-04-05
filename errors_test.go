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
