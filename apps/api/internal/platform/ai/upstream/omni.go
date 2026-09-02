package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api/internal/platform/settings/keys"
)

type OmniClient struct {
	baseURL string
	token   string
	model   string
	http    *http.Client
}

func NewOmniClient(baseURL, token, model string) *OmniClient {
	return &OmniClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		model:   model,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *OmniClient) Configured() bool {
	return c.baseURL != "" && c.token != ""
}

func (c *OmniClient) Model() string { return c.resolveModel() }

func (c *OmniClient) resolveModel() string {
	if c.model != "" {
		return c.model
	}
	return keys.AIOmniModel.Get()
}

type OmniResult struct {
	Flagged        bool
	Categories     map[string]bool
	CategoryScores map[string]float64
	Channel        string
}

type omniRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type omniResponse struct {
	Model   string `json:"model"`
	Results []struct {
		Flagged        bool               `json:"flagged"`
		Categories     map[string]bool    `json:"categories"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *OmniClient) Moderate(ctx context.Context, input string) (OmniResult, error) {
	raw, err := json.Marshal(omniRequest{Model: c.resolveModel(), Input: input})
	if err != nil {
		return OmniResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/moderations", bytes.NewReader(raw))
	if err != nil {
		return OmniResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return OmniResult{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return OmniResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return OmniResult{}, fmt.Errorf("omni http %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	var or omniResponse
	if err := json.Unmarshal(data, &or); err != nil {
		return OmniResult{}, fmt.Errorf("decode omni response: %w (body: %s)", err, truncate(string(data), 300))
	}
	if or.Error != nil {
		return OmniResult{}, fmt.Errorf("omni error: %s", or.Error.Message)
	}
	if len(or.Results) == 0 {
		return OmniResult{}, fmt.Errorf("omni returned no results")
	}
	channel := or.Model
	if channel == "" {
		channel = c.resolveModel()
	}
	return OmniResult{
		Flagged:        or.Results[0].Flagged,
		Categories:     or.Results[0].Categories,
		CategoryScores: or.Results[0].CategoryScores,
		Channel:        channel,
	}, nil
}
