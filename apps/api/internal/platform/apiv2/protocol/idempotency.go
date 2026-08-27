package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"api/internal/platform/apiv2/problem"

	"github.com/gofiber/fiber/v3"
)

const idempotencyTTL = 24 * time.Hour

type idempotencyRecord struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
	Hash        string `json:"hash"`
}

func idempotencyKey(c fiber.Ctx) string {
	return "v2:idem:" + c.IP() + ":" + c.Method() + ":" + c.Path() + ":" + c.Get("Idempotency-Key")
}

func bodyHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func replayPOST(store Store, c fiber.Ctx) (replayed bool, err error) {
	key := c.Get("Idempotency-Key")
	if key == "" || store == nil {
		return false, nil
	}
	raw, err := store.Get(c.Context(), idempotencyKey(c))
	if err != nil || len(raw) == 0 {
		return false, nil
	}
	var rec idempotencyRecord
	if json.Unmarshal(raw, &rec) != nil {
		return false, nil
	}
	h := bodyHash(c.Body())
	if h != rec.Hash {
		p := problem.New(problem.CodeIdempotencyKeyReused, problem.RequestID(c), problem.Instance(c),
			"Idempotency-Key was reused with a different request body.")
		return true, p
	}
	c.Set("Idempotency-Replayed", "true")
	if rec.ContentType != "" {
		c.Set("Content-Type", rec.ContentType)
	}
	c.Status(rec.Status)
	return true, c.Send(rec.Body)
}

func rememberPOST(store Store, c fiber.Ctx) {
	key := c.Get("Idempotency-Key")
	if key == "" || store == nil {
		return
	}
	status := c.Response().StatusCode()
	// 429 became reachable here when the limiter moved inside the chain
	// (RateLimit runs within c.Next()); remembering one would replay the
	// rate-limit refusal for 24h after the window reset.
	if status < 200 || status >= 500 || status == fiber.StatusTooManyRequests {
		return
	}
	rec := idempotencyRecord{
		Status:      status,
		ContentType: string(c.Response().Header.ContentType()),
		Body:        append([]byte(nil), c.Response().Body()...),
		Hash:        bodyHash(c.Body()),
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_ = store.Set(c.Context(), idempotencyKey(c), b, idempotencyTTL)
}
