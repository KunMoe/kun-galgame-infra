package service

import (
	"testing"
	"time"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setCreditSource(t *testing.T, creditID int64, sourceID *int16) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`UPDATE catalog_credit SET source_id = ? WHERE id = ?`, sourceID, creditID).Error)
}

func i16p(v int16) *int16 { return &v }

// D15: HumanLaneFirstNoProvenanceSQL emitted `(source_id IN (1,12)) DESC`, and a
// NULL source_id makes that expression NULL. ORDER BY ... DESC is NULLS FIRST,
// so the sourceless rows sorted ahead of the curated lane the term exists to
// promote — the lane was inverted for exactly the rows it was written for.
func TestCuratedCreditLaneIsNotInvertedByNullSources(t *testing.T) {
	cleanTables(t)
	w := createWork(t, "Curated Lane Work")
	curatedName := createCreditName(t, nil, "Curated Writer")
	nullName := createCreditName(t, nil, "Sourceless Writer")
	importedName := createCreditName(t, nil, "Imported Writer")

	curated := createCredit(t, w.ID, curatedName.ID, 1, nil)
	sourceless := createCredit(t, w.ID, nullName.ID, 1, nil)
	imported := createCredit(t, w.ID, importedName.ID, 1, nil)
	setCreditSource(t, curated.ID, i16p(12))
	setCreditSource(t, sourceless.ID, nil)
	setCreditSource(t, imported.ID, i16p(2))

	rows, err := NewReadService(testDB).WorkCredits(t.Context(), w.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	order := make([]int64, 0, len(rows))
	for _, r := range rows {
		order = append(order, r.CreditNameID)
	}
	require.Equal(t, curatedName.ID, order[0],
		"the curated row must lead its role group, got order %v", order)
	// Positive control: the term still orders something. Drop the curated row
	// and the two remaining rows must not both be "first".
	assert.Contains(t, order[1:], nullName.ID)
	assert.Contains(t, order[1:], importedName.ID)
}

// D16: the credits ORDER BY stopped at cn.id while uq_catalog_credit carries
// COALESCE(character_id, 0) as well, so one voice actor on three characters of
// one work produced three fully tied rows. works/{id}/credits pages by OFFSET,
// where a tie duplicates or drops a row at the page boundary.
func TestWorkCreditsOrderIsTotalAcrossCharacters(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()
	w := createWork(t, "Tied Credits Work")
	va := createCreditName(t, nil, "Busy Voice")
	var charIDs []int64
	for _, name := range []string{"Char A", "Char B", "Char C"} {
		charIDs = append(charIDs, createCharacter(t, name).ID)
	}
	// Inserted in DESCENDING character id so the physical row order disagrees
	// with the order the tiebreak asks for: without it the rows tie on every
	// term and come back in scan order, which is what this test must catch.
	for i := len(charIDs) - 1; i >= 0; i-- {
		createCredit(t, w.ID, va.ID, 1, &charIDs[i])
	}

	read := NewReadService(testDB)
	first, err := read.WorkCredits(t.Context(), w.ID)
	require.NoError(t, err)
	require.Len(t, first, len(charIDs))
	got := make([]int64, 0, len(first))
	for _, r := range first {
		require.NotNil(t, r.CharacterID)
		got = append(got, *r.CharacterID)
	}
	require.Equal(t, charIDs, got, "tied credit rows must come back on the declared tiebreak")

	// Page it one row at a time, exactly as works/{id}/credits does.
	seen := map[int64]int{}
	for offset := 0; offset < len(charIDs); offset++ {
		page, ok, perr := svc.WorkCredits(t.Context(), w.ID, false, 1, offset)
		require.NoError(t, perr)
		require.True(t, ok)
		require.Len(t, page.Items, 1)
		require.Len(t, page.Items[0].Credits, 1)
		seen[page.Items[0].Credits[0].CharacterID]++
	}
	require.Len(t, seen, len(charIDs), "an offset crawl must visit each row exactly once, got %v", seen)
	for id, n := range seen {
		require.Equal(t, 1, n, "character %d appeared %d times", id, n)
	}
}

// D19: claimed= tested only w.site while claim_state= also requires
// product_work_id, so a row with a site and no product id answered claimed=true
// and claim_state=none at the same time.
func TestClaimedFlagAgreesWithClaimState(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)
	wiki := "galgame_wiki"
	pw := int64(9301)

	full := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Fully Claimed")
	setClaimColumns(t, full.ID, &wiki, &pw, i16(model.ClaimStateLive))
	half := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Half Claimed")
	setClaimColumns(t, half.ID, &wiki, nil, nil)
	bare := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Unclaimed")

	yes, no := true, false
	claimedYes := idSet(listIDs(t, WorksListFilter{Sort: "id", Claimed: &yes}))
	claimedNo := idSet(listIDs(t, WorksListFilter{Sort: "id", Claimed: &no}))
	stateNone := idSet(listIDs(t, WorksListFilter{Sort: "id", ClaimStates: []string{model.ClaimStateKeyNone}}))

	require.True(t, claimedYes[full.ID], "the fully claimed row is the positive control")
	require.False(t, claimedYes[half.ID], "a site without a product work id is not a claim")
	require.True(t, claimedNo[half.ID])
	require.True(t, claimedNo[bare.ID])
	require.Equal(t, stateNone, claimedNo,
		"claimed=false and claim_state=none must select the same set")
}

