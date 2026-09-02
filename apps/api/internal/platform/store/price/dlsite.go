package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const dlsiteDefaultBase = "https://www.dlsite.com"

var dlsiteIDRe = regexp.MustCompile(`^(RJ|VJ)[0-9]{6,8}$`)

type dlsite struct {
	ua         string
	currencies map[string]struct{}
	base       string
	http       *http.Client
	jst        *time.Location
}

// proxy is the lane's egress. DLsite answers the prod box's German egress with
// `302 → https://www.google.com/` on every path, so the first deploy decoded
// Google's HTML and every quote came back unavailable; the same request from a
// Tokyo host returns the JSON.
func NewDLsite(userAgent string, currencies []string, base string, proxy *url.URL) Fetcher {
	if base == "" {
		base = dlsiteDefaultBase
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if proxy != nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.Proxy = http.ProxyURL(proxy)
		client.Transport = tr
	}
	want := make(map[string]struct{}, len(currencies))
	for _, c := range currencies {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" {
			want[c] = struct{}{}
		}
	}
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.FixedZone("JST", 9*3600)
	}
	return &dlsite{
		ua:         userAgent,
		currencies: want,
		base:       strings.TrimRight(base, "/"),
		http:       client,
		jst:        loc,
	}
}

func (d *dlsite) Source() string         { return "dlsite" }
func (d *dlsite) Regions() []string      { return []string{"jp"} }
func (d *dlsite) Batch() int             { return 100 }
func (d *dlsite) Gap() time.Duration     { return time.Second }
func (d *dlsite) Accepts(id string) bool { return dlsiteIDRe.MatchString(id) }

func (d *dlsite) URL(externalID string) string {
	floor := "maniax"
	if strings.HasPrefix(externalID, "VJ") {
		floor = "pro"
	}
	return "https://www.dlsite.com/" + floor + "/work/=/product_id/" + externalID + ".html"
}

func (d *dlsite) Fetch(ctx context.Context, region string, ids []string) (map[string]Upstream, error) {
	_ = region
	if len(ids) == 0 {
		return map[string]Upstream{}, nil
	}
	u, err := url.Parse(d.base + "/maniax/product/info/ajax")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("product_id", strings.Join(ids, ","))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "adultchecked=1")
	req.Header.Set("User-Agent", d.ua)
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("dlsite: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dlsite: status %d", resp.StatusCode)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("dlsite: decode: %w", err)
	}
	out := make(map[string]Upstream, len(members))
	for id, body := range members {
		up, ok := d.parseMember(id, body)
		if !ok {
			continue
		}
		out[id] = up
	}
	return out, nil
}

type dlsiteMember struct {
	Price         int64              `json:"price"`
	OfficialPrice int64              `json:"official_price"`
	DiscountRate  int                `json:"discount_rate"`
	OnSale        int                `json:"on_sale"`
	CampaignEnd   json.RawMessage    `json:"campaign_end_date"`
	TimesaleEnd   json.RawMessage    `json:"timesale_end"`
	CurrencyPrice map[string]float64 `json:"currency_price"`
}

func (d *dlsite) parseMember(id string, body json.RawMessage) (Upstream, bool) {
	var m dlsiteMember
	if err := json.Unmarshal(body, &m); err != nil {
		return Upstream{}, false
	}
	if m.OnSale != 1 {
		return Upstream{Found: false, URL: d.URL(id)}, true
	}
	converted := map[string]int64{}
	for cur, amt := range m.CurrencyPrice {
		u := strings.ToUpper(cur)
		if _, ok := d.currencies[u]; !ok {
			continue
		}
		if amt <= 0 {
			continue
		}
		converted[u] = Minor(amt)
	}
	return Upstream{
		Found:           true,
		URL:             d.URL(id),
		Currency:        "JPY",
		ListMinor:       m.OfficialPrice * 100,
		CurrentMinor:    m.Price * 100,
		DiscountPercent: m.DiscountRate,
		SaleEndsAt:      earlierTime(d.parseJST(m.CampaignEnd), d.parseJST(m.TimesaleEnd)),
		Converted:       converted,
	}, true
}

func (d *dlsite) parseJST(raw json.RawMessage) *time.Time {
	s := jsonString(raw)
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		t, err := time.ParseInLocation(layout, s, d.jst)
		if err == nil {
			return &t
		}
	}
	return nil
}

func jsonString(raw json.RawMessage) string {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func earlierTime(a, b *time.Time) *time.Time {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case a.Before(*b):
		return a
	default:
		return b
	}
}
