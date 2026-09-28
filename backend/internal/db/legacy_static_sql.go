package db

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// StaticSQLStatement is one SQL statement written as a compile-time string in the Go
// sources. The dropped-legacy-table probe prepares each one before and after the legacy
// tables are removed, which finds a legacy dependency the regex source scan cannot see.
type StaticSQLStatement struct {
	File string // slash path relative to the scanned root
	Line int
	SQL  string
}

var staticSQLStartRe = regexp.MustCompile(`^\s*(SELECT|INSERT|UPDATE|DELETE|WITH)\s`)

// ExtractStaticSQL parses the Go sources under root (tests and vendor excluded) and returns,
// sorted by file and line, every string that folds at compile time from literals and package
// constants and starts with an upper-case DML keyword. A concatenation with a runtime value
// yields its constant parts; fmt verbs stay in the text, so such a statement will not prepare.
func ExtractStaticSQL(root string) ([]StaticSQLStatement, error) {
	dirs := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			dirs[filepath.Dir(path)] = append(dirs[filepath.Dir(path)], path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("extract static sql: %w", err)
	}

	var out []StaticSQLStatement
	for _, paths := range dirs {
		stmts, err := extractPackageSQL(root, paths)
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// extractPackageSQL handles the files of one directory, which share their constants.
func extractPackageSQL(root string, paths []string) ([]StaticSQLStatement, error) {
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("extract static sql: %w", err)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, fmt.Errorf("extract static sql: %w", err)
		}
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("extract static sql: parse %s: %w", rel, err)
		}
		files[rel] = f
	}

	folder := staticStringFolder{consts: map[string]string{}}
	var specs []*ast.ValueSpec
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if d, ok := n.(*ast.GenDecl); ok && d.Tok == token.CONST {
				for _, s := range d.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
						specs = append(specs, vs)
					}
				}
			}
			return true
		})
	}
	// Constants may be defined from other constants in any order: resolve to a fixpoint.
	for resolved := true; resolved; {
		resolved = false
		for _, vs := range specs {
			for i, name := range vs.Names {
				if _, ok := folder.consts[name.Name]; ok {
					continue
				}
				if s, ok := folder.fold(vs.Values[i]); ok {
					folder.consts[name.Name] = s
					resolved = true
				}
			}
		}
	}

	var out []StaticSQLStatement
	for rel, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			expr, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if _, isIdent := expr.(*ast.Ident); isIdent {
				return false // a bare constant is recorded where it is declared
			}
			s, ok := folder.fold(expr)
			if !ok {
				return true
			}
			if staticSQLStartRe.MatchString(s) {
				out = append(out, StaticSQLStatement{File: rel, Line: fset.Position(expr.Pos()).Line, SQL: s})
			}
			return false
		})
	}
	return out, nil
}

type staticStringFolder struct {
	consts map[string]string
}

// fold evaluates a string literal, a known string constant, or a + concatenation of them.
func (f staticStringFolder) fold(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.Ident:
		s, ok := f.consts[e.Name]
		return s, ok
	case *ast.ParenExpr:
		return f.fold(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, ok := f.fold(e.X)
		if !ok {
			return "", false
		}
		right, ok := f.fold(e.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	}
	return "", false
}
