package handler

import (
	"context"
	"net/http"

	"api/internal/platform/store/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

type purchaseLinksInput struct {
	ProductID string `path:"product_id" doc:"DLsite product number: RJ###### (doujin) or VJ###### (commercial), 6-8 digits"`
}

type purchaseLinksOutput struct {
	CacheControl string `header:"Cache-Control" doc:"Always 'private' — the links are keyed to the calling application and must never enter a shared cache"`
	Body         Envelope[service.PurchaseLinks]
}

type meStatsInput struct {
	From string `query:"from" doc:"First JST day to report, YYYY-MM-DD (inclusive). Defaults to 29 days before 'to'"`
	To   string `query:"to" doc:"Last JST day to report, YYYY-MM-DD (inclusive). Defaults to today in JST"`
}

type meStatsOutput struct {
	CacheControl string `header:"Cache-Control" doc:"Always 'private' — these are the calling application's own numbers"`
	Body         Envelope[service.MyStats]
}

// SetupStorePublicSpec describes the /v1/store face. The handlers are stubs:
// the face is served by the fiber handlers in this package, and this exists to
// export the contract (cmd/gen-openapi -store-public → docs/store/public-openapi.yaml).
func SetupStorePublicSpec(app *fiber.App) huma.API {
	InstallErrorEnvelope()

	cfg := huma.DefaultConfig("NextMoe Open API — Store", "1.0.0")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	api := humafiber.New(app, cfg)

	tags := []string{"store-public"}
	huma.Register(api, huma.Operation{
		OperationID: "getStorePurchaseLinks", Method: http.MethodGet, Path: "/v1/store/purchase-links/{product_id}",
		Summary: "Your site's own DLsite purchase link for one product, plus the coupon link when a campaign is running",
		Description: "Returns a short link that belongs to YOUR application, not a shared one: every calling site gets its " +
			"own alias for the same product so the clicks can be attributed back to it. Send readers to purchase_url " +
			"as-is — it is the only URL that gets counted, and a bare affiliate URL bypasses the counter entirely. " +
			"coupon_url and campaign are null whenever no coupon campaign is running. " +
			"Clicks are de-duplicated: one (link, JST calendar day, fingerprint) counts once, where the fingerprint is " +
			"SHA-256 of the client IP and User-Agent. That de-duplication is a commitment we made to DLsite, not a knob. " +
			"Links are minted lazily on first request and then stable forever, so calling this per page render is fine, " +
			"but caching the result on your side is cheaper. Each application may mint links for a bounded number of " +
			"distinct products; asking for more returns 403.",
		Tags: tags,
	}, func(context.Context, *purchaseLinksInput) (*purchaseLinksOutput, error) {
		return &purchaseLinksOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getStoreStatsMe", Method: http.MethodGet, Path: "/v1/store/me/stats",
		Summary: "Your application's own click counts, per link per JST day",
		Description: "Per-day totals and de-duplicated uniques for every link this application has minted, purchase and " +
			"coupon alike — the kind field tells them apart, and product_id / campaign_id say which link a row belongs " +
			"to. Days are JST calendar days because the settlement month is DLsite's JST calendar month. The range is a " +
			"closed interval of at most 92 days and defaults to the last 30. Days with no clicks are omitted entirely. " +
			"uniques is the number settlement uses; total is the raw click count before de-duplication. The counts are " +
			"synchronised from the redirector hourly, so the current day is always partial.",
		Tags: tags,
	}, func(context.Context, *meStatsInput) (*meStatsOutput, error) { return &meStatsOutput{}, nil })

	return api
}
