package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	suitelock "api/internal/platform/trust/dbtest"
	"api/internal/platform/trust/migrate"
	"api/internal/platform/trust/model"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("cmd/import-trust-terms")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/import-trust-terms", "cannot connect to test database: %v", err)
	}
	sqlDB, _ := db.DB()
	release := suitelock.AcquireSuiteLock(sqlDB)

	if err := migrate.Run(db); err != nil {
		release()
		dbtest.SkipMainf("cmd/import-trust-terms", "trust migration failed: %v", err)
	}
	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

func cleanTerms(t *testing.T) {
	t.Helper()
	if err := testDB.Exec("TRUNCATE trust_term RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("truncate trust_term: %v", err)
	}
}

func writeFixture(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return p
}

func countActive(t *testing.T, site string) int64 {
	t.Helper()
	q := testDB.Model(&model.TrustTerm{}).Where("is_deprecated = false")
	if site == "" {
		q = q.Where("site IS NULL")
	} else {
		q = q.Where("site = ?", site)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("count active: %v", err)
	}
	return n
}

var (
	zwsp = string(rune(0x200B))
	fwd  = "ＢＡＤ"
)

func TestProcessFileCounters(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("hello\n")
	buf.WriteString(fwd + "\n")
	buf.WriteString("bad\n")
	buf.WriteString("he" + zwsp + "llo\n")
	buf.WriteString("\n")
	buf.WriteString("# a comment\n")
	buf.WriteString("ab\n")
	buf.WriteString("   \n")
	buf.Write([]byte{0x66, 0x6f, 0xff, 0x6f, '\n'})

	cfg := importConfig{site: nil, kind: model.TermKindSuspect, minRunes: 3}
	seen := map[string]struct{}{}
	existing := map[string]struct{}{}
	stats, terms, err := processFile("fix.txt", &buf, cfg, seen, existing)
	if err != nil {
		t.Fatalf("processFile: %v", err)
	}

	if stats.read != 9 {
		t.Errorf("read = %d, want 9", stats.read)
	}
	if stats.blankComment != 3 {
		t.Errorf("blank+comment = %d, want 3 (blank, comment, whitespace-only)", stats.blankComment)
	}
	if stats.invalidUTF8 != 1 {
		t.Errorf("invalid-utf8 = %d, want 1", stats.invalidUTF8)
	}
	if stats.shortFiltered != 1 {
		t.Errorf("short-filtered = %d, want 1", stats.shortFiltered)
	}
	if stats.dupInBatch != 2 {
		t.Errorf("dup-in-batch = %d, want 2 (bad, hello collapsed by norm)", stats.dupInBatch)
	}
	if stats.inserted != 2 || len(terms) != 2 {
		t.Fatalf("inserted = %d / terms = %d, want 2 (hello, bad)", stats.inserted, len(terms))
	}
	got := map[string]bool{terms[0].TermNorm: true, terms[1].TermNorm: true}
	if !got["hello"] || !got["bad"] {
		t.Errorf("inserted norms = %v, want {hello, bad}", got)
	}
	for _, tm := range terms {
		if tm.Kind != model.TermKindSuspect {
			t.Errorf("kind = %d, want suspect", tm.Kind)
		}
		if tm.Site != nil {
			t.Errorf("site = %v, want nil (global)", tm.Site)
		}
		if tm.IsDeprecated {
			t.Errorf("is_deprecated = true, want false")
		}
		if tm.Note == nil || *tm.Note != "fix.txt" {
			t.Errorf("note = %v, want fix.txt (default per-file)", tm.Note)
		}
	}
}

func TestProcessFileDedupAcrossFiles(t *testing.T) {
	cfg := importConfig{minRunes: 3}
	seen := map[string]struct{}{}
	existing := map[string]struct{}{}

	a := bytes.NewBufferString("alpha\nbeta\n")
	sa, ta, _ := processFile("a.txt", a, cfg, seen, existing)
	if sa.inserted != 2 || len(ta) != 2 {
		t.Fatalf("file a inserted = %d, want 2", sa.inserted)
	}
	b := bytes.NewBufferString("beta\ngamma\n")
	sb, tb, _ := processFile("b.txt", b, cfg, seen, existing)
	if sb.dupInBatch != 1 {
		t.Errorf("file b dup-in-batch = %d, want 1 (beta)", sb.dupInBatch)
	}
	if sb.inserted != 1 || len(tb) != 1 || tb[0].TermNorm != "gamma" {
		t.Fatalf("file b inserted = %d (%v), want 1 (gamma)", sb.inserted, tb)
	}
	if ta[0].Note == nil || *ta[0].Note != "a.txt" || tb[0].Note == nil || *tb[0].Note != "b.txt" {
		t.Errorf("per-file notes wrong: a=%v b=%v", ta[0].Note, tb[0].Note)
	}
}

func TestProcessFileExistingSkip(t *testing.T) {
	cfg := importConfig{minRunes: 3}
	seen := map[string]struct{}{}
	existing := map[string]struct{}{"known": {}}

	buf := bytes.NewBufferString("known\nfresh\n")
	stats, terms, _ := processFile("f.txt", buf, cfg, seen, existing)
	if stats.dupExisting != 1 {
		t.Errorf("dup-existing = %d, want 1", stats.dupExisting)
	}
	if stats.inserted != 1 || len(terms) != 1 || terms[0].TermNorm != "fresh" {
		t.Fatalf("inserted = %d (%v), want 1 (fresh)", stats.inserted, terms)
	}
}