// D20: the company graph walk stopped at 60 nodes / 4 hops and said so nowhere,
// so a partial family read as the complete one.
func TestCompanyGraphDeclaresItsTruncation(t *testing.T) {
	cleanTables(t)
	cleanLabelGraphTables(t)
	svc := newPublicSvc()

	// Six labels in a chain: the walk reaches five of them within four hops.
	ids := make([]int64, 0, 6)
	for _, name := range []string{"Chain 0", "Chain 1", "Chain 2", "Chain 3", "Chain 4", "Chain 5"} {
		ids = append(ids, mkLabel(t, name, ""))
	}
	for i := 0; i+1 < len(ids); i++ {
		relateMirrored(t, ids[i], ids[i+1], model.LabelRelationParent)
	}

	deep, found, err := svc.LabelRelationGraph(t.Context(), ids[0], false)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, deep.Nodes, labelGraphMaxDepth+1)
	require.True(t, deep.Truncated, "the chain continues past the depth ceiling and must say so")

	// Positive control: the same walk from the middle reaches both ends inside
	// four hops, so truncated must be false there. A flag that is always true
	// carries no information.
	whole, found, err := svc.LabelRelationGraph(t.Context(), ids[2], false)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, whole.Nodes, len(ids))
	require.False(t, whole.Truncated, "the whole family fits, so nothing was cut")
}

// D24: NamesList and PersonNames ignored link_visibility, which the credit-name
// detail face treats as the public gate on the person link.
func TestCreditNamePersonLinkFollowsLinkVisibility(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()
	p := createPerson(t, "Hidden Link Person")
	open := createCreditName(t, &p.ID, "Open Name")
	hidden := createCreditName(t, &p.ID, "Hidden Name")
	require.NoError(t, testDB.Exec(`UPDATE catalog_credit_name SET link_visibility = ? WHERE id = ?`,
		model.LinkVisibilityHidden, hidden.ID).Error)

	page, err := svc.NamesList(t.Context(), []int64{open.ID, hidden.ID}, "", "", 50)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	byID := map[int64]EntityListRow{}
	for _, it := range page.Items {
		byID[it.ID] = it
	}
	require.NotNil(t, byID[open.ID].PersonID, "the public link is the positive control")
	require.Equal(t, p.ID, *byID[open.ID].PersonID)
	require.Nil(t, byID[hidden.ID].PersonID, "a hidden link must not surface on the list lane")

	names, ok, err := svc.PersonNames(t.Context(), p.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, names, 1, "persons/{id}/credit-names must list only publicly linked names")
	require.Equal(t, open.ID, names[0].ID)
}

// D18: the redirect keyset compared a row value against a NULL merged_at, which
// is NULL and never true, so the row was skipped on every page — and the cursor
// only advanced on rows that had a merge time, so a crawl could not terminate
// once they became visible.
func TestRedirectFeedPagesThroughUndatedRows(t *testing.T) {
	cleanTables(t)
	require.NoError(t, testDB.Exec("TRUNCATE catalog_redirect").Error)
	resolve := NewResolveService(repository.NewRedirectRepository(testDB))

	merged := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, r := range []*model.CatalogRedirect{
		{EntityType: model.EntityTypeWork, OldID: 5001, CurrentID: 1},
		{EntityType: model.EntityTypeWork, OldID: 5002, CurrentID: 1, MergedAt: &merged},
	} {
		require.NoError(t, testDB.Create(r).Error)
	}

	total, err := resolve.RedirectsTotal(t.Context(), nil)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)

	seen := map[int64]int{}
	cur := repository.RedirectCursor{}
	for range 10 {
		items, next, cerr := resolve.RedirectsSince(t.Context(), nil, cur, 1)
		require.NoError(t, cerr)
		if len(items) == 0 {
			break
		}
		for _, it := range items {
			seen[it.OldID]++
		}
		require.NotEqual(t, cur, next, "the cursor must advance on every row it returned")
		cur = next
	}
	require.Equal(t, map[int64]int{5001: 1, 5002: 1}, seen,
		"a one-row-at-a-time crawl must see both redirects exactly once")
}
