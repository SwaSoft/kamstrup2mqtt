package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "kamstrup2mqtt"

// adrPackages is the package layout recorded in docs/adr/0001-package-layout.md:
// each Go package directory (relative to the repo root) with its package name and
// the internal packages its non-test code may import.
var adrPackages = map[string]struct {
	name    string
	imports []string
}{
	".":               {"main", []string{"internal/bridge", "internal/config", "internal/parser"}},
	"internal/config": {"config", nil},
	"internal/parser": {"parser", nil},
	"internal/bridge": {"bridge", []string{"internal/config", "internal/parser"}},
}

// goPackages returns, for every directory holding non-test Go files, the package
// names and the module-internal imports (relative to the module root) of those files.
func goPackages(t *testing.T) map[string]struct{ names, imports map[string]bool } {
	t.Helper()
	pkgs := map[string]struct{ names, imports map[string]bool }{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		p, ok := pkgs[dir]
		if !ok {
			p = struct{ names, imports map[string]bool }{map[string]bool{}, map[string]bool{}}
			pkgs[dir] = p
		}
		p.names[f.Name.Name] = true
		for _, imp := range f.Imports {
			ip, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(ip, modulePath+"/") {
				p.imports[strings.TrimPrefix(ip, modulePath+"/")] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestPackageLayoutFollowsADR0001 checks that the Go packages and their internal
// dependencies match ADR 0001: main.go at the root is the only entrypoint, code
// lives in internal/config, internal/parser and internal/bridge, and dependencies
// point inward.
func TestPackageLayoutFollowsADR0001(t *testing.T) {
	pkgs := goPackages(t)

	for dir := range pkgs {
		if _, ok := adrPackages[dir]; !ok {
			t.Errorf("package directory %q is not recorded in ADR 0001", dir)
		}
	}

	for dir, want := range adrPackages {
		got, ok := pkgs[dir]
		if !ok {
			t.Errorf("package directory %q from ADR 0001 is missing", dir)
			continue
		}
		if names := keys(got.names); len(names) != 1 || names[0] != want.name {
			t.Errorf("%s: package names %v, want [%s]", dir, names, want.name)
		}
		allowed := map[string]bool{}
		for _, imp := range want.imports {
			allowed[imp] = true
		}
		for _, imp := range keys(got.imports) {
			if !allowed[imp] {
				t.Errorf("%s imports %s, which ADR 0001 does not allow", dir, imp)
			}
		}
	}

	// main.go is the only entrypoint: no package main outside the repo root.
	for dir, p := range pkgs {
		if dir != "." && p.names["main"] {
			t.Errorf("%s declares package main; main.go at the root is the only entrypoint", dir)
		}
	}
}
