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

const steamDefaultBase = "https://store.steampowered.com"

var steamIDRe = regexp.MustCompile(`^[0-9]{1,12}$`)

type steam struct {
	ua      string
	regions []string
	base    string
	http    *http.Client
}

func NewSteam(userAgent string, regions []string, base string) Fetcher {
	if base == "" {
		base = steamDefaultBase
	}
	out := make([]string, 0, len(regions))
	for _, r := range regions {
		r = strings.ToLower(strings.TrimSpace(r))
		if r != "" {
			out = append(out, r)
		}
	}
	return &steam{
		ua:      userAgent,
		regions: out,
		base:    strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *steam) Source() string         { return "steam" }
func (s *steam) Regions() []string      { return append([]string{}, s.regions...) }
func (s *steam) Batch() int             { return 40 }
func (s *steam) Gap() time.Duration     { return 1500 * time.Millisecond }
func (s *steam) Accepts(id string) bool { return steamIDRe.MatchString(id) }

func (s *steam) URL(externalID string) string {
	return "https://store.steampowered.com/app/" + externalID + "/"
}

func (s *steam) Fetch(ctx context.Context, region string, ids []string) (map[string]Upstream, error) {
	if len(ids) == 0 {
		return map[string]Upstream{}, nil
	}
	u, err := url.Parse(s.base + "/api/appdetails")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("appids", strings.Join(ids, ","))
	q.Set("cc", region)
	q.Set("filters", "price_overview")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.ua)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("steam: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steam: status %d", resp.StatusCode)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("steam: decode: %w", err)
	}
	out := make(map[string]Upstream, len(members))
	for id, body := range members {
		up := parseSteamEntry(body)
		up.URL = s.URL(id)
		out[id] = up
	}
	return out, nil
}

type steamEntry struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

type steamPriceData struct {
	PriceOverview *steamPriceOverview `json:"price_overview"`
}

type steamPriceOverview struct {
	Currency        string `json:"currency"`
	Initial         int64  `json:"initial"`
	Final           int64  `json:"final"`
	DiscountPercent int    `json:"discount_percent"`
}

func parseSteamEntry(body json.RawMessage) Upstream {
	var e steamEntry
	if err := json.Unmarshal(body, &e); err != nil {
		return Upstream{Found: false}
	}
	if !e.Success {
		return Upstream{Found: false}
	}
	data := bytesTrim(e.Data)
	if len(data) == 0 || data[0] == '[' {
		return Upstream{Found: false}
	}
	var parsed steamPriceData
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.PriceOverview == nil {
		return Upstream{Found: false}
	}
	po := parsed.PriceOverview
	return Upstream{
		Found:           true,
		Currency:        po.Currency,
		ListMinor:       po.Initial,
		CurrentMinor:    po.Final,
		DiscountPercent: po.DiscountPercent,
	}
}
