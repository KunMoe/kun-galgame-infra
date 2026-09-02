package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"api/internal/platform/settings/keys"
	"api/internal/platform/store/model"
	"api/internal/platform/store/shortener"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidProductID = errors.New("store: product_id must match ^(RJ|VJ)[0-9]{6,8}$")
	ErrQuotaExceeded    = errors.New("store: purchase-link quota exhausted for this client")
	ErrShortenerDown    = errors.New("store: shortener unavailable")
	ErrNotConfigured    = errors.New("store: shortener is not configured")
)

var productIDRe = regexp.MustCompile(`^(RJ|VJ)\d{6,8}$`)

// Minter is the shortener leg the service needs; the httptest fake in the tests
// implements it directly.
type Minter interface {
	Mint(ctx context.Context, destinationURL, description string) (*shortener.Link, error)
}

type Options struct {
	// AffTemplateManiax serves RJ (doujin) product ids, AffTemplatePro serves VJ
	// (commercial). Both carry a {product_id} slot and the affiliate id, and both
	// are configuration rather than code: the real aff id is supplied per
	// deployment (00-workflow §1.11).
	AffTemplateManiax string
	AffTemplatePro    string
}

type Service struct {
	db     *gorm.DB
	minter Minter
	opts   Options
}

func New(db *gorm.DB, minter Minter, opts Options) *Service {
	return &Service{db: db, minter: minter, opts: opts}
}

func (s *Service) Configured() bool { return s.minter != nil }

type CampaignView struct {
	ID   int64  `json:"id" doc:"Campaign id; pair it with the coupon link when you render the offer"`
	Name string `json:"name" doc:"Human-readable campaign name, safe to show to readers"`
}

type PurchaseLinks struct {
	ProductID   string        `json:"product_id" doc:"The DLsite product number that was asked for, echoed back"`
	PurchaseURL string        `json:"purchase_url" doc:"Short link to send readers to. Belongs to your application alone; use it verbatim"`
	CouponURL   *string       `json:"coupon_url" doc:"Short link to the coupon claim page, or null when no campaign is running"`
	Campaign    *CampaignView `json:"campaign" doc:"The campaign coupon_url belongs to, or null when no campaign is running"`
}

func ValidProductID(productID string) bool { return productIDRe.MatchString(productID) }

// ReplaceAll silently does nothing when the template spells the slot some other
// way, and the forum's own template env calls it {workno}. An unsubstituted
// template mints one alias per product all pointing at the same dead URL, and
// pins them there: the alias is fixed per (client, product) and the shortener
// has no re-point face.
const productIDSlot = "{product_id}"

func ValidAffTemplate(tmpl string) bool { return strings.Contains(tmpl, productIDSlot) }

func (s *Service) affURL(productID string) (string, error) {
	tmpl := s.opts.AffTemplatePro
	if strings.HasPrefix(productID, "RJ") {
		tmpl = s.opts.AffTemplateManiax
	}
	if !ValidAffTemplate(tmpl) {
		return "", ErrNotConfigured
	}
	return strings.ReplaceAll(tmpl, productIDSlot, productID), nil
}

