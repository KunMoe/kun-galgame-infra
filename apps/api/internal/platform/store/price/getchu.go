package price

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	skeys "api/internal/platform/settings/keys"

	"golang.org/x/text/encoding/japanese"
)

const getchuDefaultBase = "https://www.getchu.com"

var (
	getchuIDRe      = regexp.MustCompile(`^[0-9]{1,12}$`)
	getchuCurrentRe = regexp.MustCompile(`価格（税込）[^￥]{0,60}￥([0-9,]+)`)
	getchuListRe    = regexp.MustCompile(`定価：[^￥]{0,200}￥([0-9,]+)\s*\(税込￥([0-9,]+)\)`)
)

type getchu struct {
	base string
	http *http.Client
}

func NewGetchu(base string) Fetcher {
	if base == "" {
		base = getchuDefaultBase
	}
	return &getchu{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{
			Timeout: 10 * time.Second,
			// A cookieless request is answered 302 → php/attestation.html (the
			// age gate), whose page parses as "no price": following redirects
			// would write every quote unavailable with nothing in the logs.
			// Surface any redirect as a fetch error instead.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (g *getchu) Source() string         { return "getchu" }
func (g *getchu) Regions() []string      { return []string{"jp"} }
func (g *getchu) Batch() int             { return 1 }
func (g *getchu) Gap() time.Duration     { return 2 * time.Second }
func (g *getchu) Accepts(id string) bool { return getchuIDRe.MatchString(id) }

func (g *getchu) URL(externalID string) string {
	return "https://www.getchu.com/item/" + externalID + "/"
}

func (g *getchu) Fetch(ctx context.Context, region string, ids []string) (map[string]Upstream, error) {
	_ = region
	out := make(map[string]Upstream, len(ids))
	for _, id := range ids {
		up, err := g.fetchOne(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = up
	}
	return out, nil
}

func (g *getchu) fetchOne(ctx context.Context, id string) (Upstream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+"/item/"+id+"/", nil)
	if err != nil {
		return Upstream{}, err
	}
	req.Header.Set("Cookie", "getchu_adalt_flag=getchu.com")
	req.Header.Set("User-Agent", skeys.StorePriceUserAgent.Get())
	resp, err := g.http.Do(req)
	if err != nil {
		return Upstream{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(japanese.EUCJP.NewDecoder().Reader(io.LimitReader(resp.Body, 1<<20)))
	if err != nil {
		return Upstream{}, fmt.Errorf("getchu: read body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return Upstream{Found: false, URL: g.URL(id)}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Upstream{}, fmt.Errorf("getchu: status %d", resp.StatusCode)
	}
	return g.parse(id, string(raw))
}

func (g *getchu) parse(id, page string) (Upstream, error) {
	cur := getchuCurrentRe.FindStringSubmatch(page)
	list := getchuListRe.FindStringSubmatch(page)
	if cur == nil {
		if list != nil || strings.Contains(page, "ご注文の受付は停止中です") {
			return Upstream{Found: false, URL: g.URL(id)}, nil
		}
		return Upstream{}, fmt.Errorf("getchu: no price markup on item page %s", id)
	}
	current, err := getchuYenMinor(cur[1])
	if err != nil {
		return Upstream{}, fmt.Errorf("getchu: item %s: %w", id, err)
	}
	listMinor := current
	if list != nil {
		listMinor, err = getchuYenMinor(list[2])
		if err != nil {
			return Upstream{}, fmt.Errorf("getchu: item %s: %w", id, err)
		}
	}
	up := Upstream{
		Found:        true,
		URL:          g.URL(id),
		Currency:     "JPY",
		ListMinor:    listMinor,
		CurrentMinor: current,
	}
	if listMinor > current {
		up.DiscountPercent = int(((listMinor-current)*100 + listMinor/2) / listMinor)
	}
	return up, nil
}

func getchuYenMinor(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.ReplaceAll(s, ",", ""), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse yen %q: %w", s, err)
	}
	return n * 100, nil
}
