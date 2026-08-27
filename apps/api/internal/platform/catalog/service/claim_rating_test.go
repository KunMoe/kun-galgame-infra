package service

import (
	"testing"
	"time"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"
	srcb "api/internal/platform/catalog/srcbangumi"
	srcv "api/internal/platform/catalog/srcvndb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func seedRatingSources(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`TRUNCATE src_vndb.releases, src_vndb.releases_vn, src_bangumi.subject RESTART IDENTITY CASCADE`).Error)
	age18, age0 := int16(18), int16(0)
	for _, r := range []srcv.Release{
		{ID: "r1", OLang: "ja", MinAge: &age18},
		{ID: "r2", OLang: "ja", MinAge: &age18, Patch: true},
		{ID: "r3", OLang: "ja", MinAge: &age0},
	} {
		require.NoError(t, testDB.Create(&r).Error)
	}
	for _, rv := range []srcv.ReleaseVN{
		{ID: "r1", VID: "v1", RType: "complete"},
		{ID: "r2", VID: "v2", RType: "complete"},
		{ID: "r3", VID: "v3", RType: "complete"},
	} {
		require.NoError(t, testDB.Create(&rv).Error)
	}
	for _, sub := range []srcb.Subject{
		{ID: 801, Type: 4, Name: "sub-801", NSFW: true,
			ParserVersion: srcb.ParserVersion, IngestedAt: time.Now()},
		{ID: 802, Type: 4, Name: "sub-802", MetaTags: datatypes.JSON([]byte(`["游戏", "R18"]`)),
			ParserVersion: srcb.ParserVersion, IngestedAt: time.Now()},
		{ID: 803, Type: 4, Name: "sub-803", MetaTags: datatypes.JSON([]byte(`["游戏", "全年龄"]`)),
			ParserVersion: srcb.ParserVersion, IngestedAt: time.Now()},
	} {
		require.NoError(t, testDB.Create(&sub).Error)
	}
}

func TestDeriveContentRatingFromRefs(t *testing.T) {
	cleanTables(t)
	seedRatingSources(t)
	s := NewClaimLifecycleService(testDB)
	ctx := t.Context()

	cases := []struct {
		name string
		refs []ClaimRef
		want int16
	}{
		{"vndb minage 18", []ClaimRef{{Source: "vndb", ExternalID: "v1"}}, model.ContentRatingR18},
		{"vndb 18+ patch only", []ClaimRef{{Source: "vndb", ExternalID: "v2"}}, 0},
		{"vndb all ages", []ClaimRef{{Source: "vndb", ExternalID: "v3"}}, 0},
		{"vndb release-level ref", []ClaimRef{{Source: "vndb", ExternalID: "r1"}}, model.ContentRatingR18},
		{"bangumi nsfw flag", []ClaimRef{{Source: "bangumi", ExternalID: "801"}}, model.ContentRatingR18},
		{"bangumi R18 meta_tag", []ClaimRef{{Source: "bangumi", ExternalID: "802"}}, model.ContentRatingR18},
		{"bangumi plain", []ClaimRef{{Source: "bangumi", ExternalID: "803"}}, 0},
		{"unknown source", []ClaimRef{{Source: "dlsite", ExternalID: "RJ01"}}, 0},
		{"second ref decides", []ClaimRef{{Source: "vndb", ExternalID: "v3"}, {Source: "bangumi", ExternalID: "801"}}, model.ContentRatingR18},
		{"no refs", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, s.DeriveContentRating(ctx, c.refs))
		})
	}
}

func TestSubmitWorkCarriesDerivedRating(t *testing.T) {
	s := newLifecycle(t)
	res, err := s.SubmitWork(t.Context(), SubmitWorkParams{
		Site: submitSite, ProductWorkID: 90055, ActorUID: 7,
		ContentRating: model.ContentRatingR18,
		Fields:        map[string]any{editspec.FieldWorkDisplayName: "派生分级"},
	})
	require.NoError(t, err)
	var w model.CatalogWork
	require.NoError(t, testDB.First(&w, res.WorkID).Error)
	assert.Equal(t, model.ContentRatingR18, w.ContentRating)
}

func TestClaimWorkDerivesRatingFromAnchors(t *testing.T) {
	cleanTables(t)
	seedRatingSources(t)
	ctx := t.Context()
	var vndbID int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'vndb'`).Scan(&vndbID).Error)

	id, created, err := testWork.ClaimWork(ctx, ClaimWorkParams{
		MediumID: 1, Site: "kungal", ProductWorkID: 4242, DisplayName: "derived",
		Anchors: []ExternalAnchor{{SourceID: vndbID, ExternalID: "v1", MatchedBy: "rule:test", EntityType: model.EntityTypeWork}},
	})
	require.NoError(t, err)
	require.True(t, created)
	var w model.CatalogWork
	require.NoError(t, testDB.First(&w, id).Error)
	assert.Equal(t, model.ContentRatingR18, w.ContentRating)

	id2, created2, err := testWork.ClaimWork(ctx, ClaimWorkParams{
		MediumID: 1, Site: "kungal", ProductWorkID: 4243, DisplayName: "caller wins",
		ContentRating: model.ContentRatingSensitive,
		Anchors:       []ExternalAnchor{{SourceID: vndbID, ExternalID: "v2", MatchedBy: "rule:test", EntityType: model.EntityTypeWork}},
	})
	require.NoError(t, err)
	require.True(t, created2)
	var w2 model.CatalogWork
	require.NoError(t, testDB.First(&w2, id2).Error)
	assert.Equal(t, model.ContentRatingSensitive, w2.ContentRating,
		"an explicit caller rating is never re-derived")
}
