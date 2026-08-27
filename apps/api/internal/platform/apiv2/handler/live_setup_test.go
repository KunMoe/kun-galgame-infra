package handler

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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
	newsmigrate "api/internal/platform/news/migrate"
	newsmodel "api/internal/platform/news/model"
	"api/internal/platform/news/newstest"
	newssvc "api/internal/platform/news/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	liveAppKey    = mustLiveV2Key()
	liveAppKeyB   = mustLiveV2Key()
	liveUserToken = "user-live-token"
	liveUID       = int64(7)
	liveClient    = "kungal-client"
	liveSite      = "kungal"
)

// liveUnlimitedStore never rate-limits, but it does remember: without a real
// Get/Set the Idempotency-Key replay path is dead code in every live test, and
// a POST that mints a second row on retry would pass unnoticed.
type liveUnlimitedStore struct {
	mu   sync.Mutex
	kept map[string][]byte
}

func (*liveUnlimitedStore) Incr(context.Context, string, time.Duration) (int64, error) {
	return 1, nil
}
func (*liveUnlimitedStore) Decr(context.Context, string) error { return nil }
func (s *liveUnlimitedStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kept[key], nil
}
func (s *liveUnlimitedStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kept == nil {
		s.kept = map[string][]byte{}
	}
	s.kept[key] = value
	return nil
}

func mustLiveV2Key() string {
	k, err := devapi.GenerateV2Key(true)
	if err != nil {
		panic(err)
	}
	return k
}

type liveFix struct {
	Work, Pending, Claimable, Anchored  int64
	Company, Tag, Series, Engine        int64
	Release, Character, Person, Credit  int64
	Trait, Cover                        int64
	NewsItem                            int64
	CompanyEmpty, TagSexual, Attributed int64
	CompanyLogo, NSFWMember, Sibling    int64
	TraitMinor, CharacterNoLang         int64
	BulkWork, BulkCharacter             int64
	AnchoredPerson, AnchoredCredit      int64
	AnchorExt                           string
}

// One embedded block used to truncate at 100 rows; both fixtures sit just past
// that edge so the removal stays proven rather than assumed.
const liveBulkRows = 105

