package devapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	siteModel "api/internal/platform/site/model"

	"gorm.io/datatypes"
)

const (
	MaxAppsPerOwner     = 5
	MaxActiveKeysPerApp = 5
	maxAppNameLen       = 100
	maxAppDescLen       = 100
)

// Scopes a key owner may tick unilaterally. galgame:read left this list when the
// /v1/galgame face retired to a 410 tombstone (wave 146): no live route consumes
// it, so offering it minted a permission over nothing.
var selfServiceScopes = []string{ScopeCatalogRead}

// Scopes a key owner may hold only after we approve an application. The news
// partners authorised NextMoe's index, not whoever asks us for it, so who
// carries news:read stays our decision — the application only mechanises the
// paperwork, it does not make the scope self-service.
var grantableScopes = []string{ScopeNewsRead}

var (
	ErrAppLimitReached = errors.New("devapi: application limit reached")
	ErrKeyLimitReached = errors.New("devapi: active key limit reached")
	ErrScopeNotAllowed = errors.New("devapi: scope not permitted (want catalog:read)")
	ErrScopeNeedsGrant = errors.New("devapi: scope requires an approved application")
	ErrNameRequired    = errors.New("devapi: name is required")
	ErrNameTooLong     = errors.New("devapi: name too long (max 100)")
	ErrDescTooLong     = errors.New("devapi: description too long (max 100)")
)

type SelfServiceService struct {
	repo  *Repository
	admin *AdminService
	store Store
}

func NewSelfServiceService(repo *Repository, admin *AdminService, store Store) *SelfServiceService {
	return &SelfServiceService{repo: repo, admin: admin, store: store}
}

func (s *SelfServiceService) CreateApp(ctx context.Context, ownerUserID uint, name, description string, login *UserLoginRequest) (*siteModel.OAuthClient, error) {
	mode, err := s.repo.PolicyMode(ctx, CapabilityAppCreate)
	if err != nil {
		return nil, err
	}
	if mode == PolicyDisabled {
		return nil, ErrCapabilityDisabled
	}
	if err := validateAppMeta(name, description, true); err != nil {
		return nil, err
	}
	if err := validateAppName(name); err != nil {
		return nil, err
	}
	redirectURIs, grants, userScopes := "[]", "[]", ""
	isPublic := false
	if login != nil {
		scopes, err := validateUserLogin(*login)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(login.RedirectURIs)
		if err != nil {
			return nil, err
		}
		redirectURIs = string(encoded)
		grants = `["authorization_code","refresh_token"]`
		userScopes = strings.Join(scopes, " ")
		isPublic = true
	}
	n, err := s.repo.CountAppsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if n >= MaxAppsPerOwner {
		return nil, ErrAppLimitReached
	}

	clientID, err := generateHex(16)
	if err != nil {
		return nil, err
	}
	secret, err := generateHex(32)
	if err != nil {
		return nil, err
	}

	owner := ownerUserID
	reviewStatus, enabled := AppReviewApproved, true
	if mode == PolicyApproval {
		reviewStatus, enabled = AppReviewPending, false
	}
	app := &siteModel.OAuthClient{
		ID:              clientID,
		Name:            name,
		Secret:          siteModel.HashOAuthClientSecret(secret),
		RedirectURIs:    datatypes.JSON([]byte(redirectURIs)),
		Grants:          datatypes.JSON([]byte(grants)),
		IsPublic:        isPublic,
		AllowedScopes:   datatypes.JSON(appAllowedScopes(userScopes)),
		Tagline:         description,
		OwnerUserID:     &owner,
		DevEnabled:      enabled,
		DevTier:         TierFree,
		DevRatePerMin:   0,
		DevQuotaDaily:   0,
		DevReviewStatus: reviewStatus,
	}
	if err := s.repo.CreateApp(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *SelfServiceService) ListApps(ctx context.Context, ownerUserID uint) ([]AppView, error) {
	apps, err := s.repo.ListAppsByOwner(ctx, ownerUserID)
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

func (s *SelfServiceService) GetApp(ctx context.Context, ownerUserID uint, clientID string) (*AppView, error) {
	app, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID)
	if err != nil {
		return nil, err
	}
	n, err := s.repo.CountKeysByClient(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	return &AppView{Client: app, KeyCount: n}, nil
}

func (s *SelfServiceService) UpdateApp(ctx context.Context, ownerUserID uint, clientID string, name, description *string, login *UserLoginRequest) (*siteModel.OAuthClient, error) {
	app, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCapability(ctx, CapabilityAppManage); err != nil {
		return nil, err
	}
	if err := validateAppMetaPtr(name, description); err != nil {
		return nil, err
	}
	if name != nil {
		if err := validateAppName(*name); err != nil {
			return nil, err
		}
	}
	fields := map[string]any{}
	if login != nil {
		scopes, err := validateUserLogin(*login)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(login.RedirectURIs)
		if err != nil {
			return nil, err
		}
		fields["redirect_uris"] = datatypes.JSON(encoded)
		fields["grants"] = datatypes.JSON([]byte(`["authorization_code","refresh_token"]`))
		fields["allowed_scopes"] = datatypes.JSON(appAllowedScopes(strings.Join(scopes, " ")))
		fields["is_public"] = true
	}
	if name != nil {
		fields["name"] = *name
	}
	if description != nil {
		fields["tagline"] = *description
	}
	if err := s.repo.UpdateAppFields(ctx, app.ID, fields); err != nil {
		return nil, err
	}
	return s.repo.GetApp(ctx, app.ID)
}

func (s *SelfServiceService) DeactivateApp(ctx context.Context, ownerUserID uint, clientID string) error {
	app, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID)
	if err != nil {
		return err
	}
	if err := s.requireCapability(ctx, CapabilityAppManage); err != nil {
		return err
	}
	// A pending row was never enabled and a declined one is already inert, so
	// there is nothing to deactivate; letting it through would silently turn
	// "waiting for review" into "gone" with no way back.
	if appAwaitsReview(app.DevReviewStatus) {
		return ErrAppNotApproved
	}
	keys, err := s.repo.ListKeysByClient(ctx, app.ID)
	if err != nil {
		return err
	}
	for i := range keys {
		if keys[i].RevokedAt != nil {
			continue
		}
		if err := s.admin.RevokeKey(ctx, keys[i].ID); err != nil {
			return err
		}
	}
	return s.repo.UpdateAppFields(ctx, app.ID, map[string]any{"dev_enabled": false})
}

func (s *SelfServiceService) MintKey(ctx context.Context, ownerUserID uint, clientID string, in MintKeyInput) (*DeveloperAPIKey, string, error) {
	app, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID)
	if err != nil {
		return nil, "", err
	}
	if err := s.requireCapability(ctx, CapabilityKeyMint); err != nil {
		return nil, "", err
	}
	if appAwaitsReview(app.DevReviewStatus) {
		return nil, "", ErrAppNotApproved
	}
	if in.Name == "" {
		return nil, "", ErrNameRequired
	}
	if len(in.Name) > maxAppNameLen {
		return nil, "", ErrNameTooLong
	}
	if err := s.checkMintScopes(ctx, ownerUserID, in.Scopes); err != nil {
		return nil, "", err
	}
	active, err := s.repo.CountActiveKeysByClient(ctx, clientID, time.Now())
	if err != nil {
		return nil, "", err
	}
	if active >= MaxActiveKeysPerApp {
		return nil, "", ErrKeyLimitReached
	}
	return s.admin.MintKey(ctx, clientID, in, ownerUserID)
}

