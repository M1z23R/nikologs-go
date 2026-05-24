package nikologs

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultBaseURL       = "https://nikologs.dimitrije.dev"
	defaultFlushInterval = 5 * time.Second
	defaultBatchSize     = 100
)

// Client is a buffered client for the Nikologs log ingestion API.
type Client struct {
	baseURL       string
	apiKey        string
	uploadKey     string
	source        string
	defaultTags   []string
	flushInterval time.Duration
	batchSize     int
	httpClient    *http.Client
	onError       func(error)

	entries  chan *entry
	quit     chan struct{}
	done     chan struct{}
	shutdown bool
}

// Option configures the Client.
type Option func(*Client)

// New creates a new Client with the given API key and options.
// The client starts a background goroutine that flushes buffered logs.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:       defaultBaseURL,
		apiKey:        apiKey,
		flushInterval: defaultFlushInterval,
		batchSize:     defaultBatchSize,
		httpClient:    http.DefaultClient,
		onError:       func(error) {},
		quit:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	c.entries = make(chan *entry, c.batchSize*10)
	go c.flushLoop()
	return c
}

// Shutdown flushes remaining buffered logs and stops the background goroutine.
// It blocks until all pending logs are flushed or the context expires.
func (c *Client) Shutdown(ctx context.Context) error {
	c.shutdown = true
	close(c.quit)
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WithBaseURL sets the base URL for the Nikologs API.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithSource sets the default source field for all log entries.
func WithSource(source string) Option {
	return func(c *Client) {
		c.source = source
	}
}

// WithDefaultTags sets tags applied to every log entry. Per-call tags
// supplied via WithTags are appended on top. Server-side normalization
// lowercases, trims, and dedupes; the cap is 20 unique tags per entry.
func WithDefaultTags(tags ...string) Option {
	return func(c *Client) {
		c.defaultTags = append(c.defaultTags, tags...)
	}
}

// WithFlushInterval sets how often the buffer is flushed.
func WithFlushInterval(d time.Duration) Option {
	return func(c *Client) {
		c.flushInterval = d
	}
}

// WithBatchSize sets the maximum number of log entries per flush.
func WithBatchSize(size int) Option {
	return func(c *Client) {
		c.batchSize = size
	}
}

// WithHTTPClient sets a custom HTTP client for API calls.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithUploadKey sets the upload API key (nku_ prefix) for attachment uploads.
func WithUploadKey(key string) Option {
	return func(c *Client) {
		c.uploadKey = key
	}
}

// WithOnError sets a callback invoked when a flush or send fails after retries.
func WithOnError(fn func(error)) Option {
	return func(c *Client) {
		c.onError = fn
	}
}
