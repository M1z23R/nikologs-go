package nikologs_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	nikologs "github.com/M1z23r/nikologs-go"
)

func ExampleNew() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
		nikologs.WithFlushInterval(3*time.Second),
		nikologs.WithBatchSize(200),
	)
	defer nlog.Shutdown(context.Background())

	nlog.Info("service started", nikologs.Fields{"version": "1.0.0"})
	// Output:
}

func ExampleClient_Info() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer nlog.Shutdown(context.Background())

	nlog.Info("user signed in", nikologs.Fields{
		"user_id": "u-123",
		"method":  "oauth",
	})
	// Output:
}

func ExampleClient_Log() {
	nlog := nikologs.New("nk_your_api_key")
	defer nlog.Shutdown(context.Background())

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	nlog.Log(nikologs.LevelWarn, "disk usage high", nikologs.Fields{
		"percent": 92,
	}, nikologs.WithTimestamp(ts))
	// Output:
}

func ExampleClient_tags() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
		nikologs.WithDefaultTags("env:prod", "region:eu"),
	)
	defer nlog.Shutdown(context.Background())

	// Sent with tags: ["env:prod", "region:eu", "payment", "stripe"]
	nlog.Error("charge failed", nikologs.Fields{"amount": 4200},
		nikologs.WithTags("payment", "stripe"))
	// Output:
}

func ExampleClient_UploadAttachment() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithUploadKey("nku_your_upload_key"),
	)
	defer nlog.Shutdown(context.Background())

	file := strings.NewReader(`{"error": "stack trace here"}`)
	resp, err := nlog.UploadAttachment(context.Background(), file, "error.json")
	if err != nil {
		// handle upload error
		_ = err
		return
	}

	nlog.Error("crash report", nil, nikologs.WithFileID(resp.ID))
	// Output:
}

func ExampleNewSlogHandler() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer nlog.Shutdown(context.Background())

	handler := nikologs.NewSlogHandler(nlog, &nikologs.SlogHandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	logger.Info("request handled", "method", "GET", "status", 200)
	// Output:
}

func ExampleNewSlogHandler_tags() {
	nlog := nikologs.New("nk_your_api_key")
	defer nlog.Shutdown(context.Background())

	logger := slog.New(nikologs.NewSlogHandler(nlog, nil))

	// The reserved "tags" attr becomes the entry's tags, not metadata.
	logger.Error("charge failed",
		slog.Any("tags", []string{"payment", "stripe"}),
		slog.Int("amount", 4200))
	// Output:
}

func ExampleWithOnError() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithOnError(func(err error) {
			fmt.Println("nikologs error:", err)
		}),
	)
	defer nlog.Shutdown(context.Background())
	// Output:
}

func ExampleWithHTTPClient() {
	custom := &http.Client{Timeout: 30 * time.Second}
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithHTTPClient(custom),
	)
	defer nlog.Shutdown(context.Background())
	// Output:
}

// ExampleClient_passing shows the recommended pattern: declare nlog once and
// pass it into services that need structured logging.
func ExampleClient_passing() {
	nlog := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer nlog.Shutdown(context.Background())

	svc := NewUserService(nlog)
	_ = svc.SignIn("u-123")
	// Output:
}

type UserService struct {
	nlog *nikologs.Client
}

func NewUserService(nlog *nikologs.Client) *UserService {
	return &UserService{nlog: nlog}
}

func (s *UserService) SignIn(userID string) error {
	s.nlog.Info("sign-in attempt", nikologs.Fields{"user_id": userID})
	s.nlog.Success("sign-in ok", nikologs.Fields{"user_id": userID})
	return nil
}