func TestProcessFileFixedNote(t *testing.T) {
	cfg := importConfig{minRunes: 3, note: "konsheng MIT"}
	_, terms, _ := processFile("porn.txt", bytes.NewBufferString("word one\n"), cfg, map[string]struct{}{}, map[string]struct{}{})
	if len(terms) != 1 || terms[0].Note == nil || *terms[0].Note != "konsheng MIT" {
		t.Fatalf("note = %v, want fixed 'konsheng MIT'", terms[0].Note)
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	cleanTerms(t)
	fx := writeFixture(t, "terms.txt", []byte("alpha\nbeta\ngamma\n"))

	var out bytes.Buffer
	res, err := run(testDB, &out, importConfig{minRunes: 3, apply: false}, []string{fx})
	if err != nil {
		t.Fatalf("run dry: %v", err)
	}
	if res.total.inserted != 3 {
		t.Errorf("would-insert = %d, want 3", res.total.inserted)
	}
	if n := countActive(t, ""); n != 0 {
		t.Fatalf("dry-run wrote %d rows, want 0", n)
	}
	if !strings.Contains(out.String(), "would-insert") {
		t.Errorf("dry-run output missing 'would-insert':\n%s", out.String())
	}
	if !strings.Contains(out.String(), "samples (raw -> norm)") {
		t.Errorf("dry-run output missing samples preview:\n%s", out.String())
	}
}

func TestRunApplyWritesCorrectly(t *testing.T) {
	cleanTerms(t)
	content := []byte("keyword\n" + fwd + "\n" + "he" + zwsp + "llo\n" + "hello\n" + "ab\n" + "# note\n")
	fx := writeFixture(t, "lexicon.txt", content)

	var out bytes.Buffer
	res, err := run(testDB, &out, importConfig{minRunes: 3, kind: model.TermKindBanned, apply: true}, []string{fx})
	if err != nil {
		t.Fatalf("run apply: %v", err)
	}
	if res.total.inserted != 3 {
		t.Fatalf("inserted = %d, want 3", res.total.inserted)
	}
	if n := countActive(t, ""); n != 3 {
		t.Fatalf("db active rows = %d, want 3", n)
	}

	var rows []model.TrustTerm
	if err := testDB.Where("is_deprecated = false").Order("term_norm").Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	norms := []string{}
	for _, r := range rows {
		norms = append(norms, r.TermNorm)
		if r.Site != nil {
			t.Errorf("row %q site = %v, want NULL (global)", r.TermNorm, r.Site)
		}
		if r.Kind != model.TermKindBanned {
			t.Errorf("row %q kind = %d, want banned", r.TermNorm, r.Kind)
		}
		if r.IsDeprecated {
			t.Errorf("row %q is_deprecated = true, want false", r.TermNorm)
		}
		if r.Note == nil || *r.Note != "lexicon.txt" {
			t.Errorf("row %q note = %v, want lexicon.txt", r.TermNorm, r.Note)
		}
	}
	want := []string{"bad", "hello", "keyword"}
	if strings.Join(norms, ",") != strings.Join(want, ",") {
		t.Fatalf("stored norms = %v, want %v", norms, want)
	}
	if !strings.Contains(out.String(), "inserted") {
		t.Errorf("apply output missing 'inserted':\n%s", out.String())
	}
}

func TestRunApplyIdempotent(t *testing.T) {
	cleanTerms(t)
	fx := writeFixture(t, "terms.txt", []byte("alpha\nbeta\ngamma\n"))

	var out1 bytes.Buffer
	res1, err := run(testDB, &out1, importConfig{minRunes: 3, apply: true}, []string{fx})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if res1.total.inserted != 3 {
		t.Fatalf("first run inserted = %d, want 3", res1.total.inserted)
	}

	var out2 bytes.Buffer
	res2, err := run(testDB, &out2, importConfig{minRunes: 3, apply: true}, []string{fx})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if res2.total.inserted != 0 {
		t.Errorf("second run inserted = %d, want 0 (idempotent)", res2.total.inserted)
	}
	if res2.total.dupExisting != 3 {
		t.Errorf("second run dup-existing = %d, want 3", res2.total.dupExisting)
	}
	if n := countActive(t, ""); n != 3 {
		t.Fatalf("db active rows after re-run = %d, want 3 (no duplicates)", n)
	}
}

func TestRunSiteScoped(t *testing.T) {
	cleanTerms(t)
	if err := testDB.Create(&model.TrustTerm{TermNorm: "shared", Kind: model.TermKindSuspect}).Error; err != nil {
		t.Fatalf("seed global: %v", err)
	}
	fx := writeFixture(t, "kungal.txt", []byte("shared\nkungalonly\n"))

	site := "kungal"
	res, err := run(testDB, new(bytes.Buffer), importConfig{site: &site, minRunes: 3, apply: true}, []string{fx})
	if err != nil {
		t.Fatalf("run site: %v", err)
	}
	if res.total.inserted != 2 {
		t.Fatalf("inserted = %d, want 2 (per-site shared + kungalonly)", res.total.inserted)
	}
	if n := countActive(t, "kungal"); n != 2 {
		t.Fatalf("kungal active rows = %d, want 2", n)
	}
	if n := countActive(t, ""); n != 1 {
		t.Fatalf("global active rows = %d, want 1 (untouched)", n)
	}
}
