package sdk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Base   string
	Key    string
	HTTP   *http.Client
	Header http.Header
}

func New(base, key string) *Client {
	return &Client{
		Base: strings.TrimRight(base, "/"),
		Key:  key,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	u := c.Base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, nil, err
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	for k, vs := range c.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (c *Client) Problems(ctx context.Context) (int, []byte, error) {
	return c.Get(ctx, "/v2/problems", nil)
}

func (c *Client) Vocabulary(ctx context.Context, name string) (int, []byte, error) {
	return c.Get(ctx, "/v2/vocabularies/"+url.PathEscape(name), nil)
}

func (c *Client) Work(ctx context.Context, id string) (int, []byte, error) {
	if id == "" {
		return 0, nil, fmt.Errorf("work id required")
	}
	return c.Get(ctx, "/v2/catalog/works/"+url.PathEscape(id), nil)
}