func (s *SelfServiceService) ListKeys(ctx context.Context, ownerUserID uint, clientID string) ([]DeveloperAPIKey, error) {
	if _, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID); err != nil {
		return nil, err
	}
	return s.admin.ListKeys(ctx, clientID)
}

func (s *SelfServiceService) RotateKey(ctx context.Context, ownerUserID uint, clientID string, keyID uint) (*DeveloperAPIKey, string, error) {
	if _, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID); err != nil {
		return nil, "", err
	}
	if err := s.requireCapability(ctx, CapabilityKeyMint); err != nil {
		return nil, "", err
	}
	key, err := s.admin.GetKeyForClient(ctx, clientID, keyID)
	if err != nil {
		return nil, "", err
	}
	if key == nil {
		return nil, "", nil
	}
	return s.admin.RotateKey(ctx, keyID, ownerUserID)
}

func (s *SelfServiceService) RevokeKey(ctx context.Context, ownerUserID uint, clientID string, keyID uint) (found bool, err error) {
	if _, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID); err != nil {
		return false, err
	}
	key, err := s.admin.GetKeyForClient(ctx, clientID, keyID)
	if err != nil {
		return false, err
	}
	if key == nil {
		return false, nil
	}
	return true, s.admin.RevokeKey(ctx, keyID)
}

type scopeGate int

const (
	scopeGateDenied scopeGate = iota
	scopeGateSelfService
	scopeGateGrant
)

func gateForScope(sc string) scopeGate {
	switch {
	case slices.Contains(selfServiceScopes, sc):
		return scopeGateSelfService
	case slices.Contains(grantableScopes, sc):
		return scopeGateGrant
	default:
		return scopeGateDenied
	}
}

func (s *SelfServiceService) checkMintScopes(ctx context.Context, ownerUserID uint, scopes []string) error {
	for _, sc := range scopes {
		switch gateForScope(sc) {
		case scopeGateSelfService:
		case scopeGateGrant:
			approved, err := s.repo.HasApprovedScopeApplication(ctx, ownerUserID, sc)
			if err != nil {
				return err
			}
			if !approved {
				return ErrScopeNeedsGrant
			}
		default:
			return ErrScopeNotAllowed
		}
	}
	return nil
}

func validateAppMeta(name, description string, requireName bool) error {
	if requireName && name == "" {
		return ErrNameRequired
	}
	if len(name) > maxAppNameLen {
		return ErrNameTooLong
	}
	if len(description) > maxAppDescLen {
		return ErrDescTooLong
	}
	return nil
}

func validateAppMetaPtr(name, description *string) error {
	if name != nil {
		if *name == "" {
			return ErrNameRequired
		}
		if len(*name) > maxAppNameLen {
			return ErrNameTooLong
		}
	}
	if description != nil && len(*description) > maxAppDescLen {
		return ErrDescTooLong
	}
	return nil
}

func generateHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
