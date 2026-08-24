package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/seed"
	catsvc "api/internal/platform/catalog/service"
	srcb "api/internal/platform/catalog/srcbangumi"
	"api/internal/platform/devapi"
	"api/internal/platform/editing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	liveAppKey    = "nm_live_v2test"
	liveUserToken = "user-live-token"
	liveUID       = int64(7)
	liveClient    = "kungal-client"
	liveSite      = "kungal"
)

type liveFix struct {
	Work, Pending, Claimable, Anchored int64
	Company, Tag, Series, Engine       int64
	Release, Character, Person, Credit int64
	Trait, Cover                       int64
	AnchorExt                          string
}

type liveEnv struct {
	app *fiber.App
	db  *gorm.DB
	cat *Catalog
	fx  liveFix
}

var (
	liveOnce sync.Once
	liveHold *liveEnv
	liveErr  error
)

func liveCatalog(t *testing.T) *liveEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN unset")
	}
	liveOnce.Do(func() {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			liveErr = err
			return
		}
		if err := migrate.Run(db); err != nil {
			liveErr = err
			return
		}
		if err := seed.Run(db); err != nil {
			liveErr = err
			return
		}
		if err := srcb.EnsureSchema(db); err != nil {
			liveErr = err
			return
		}
		read := catsvc.NewReadService(db)
		resolve := catsvc.NewResolveService(repository.NewRedirectRepository(db))
		pub := catsvc.NewPublicService(db, read, resolve, "")
		reg := editing.NewRegistry()
		if err := editspec.RegisterAll(reg, db); err != nil {
			liveErr = err
			return
		}
		cat := &Catalog{
			Public:     pub,
			Resolve:    resolve,
			StatsSvc:   catsvc.NewStatsService(db),
			EditTypes:  reg,
			Playtime:   catsvc.NewUserPlaytimeService(db),
			CoverVotes: catsvc.NewCoverVoteService(db),
			Claims:     catsvc.NewClaimLifecycleService(db),
			Engine:     editing.NewEngine(db, reg),
		}
		app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
		SetupWith(app, Options{
			Catalog: cat,
			LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
				if raw != liveAppKey {
					return nil, nil
				}
				return &devapi.Credential{
					KeyID: 1, NSFWAllowed: true, Scopes: []string{devapi.ScopeCatalogRead},
				}, nil
			},
			LookupUser: func(_ context.Context, raw string) (UserIdentity, error) {
				if raw != liveUserToken {
					return UserIdentity{}, os.ErrPermission
				}
				return UserIdentity{UID: liveUID, ClientID: liveClient, Roles: []string{"admin"}}, nil
			},
			LookupSite: func(_ context.Context, clientID string) (string, error) {
				if clientID == liveClient {
					return liveSite, nil
				}
				return "", nil
			},
		})
		fx, ferr := seedLiveFixtures(db, cat.Claims)
		if ferr != nil {
			liveErr = ferr
			return
		}
		liveHold = &liveEnv{app: app, db: db, cat: cat, fx: fx}
	})
	if liveErr != nil {
		t.Fatalf("live catalog setup: %v", liveErr)
	}
	return liveHold
}

