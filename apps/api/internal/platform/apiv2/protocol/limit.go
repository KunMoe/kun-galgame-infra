package protocol

import (
	"context"
	"strconv"
	"strings"
	"time"

	"api/internal/platform/apiv2/problem"

	"github.com/gofiber/fiber/v3"
)

const (
	RatePerMinute = 100
	QuotaPerDay   = 10000
)

const (
	localsRateKey  = "v2_rate_key"
	localsQuotaKey = "v2_quota_key"
	localsCounted  = "v2_counted"
)

type LimitIdentity struct {
	Key       string
	Rate      int
	Quota     int
	Unlimited bool
}

type IdentityFunc func(c fiber.Ctx) (LimitIdentity, bool)

// RateLimit must be registered AFTER the auth middlewares and INSIDE
// Middleware: the counting decision needs the resolved credential, which does
// not exist yet when Middleware runs, while the 304-decrement must happen
// after applyETag rewrites the status, which only Middleware sees. The two
// halves meet through the locals keys.
func RateLimit(store Store, ident IdentityFunc) fiber.Handler {
	lim := newLimiter(store)
	return func(c fiber.Ctx) error {
		if !strings.HasPrefix(c.Path(), "/v2") {
			return c.Next()
		}
		id := LimitIdentity{Rate: RatePerMinute, Quota: QuotaPerDay}
		if ident != nil {
			if ki, ok := ident(c); ok {
				id = ki
			}
		}
		if id.Unlimited {
			return c.Next()
		}
		if id.Key == "" {
			id.Key = c.IP()
			if id.Key == "" {
				id.Key = "unknown"
			}
		}
		if err := lim.before(c, id); err != nil {
			return writeErr(c, err)
		}
		return c.Next()
	}
}

type limiter struct {
	store Store
}

func newLimiter(store Store) *limiter {
	if store == nil {
		store = NewMemory()
	}
	return &limiter{store: store}
}

func (l *limiter) before(c fiber.Ctx, id LimitIdentity) error {
	now := time.Now().UTC()
	minute := now.Unix() / 60
	rateKey := "v2:rate:" + id.Key + ":" + strconv.FormatInt(minute, 10)
	quotaKey := "v2:quota:" + id.Key + ":" + now.Format("2006-01-02")
	ctx := c.Context()

	n, err := l.store.Incr(ctx, rateKey, 70*time.Second)
	if err != nil {
		return nil
	}
	qn, err := l.store.Incr(ctx, quotaKey, untilTomorrow(now))
	if err != nil {
		_ = l.store.Decr(ctx, rateKey)
		return nil
	}
	c.Locals(localsRateKey, rateKey)
	c.Locals(localsQuotaKey, quotaKey)
	c.Locals(localsCounted, true)

	rateRemaining := id.Rate - int(n)
	if rateRemaining < 0 {
		rateRemaining = 0
	}
	quotaRemaining := id.Quota - int(qn)
	if quotaRemaining < 0 {
		quotaRemaining = 0
	}
	rateReset := int((minute+1)*60 - now.Unix())
	if rateReset < 1 {
		rateReset = 1
	}
	quotaReset := int(untilTomorrow(now) / time.Second)
	if quotaReset < 1 {
		quotaReset = 1
	}

	writeLimitHeaders(c, id, rateRemaining, rateReset)

	if int(n) > id.Rate {
		c.Set("Retry-After", strconv.Itoa(rateReset))
		return tooMany(c, problem.CodeRateLimited, rateReset)
	}
	if int(qn) > id.Quota {
		c.Set("Retry-After", strconv.Itoa(quotaReset))
		return tooMany(c, problem.CodeQuotaExceeded, quotaReset)
	}
	return nil
}

func (l *limiter) after(c fiber.Ctx) {
	if c.Response().StatusCode() != fiber.StatusNotModified {
		return
	}
	counted, _ := c.Locals(localsCounted).(bool)
	if !counted {
		return
	}
	ctx := context.Background()
	if k, _ := c.Locals(localsRateKey).(string); k != "" {
		_ = l.store.Decr(ctx, k)
	}
	if k, _ := c.Locals(localsQuotaKey).(string); k != "" {
		_ = l.store.Decr(ctx, k)
	}
}

func writeLimitHeaders(c fiber.Ctx, id LimitIdentity, rateRemaining, rateReset int) {
	c.Set("RateLimit", "limit="+strconv.Itoa(id.Rate)+", remaining="+strconv.Itoa(rateRemaining)+", reset="+strconv.Itoa(rateReset))
	c.Set("RateLimit-Policy", strconv.Itoa(id.Rate)+";w=60, "+strconv.Itoa(id.Quota)+";w=86400")
	c.Set("X-RateLimit-Limit", strconv.Itoa(id.Rate))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(rateRemaining))
	c.Set("X-RateLimit-Reset", strconv.Itoa(rateReset))
}

func untilTomorrow(now time.Time) time.Duration {
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return tomorrow.Sub(now)
}

func tooMany(c fiber.Ctx, code string, retryAfter int) error {
	p := problem.New(code, problem.RequestID(c), problem.Instance(c), "Rate or quota exceeded.")
	if code == problem.CodeQuotaExceeded {
		p.Detail = "Daily quota exceeded."
	} else {
		p.Detail = "Short-window rate limit exceeded."
	}
	_ = retryAfter
	return p
}
