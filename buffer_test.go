package nikologs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlushOnBatchSize(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(5),
		WithFlushInterval(time.Hour),
	)

	for i := 0; i < 5; i++ {
		c.Info("msg", nil)
	}

	deadline := time.After(2 * time.Second)
	for received.Load() < 5 {
		select {
		case <-deadline:
			t.Fatalf("timed out, received %d logs", received.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestFlushOnTimer(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(100),
		WithFlushInterval(50*time.Millisecond),
	)

	c.Info("timer-flush", nil)

	deadline := time.After(2 * time.Second)
	for received.Load() < 1 {
		select {
		case <-deadline:
			t.Fatalf("timed out, received %d", received.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestShutdownFlushesRemaining(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(100),
		WithFlushInterval(time.Hour),
	)

	c.Info("one", nil)
	c.Info("two", nil)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := c.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	if got := received.Load(); got != 2 {
		t.Errorf("received = %d, want 2", got)
	}
}

func TestRetryOnFailure(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: 1})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(1),
		WithFlushInterval(time.Hour),
	)
	c.Info("retry-me", nil)

	deadline := time.After(15 * time.Second)
	for attempts.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for retries, got %d attempts", attempts.Load())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
	if got := attempts.Load(); got < 3 {
		t.Errorf("attempts = %d, want >= 3", got)
	}
}

func TestRetryExhaustedCallsOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var mu sync.Mutex
	var errors []error
	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(1),
		WithFlushInterval(time.Hour),
		WithOnError(func(err error) {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}),
	)
	c.Info("fail-me", nil)

	deadline := time.After(15 * time.Second)
	for {
		mu.Lock()
		n := len(errors)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for onError")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.Shutdown(t.Context())
}

func TestConcurrentLogging(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Logs []any `json:"logs"` }
		json.NewDecoder(r.Body).Decode(&body)
		received.Add(int32(len(body.Logs)))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: len(body.Logs)})
	}))
	defer srv.Close()

	c := New("nk_test",
		WithBaseURL(srv.URL),
		WithBatchSize(10),
		WithFlushInterval(50*time.Millisecond),
	)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Info("concurrent", nil)
		}()
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	c.Shutdown(ctx)

	if got := received.Load(); got != 50 {
		t.Errorf("received = %d, want 50", got)
	}
}
