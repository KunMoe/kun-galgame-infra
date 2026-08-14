package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var retiredCatalogTables = []string{
	"galgame_series",
	"galgame_tag",
	"galgame_tag_alias",
	"galgame_official",
	"galgame_official_alias",
	"galgame_engine",
	"galgame",
	"galgame_tag_edge",
	"galgame_alias",
	"galgame_tag_relation",
	"galgame_official_relation",
	"galgame_engine_relation",
	"galgame_link",
	"galgame_cover",
	"galgame_screenshot",
	"galgame_pr",
	"galgame_revision",
	"taxonomy_revision",
	"galgame_history",
	"galgame_contributor",
	"galgame_migrations",
	"galgame_message",
	"galgame_bangumi_meta",
	"galgame_vndb_meta",
	"galgame_eg_meta",
	"galgame_dlsite_meta",
	"galgame_stats",
}

// retiredTableReadAllowlist exempts a specific (tool directory, retired table)
// read. These are one-shot migration reconciliation tools that must read a
// retired wiki-era audit table to fix data the migration left inconsistent —
// the table is kept solely for that purpose and has no runtime reader, so the
// read is the tool's whole job rather than a new runtime dependency.
var retiredTableReadAllowlist = map[string]map[string]bool{
	"cmd/reclaim-foreign-claims": {"galgame_migrations": true},
}

func TestP0ARuntimeSQLDoesNotReferenceRetiredCatalogTables(t *testing.T) {
	root := moduleRoot(t)
	tablePattern := retiredTableReferencePattern()
	retired := make(map[string]struct{}, len(retiredCatalogTables))
	for _, table := range retiredCatalogTables {
		retired[table] = struct{}{}
	}

	var violations []string
	for _, rel := range []string{"cmd", "internal", "pkg"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		found, err := findRetiredTableReferences(path, root, tablePattern, retired)
		if err != nil {
			t.Fatalf("scan %s: %v", rel, err)
		}
		violations = append(violations, found...)
	}

	if len(violations) > 0 {
		slices.Sort(violations)
		t.Errorf("P0-A runtime SQL must not reference retired catalog tables:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func retiredTableReferencePattern() *regexp.Regexp {
	names := make([]string, 0, len(retiredCatalogTables))
	for _, table := range retiredCatalogTables {
		names = append(names, regexp.QuoteMeta(table))
	}
	slices.SortFunc(names, func(a, b string) int { return len(b) - len(a) })
	return regexp.MustCompile(`(?i)\b(?:from|join|update|into|table|truncate)\s+(?:only\s+)?(?:"?[a-z_][a-z0-9_]*"?\.)?"?(` +
		strings.Join(names, "|") + `)"?\b`)
}

func findRetiredTableReferences(
	path string,
	root string,
	tablePattern *regexp.Regexp,
	retired map[string]struct{},
) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(path, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(filename, ".go") || strings.HasSuffix(filename, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, filename, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filename, err)
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			rel = filename
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.BasicLit:
				value, ok := goStringLiteral(n)
				if !ok {
					return true
				}
				for _, match := range tablePattern.FindAllStringSubmatch(value, -1) {
					if allowedRetiredRead(rel, match[1]) {
						continue
					}
					violations = append(violations, fmt.Sprintf("%s:%d SQL references %s",
						rel, fset.Position(n.Pos()).Line, match[1]))
				}
			case *ast.CallExpr:
				table, ok := gormTableLiteral(n)
				if !ok {
					return true
				}
				table = strings.Trim(table, `"`)
				if dot := strings.LastIndexByte(table, '.'); dot >= 0 {
					table = strings.Trim(table[dot+1:], `"`)
				}
				if _, forbidden := retired[strings.ToLower(table)]; forbidden && !allowedRetiredRead(rel, table) {
					violations = append(violations, fmt.Sprintf("%s:%d GORM Table references %s",
						rel, fset.Position(n.Pos()).Line, table))
				}
			}
			return true
		})
		return nil
	})
	return violations, err
}

func goStringLiteral(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func gormTableLiteral(call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Table" || len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return "", false
	}
	return goStringLiteral(lit)
}

func allowedRetiredRead(rel, table string) bool {
	for dir, tables := range retiredTableReadAllowlist {
		if strings.HasPrefix(rel, dir+"/") && tables[strings.ToLower(table)] {
			return true
		}
	}
	return false
}
