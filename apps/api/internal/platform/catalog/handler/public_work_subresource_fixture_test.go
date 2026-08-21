package handler

import (
	"testing"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const subresCDN = "https://cdn.subresource.test"

func workSubresourceApp(db *gorm.DB) *fiber.App {
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, subresCDN)
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/works/search", h.WorksSearch)
	app.Get("/v1/catalog/works/:id", h.WorkDetail)
	app.Get("/v1/catalog/works/:id/covers", h.WorkCovers)
	app.Get("/v1/catalog/works/:id/screenshots", h.WorkScreenshots)
	app.Get("/v1/catalog/works/:id/tags", h.WorkTags)
	app.Get("/v1/catalog/works/:id/characters", h.WorkCharacters)
	app.Get("/v1/catalog/works/:id/credits", h.WorkCredits)
	app.Get("/v1/catalog/works/:id/releases", h.WorkReleases)
	app.Get("/v1/catalog/works/:id/intros", h.WorkIntros)
	app.Get("/v1/catalog/works/:id/ratings", h.WorkRatings)
	app.Get("/v1/catalog/works/:id/relations", h.WorkRelations)
	app.Get("/v1/catalog/works/:id/series", h.WorkSeries)
	app.Get("/v1/catalog/works/:id/links", h.WorkLinks)
	app.Get("/v1/catalog/works/:id/engines", h.WorkEngines)
	return app
}

// workSubresourceLanes pairs every sub-resource path with the key of the
// works/{id} block it pages. The parity test walks this list, so a thirteenth
// endpoint added without a parent block to compare against has nowhere to go.
var workSubresourceLanes = []struct{ suffix, block string }{
	{"covers", "covers"},
	{"screenshots", "screenshots"},
	{"tags", "tags"},
	{"characters", "characters"},
	{"credits", "credits"},
	{"releases", "releases"},
	{"intros", "intros"},
	{"ratings", "ratings"},
	{"relations", "relations"},
	{"series", "series"},
	{"links", "links"},
	{"engines", "engines"},
}

type richWork struct {
	work       int64
	related    int64
	relatedR18 int64
	r18        int64
	stub       int64
	deleted    int64
}

