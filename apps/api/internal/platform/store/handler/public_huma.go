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

	return api
}