func (s *Service) PurchaseLinks(ctx context.Context, clientID, productID string) (*PurchaseLinks, error) {
	if !ValidProductID(productID) {
		return nil, ErrInvalidProductID
	}
	if !s.Configured() {
		return nil, ErrNotConfigured
	}

	purchase, err := s.purchaseLink(ctx, clientID, productID)
	if err != nil {
		return nil, err
	}
	out := &PurchaseLinks{ProductID: productID, PurchaseURL: purchase.ShortURL}

	campaign, err := s.activeCampaign(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	if campaign == nil {
		return out, nil
	}
	coupon, err := s.couponLink(ctx, clientID, campaign)
	if err != nil {
		return nil, err
	}
	out.CouponURL = &coupon.ShortURL
	out.Campaign = &CampaignView{ID: campaign.ID, Name: campaign.Name}
	return out, nil
}

func (s *Service) purchaseLink(ctx context.Context, clientID, productID string) (*model.PurchaseLink, error) {
	existing, err := s.findPurchaseLink(ctx, clientID, productID)
	if err != nil || existing != nil {
		return existing, err
	}

	var minted int64
	if err := s.db.WithContext(ctx).Model(&model.PurchaseLink{}).
		Where("client_id = ?", clientID).Count(&minted).Error; err != nil {
		return nil, err
	}
	// Caps how many distinct products one calling site may mint links for, so a
	// site feeding us junk ids cannot flood the alias space.
	if minted >= int64(keys.StoreLinkQuotaPerClient.Get()) {
		return nil, ErrQuotaExceeded
	}

	dest, err := s.affURL(productID)
	if err != nil {
		return nil, err
	}
	link, err := s.minter.Mint(ctx, dest, description(clientID, productID, model.KindPurchase))
	if err != nil {
		slog.Error("store: mint purchase link failed", "client_id", clientID, "product_id", productID, "error", err)
		return nil, fmt.Errorf("%w: %s", ErrShortenerDown, err)
	}
	slog.Info("store: minted purchase link", "client_id", clientID, "product_id", productID, "alias", link.Alias, "minted_for_client", minted+1)

	row := &model.PurchaseLink{ClientID: clientID, ProductID: productID, Alias: link.Alias, ShortURL: link.ShortURL}
	// DoNothing, not DoUpdates: a concurrent request already minted this pair's
	// alias and that alias is the one the shortener is already counting against.
	// Overwriting it here would move the attribution to a link nobody clicked.
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "client_id"}, {Name: "product_id"}}, DoNothing: true}).
		Create(row).Error; err != nil {
		return nil, err
	}
	return s.requirePurchaseLink(ctx, clientID, productID)
}

func (s *Service) findPurchaseLink(ctx context.Context, clientID, productID string) (*model.PurchaseLink, error) {
	var row model.PurchaseLink
	err := s.db.WithContext(ctx).Where("client_id = ? AND product_id = ?", clientID, productID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) requirePurchaseLink(ctx context.Context, clientID, productID string) (*model.PurchaseLink, error) {
	row, err := s.findPurchaseLink(ctx, clientID, productID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("store: purchase link vanished after insert (%s/%s)", clientID, productID)
	}
	return row, nil
}

func (s *Service) couponLink(ctx context.Context, clientID string, campaign *model.Campaign) (*model.CouponLink, error) {
	existing, err := s.findCouponLink(ctx, clientID, campaign.ID)
	if err != nil || existing != nil {
		return existing, err
	}

	link, err := s.minter.Mint(ctx, campaign.CouponURL, description(clientID, fmt.Sprint(campaign.ID), model.KindCoupon))
	if err != nil {
		slog.Error("store: mint coupon link failed", "client_id", clientID, "campaign_id", campaign.ID, "error", err)
		return nil, fmt.Errorf("%w: %s", ErrShortenerDown, err)
	}
	slog.Info("store: minted coupon link", "client_id", clientID, "campaign_id", campaign.ID, "alias", link.Alias)

	row := &model.CouponLink{ClientID: clientID, CampaignID: campaign.ID, Alias: link.Alias, ShortURL: link.ShortURL}
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "client_id"}, {Name: "campaign_id"}}, DoNothing: true}).
		Create(row).Error; err != nil {
		return nil, err
	}
	existing, err = s.findCouponLink(ctx, clientID, campaign.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("store: coupon link vanished after insert (%s/%d)", clientID, campaign.ID)
	}
	return existing, nil
}

func (s *Service) findCouponLink(ctx context.Context, clientID string, campaignID int64) (*model.CouponLink, error) {
	var row model.CouponLink
	err := s.db.WithContext(ctx).Where("client_id = ? AND campaign_id = ?", clientID, campaignID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) activeCampaign(ctx context.Context, now time.Time) (*model.Campaign, error) {
	var row model.Campaign
	err := s.db.WithContext(ctx).
		Where("starts_at <= ? AND ends_at > ?", now, now).
		Order("starts_at DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func description(clientID, key, kind string) string {
	return fmt.Sprintf("store:%s:%s:%s", clientID, key, kind)
}
