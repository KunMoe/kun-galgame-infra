package personmint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcb "api/internal/platform/catalog/srcbangumi"
	srcv "api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	testDB  *gorm.DB
	testDSN string
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/personmint")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/personmint", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/personmint", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/personmint", "catalog seed failed: %v", err)
	}
	if err := srcv.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/personmint", "src_vndb schema failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/personmint", "src_bangumi schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_external_ref, catalog_credit_name, catalog_person,
		catalog_label_alias, catalog_label, src_vndb.staff_alias, src_vndb.staff, src_bangumi.person
		RESTART IDENTITY CASCADE`).Error)
}

func mkCreditName(t *testing.T, name string, personID *int64) int64 {
	t.Helper()
	cn := model.CatalogCreditName{Name: name, PersonID: personID, Kind: model.CreditNameKindMain}
	require.NoError(t, testDB.Create(&cn).Error)
	return cn.ID
}

func mkPerson(t *testing.T, displayName string, gender *int16) int64 {
	t.Helper()
	p := model.CatalogPerson{DisplayName: displayName, Gender: gender, FieldProvenance: datatypes.JSON("{}")}
	require.NoError(t, testDB.Create(&p).Error)
	return p.ID
}

func mkAnchor(t *testing.T, creditNameID int64, source int16, externalID string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeCreditName, EntityID: creditNameID,
		SourceID: source, ExternalID: externalID, LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error)
}

func mkStaff(t *testing.T, id string, mainAID int, gender string, aliases map[int]string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcv.Staff{ID: id, Main: mainAID, Gender: gender}).Error)
	for aid, name := range aliases {
		require.NoError(t, testDB.Create(&srcv.StaffAlias{AID: aid, ID: id, Name: name}).Error)
	}
}

func mkBGMPerson(t *testing.T, id int64, name, infobox string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Person{
		ID: id, Name: name, Type: 1, InfoboxParsed: datatypes.JSON(infobox),
		ParserVersion: "test", IngestedAt: time.Now(),
	}).Error)
}

func mkLabel(t *testing.T, name string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogLabel{DisplayName: name, Kind: model.LabelKindDoujinCircle}).Error)
}

func sourceID(t *testing.T, key string) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func person(t *testing.T, id int64) model.CatalogPerson {
	t.Helper()
	var p model.CatalogPerson
	require.NoError(t, testDB.First(&p, id).Error)
	return p
}

func personIDOf(t *testing.T, creditNameID int64) *int64 {
	t.Helper()
	var cn model.CatalogCreditName
	require.NoError(t, testDB.First(&cn, creditNameID).Error)
	return cn.PersonID
}

func personAnchors(t *testing.T, personID int64) []string {
	t.Helper()
	var rows []model.CatalogExternalRef
	require.NoError(t, testDB.Where("entity_type = ? AND entity_id = ?", model.EntityTypePerson, personID).
		Order("source_id, external_id").Find(&rows).Error)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%d:%s:%s", r.SourceID, r.ExternalID, r.MatchedBy))
	}
	return out
}

func countRows(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	q := testDB.Table(table)
	if where != "" {
		q = q.Where(where, args...)
	}
	require.NoError(t, q.Count(&n).Error)
	return n
}

func clusterFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clusters.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}

func clusterLine(id, tier string, members ...int64) string {
	b, _ := json.Marshal(Cluster{ClusterID: id, CreditNameIDs: members, Tier: tier})
	return string(b)
}

func TestRun(t *testing.T) {
	clean(t)
	ctx := t.Context()
	vndb, bgm := sourceID(t, "vndb"), sourceID(t, "bangumi")
	dlsite, eg := sourceID(t, "dlsite"), sourceID(t, "erogamescape")

	caPen := mkCreditName(t, "ひと美", nil)
	caMain := mkCreditName(t, "北都南", nil)
	mkStaff(t, "s1", 101, "f", map[int]string{100: "ひと美", 101: "北都南"})
	mkAnchor(t, caPen, vndb, "100")
	mkAnchor(t, caMain, vndb, "101")
	mkAnchor(t, caMain, bgm, "500")
	mkAnchor(t, caPen, dlsite, "900")
	mkBGMPerson(t, 500, "北都南", `{"Fields":[{"Key":"性别","Value":"女"},{"Key":"生日","Value":"1978年3月4日"}]}`)

	female := model.GenderFemale
	hostB := mkPerson(t, "既存人物", &female)
	cbLinked := mkCreditName(t, "既存名義", &hostB)
	cbOrphan := mkCreditName(t, "別名義", nil)
	mkAnchor(t, cbLinked, bgm, "501")
	mkAnchor(t, cbOrphan, eg, "700")
	mkBGMPerson(t, 501, "既存人物", `{"Fields":[{"Key":"性别","Value":"男"},{"Key":"生日","Value":"12月25日"}]}`)

	ccA := mkCreditName(t, "井上", nil)
	ccB := mkCreditName(t, "井上", nil)
	mkAnchor(t, ccA, vndb, "110")
	mkStaff(t, "s2", 110, "m", map[int]string{110: "井上"})

	mkLabel(t, "少女病")
	cdA := mkCreditName(t, "少女病", nil)
	cdB := mkCreditName(t, "Shoujobyou", nil)

	ceA := mkCreditName(t, "Studio e.go!", nil)
	ceB := mkCreditName(t, "studio ego", nil)

	p1 := mkPerson(t, "person-1", nil)
	p2 := mkPerson(t, "person-2", nil)
	cfA := mkCreditName(t, "cf-a", &p1)
	cfB := mkCreditName(t, "cf-b", &p2)

	p3 := mkPerson(t, "person-3", nil)
	cgA := mkCreditName(t, "cg-a", &p3)
	cgB := mkCreditName(t, "cg-b", nil)
	chA := mkCreditName(t, "ch-a", &p3)
	chB := mkCreditName(t, "ch-b", nil)

	ciA := mkCreditName(t, "ci-a", nil)
	ciB := mkCreditName(t, "ci-b", nil)
	mkStaff(t, "s3", 120, "m", map[int]string{120: "ci-a"})
	mkAnchor(t, ciA, vndb, "120")
	mkAnchor(t, ciB, bgm, "502")
	mkBGMPerson(t, 502, "ci-b", `{"Fields":[{"Key":"性别","Value":"女"},{"Key":"生日","Value":"未知"}]}`)

	crA := mkCreditName(t, "cr-a", nil)
	crB := mkCreditName(t, "cr-b", nil)

	clusters := clusterFile(t,
		clusterLine("CA", "auto", caPen, caMain),
		clusterLine("CB", "auto", cbLinked, cbOrphan),
		clusterLine("CC", "auto", ccA, ccB),
		clusterLine("CD", "auto", cdA, cdB),
		clusterLine("CE", "auto", ceA, ceB),
		clusterLine("CF", "auto", cfA, cfB),
		clusterLine("CG", "auto", cgA, cgB),
		clusterLine("CH", "auto", chA, chB),
		clusterLine("CI", "auto", ciA, ciB),
		clusterLine("CR", "review", crA, crB),
	)
	worklist := writeFile(t, "e4.jsonl", fmt.Sprintf("{\"credit_name_id\":%d}\n", ccA))
	opts := Opts{DSN: testDSN, ClustersPath: clusters, SplitWorklistPath: worklist}

	personsBefore := countRows(t, "catalog_person", "")
	linksBefore := countRows(t, "catalog_credit_name", "person_id IS NOT NULL")

	st, err := Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 10, st.ClustersTotal)
	assert.Equal(t, 9, st.ClustersAuto, "the review cluster is not consumed")
	assert.Equal(t, 18, st.Members)
	assert.Equal(t, 3, st.Minted, "CA + CB + CI")
	assert.Equal(t, 2, st.MintedNew)
	assert.Equal(t, 1, st.MintedReuse)
	assert.Equal(t, 6, st.Deferred)
	assert.Equal(t, map[DeferReason]int{
		DeferE4Split: 1, DeferOrgLabel: 1, DeferOrgPattern: 1,
		DeferPersonMulti: 1, DeferPersonCrossCluster: 2,
	}, st.Defers)
	assert.Equal(t, 2, st.WouldCreatePerson)
	assert.Equal(t, 5, st.WouldLink, "CA 2 + CB 1 + CI 2")
	assert.Equal(t, 1, st.LinksAlready, "CB's already-linked member")
	assert.Equal(t, 7, st.WouldAnchor, "CA: s1+bgm500+dlsite900 · CB: bgm501+eg700 · CI: s3+bgm502")
	assert.Equal(t, 1, st.WouldSetGender, "CA only: CB's host already has one, CI conflicts")
	assert.Equal(t, 1, st.GenderKept)
	assert.Equal(t, 1, st.GenderConflicts)
	assert.Equal(t, 2, st.WouldSetBirth, "CA's full date and CB's month/day")
	assert.Zero(t, st.PersonsCreated+st.LinksWritten+st.AnchorsWritten+st.PersonsUpdated)
	assert.Equal(t, personsBefore, countRows(t, "catalog_person", ""), "a dry run creates no person")
	assert.Equal(t, linksBefore, countRows(t, "catalog_credit_name", "person_id IS NOT NULL"))
	assert.Zero(t, countRows(t, "catalog_external_ref", "entity_type = ?", model.EntityTypePerson))

	st, err = Run(ctx, Opts{DSN: testDSN, ClustersPath: clusters, SplitWorklistPath: worklist, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 2, st.PersonsCreated)
	assert.Equal(t, 5, st.LinksWritten)
	assert.Equal(t, 7, st.AnchorsWritten)
	assert.Equal(t, 1, st.PersonsUpdated, "the reused host gains only its empty birth columns")
	assert.Zero(t, st.Errors)

	caHost := personIDOf(t, caMain)
	require.NotNil(t, caHost)
	assert.Equal(t, caHost, personIDOf(t, caPen), "both pen names point at one person")
	ca := person(t, *caHost)
	assert.Equal(t, "北都南", ca.DisplayName)
	require.NotNil(t, ca.PrimaryCreditNameID)
	assert.Equal(t, caMain, *ca.PrimaryCreditNameID)
	require.NotNil(t, ca.Gender)
	assert.Equal(t, model.GenderFemale, *ca.Gender)
	assert.EqualValues(t, 1978, *ca.BirthY)
	assert.EqualValues(t, 3, *ca.BirthM)
	assert.EqualValues(t, 4, *ca.BirthD)
	assert.Equal(t, []string{
		fmt.Sprintf("%d:s1:%s", vndb, matchedBy),
		fmt.Sprintf("%d:500:%s", bgm, matchedBy),
		fmt.Sprintf("%d:900:%s", dlsite, matchedBy),
	}, personAnchors(t, *caHost))
	var prov map[string][]provEntry
	require.NoError(t, json.Unmarshal(ca.FieldProvenance, &prov))
	require.Len(t, prov["gender"], 1)
	assert.Equal(t, sourceVNDB, prov["gender"][0].Source, "vndb outranks bangumi as the writer of record")
	assert.Equal(t, sourceBangumi, prov["birth_y"][0].Source)

	assert.Equal(t, &hostB, personIDOf(t, cbOrphan))
	cb := person(t, hostB)
	assert.Equal(t, "既存人物", cb.DisplayName, "an existing display name is never re-elected")
	require.NotNil(t, cb.Gender)
	assert.Equal(t, model.GenderFemale, *cb.Gender, "the bangumi 男 must NOT overwrite the declared gender")
	assert.Nil(t, cb.BirthY, "a month/day-only birthday leaves the year NULL")
	assert.EqualValues(t, 12, *cb.BirthM)
	assert.EqualValues(t, 25, *cb.BirthD)

	ciHost := personIDOf(t, ciA)
	require.NotNil(t, ciHost)
	assert.Nil(t, person(t, *ciHost).Gender, "disagreeing sources leave the column NULL")

	for _, id := range []int64{ccA, ccB, cdA, cdB, ceA, ceB, cgB, chB, crA, crB} {
		assert.Nil(t, personIDOf(t, id), "deferred/unconsumed member %d must keep its NULL link", id)
	}
	assert.Equal(t, &p1, personIDOf(t, cfA), "an existing link is never repointed")
	assert.Equal(t, &p2, personIDOf(t, cfB))
	assert.Equal(t, &p3, personIDOf(t, cgA))
	for _, pid := range []int64{p1, p2, p3} {
		assert.Empty(t, personAnchors(t, pid), "a deferred cluster mints no anchor")
	}
	assert.Equal(t, personsBefore+2, countRows(t, "catalog_person", ""))

	st, err = Run(ctx, Opts{DSN: testDSN, ClustersPath: clusters, SplitWorklistPath: worklist, Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.WouldCreatePerson)
	assert.Zero(t, st.WouldLink)
	assert.Zero(t, st.WouldAnchor)
	assert.Zero(t, st.WouldSetGender)
	assert.Zero(t, st.WouldSetBirth)
	assert.Zero(t, st.PersonsCreated+st.LinksWritten+st.AnchorsWritten+st.PersonsUpdated)
	assert.Equal(t, 3, st.Minted, "the same three clusters resolve, now through their own et=0 anchors")
	assert.Equal(t, 6, st.LinksAlready)
	assert.Equal(t, 7, st.AnchorsAlready)
	assert.Equal(t, 6, st.Deferred, "the defer buckets do not drift after an apply")
	assert.Equal(t, personsBefore+2, countRows(t, "catalog_person", ""))
}

func TestAnchorContradictionAborts(t *testing.T) {
	clean(t)
	vndb := sourceID(t, "vndb")
	mkStaff(t, "s9", 900, "m", map[int]string{900: "a", 901: "b", 902: "c", 903: "d"})
	a1, a2 := mkCreditName(t, "a", nil), mkCreditName(t, "b", nil)
	b1, b2 := mkCreditName(t, "c", nil), mkCreditName(t, "d", nil)
	mkAnchor(t, a1, vndb, "900")
	mkAnchor(t, a2, vndb, "901")
	mkAnchor(t, b1, vndb, "902")
	mkAnchor(t, b2, vndb, "903")

	clusters := clusterFile(t, clusterLine("CA", "auto", a1, a2), clusterLine("CB", "auto", b1, b2))
	worklist := writeFile(t, "e4.jsonl", "")
	_, err := Run(t.Context(), Opts{DSN: testDSN, ClustersPath: clusters, SplitWorklistPath: worklist, Apply: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contradicts itself")
	assert.EqualValues(t, 1, countRows(t, "catalog_external_ref", "entity_type = ?", model.EntityTypePerson))
}
