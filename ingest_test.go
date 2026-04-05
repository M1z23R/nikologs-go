package nikologs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendBatchSuccess(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{Accepted: 2})
	}))
	defer srv.Close()

	c := New("nk_test123", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	entries := []*entry{
		{Level: LevelInfo, Message: "one", Source: "svc"},
		{Level: LevelError, Message: "two"},
	}

	resp, err := c.sendBatch(entries)
	if err != nil {
		t.Fatalf("sendBatch error: %v", err)
	}
	if resp.Accepted != 2 {
		t.Errorf("accepted = %d, want 2", resp.Accepted)
	}
	if gotAuth != "Bearer nk_test123" {
		t.Errorf("auth = %q", gotAuth)
	}
	logs, ok := gotBody["logs"].([]any)
	if !ok || len(logs) != 2 {
		t.Fatalf("expected 2 logs in body, got %v", gotBody)
	}
}

func TestSendBatchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.sendBatch([]*entry{{Level: LevelInfo, Message: "x"}})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendBatchWithPartialErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(IngestResponse{
			Accepted: 1,
			Errors:   []IngestError{{Index: 0, Error: "invalid level"}},
		})
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	resp, err := c.sendBatch([]*entry{{Level: LevelInfo, Message: "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasErrors() {
		t.Error("expected HasErrors() == true")
	}
	if resp.Errors[0].Error != "invalid level" {
		t.Errorf("error = %q", resp.Errors[0].Error)
	}
}

func TestUploadAttachmentSuccess(t *testing.T) {
	var gotContentType string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		r.ParseMultipartForm(1 << 20)
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile error: %v", err)
			return
		}
		defer f.Close()
		if header.Filename != "test.png" {
			t.Errorf("filename = %q, want test.png", header.Filename)
		}
		data, _ := io.ReadAll(f)
		if string(data) != "image-data" {
			t.Errorf("file content = %q", string(data))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AttachmentResponse{ID: "uuid-1", Kind: "image", Token: "abc"})
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithUploadKey("nku_upload"), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	resp, err := c.UploadAttachment(t.Context(), strings.NewReader("image-data"), "test.png")
	if err != nil {
		t.Fatalf("UploadAttachment error: %v", err)
	}
	if resp.ID != "uuid-1" || resp.Kind != "image" || resp.Token != "abc" {
		t.Errorf("resp = %+v", resp)
	}
	if gotAuth != "Bearer nku_upload" {
		t.Errorf("auth = %q", gotAuth)
	}
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("content-type = %q", gotContentType)
	}
}

func TestUploadAttachmentNoKey(t *testing.T) {
	c := New("nk_test", WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.UploadAttachment(t.Context(), strings.NewReader("data"), "f.txt")
	if err != ErrNoUploadKey {
		t.Errorf("expected ErrNoUploadKey, got %v", err)
	}
}

func TestUploadAttachmentServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("too large"))
	}))
	defer srv.Close()

	c := New("nk_test", WithBaseURL(srv.URL), WithUploadKey("nku_key"), WithFlushInterval(100*60*time.Second))
	defer c.Shutdown(t.Context())

	_, err := c.UploadAttachment(t.Context(), strings.NewReader("data"), "f.txt")
	if err == nil {
		t.Fatal("expected error for 413")
	}
}
