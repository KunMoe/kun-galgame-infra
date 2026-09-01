package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api/pkg/config"
)

const apiErrorBodyCap = 2048

type Client struct {
	host   string
	prefix string
	http   *http.Client
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("opensearch HTTP %d", e.Status)
	}
	return fmt.Sprintf("opensearch HTTP %d: %s", e.Status, e.Body)
}

func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

func NewClient(cfg config.OpenSearchConfig) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("opensearch host is required")
	}
	return &Client{
		host:   strings.TrimRight(cfg.Host, "/"),
		prefix: cfg.IndexPrefix,
		http:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *Client) IndexName(uid string) string {
	return c.prefix + uid
}

func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	_, err := c.do(ctx, method, path, "application/json", body, out)
	return err
}

func (c *Client) Health(ctx context.Context) error {
	return c.Do(ctx, http.MethodGet, "/_cluster/health", nil, nil)
}

func (c *Client) Plugins(ctx context.Context) ([]string, error) {
	var rows []map[string]any
	if err := c.Do(ctx, http.MethodGet, "/_cat/plugins?format=json", nil, &rows); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		component, _ := row["component"].(string)
		if component != "" {
			names = append(names, component)
		}
	}
	return names, nil
}

func (c *Client) BulkNDJSON(ctx context.Context, payload []byte) error {
	if len(payload) > 0 && payload[len(payload)-1] != '\n' {
		payload = append(payload, '\n')
	}
	var resp struct {
		Errors bool                         `json:"errors"`
		Items  []map[string]json.RawMessage `json:"items"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/_bulk", "application/x-ndjson", payload, &resp); err != nil {
		return err
	}
	if !resp.Errors {
		return nil
	}
	for i, item := range resp.Items {
		for action, raw := range item {
			var meta struct {
				Error json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(raw, &meta); err != nil {
				continue
			}
			if len(meta.Error) == 0 || string(meta.Error) == "null" {
				continue
			}
			return fmt.Errorf("opensearch bulk item %d %s: %s", i, action, capBody(meta.Error))
		}
	}
	return fmt.Errorf("opensearch bulk reported errors but no item error was present")
}

func (c *Client) do(ctx context.Context, method, path, contentType string, body, out any) (int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rdr = bytes.NewReader(b)
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(body); err != nil {
			return 0, fmt.Errorf("opensearch encode %s %s: %w", method, path, err)
		}
		rdr = &buf
	}

	req, err := http.NewRequestWithContext(ctx, method, c.host+path, rdr)
	if err != nil {
		return 0, err
	}
	if rdr != nil {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("opensearch %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("opensearch read %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, &APIError{Status: resp.StatusCode, Body: capBody(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("opensearch decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

func capBody(b []byte) string {
	if len(b) > apiErrorBodyCap {
		return string(b[:apiErrorBodyCap])
	}
	return string(b)
}
