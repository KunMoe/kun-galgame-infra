package repr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOmitemptyOnlyOnPointers(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sf, ok := n.(*ast.Field)
				if !ok || sf.Tag == nil {
					return true
				}
				tag := sf.Tag.Value
				if !strings.Contains(tag, "omitempty") {
					return true
				}
				if !isPointerType(sf.Type) {
					t.Errorf("%s: omitempty on non-pointer %s", filepath.Base(name), fieldName(sf))
				}
				return true
			})
		}
	}
}

func isPointerType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.StarExpr:
		return true
	case *ast.Ident:
		return false
	case *ast.SelectorExpr:
		return false
	case *ast.ArrayType:
		return false
	case *ast.MapType:
		return false
	case *ast.IndexExpr:
		return isPointerType(t.X)
	case *ast.IndexListExpr:
		return isPointerType(t.X)
	default:
		return false
	}
}

func fieldName(f *ast.Field) string {
	if len(f.Names) == 0 {
		return "(embedded)"
	}
	return f.Names[0].Name
}
