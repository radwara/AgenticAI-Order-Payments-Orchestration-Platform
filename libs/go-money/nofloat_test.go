package money

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"
)

// floatIn returns a description of the first floating-point (or complex)
// component found in typ, or "" if there is none. Composite types are walked
// so that tuples such as (float64, error), []float64 or func(float64) are caught.
func floatIn(typ types.Type, seen map[types.Type]bool) string {
	if typ == nil || seen[typ] {
		return ""
	}
	seen[typ] = true
	if n, ok := typ.(*types.Named); ok {
		if o := n.Obj(); o.Pkg() != nil && o.Pkg().Path() == "math/big" && o.Name() == "Float" {
			return "big.Float"
		}
	}
	switch u := typ.Underlying().(type) {
	case *types.Basic:
		if u.Info()&(types.IsFloat|types.IsComplex) != 0 {
			return u.Name()
		}
	case *types.Pointer:
		return floatIn(u.Elem(), seen)
	case *types.Slice:
		return floatIn(u.Elem(), seen)
	case *types.Array:
		return floatIn(u.Elem(), seen)
	case *types.Chan:
		return floatIn(u.Elem(), seen)
	case *types.Map:
		if s := floatIn(u.Key(), seen); s != "" {
			return s
		}
		return floatIn(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if s := floatIn(u.Field(i).Type(), seen); s != "" {
				return s
			}
		}
	case *types.Tuple:
		for i := 0; i < u.Len(); i++ {
			if s := floatIn(u.At(i).Type(), seen); s != "" {
				return s
			}
		}
	case *types.Signature:
		if s := floatIn(u.Params(), seen); s != "" {
			return s
		}
		return floatIn(u.Results(), seen)
	}
	return ""
}

// floatViolations type-checks files (resolving imports from the standard
// library's source) and returns a message for every expression, definition or
// use whose type contains a floating-point type, including inferred types such
// as the result of math.Sqrt or strconv.ParseFloat.
func floatViolations(fset *token.FileSet, files []*ast.File) ([]string, error) {
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("money", fset, files, info); err != nil {
		return nil, err
	}
	var out []string
	for e, tv := range info.Types {
		if s := floatIn(tv.Type, map[types.Type]bool{}); s != "" {
			out = append(out, fmt.Sprintf("%s: expression of floating-point type %s", fset.Position(e.Pos()), s))
		}
	}
	for id, o := range info.Defs {
		if o == nil {
			continue
		}
		if s := floatIn(o.Type(), map[types.Type]bool{}); s != "" {
			out = append(out, fmt.Sprintf("%s: %s declared with floating-point type %s", fset.Position(id.Pos()), id.Name, s))
		}
	}
	for id, o := range info.Uses {
		if s := floatIn(o.Type(), map[types.Type]bool{}); s != "" {
			out = append(out, fmt.Sprintf("%s: use of %s with floating-point type %s", fset.Position(id.Pos()), id.Name, s))
		}
	}
	sort.Strings(out)
	return out, nil
}

// TestNoFloatingPoint guards the "no floating point anywhere in the package"
// constraint using full type information, so inferred float types cannot slip
// through (unlike an identifier/literal grep).
func TestNoFloatingPoint(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no source files checked")
	}
	v, err := floatViolations(fset, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range v {
		t.Error(msg)
	}
}

// The guard itself must catch float use that the old identifier/literal grep
// missed: inferred types, tuple results, big.Float and float constants.
func TestFloatGuardCatchesInferredFloats(t *testing.T) {
	cases := map[string]string{
		"sqrt":     "package p\nimport \"math\"\nfunc f() int64 { x := math.Sqrt(2); return int64(x) }",
		"parse":    "package p\nimport \"strconv\"\nfunc f() int64 { x, _ := strconv.ParseFloat(\"1\", 64); return int64(x) }",
		"pi":       "package p\nimport \"math\"\nvar y = math.Pi",
		"bigfloat": "package p\nimport \"math/big\"\nfunc f() { _ = new(big.Float) }",
		"floor":    "package p\nimport \"math\"\nfunc f(a int64) int64 { return int64(math.Floor(float64(a))) }",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, name+".go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			v, err := floatViolations(fset, []*ast.File{f})
			if err != nil {
				t.Fatal(err)
			}
			if len(v) == 0 {
				t.Fatalf("guard missed float use in %q", src)
			}
		})
	}
	t.Run("clean", func(t *testing.T) {
		fset := token.NewFileSet()
		src := "package p\nimport \"math/big\"\nfunc f(a int64) *big.Int { return big.NewInt(a) }"
		f, err := parser.ParseFile(fset, "c.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		v, err := floatViolations(fset, []*ast.File{f})
		if err != nil || len(v) != 0 {
			t.Fatalf("false positive: %v %v", v, err)
		}
	})
}