const (
	liveNewsSource = "moyu"
	liveCDNBase    = "https://img.example.test/image"
	liveLogoHash   = "1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f809"
	livePhotoHash  = "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4d3c2b1a0"
)

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
		if err := newsmigrate.Run(db); err != nil {
			liveErr = err
			return
		}
		read := catsvc.NewReadService(db)
		resolve := catsvc.NewResolveService(repository.NewRedirectRepository(db))
		pub := catsvc.NewPublicService(db, read, resolve, liveCDNBase)
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
			News:       newssvc.NewPublicService(db, "https://image.example.test/image"),
			NewsWrite:  newssvc.NewSubmissionService(db),
		}
		cat.EditHistory = catsvc.NewEditHistoryService(db)
		app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
		SetupWith(app, Options{
			Store:   &liveUnlimitedStore{},
			Catalog: cat,
			LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
				switch raw {
				case liveAppKey:
					return &devapi.Credential{
						KeyID: 1, Scopes: []string{devapi.ScopeCatalogRead},
					}, nil
				case liveAppKeyB:
					return &devapi.Credential{
						KeyID: 2, Scopes: []string{devapi.ScopeCatalogRead},
					}, nil
				default:
					return nil, nil
				}
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
	// The edit_* tables belong in this list: catalog_work restarts its identity
	// on every run while edit_revision did not, so run N+1's fresh work id
	// inherited run N's revision chain and the entity's history read back with
	// another run's snapshots.
	if err := db.Exec(`TRUNCATE
		catalog_claim_event, catalog_external_ref, catalog_work_cover, catalog_release,
		catalog_work, catalog_label, catalog_tag, catalog_series, catalog_engine,
		catalog_character, catalog_credit_name, catalog_person, catalog_character_trait,
		catalog_work_label, catalog_label_alias, catalog_series_member, catalog_character_trait_link,
		catalog_work_rating, catalog_tag_intro, catalog_series_intro, catalog_person_intro,
		catalog_name_alias, catalog_credit, catalog_work_tag, catalog_label_relation,
		catalog_character_alias, catalog_character_intro,
		edit_revision, edit_proposal, edit_proposal_amendment
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

	// The taxonomy work counts (has_works=, series/company work_count) only
	// count works with a LIVE claim — claimStateWhere(taxonomyLiveClaim) — so an
	// unclaimed fixture work attributes to a label yet counts as zero.
	attributed := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Attributed Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(attributed).Error; err != nil {
		return fx, err
	}
	fx.Attributed = attributed.ID
	attrProduct := int64(10002)
	for _, action := range []catsvc.ClaimAction{catsvc.ClaimActionClaim, catsvc.ClaimActionSubmit, catsvc.ClaimActionApprove} {
		if _, err := claims.Act(context.Background(), catsvc.ClaimActionParams{
			WorkID: attributed.ID, Action: action, Site: liveSite,
			ProductWorkID: &attrProduct, ActorUID: liveUID,
		}); err != nil {
			return fx, err
		}
	}

	co := &model.CatalogLabel{DisplayName: "Live Brand", Lang: "ja", Kind: model.LabelKindGameBrand, FieldProvenance: empty}
	if err := db.Create(co).Error; err != nil {
		return fx, err
	}
	fx.Company = co.ID
	if err := db.Create(&model.CatalogWorkLabel{WorkID: attributed.ID, LabelID: co.ID, Kind: model.WorkLabelKindBrand}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogLabelAlias{
		LabelID: co.ID, Name: "ライブブランド", Lang: "ja",
		Kind: model.AliasKindSpellingVariant, Provenance: model.AliasProvenanceSource,
	}).Error; err != nil {
		return fx, err
	}

	// Lang deliberately unset: it is the null-side control for company.lang.
	empt := &model.CatalogLabel{DisplayName: "Empty Brand", Kind: model.LabelKindGameBrand, FieldProvenance: empty}
	if err := db.Create(empt).Error; err != nil {
		return fx, err
	}
	fx.CompanyEmpty = empt.ID

	// Both hang off the unclaimed "Live Work": the taxonomy work counts only see
	// LIVE-claimed works, so attributing here leaves has_works= and work_count
	// untouched while still giving the work-detail companies block one row with a
	// logo and one without.
	logo := &model.CatalogLabel{
		DisplayName: "Logo Brand", Lang: "ja", Kind: model.LabelKindGameBrand,
		LogoHash: liveLogoHash, FieldProvenance: empty,
	}
	if err := db.Create(logo).Error; err != nil {
		return fx, err
	}
	fx.CompanyLogo = logo.ID
	for _, wl := range []*model.CatalogWorkLabel{
		{WorkID: w.ID, LabelID: logo.ID, Kind: model.WorkLabelKindBrand},
		{WorkID: w.ID, LabelID: co.ID, Kind: model.WorkLabelKindDeveloper},
	} {
		if err := db.Create(wl).Error; err != nil {
			return fx, err
		}
	}
	if err := db.Create(&model.CatalogLabelRelation{
		LabelID: co.ID, OtherLabelID: logo.ID,
		Relation: model.LabelRelationParent, SourceID: 2, MatchedBy: "test",
	}).Error; err != nil {
		return fx, err
	}

	tg := &model.CatalogTag{Name: "live-tag", Tier: model.TagTierCore, Kind: model.TagKindContent}
	if err := db.Create(tg).Error; err != nil {
		return fx, err
	}
	fx.Tag = tg.ID

	ero := &model.CatalogTag{Name: "live-ero-tag", Tier: model.TagTierCore, Kind: model.TagKindContent, Sexual: true}
	if err := db.Create(ero).Error; err != nil {
		return fx, err
	}
	fx.TagSexual = ero.ID

	if err := db.Create(&model.CatalogTagIntro{TagID: tg.ID, Lang: "zh-Hans", Intro: "标签说明", SourceID: 1}).Error; err != nil {
		return fx, err
	}

	for _, wt := range []*model.CatalogWorkTag{
		{WorkID: w.ID, Name: "live-tag", Count: 9, SourceID: 2, Spoiler: 0},
		{WorkID: w.ID, Name: "live-tag-minor", Count: 5, SourceID: 2, Spoiler: 1},
		{WorkID: w.ID, Name: "live-tag-major", Count: 3, SourceID: 2, Spoiler: 2},
	} {
		if err := db.Create(wt).Error; err != nil {
			return fx, err
		}
	}

	se := &model.CatalogSeries{DisplayName: "Live Series", SourceID: 2, ExternalID: "s-live-1"}
	if err := db.Create(se).Error; err != nil {
		return fx, err
	}
	fx.Series = se.ID
	if err := db.Create(&model.CatalogSeriesMember{SeriesID: se.ID, WorkID: attributed.ID, Position: 1, Kind: model.SeriesMemberKindMain}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogSeriesIntro{SeriesID: se.ID, Lang: "ja", Intro: "シリーズ説明", SourceID: 2}).Error; err != nil {
		return fx, err
	}

	// has_nsfw is the display gate, not content_rating: a CLAIMED member only
	// counts as nsfw through display_nsfw. r18 keeps it out of the sfw work_count
	// at the same time, so the series reads work_count 1 / has_nsfw true.
	nsfwMember := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "NSFW Member Work",
		ContentRating: model.ContentRatingR18, Status: model.WorkStatusLive, DisplayNSFW: true,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(nsfwMember).Error; err != nil {
		return fx, err
	}
	fx.NSFWMember = nsfwMember.ID
	nsfwProduct := int64(10003)
	for _, action := range []catsvc.ClaimAction{catsvc.ClaimActionClaim, catsvc.ClaimActionSubmit, catsvc.ClaimActionApprove} {
		if _, err := claims.Act(context.Background(), catsvc.ClaimActionParams{
			WorkID: nsfwMember.ID, Action: action, Site: liveSite,
			ProductWorkID: &nsfwProduct, ActorUID: liveUID,
		}); err != nil {
			return fx, err
		}
	}
	if err := db.Create(&model.CatalogSeriesMember{SeriesID: se.ID, WorkID: nsfwMember.ID, Position: 2, Kind: model.SeriesMemberKindMain}).Error; err != nil {
		return fx, err
	}

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
	if err := db.Create(&model.CatalogCharacterAlias{
		CharacterID: ch.ID, Name: "Live Char Alias", Lang: "en",
		Kind: model.AliasKindTranslation, Provenance: model.AliasProvenanceSource,
	}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogCharacterIntro{
		CharacterID: ch.ID, Lang: "zh-Hans", Intro: "角色简介", SourceID: 2,
	}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeCharacter, EntityID: ch.ID, SourceID: 2,
		ExternalID: "c-live-1", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error; err != nil {
		return fx, err
	}

	// Lang deliberately unset: it is the null-side control for character.lang.
	chNoLang := &model.CatalogCharacter{DisplayName: "Live Char No Lang", Extra: empty, FieldProvenance: empty}
	if err := db.Create(chNoLang).Error; err != nil {
		return fx, err
	}
	fx.CharacterNoLang = chNoLang.ID

	birthY, birthM, gender := int16(1979), int16(4), model.GenderFemale
	pe := &model.CatalogPerson{
		DisplayName: "Live Person", PhotoHash: livePhotoHash,
		Gender: &gender, BirthY: &birthY, BirthM: &birthM, FieldProvenance: empty,
	}
	if err := db.Create(pe).Error; err != nil {
		return fx, err
	}
	fx.Person = pe.ID
	if err := db.Create(&model.CatalogPersonIntro{
		PersonID: pe.ID, Lang: "zh-Hans", Intro: "人物简介", SourceID: 3,
	}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypePerson, EntityID: pe.ID, SourceID: 10,
		ExternalID: "live_person", LinkKind: model.LinkKindRelated, MatchedBy: "test",
	}).Error; err != nil {
		return fx, err
	}

	cn := &model.CatalogCreditName{PersonID: &pe.ID, Name: "Live Credit", Lang: "ja", Kind: model.CreditNameKindMain, FieldProvenance: empty}
	if err := db.Create(cn).Error; err != nil {
		return fx, err
	}
	fx.Credit = cn.ID
	if err := db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeCreditName, EntityID: cn.ID, SourceID: 2,
		ExternalID: "s-live-va", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogNameAlias{
		CreditNameID: cn.ID, Name: "Live Credit Alias", Lang: "en",
		Kind: model.AliasKindTranslation, Provenance: model.AliasProvenanceSource,
	}).Error; err != nil {
		return fx, err
	}

	// Lang deliberately unset: it is the null-side control for credit_name.lang.
	sib := &model.CatalogCreditName{PersonID: &pe.ID, Name: "Live Credit Sibling", Kind: model.CreditNameKindPenName, FieldProvenance: empty}
	if err := db.Create(sib).Error; err != nil {
		return fx, err
	}
	fx.Sibling = sib.ID

	for _, credit := range []*model.CatalogCredit{
		{WorkID: w.ID, CreditNameID: cn.ID, RoleID: 1, CharacterID: &ch.ID},
		{WorkID: w.ID, CreditNameID: cn.ID, RoleID: 2},
	} {
		if err := db.Create(credit).Error; err != nil {
			return fx, err
		}
	}

	rank := 12
	if err := db.Create(&model.CatalogWorkRating{
		WorkID: w.ID, SourceID: 3, Score: 7.9, VoteCount: 12, Rank: &rank,
		Distribution: datatypes.JSON([]byte(`{"10":2,"9":7,"7":3}`)),
		Stats:        datatypes.JSON([]byte(`{"average":8.1,"stdev":1.2}`)),
	}).Error; err != nil {
		return fx, err
	}
	if err := db.Create(&model.CatalogWorkRating{
		WorkID: w.ID, SourceID: 4, Score: 4.5, VoteCount: 4,
	}).Error; err != nil {
		return fx, err
	}

	tr := &model.CatalogCharacterTrait{
		VndbTID: "i99999", Name: "Live Trait", GroupTID: "i1", Alias: "", Description: "",
	}
	if err := db.Create(tr).Error; err != nil {
		return fx, err
	}
	fx.Trait = tr.ID
	if err := db.Create(&model.CatalogCharacterTraitLink{CharacterID: ch.ID, TraitID: tr.ID, SpoilerLevel: 0}).Error; err != nil {
		return fx, err
	}

	// Sorted after "Live Trait" by the trait read's ORDER BY group_tid, gorder, name,
	// so the default-ceiling assertions that index traits[0] keep their row.
	trMinor := &model.CatalogCharacterTrait{
		VndbTID: "i99998", Name: "Live Trait Minor", GroupTID: "i1", Alias: "", Description: "",
	}
	if err := db.Create(trMinor).Error; err != nil {
		return fx, err
	}
	fx.TraitMinor = trMinor.ID
	if err := db.Create(&model.CatalogCharacterTraitLink{CharacterID: ch.ID, TraitID: trMinor.ID, SpoilerLevel: 1}).Error; err != nil {
		return fx, err
	}

	cv := &model.CatalogWorkCover{WorkID: w.ID, ImageHash: "livecoverhash1", Kind: "main", SourceID: 2}
	if err := db.Create(cv).Error; err != nil {
		return fx, err
	}
	fx.Cover = cv.ID

	if err := seedLiveBulkBlocks(db, &fx, empty); err != nil {
		return fx, err
	}
	if err := seedLiveAnchoredPerson(db, &fx, empty); err != nil {
		return fx, err
	}

	newsID, nerr := seedLiveNews(db)
	if nerr != nil {
		return fx, nerr
	}
	fx.NewsItem = newsID
	return fx, nil
}

// The bulk rows hang off their OWN work and character: the spoiler-ceiling and
// include-block tests assert exact row sets on fx.Work / fx.Character, and
// 105 extra rows there would have rewritten those expectations instead of
// testing the cap. Every row is spoiler none so the default ceiling admits all
// of them and the expected count is exactly liveBulkRows.
func seedLiveBulkBlocks(db *gorm.DB, fx *liveFix, empty datatypes.JSON) error {
	bulk := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Bulk Block Work",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	if err := db.Create(bulk).Error; err != nil {
		return err
	}
	fx.BulkWork = bulk.ID
	tags := make([]*model.CatalogWorkTag, 0, liveBulkRows)
	for i := range liveBulkRows {
		tags = append(tags, &model.CatalogWorkTag{
			WorkID: bulk.ID, Name: fmt.Sprintf("bulk-tag-%03d", i),
			Count: 1, SourceID: 2, Spoiler: 0,
		})
	}
	if err := db.Create(&tags).Error; err != nil {
		return err
	}

	bulkChar := &model.CatalogCharacter{DisplayName: "Bulk Block Char", Lang: "ja", Extra: empty, FieldProvenance: empty}
	if err := db.Create(bulkChar).Error; err != nil {
		return err
	}
	fx.BulkCharacter = bulkChar.ID
	for i := range liveBulkRows {
		tr := &model.CatalogCharacterTrait{
			VndbTID: fmt.Sprintf("i8%04d", i), Name: fmt.Sprintf("Bulk Trait %03d", i),
			GroupTID: "i1", Alias: "", Description: "",
		}
		if err := db.Create(tr).Error; err != nil {
			return err
		}
		if err := db.Create(&model.CatalogCharacterTraitLink{
			CharacterID: bulkChar.ID, TraitID: tr.ID, SpoilerLevel: 0,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// A second person, because fx.Person is the fixture that proves a person with
// ONLY a related link answers exactly one link.
func seedLiveAnchoredPerson(db *gorm.DB, fx *liveFix, empty datatypes.JSON) error {
	pe := &model.CatalogPerson{DisplayName: "Anchored Person", FieldProvenance: empty}
	if err := db.Create(pe).Error; err != nil {
		return err
	}
	fx.AnchoredPerson = pe.ID
	for _, ref := range []*model.CatalogExternalRef{
		{SourceID: 2, ExternalID: "s131", LinkKind: model.LinkKindExact},
		{SourceID: 3, ExternalID: "4423", LinkKind: model.LinkKindExact},
		{SourceID: 4, ExternalID: "creater-99", LinkKind: model.LinkKindExact},
		{SourceID: 5, ExternalID: "8080", LinkKind: model.LinkKindExact},
		{SourceID: 2, ExternalID: "v777", LinkKind: model.LinkKindProbable},
	} {
		ref.EntityType, ref.EntityID, ref.MatchedBy = model.EntityTypePerson, pe.ID, "test"
		if err := db.Create(ref).Error; err != nil {
			return err
		}
	}

	cn := &model.CatalogCreditName{
		PersonID: &pe.ID, Name: "Anchored Credit", Lang: "ja",
		Kind: model.CreditNameKindMain, FieldProvenance: empty,
	}
	if err := db.Create(cn).Error; err != nil {
		return err
	}
	fx.AnchoredCredit = cn.ID
	// A bare numeric alias id, the shape production actually stores on a credit
	// name — "1", "10", "100". Rendering it as a vndb URL reaches /1, an
	// unrelated VN. The links block must not grow one from this row.
	return db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeCreditName, EntityID: cn.ID, SourceID: 2,
		ExternalID: "1234", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error
}

// seedLiveNews leaves ONE published item behind for the spec walk to address.
// Nothing may mutate it: the walk sends PATCH {} at it, which must stay a
// validation refusal rather than a state change.
func seedLiveNews(db *gorm.DB) (int64, error) {
	if err := newstest.Truncate(db); err != nil {
		return 0, err
	}
	if err := db.Exec(`
		INSERT INTO news_source (key, display_name, homepage_url, attribution, publisher_uid, column_url, active)
		VALUES (?, 'Moyu', 'https://example.test', 'attribution text', ?, '', true)
		ON CONFLICT (key) DO UPDATE SET publisher_uid = EXCLUDED.publisher_uid, active = true`,
		liveNewsSource, liveUID).Error; err != nil {
		return 0, err
	}
	item := newsmodel.NewsItem{
		SourceKey: liveNewsSource, Lane: newsmodel.LaneNews, ExternalID: "seed-published",
		Title: "Seeded", Preview: "Seeded lede", SourceURL: "https://example.test/seed",
		PublishedAt: time.Now().UTC(), Status: newsmodel.StatusPublished,
	}
	if err := db.Create(&item).Error; err != nil {
		return 0, err
	}
	return item.ID, nil
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

func liveETag(t *testing.T, env *liveEnv, path, token string) string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := env.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode, path)
	tag := resp.Header.Get("ETag")
	require.NotEmpty(t, tag, "no ETag on "+path)
	return tag
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
	case strings.Contains(path, "/news"):
		id = fx.NewsItem
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
