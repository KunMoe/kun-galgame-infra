package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Cursor lane. Cursor exposes no chat/completions route — api.cursor.com is
// Admin/Analytics/Cloud-Agents only — so the one inference face is an agent
// run: POST /v1/agents, then poll the run until it reaches a terminal status
// and read its result text. Every run pays ~14k input tokens of fixed
// overhead, so the BATCH is the economic lever, not the concurrency: wave 210
// measured $0.00032 per item at batch 200 against $0.0008 at batch 50.
const cursorBase = "https://api.cursor.com"

type cursorClient struct {
	base    string
	token   string
	model   string
	effort  string
	http    *http.Client
	timeout time.Duration
	poll    time.Duration
}

func newCursorClient(token, model, effort string, timeout time.Duration) *cursorClient {
	return &cursorClient{
		base:    cursorBase,
		token:   token,
		model:   model,
		effort:  effort,
		http:    &http.Client{Timeout: 120 * time.Second},
		timeout: timeout,
		poll:    5 * time.Second,
	}
}

func (c *cursorClient) Configured() bool { return c.token != "" }

// runAgent creates one agent run and blocks until it reports a terminal status.
func (c *cursorClient) runAgent(ctx context.Context, prompt, label string) (string, error) {
	body := map[string]any{
		"prompt": map[string]any{"text": prompt},
		"model": map[string]any{
			"id": c.model,
			"params": []map[string]any{
				{"id": "effort", "value": c.effort},
				{"id": "fast", "value": "true"},
			},
		},
		"name": label,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	created, err := c.request(ctx, http.MethodPost, "/v1/agents", raw)
	if err != nil {
		return "", err
	}
	agentID, runID := cursorIDs(created)
	if agentID == "" || runID == "" {
		return "", fmt.Errorf("create agent: no run in reply: %s", truncate(string(created), 200))
	}
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-time.After(c.poll):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		data, err := c.request(ctx, http.MethodGet, "/v1/agents/"+agentID+"/runs/"+runID, nil)
		if err != nil {
			return "", err
		}
		var r struct {
			Run *struct {
				Status string `json:"status"`
				Result string `json:"result"`
			} `json:"run"`
			Status string `json:"status"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return "", fmt.Errorf("decode run: %w", err)
		}
		status, result := r.Status, r.Result
		if r.Run != nil {
			status, result = r.Run.Status, r.Run.Result
		}
		switch status {
		case "FINISHED":
			return result, nil
		case "ERROR", "CANCELLED", "EXPIRED":
			return "", fmt.Errorf("run %s: %s", status, truncate(result, 200))
		}
	}
	return "", fmt.Errorf("run %s exceeded %s", runID, c.timeout)
}

func cursorIDs(created []byte) (agentID, runID string) {
	var resp struct {
		ID    string `json:"id"`
		Agent *struct {
			ID string `json:"id"`
		} `json:"agent"`
		Run *struct {
			ID string `json:"id"`
		} `json:"run"`
	}
	if err := json.Unmarshal(created, &resp); err != nil {
		return "", ""
	}
	agentID = resp.ID
	if resp.Agent != nil && resp.Agent.ID != "" {
		agentID = resp.Agent.ID
	}
	if resp.Run != nil {
		runID = resp.Run.ID
	}
	return agentID, runID
}

func (c *cursorClient) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, status, err := c.requestOnce(ctx, method, path, body)
		if err == nil {
			return data, nil
		}
		retryable := status == 0 || status == http.StatusTooManyRequests ||
			status == http.StatusRequestTimeout || status >= http.StatusInternalServerError
		if !retryable || attempt >= len(retryBackoff) {
			return nil, err
		}
		select {
		case <-time.After(retryBackoff[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *cursorClient) requestOnce(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("cursor http %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	return data, resp.StatusCode, nil
}

// callBatch runs one batched prompt as an agent run. The agent face has no
// system role, so the two halves of the call travel as one prompt.
func (c *cursorClient) callBatch(ctx context.Context, b batchCall) (string, string, error) {
	text, err := c.runAgent(ctx, b.Rules+"\n\n"+b.Items, b.Label)
	return text, c.model, err
}

func (c *cursorClient) ExtractBatch(ctx context.Context, batch []candidateWork) []extraction {
	return extractBatchVia(ctx, c, batch, 0)
}

func (c *cursorClient) CompareBatch(ctx context.Context, batch []comparison) []comparisonResult {
	return compareBatchVia(ctx, c, batch, 0)
}