func seedLiveFixtures(db *gorm.DB, claims *catsvc.ClaimLifecycleService) (liveFix, error) {
	if err := db.Exec(`TRUNCATE
		catalog_claim_event, catalog_external_ref, catalog_work_cover, catalog_release,
		catalog_work, catalog_label, catalog_tag, catalog_series, catalog_engine,
		catalog_character, catalog_credit_name, catalog_person, catalog_character_trait
		RESTART IDENTITY CASCADE`).Error; err != nil {
		return liveFix{}, err
	}
	empty := datatypes.JSON([]byte("{}"))
	fx := liveFix{AnchorExt: "v88888"}

	w := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Live Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(w).Error; err != nil {
		return fx, err
	}
	fx.Work = w.ID

	pending := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Pending Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(pending).Error; err != nil {
		return fx, err
	}
	fx.Pending = pending.ID
	product := int64(10001)
	if _, err := claims.Act(context.Background(), catsvc.ClaimActionParams{
		WorkID: pending.ID, Action: catsvc.ClaimActionClaim, Site: liveSite,
		ProductWorkID: &product, ActorUID: liveUID,
	}); err != nil {
		return fx, err
	}
	if _, err := claims.Act(context.Background(), catsvc.ClaimActionParams{
		WorkID: pending.ID, Action: catsvc.ClaimActionSubmit, Site: liveSite, ActorUID: liveUID,
	}); err != nil {
		return fx, err
	}

	claimable := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Claimable Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(claimable).Error; err != nil {
		return fx, err
	}
	fx.Claimable = claimable.ID

	anchored := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Anchored Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(anchored).Error; err != nil {
		return fx, err
	}
	fx.Anchored = anchored.ID
	if err := db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: anchored.ID, SourceID: 2,
		ExternalID: fx.AnchorExt, LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error; err != nil {
		return fx, err
	}

	co := &model.CatalogLabel{DisplayName: "Live Brand", Lang: "ja", Kind: model.LabelKindGameBrand, FieldProvenance: empty}
	if err := db.Create(co).Error; err != nil {
		return fx, err
	}
	fx.Company = co.ID

	tg := &model.CatalogTag{Name: "live-tag", Tier: model.TagTierCore, Kind: model.TagKindContent}
	if err := db.Create(tg).Error; err != nil {
		return fx, err
	}
	fx.Tag = tg.ID

	se := &model.CatalogSeries{DisplayName: "Live Series", SourceID: 2, ExternalID: "s-live-1"}
	if err := db.Create(se).Error; err != nil {
		return fx, err
	}
	fx.Series = se.ID

	en := &model.CatalogEngine{Name: "LiveEngine", Description: "test", Aliases: datatypes.JSON([]byte("[]"))}
	if err := db.Create(en).Error; err != nil {
		return fx, err
	}
	fx.Engine = en.ID

	y, m := int16(2024), int16(1)
	rel := &model.CatalogRelease{WorkID: w.ID, Kind: model.ReleaseKindDefault, ReleasedY: &y, ReleasedM: &m, Extra: empty, FieldProvenance: empty}
	if err := db.Create(rel).Error; err != nil {
		return fx, err
	}
	fx.Release = rel.ID

	ch := &model.CatalogCharacter{DisplayName: "Live Char", Lang: "ja", Extra: empty, FieldProvenance: empty}
	if err := db.Create(ch).Error; err != nil {
		return fx, err
	}
	fx.Character = ch.ID

	pe := &model.CatalogPerson{DisplayName: "Live Person", FieldProvenance: empty}
	if err := db.Create(pe).Error; err != nil {
		return fx, err
	}
	fx.Person = pe.ID

	cn := &model.CatalogCreditName{PersonID: &pe.ID, Name: "Live Credit", Lang: "ja", Kind: model.CreditNameKindMain, FieldProvenance: empty}
	if err := db.Create(cn).Error; err != nil {
		return fx, err
	}
	fx.Credit = cn.ID

	tr := &model.CatalogCharacterTrait{
		VndbTID: "i99999", Name: "Live Trait", GroupTID: "i1", Alias: "", Description: "",
	}
	if err := db.Create(tr).Error; err != nil {
		return fx, err
	}
	fx.Trait = tr.ID

	cv := &model.CatalogWorkCover{WorkID: w.ID, ImageHash: "livecoverhash1", Kind: "main", SourceID: 2}
	if err := db.Create(cv).Error; err != nil {
		return fx, err
	}
	fx.Cover = cv.ID
	return fx, nil
}

func liveDo(t *testing.T, env *liveEnv, method, path, token, body string) (int, string, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := env.app.Test(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Content-Type"), raw
}

func liveDoHeader(t *testing.T, env *liveEnv, method, path, token, body string, extra map[string]string) (int, string, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := env.app.Test(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Content-Type"), raw
}

func liveAuthPath(path string) string {
	if strings.HasPrefix(path, "/v2/me/") || strings.HasPrefix(path, "/v2/moderation/") {
		return liveUserToken
	}
	if strings.HasPrefix(path, "/v2/catalog/") && path != "/v2/catalog/stats" && !strings.HasPrefix(path, "/v2/catalog/schemas/") {
		return liveAppKey
	}
	return ""
}

func idstr(id int64) string { return strconv.FormatInt(id, 10) }

func liveSubstitute(path string, fx liveFix) string {
	id := fx.Work
	switch {
	case strings.Contains(path, "/companies"):
		id = fx.Company
	case strings.Contains(path, "/credit-names"):
		id = fx.Credit
	case strings.Contains(path, "/characters"):
		id = fx.Character
	case strings.Contains(path, "/tags"):
		id = fx.Tag
	case strings.Contains(path, "/series"):
		id = fx.Series
	case strings.Contains(path, "/engines"):
		id = fx.Engine
	case strings.Contains(path, "/releases"):
		id = fx.Release
	case strings.Contains(path, "/persons"):
		id = fx.Person
	case strings.Contains(path, "/traits"):
		id = fx.Trait
	case strings.Contains(path, "/claims"):
		id = fx.Pending
	case strings.Contains(path, "/cover-votes"):
		id = fx.Cover
	case strings.Contains(path, "/playtimes"):
		id = fx.Work
	case strings.Contains(path, "/snapshots/"):
		id = fx.Work
	}
	path = strings.ReplaceAll(path, "{code}", problem.CodeRateLimited)
	path = strings.ReplaceAll(path, "{name}", "medium")
	path = strings.ReplaceAll(path, "{object}", "work")
	path = strings.ReplaceAll(path, "{work_id}", idstr(fx.Work))
	path = strings.ReplaceAll(path, "{cover_id}", idstr(fx.Cover))
	path = strings.ReplaceAll(path, "{id}", idstr(id))
	if path == "/v2/catalog/search" {
		path += "?object=work"
	}
	return path
}
