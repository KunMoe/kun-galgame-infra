package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ScanGateway interface {
	Configured() bool
	Moderate(ctx context.Context, text, subjectKind string, authorID *int64) (GatewayVerdict, error)
}

type GatewayVerdict struct {
	Flagged    bool
	Score      *float32
	Categories []string
	Channel    string
	Degraded   bool
}

const gatewayTimeout = 120 * time.Second

type AIGatewayClient struct {
	baseURL  string
	clientID string
	secret   string
	http     *http.Client
}

func NewAIGatewayClient(baseURL, clientID, secret string) *AIGatewayClient {
	return &AIGatewayClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		clientID: clientID,
		secret:   secret,
		http:     &http.Client{Timeout: gatewayTimeout},
	}
}

func (c *AIGatewayClient) Configured() bool {
	return c.baseURL != "" && c.clientID != "" && c.secret != ""
}

type moderateReq struct {
	Text        string `json:"text"`
	SubjectKind string `json:"subject_kind,omitempty"`
	AuthorID    *int64 `json:"author_id,omitempty"`
}

type moderateEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Route      string   `json:"route"`
		Flagged    bool     `json:"flagged"`
		Categories []string `json:"categories"`
		Score      *float32 `json:"score"`
		Channel    string   `json:"channel"`
		Degraded   bool     `json:"degraded"`
	} `json:"data"`
}

func (c *AIGatewayClient) Moderate(ctx context.Context, text, subjectKind string, authorID *int64) (GatewayVerdict, error) {
	raw, err := json.Marshal(moderateReq{Text: text, SubjectKind: subjectKind, AuthorID: authorID})
	if err != nil {
		return GatewayVerdict{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ai/moderate-text", bytes.NewReader(raw))
	if err != nil {
		return GatewayVerdict{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+basicCred(c.clientID, c.secret))

	resp, err := c.http.Do(req)
	if err != nil {
		return GatewayVerdict{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return GatewayVerdict{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return GatewayVerdict{}, fmt.Errorf("ai gateway http %d", resp.StatusCode)
	}
	var env moderateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return GatewayVerdict{}, fmt.Errorf("decode moderate-text response: %w", err)
	}
	if env.Code != 0 {
		return GatewayVerdict{}, fmt.Errorf("ai gateway code %d: %s", env.Code, env.Message)
	}
	return GatewayVerdict{
		Flagged:    env.Data.Flagged,
		Score:      env.Data.Score,
		Categories: env.Data.Categories,
		Channel:    env.Data.Channel,
		Degraded:   env.Data.Degraded,
	}, nil
}

func basicCred(id, secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}
