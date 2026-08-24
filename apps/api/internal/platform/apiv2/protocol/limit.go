package protocol

import (
	"context"
	"strconv"
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

type limiter struct {
	store Store
}

func newLimiter(store Store) *limiter {
	if store == nil {
		store = NewMemory()
	}
	return &limiter{store: store}
}

func (l *limiter) before(c fiber.Ctx) error {
	id := c.IP()
	if id == "" {
		id = "unknown"
	}
	now := time.Now().UTC()
	minute := now.Unix() / 60
	rateKey := "v2:rate:" + id + ":" + strconv.FormatInt(minute, 10)
	quotaKey := "v2:quota:" + id + ":" + now.Format("2006-01-02")
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

	rateRemaining := RatePerMinute - int(n)
	if rateRemaining < 0 {
		rateRemaining = 0
	}
	quotaRemaining := QuotaPerDay - int(qn)
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

	writeLimitHeaders(c, rateRemaining, rateReset, quotaRemaining, quotaReset)

	if int(n) > RatePerMinute {
		c.Set("Retry-After", strconv.Itoa(rateReset))
		return tooMany(c, problem.CodeRateLimited, rateReset)
	}
	if int(qn) > QuotaPerDay {
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

func writeLimitHeaders(c fiber.Ctx, rateRemaining, rateReset, quotaRemaining, quotaReset int) {
	c.Set("RateLimit", "limit="+strconv.Itoa(RatePerMinute)+", remaining="+strconv.Itoa(rateRemaining)+", reset="+strconv.Itoa(rateReset))
	c.Set("RateLimit-Policy", "100;w=60, 10000;w=86400")
	c.Set("X-RateLimit-Limit", strconv.Itoa(RatePerMinute))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(rateRemaining))
	c.Set("X-RateLimit-Reset", strconv.Itoa(rateReset))
	_ = quotaRemaining
	_ = quotaReset
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
