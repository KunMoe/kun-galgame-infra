package repr

type WorkPrices struct {
	_      struct{}     `json:"-" additionalProperties:"true"`
	Object string       `json:"object" enum:"work_prices" doc:"Type discriminant. Always work_prices."`
	WorkID string       `json:"work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog work id this row is about."`
	Quotes []PriceQuote `json:"quotes" doc:"One quote per storefront anchor and region. Empty array, never null."`
}

type PriceQuote struct {
	_               struct{} `json:"-" additionalProperties:"true"`
	Object          string   `json:"object" enum:"price_quote" doc:"Type discriminant. Always price_quote."`
	Source          string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	ExternalID      string   `json:"external_id" maxLength:"256" doc:"Verbatim upstream id. Must not be used as a discriminant beyond exact match. It is an identity anchor at this entity's own granularity, not necessarily an addressable page: a credit-name vndb ref is a staff-alias id with no page of its own. Browseable URLs come from links."`
	Region          string   `json:"region" pattern:"^[a-z]{2}$" maxLength:"2" doc:"ISO 3166-1 alpha-2 storefront region, lowercase. jp for DLsite."`
	QuoteState      string   `json:"quote_state" enum:"priced,pending,unavailable" doc:"priced has list and current; pending means a fetch is in flight; unavailable means the storefront has no price."`
	URL             string   `json:"url" format:"uri" maxLength:"1024" doc:"Absolute URL."`
	List            *Money   `json:"list" doc:"List price, tax-inclusive as the store shows it. null unless quote_state is priced."`
	Current         *Money   `json:"current" doc:"Current price, tax-inclusive as the store shows it. null unless quote_state is priced."`
	DiscountPercent int      `json:"discount_percent" minimum:"0" maximum:"100" doc:"Percent off list. Zero when not on sale or not priced."`
	SaleEndsAt      *string  `json:"sale_ends_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC instant the published sale ends. null when the store publishes no end."`
	Converted       []Money  `json:"converted" doc:"The storefront's own conversions of current, informational, not a second price. Empty array, never null."`
	FetchedAt       *string  `json:"fetched_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC instant this quote was observed. null when pending."`
	ExpiresAt       *string  `json:"expires_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC instant after which this quote is stale. null when pending."`
	Stale           bool     `json:"stale" doc:"true when served past expires_at; a refresh is in flight."`
}

type Money struct {
	_        struct{} `json:"-" additionalProperties:"true"`
	Currency string   `json:"currency" pattern:"^[A-Z]{3}$" maxLength:"3" doc:"ISO 4217 currency code."`
	Amount   string   `json:"amount" pattern:"^[0-9]{1,9}\\.[0-9]{2}$" maxLength:"12" doc:"Decimal string with two fraction digits, tax-inclusive as the store shows it."`
}
