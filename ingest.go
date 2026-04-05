package nikologs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	ingestPath     = "/api/v1/ingest/logs"
	attachmentPath = "/api/v1/ingest/attachments"
)

// sendBatch sends a batch of entries to the ingest API.
func (c *Client) sendBatch(entries []*entry) (*IngestResponse, error) {
	logs := make([]map[string]any, len(entries))
	for i, e := range entries {
		logs[i] = e.toPayload()
	}
	body, err := json.Marshal(map[string]any{"logs": logs})
	if err != nil {
		return nil, fmt.Errorf("nikologs: marshal error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+ingestPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("nikologs: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nikologs: send error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nikologs: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result IngestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("nikologs: decode error: %w", err)
	}
	return &result, nil
}

// UploadAttachment uploads a file to the attachment API.
// It requires an upload key (nku_ prefix) to be configured via WithUploadKey.
func (c *Client) UploadAttachment(ctx context.Context, file io.Reader, filename string) (*AttachmentResponse, error) {
	if c.uploadKey == "" {
		return nil, ErrNoUploadKey
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("nikologs: multipart error: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("nikologs: copy error: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+attachmentPath, &buf)
	if err != nil {
		return nil, fmt.Errorf("nikologs: request error: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.uploadKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nikologs: upload error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nikologs: upload status %d: %s", resp.StatusCode, string(respBody))
	}

	var result AttachmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("nikologs: decode error: %w", err)
	}
	return &result, nil
}
