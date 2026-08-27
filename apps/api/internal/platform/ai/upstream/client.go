package upstream

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
)

const llmTimeout = 90 * time.Second

type Client struct {
	baseURL string
	token   string
	model   string
	http    *http.Client
}

func NewClient(baseURL, token, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		model:   model,
		http:    &http.Client{Timeout: llmTimeout},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.token != ""
}

func (c *Client) Model() string { return c.model }

// StatusError carries the upstream HTTP status so a caller can tell transient
// contention from a permanent failure. Cloudflare delivers its per-minute
// inference limit as a real 429 with body code 3021 — not as a 200 carrying an
// error object, and not as the 401 it returns when merely overloaded — so
// matching on the status is enough. The Error text is unchanged from the
// fmt.Errorf it replaced; log lines and their greps still match.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("upstream http %d: %s", e.Code, e.Body)
}

func IsRateLimited(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == http.StatusTooManyRequests
}

type ChatResult struct {
	Content          string
	Channel          string
	FinishReason     string
	PromptTokens     int
	CompletionTokens int
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	MaxTokens      int            `json:"max_tokens"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) ChatJSON(ctx context.Context, system, user string, maxTokens int) (ChatResult, error) {
	body := chatRequest{
		Model:       c.model,
		MaxTokens:   maxTokens,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return ChatResult{}, &StatusError{Code: resp.StatusCode, Body: truncate(string(data), 300)}
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return ChatResult{}, fmt.Errorf("decode chat response: %w (body: %s)", err, truncate(string(data), 300))
	}
	if cr.Error != nil {
		return ChatResult{}, fmt.Errorf("upstream error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("upstream returned no choices")
	}
	channel := cr.Model
	if channel == "" {
		channel = c.model
	}
	return ChatResult{
		Content:          strings.TrimSpace(cr.Choices[0].Message.Content),
		Channel:          channel,
		FinishReason:     cr.Choices[0].FinishReason,
		PromptTokens:     cr.Usage.PromptTokens,
		CompletionTokens: cr.Usage.CompletionTokens,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
