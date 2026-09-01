package service

import (
	"strings"
	"testing"

	"api/internal/platform/catalog/model"
	catsearch "api/internal/platform/catalog/search"
)

const introOnlyTerm = "夏至祭"

func seedIntroCorpus(t *testing.T) (*PublicService, map[string]int64) {
	t.Helper()
	cleanTables(t)
	idx := worksSearchIndexer(t)
	svc := newPublicSvc().WithWorksSearch(idx)

	withIntro := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "陽だまりの詩")
	titled := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "灯火の記憶")
	plain := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "無縁の物語")

	indexWorks(t, idx, []catsearch.WorkDocInput{
		{
			ID: withIntro.ID, DisplayName: "陽だまりの詩", OLang: "ja",
			ContentRating: model.ContentRatingAllAges, UpdatedTS: 1700000001, Popularity: 1,
			SourceKeys: []string{"vndb"},
			Intros: []catsearch.WorkDocIntro{
				{Lang: "ja", Text: "主人公は" + introOnlyTerm + "の夜に灯火を見つける物語。"},
				{Lang: "zh-Hans", Text: "主角在祭典之夜找到灯火的故事。"},
			},
		},
		{
			ID: titled.ID, DisplayName: "灯火の記憶", OLang: "ja",
			ContentRating: model.ContentRatingAllAges, UpdatedTS: 1700000002, Popularity: 1,
			SourceKeys: []string{"vndb"},
		},
		{
			ID: plain.ID, DisplayName: "無縁の物語", OLang: "ja",
			ContentRating: model.ContentRatingAllAges, UpdatedTS: 1700000003, Popularity: 1,
			SourceKeys: []string{"vndb"},
		},
	})
	return svc, map[string]int64{"intro": withIntro.ID, "titled": titled.ID, "plain": plain.ID}
}

func TestWorksSearchIntroIsOptIn(t *testing.T) {
	svc, ids := seedIntroCorpus(t)

	if got := searchIDs(t, svc, WorksSearchFilter{Q: introOnlyTerm}); len(got) != 0 {
		t.Fatalf("default search matched the synopsis-only term: %v — attributesToSearchOn restriction is gone", got)
	}
	got := searchIDs(t, svc, WorksSearchFilter{Q: introOnlyTerm, SearchIntro: true})
	if len(got) != 1 || got[0] != ids["intro"] {
		t.Fatalf("search_intro=1 got %v, want exactly [%d]", got, ids["intro"])
	}

	for _, si := range []bool{false, true} {
		got := searchIDs(t, svc, WorksSearchFilter{Q: "陽だまり", SearchIntro: si})
		if len(got) != 1 || got[0] != ids["intro"] {
			t.Fatalf("title search (search_intro=%v) got %v, want [%d]", si, got, ids["intro"])
		}
	}
}

func TestWorksSearchIntroNeverOutranksATitle(t *testing.T) {
	svc, ids := seedIntroCorpus(t)

	got := searchIDs(t, svc, WorksSearchFilter{Q: "灯火", SearchIntro: true})
	if len(got) != 2 {
		t.Fatalf("got %v, want both the titled work and the synopsis mention", got)
	}
	if got[0] != ids["titled"] {
		t.Fatalf("ranked %v first; a title match must precede a synopsis match (want %d)", got[0], ids["titled"])
	}
}

func TestWorksSearchIntroIsLanguageBucketed(t *testing.T) {
	svc, ids := seedIntroCorpus(t)

	got := searchIDs(t, svc, WorksSearchFilter{Q: "祭典之夜", SearchIntro: true})
	if len(got) != 1 || got[0] != ids["intro"] {
		t.Fatalf("zh synopsis search got %v, want [%d]", got, ids["intro"])
	}
}

func TestWorkDocIntroBucketingAndTruncation(t *testing.T) {
	long := strings.Repeat("あ", catsearch.IntroMaxRunes+500)
	doc := catsearch.BuildWorkDoc(catsearch.WorkDocInput{
		ID: 1, DisplayName: "Doc", OLang: "ja",
		Intros: []catsearch.WorkDocIntro{
			{Lang: "ja", Text: long},
			{Lang: "zh-Hans", Text: "简体"},
			{Lang: "zh-Hant", Text: "繁體"},
			{Lang: "en", Text: "english"},
			{Lang: "ja", Text: "  "},
		},
	})

	if n := len([]rune(doc.IntroJa)); n != catsearch.IntroMaxRunes {
		t.Fatalf("ja intro = %d runes, want the %d cap", n, catsearch.IntroMaxRunes)
	}
	if !utf8Valid(doc.IntroJa) {
		t.Fatalf("truncation cut mid-rune — the document would be invalid UTF-8")
	}
	if doc.IntroZh != "简体\n繁體" {
		t.Fatalf("zh bucket = %q, want both variants joined", doc.IntroZh)
	}
	if doc.IntroOther != "english" {
		t.Fatalf("other bucket = %q", doc.IntroOther)
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestTagSexualReachesBothFacesFromTheColumn(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)
	svc := newPublicSvc()
	ctx := t.Context()

	sexualTag := createCanonicalTag(t, "エロ(a2-1f)", model.TagTierCore, model.TagKindContent)
	contentTag := createCanonicalTag(t, "純愛(a2-1f)", model.TagTierCore, model.TagKindContent)
	techTag := createCanonicalTag(t, "実写(a2-1f)", model.TagTierCore, model.TagKindMeta)
	folkTag := createCanonicalTag(t, "百合(a2-1f)", model.TagTierCore, model.TagKindContent)
	if err := testDB.Exec(
		`UPDATE catalog_tag SET sexual = true WHERE id = ?`, sexualTag).Error; err != nil {
		t.Fatalf("flag the sexual tag: %v", err)
	}

	want := map[int64]bool{
		sexualTag: true, contentTag: false, techTag: false, folkTag: false,
	}

	list, err := svc.TagsList(ctx, TagsListFilter{}, "", 50)
	if err != nil {
		t.Fatalf("TagsList: %v", err)
	}
	seen := 0
	for _, it := range list.Items {
		exp, ok := want[it.ID]
		if !ok {
			continue
		}
		if it.Sexual != exp {
			t.Fatalf("list tag %d (%s) sexual=%v, want %v", it.ID, it.Name, it.Sexual, exp)
		}
		seen++
	}
	if seen != len(want) {
		t.Fatalf("browse lane covered %d fixture tags, want %d", seen, len(want))
	}

	for id, exp := range want {
		rec, found, err := svc.TagDetail(ctx, id, false, false, 50, 0)
		if err != nil || !found {
			t.Fatalf("TagDetail %d: found=%v err=%v", id, found, err)
		}
		if rec.Sexual != exp {
			t.Fatalf("detail tag %d sexual=%v, want %v", id, rec.Sexual, exp)
		}
	}
}