func seedRichWork(t *testing.T, db *gorm.DB) richWork {
	t.Helper()
	ensureGalgameStub(t, db)
	for _, tbl := range []string{
		"catalog_work_cover", "catalog_work_screenshot", "catalog_work_tag", "catalog_work_character",
		"catalog_credit", "catalog_work_intro", "catalog_work_rating", "catalog_work_relation",
		"catalog_series_member", "catalog_series", "catalog_work_engine", "catalog_engine",
		"catalog_release_label", "catalog_work_label", "catalog_label", "catalog_label_alias",
		"catalog_external_ref", "catalog_release", "catalog_work_title", "catalog_work",
		"catalog_character", "catalog_character_alias", "catalog_credit_name", "catalog_name_alias",
		"catalog_tag_source_map", "catalog_tag", "catalog_tag_work_count", "edit_suppressed_row",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}

	var srcVNDB, srcBangumi, srcTwitter int16
	db.Raw("SELECT id FROM catalog_source WHERE key='vndb'").Scan(&srcVNDB)
	db.Raw("SELECT id FROM catalog_source WHERE key='bangumi'").Scan(&srcBangumi)
	db.Raw("SELECT id FROM catalog_source WHERE key='twitter'").Scan(&srcTwitter)
	require.NotZero(t, srcVNDB)
	require.NotZero(t, srcBangumi)
	require.NotZero(t, srcTwitter, "the twitter source key is what makes a related-kind ref render as a link")
	var roleScenarioID int64
	db.Raw("SELECT id FROM catalog_role WHERE key='scenario'").Scan(&roleScenarioID)
	require.NotZero(t, roleScenarioID)

	claimed := int64(0)
	// The work is CLAIMED and live because the nsfw-aware work_count on tags and
	// engines is counted over claimed-live works only: an unclaimed fixture makes
	// every count 0, which is the one value a page-scoped and a block-scoped
	// count cannot disagree on.
	newWork := func(name string, rating, status int16) int64 {
		w := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: name, ContentRating: rating, Status: status}
		claimed++
		w.Site, w.ProductWorkID = strptr("galgame_wiki"), ptrI64(770000+claimed)
		require.NoError(t, db.Create(&w).Error)
		return w.ID
	}
	f := richWork{
		work:       newWork("子資源の親作品", model.ContentRatingAllAges, model.WorkStatusLive),
		related:    newWork("続編", model.ContentRatingAllAges, model.WorkStatusLive),
		relatedR18: newWork("成人版", model.ContentRatingR18, model.WorkStatusLive),
		r18:        newWork("R18 単体", model.ContentRatingR18, model.WorkStatusLive),
		stub:       newWork("スタブ", model.ContentRatingAllAges, model.WorkStatusStub),
		deleted:    newWork("削除済み", model.ContentRatingAllAges, model.WorkStatusLive),
	}
	require.NoError(t, db.Delete(&model.CatalogWork{}, f.deleted).Error)

	for _, tl := range []struct {
		work  int64
		lang  string
		title string
	}{
		{f.work, "ja", "子資源の親作品"},
		{f.work, "zh-Hans", "子资源的父作品"},
		{f.related, "zh-Hans", "续编"},
	} {
		require.NoError(t, db.Create(&model.CatalogWorkTitle{
			WorkID: tl.work, Lang: tl.lang, Title: tl.title,
			Kind: model.WorkTitleKindOfficial, Provenance: model.WorkTitleProvenanceSource,
		}).Error)
	}

	for i, h := range []string{"cover-a", "cover-b", "cover-c"} {
		require.NoError(t, db.Create(&model.CatalogWorkCover{
			WorkID: f.work, ImageHash: h, SortOrder: i, Kind: "pkgfront",
			Sexual: int16(i), Violence: 0, SourceID: srcVNDB,
		}).Error)
	}
	require.NoError(t, db.Create(&model.CatalogWorkCover{
		WorkID: f.work, ImageHash: "", SortOrder: 9, SourceID: srcVNDB,
	}).Error)
	for i, h := range []string{"shot-a", "shot-b", "shot-c"} {
		require.NoError(t, db.Create(&model.CatalogWorkScreenshot{
			WorkID: f.work, ImageHash: h, SortOrder: i, Caption: "場面" + string(rune('A'+i)), SourceID: srcVNDB,
		}).Error)
	}

	for _, tg := range []model.CatalogWorkTag{
		{WorkID: f.work, Name: "純愛", Count: 90, SourceID: srcBangumi},
		{WorkID: f.work, Name: "学園", Count: 40, SourceID: srcBangumi},
		{WorkID: f.work, Name: "泣きゲー", Count: 10, SourceID: srcVNDB},
		{WorkID: f.work, Name: "重大ネタバレ", Count: 5, SourceID: srcVNDB, Spoiler: 2},
	} {
		require.NoError(t, db.Create(&tg).Error)
	}
	canonical := model.CatalogTag{Name: "pure love", Tier: model.TagTierCore, Kind: model.TagKindContent}
	require.NoError(t, db.Create(&canonical).Error)
	require.NoError(t, db.Create(&model.CatalogTagSourceMap{
		SourceID: srcBangumi, SourceName: "純愛", TagID: canonical.ID,
	}).Error)

	chars := make([]int64, 0, 2)
	for i, name := range []string{"主人公", "ヒロイン"} {
		ch := model.CatalogCharacter{DisplayName: name, Lang: "ja"}
		require.NoError(t, db.Create(&ch).Error)
		chars = append(chars, ch.ID)
		require.NoError(t, db.Create(&model.CatalogWorkCharacter{
			WorkID: f.work, CharacterID: ch.ID, Kind: int16(i + 1), Spoiler: int16(i * 2), MatchedBy: "import:test",
		}).Error)
		require.NoError(t, db.Create(&model.CatalogCharacterAlias{
			CharacterID: ch.ID, Name: name + "(中)", Lang: "zh-Hans", Kind: model.AliasKindTranslation,
		}).Error)
	}

	src := srcVNDB
	for i, name := range []string{"声優あ", "声優い"} {
		cn := model.CatalogCreditName{Name: name, Lang: "ja"}
		require.NoError(t, db.Create(&cn).Error)
		require.NoError(t, db.Create(&model.CatalogCredit{
			WorkID: f.work, CreditNameID: cn.ID, RoleID: roleVoiceActor,
			CharacterID: &chars[i], SourceID: &src,
		}).Error)
	}
	for _, name := range []string{"脚本あ", "脚本い", "脚本う"} {
		cn := model.CatalogCreditName{Name: name, Lang: "ja"}
		require.NoError(t, db.Create(&cn).Error)
		require.NoError(t, db.Create(&model.CatalogCredit{
			WorkID: f.work, CreditNameID: cn.ID, RoleID: roleScenarioID, SourceID: &src,
		}).Error)
	}

	label := model.CatalogLabel{DisplayName: "テストブランド", Kind: model.LabelKindGameBrand, Lang: "ja"}
	require.NoError(t, db.Create(&label).Error)
	require.NoError(t, db.Create(&model.CatalogWorkLabel{
		WorkID: f.work, LabelID: label.ID, Kind: model.WorkLabelKindBrand,
	}).Error)

	y, m, d := int16(2019), int16(6), int16(28)
	for i, kind := range []int16{model.ReleaseKindDigital, model.ReleaseKindPhysical} {
		rel := model.CatalogRelease{
			WorkID: f.work, Kind: kind, ReleasedY: &y, ReleasedM: &m, ReleasedD: &d,
			Lang: strptr("ja"), Platform: strptr("win"),
		}
		require.NoError(t, db.Create(&rel).Error)
		require.NoError(t, db.Create(&model.CatalogExternalRef{
			EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: srcVNDB,
			ExternalID: "r" + itoa(int64(i+1)), LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
		}).Error)
		require.NoError(t, db.Create(&model.CatalogReleaseLabel{
			ReleaseID: rel.ID, LabelID: label.ID, Kind: model.WorkLabelKindPublisher,
		}).Error)
	}

	require.NoError(t, db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: f.work, SourceID: srcTwitter,
		ExternalID: "subres_test", LinkKind: model.LinkKindRelated, MatchedBy: "rule:test",
	}).Error)

	for _, in := range []model.CatalogWorkIntro{
		{WorkID: f.work, Lang: "ja", Intro: "あらすじ。", SourceID: srcVNDB},
		{WorkID: f.work, Lang: "zh-Hans", Intro: "简介。", SourceID: srcBangumi, Provenance: 1},
	} {
		require.NoError(t, db.Create(&in).Error)
	}

	rank := 42
	require.NoError(t, db.Create(&model.CatalogWorkRating{
		WorkID: f.work, SourceID: srcVNDB, Score: 8.4, VoteCount: 1200, Rank: &rank,
		Distribution: datatypes.JSON(`{"8":700,"9":500}`), Stats: datatypes.JSON(`{"average":8.1}`),
	}).Error)
	require.NoError(t, db.Create(&model.CatalogWorkRating{
		WorkID: f.work, SourceID: srcBangumi, Score: 7.9, VoteCount: 800,
		Distribution: datatypes.JSON(`{"7":300,"8":500}`),
	}).Error)

	for _, rel := range []model.CatalogWorkRelation{
		{AWorkID: f.work, BWorkID: f.related, RelationTypeID: 2},
		{AWorkID: f.work, BWorkID: f.relatedR18, RelationTypeID: 4},
	} {
		require.NoError(t, db.Create(&rel).Error)
	}

	series := model.CatalogSeries{DisplayName: "テストシリーズ", SourceID: srcVNDB, ExternalID: "s1"}
	require.NoError(t, db.Create(&series).Error)
	for _, w := range []int64{f.work, f.related} {
		require.NoError(t, db.Create(&model.CatalogSeriesMember{SeriesID: series.ID, WorkID: w}).Error)
	}

	for _, name := range []string{"KiriKiri", "Artemis"} {
		e := model.CatalogEngine{Name: name, Aliases: datatypes.JSON(`[]`)}
		require.NoError(t, db.Create(&e).Error)
		require.NoError(t, db.Create(&model.CatalogWorkEngine{
			WorkID: f.work, EngineID: e.ID, SourceID: srcVNDB,
		}).Error)
	}
	return f
}
