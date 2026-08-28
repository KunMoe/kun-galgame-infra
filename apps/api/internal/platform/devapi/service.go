package devapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	siteModel "api/internal/platform/site/model"

	"gorm.io/datatypes"
)

const rotateGrace = 72 * time.Hour

var ErrInvalidTier = errors.New("devapi: invalid tier (want free|trusted|internal)")

type AdminService struct {
	repo  *Repository
	store Store
}

func NewAdminService(repo *Repository, store Store) *AdminService {
	return &AdminService{repo: repo, store: store}
}

type AppConfig struct {
	OwnerUserID    *uint
	DevEnabled     *bool
	DevTier        *string
	DevRatePerMin  *int
	DevQuotaDaily  *int
}

type AppView struct {
	Client   *siteModel.OAuthClient
	KeyCount int64
}

func (s *AdminService) UpdateAppConfig(ctx context.Context, clientID string, cfg AppConfig) (*siteModel.OAuthClient, error) {
	fields := map[string]any{}
	if cfg.OwnerUserID != nil {
		fields["owner_user_id"] = *cfg.OwnerUserID
	}
	if cfg.DevEnabled != nil {
		fields["dev_enabled"] = *cfg.DevEnabled
		// The console's enable path is itself an approval: without this a row
		// enabled straight from the app list would stay 'pending' and its owner
		// would still be refused a key on a live application.
		if *cfg.DevEnabled {
			fields["dev_review_status"] = AppReviewApproved
			fields["dev_review_note"] = ""
		}
	}
	if cfg.DevTier != nil {
		if !validTier(*cfg.DevTier) {
			return nil, ErrInvalidTier
		}
		fields["dev_tier"] = *cfg.DevTier
	}
	if cfg.DevRatePerMin != nil {
		fields["dev_rate_per_min"] = *cfg.DevRatePerMin
	}
	if cfg.DevQuotaDaily != nil {
		fields["dev_quota_daily"] = *cfg.DevQuotaDaily
	}
	if err := s.repo.UpdateAppDevConfig(ctx, clientID, fields); err != nil {
		return nil, err
	}
	s.bustAppCredentials(ctx, clientID)
	return s.repo.GetApp(ctx, clientID)
}

func (s *AdminService) bustAppCredentials(ctx context.Context, clientID string) {
	keys, err := s.repo.ListKeysByClient(ctx, clientID)
	if err != nil {
		slog.Warn("devapi: app config changed but its resolve cache could not be busted",
			"client_id", clientID, "err", err)
		return
	}
	for i := range keys {
		if key, ok := credCacheKeyForStoredHash(keys[i].KeyHash); ok {
			_ = s.store.Del(ctx, key)
		}
	}
}

func (s *AdminService) ListApps(ctx context.Context, filter string) ([]AppView, error) {
	apps, err := s.repo.ListAppsByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]AppView, len(apps))
	for i := range apps {
		n, err := s.repo.CountKeysByClient(ctx, apps[i].ID)
		if err != nil {
			return nil, err
		}
		out[i] = AppView{Client: &apps[i], KeyCount: n}
	}
	return out, nil
}

type MintKeyInput struct {
	Name   string
	Test   bool
	Scopes []string
}

func (s *AdminService) MintKey(ctx context.Context, clientID string, in MintKeyInput, createdBy uint) (*DeveloperAPIKey, string, error) {
	if _, err := s.repo.GetApp(ctx, clientID); err != nil {
		return nil, "", err
	}
	scopes := in.Scopes
	if len(scopes) == 0 {
		scopes = []string{ScopeCatalogRead}
	}
	key, plaintext, err := s.buildKey(clientID, in.Name, in.Test, scopes, createdBy)
	if err != nil {
		return nil, "", err
	}
	if err := s.repo.CreateKey(ctx, key); err != nil {
		return nil, "", err
	}
	slog.Info("devapi key minted", "client_id", clientID, "key_id", key.ID, "prefix", key.KeyPrefix, "last4", key.Last4, "by", createdBy)
	return key, plaintext, nil
}

func (s *AdminService) ListKeys(ctx context.Context, clientID string) ([]DeveloperAPIKey, error) {
	return s.repo.ListKeysByClient(ctx, clientID)
}

func (s *AdminService) ListAllKeys(ctx context.Context, f KeyListFilter) ([]AdminKeyRow, int64, error) {
	return s.repo.ListAllKeys(ctx, f, time.Now())
}

func (s *AdminService) RotateKey(ctx context.Context, keyID, createdBy uint) (*DeveloperAPIKey, string, error) {
	old, err := s.repo.GetKey(ctx, keyID)
	if err != nil {
		return nil, "", err
	}
	var scopes []string
	if len(old.Scopes) > 0 {
		_ = json.Unmarshal(old.Scopes, &scopes)
	}
	test := strings.HasPrefix(old.KeyPrefix, TestPrefix) || strings.HasPrefix(old.KeyPrefix, V2TestPrefix)
	newKey, plaintext, err := s.buildKey(old.ClientID, old.Name, test, scopes, createdBy)
	if err != nil {
		return nil, "", err
	}
	if err := s.repo.CreateKey(ctx, newKey); err != nil {
		return nil, "", err
	}
	if err := s.repo.SetKeyExpiry(ctx, old.ID, time.Now().Add(rotateGrace)); err != nil {
		return nil, "", err
	}
	slog.Info("devapi key rotated", "client_id", old.ClientID, "old_key_id", old.ID, "new_key_id", newKey.ID, "by", createdBy)
	return newKey, plaintext, nil
}

func (s *AdminService) RevokeKey(ctx context.Context, keyID uint) error {
	key, err := s.repo.GetKey(ctx, keyID)
	if err != nil {
		return err
	}
	if err := s.repo.RevokeKey(ctx, key.ID, time.Now()); err != nil {
		return err
	}
	if cacheKey, ok := credCacheKeyForStoredHash(key.KeyHash); ok {
		_ = s.store.Del(ctx, cacheKey)
	}
	slog.Info("devapi key revoked", "client_id", key.ClientID, "key_id", key.ID)
	return nil
}

func (s *AdminService) GetKeyForClient(ctx context.Context, clientID string, keyID uint) (*DeveloperAPIKey, error) {
	key, err := s.repo.GetKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if key.ClientID != clientID {
		return nil, nil
	}
	return key, nil
}

func (s *AdminService) buildKey(clientID, name string, test bool, scopes []string, createdBy uint) (*DeveloperAPIKey, string, error) {
	plaintext, err := GenerateV2Key(!test)
	if err != nil {
		return nil, "", err
	}
	kp, last4 := KeyMetadata(plaintext)
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return nil, "", err
	}
	return &DeveloperAPIKey{
		ClientID:        clientID,
		Name:            name,
		KeyHash:         HashKey(plaintext),
		KeyPrefix:       kp,
		Last4:           last4,
		Scopes:          datatypes.JSON(scopesJSON),
		CreatedByUserID: createdBy,
	}, plaintext, nil
}

func validTier(t string) bool {
	return t == TierFree || t == TierTrusted || t == TierInternal
}
