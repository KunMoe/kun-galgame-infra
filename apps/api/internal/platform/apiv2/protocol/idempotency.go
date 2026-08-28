package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"api/internal/platform/apiv2/problem"
	"api/pkg/routepath"

	"github.com/gofiber/fiber/v3"
)

const idempotencyTTL = 24 * time.Hour

type idempotencyRecord struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
	Hash        string `json:"hash"`
}

// Idempotency must be registered AFTER the auth middlewares. The key was
// c.IP() + method + path + Idempotency-Key while this lived in Middleware
// (pre-auth), and a first-party backend relays every one of its users from one
// egress IP — so two different users sending the same Idempotency-Key on the
// same path replayed each other's writes.
func Idempotency(store Store, ident IdentityFunc) fiber.Handler {
	return func(c fiber.Ctx) error {
		if store == nil || c.Method() != fiber.MethodPost || !strings.HasPrefix(routepath.Normalize(c.Path()), "/v2") {
			return c.Next()
		}
		if c.Get("Idempotency-Key") == "" {
			return c.Next()
		}
		key := idempotencyKey(c, ident)
		if replayed, err := replayPOST(store, c, key); replayed {
			if err != nil {
				return writeErr(c, err)
			}
			return nil
		}
		err := c.Next()
		rememberPOST(store, c, key)
		return err
	}
}

func idempotencyKey(c fiber.Ctx, ident IdentityFunc) string {
	who := "ip:" + clientIP(c)
	if ident != nil {
		if id, ok := ident(c); ok && id.Key != "" {
			who = id.Key
		}
	}
	return "v2:idem:" + who + ":" + c.Method() + ":" + routepath.Normalize(c.Path()) + ":" + c.Get("Idempotency-Key")
}

func bodyHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func replayPOST(store Store, c fiber.Ctx, key string) (replayed bool, err error) {
	raw, err := store.Get(c.Context(), key)
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

func rememberPOST(store Store, c fiber.Ctx, key string) {
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
	_ = store.Set(c.Context(), key, b, idempotencyTTL)
}
