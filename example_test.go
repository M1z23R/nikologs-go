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
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
		nikologs.WithFlushInterval(3*time.Second),
		nikologs.WithBatchSize(200),
	)
	defer client.Shutdown(context.Background())

	client.Info("service started", nikologs.Fields{"version": "1.0.0"})
	// Output:
}

func ExampleClient_Info() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer client.Shutdown(context.Background())

	client.Info("user signed in", nikologs.Fields{
		"user_id": "u-123",
		"method":  "oauth",
	})
	// Output:
}

func ExampleClient_Log() {
	client := nikologs.New("nk_your_api_key")
	defer client.Shutdown(context.Background())

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	client.Log(nikologs.LevelWarn, "disk usage high", nikologs.Fields{
		"percent": 92,
	}, nikologs.WithTimestamp(ts))
	// Output:
}

func ExampleClient_UploadAttachment() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithUploadKey("nku_your_upload_key"),
	)
	defer client.Shutdown(context.Background())

	file := strings.NewReader(`{"error": "stack trace here"}`)
	resp, err := client.UploadAttachment(context.Background(), file, "error.json")
	if err != nil {
		// handle upload error
		_ = err
		return
	}

	client.Error("crash report", nil, nikologs.WithFileID(resp.ID))
	// Output:
}

func ExampleNewSlogHandler() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithSource("my-service"),
	)
	defer client.Shutdown(context.Background())

	handler := nikologs.NewSlogHandler(client, &nikologs.SlogHandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	logger.Info("request handled", "method", "GET", "status", 200)
	// Output:
}

func ExampleWithOnError() {
	client := nikologs.New("nk_your_api_key",
		nikologs.WithOnError(func(err error) {
			fmt.Println("nikologs error:", err)
		}),
	)
	defer client.Shutdown(context.Background())
	// Output:
}

func ExampleWithHTTPClient() {
	custom := &http.Client{Timeout: 30 * time.Second}
	client := nikologs.New("nk_your_api_key",
		nikologs.WithHTTPClient(custom),
	)
	defer client.Shutdown(context.Background())
	// Output:
}
