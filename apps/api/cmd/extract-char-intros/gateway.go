package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Cloudflare answers 524 when the origin passes its 100s cap. net/http has no
// name for it because it is not an IANA status.
const statusCloudflareTimeout = 524

// The ladder has to outlast ONE call, not one hiccup. A gateway that queues
// answers 429 for as long as the call ahead is still running, and a batched
// extraction runs ~100s: the old 5/10/20/40/60 ladder spent all five retries
// inside a single legitimate wait and then failed, which cost the 2026-08-22
// panel run 20% of its characters. Splitting cannot rescue this — the batch was
// never too big — so patience is the only fix.
var retryBackoff = []time.Duration{
	5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
	60 * time.Second, 60 * time.Second, 60 * time.Second, 60 * time.Second,
	60 * time.Second, 60 * time.Second,
}

// postChat retries throttled and transient gateway failures with backoff. Without
// this a 429 turns the pacing from inference latency (~20s) into delay-only fast
// failures, and parallel shards then hammer the gateway at thousands of requests
// per minute exactly when it asked to slow down (08-14: 16 shards burned 7.7k
// 429s in two minutes).
func (t *httpExtractor) postChat(ctx context.Context, raw []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, status, err := t.postOnce(ctx, raw)
		if err == nil {
			return data, nil
		}
		retryable := status == 0 || status == http.StatusTooManyRequests ||
			status == http.StatusRequestTimeout || status >= http.StatusInternalServerError
		if !retryable {
			return nil, err
		}
		// Cloudflare's 524 is a duration cap, not a queue signal: the origin went
		// past 100s, and the same call goes past it again. Waiting is the wrong
		// answer here even though waiting is right for the 429 below. The
		// 2026-08-23 panel run spent 42 minutes emptying the ladder into one
		// 25-work batch and then got both halves on the first try.
		if status == statusCloudflareTimeout {
			return nil, fmt.Errorf("%w: %v", errBatchOversize, err)
		}
		// Out of retries on a failure a SMALLER call might survive, so hand it to
		// the split retry rather than failing the whole batch.
		if attempt >= len(retryBackoff) {
			return nil, fmt.Errorf("%w: %v", errBatchOversize, err)
		}
		select {
		case <-time.After(retryBackoff[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (t *httpExtractor) postOnce(ctx context.Context, raw []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.token)
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("gateway http %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	return data, resp.StatusCode, nil
}
