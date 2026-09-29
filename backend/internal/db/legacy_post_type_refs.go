package db

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Legacy post types referenced from Go rather than from SQL. The SQL scan sees a legacy type
// compared inside a query but not one passed as a bound parameter, compared in a handler or
// listed in an API schema; each such file changes behavior once
// check:posts.posts_type_check narrows the column to 'post', so each needs a disposition.

// legacyPostTypeConsts maps the models constants of the legacy post types to their values.
var legacyPostTypeConsts = map[string]string{
	"PostTypeProblem": "problem", "PostTypeQuestion": "question", "PostTypeIdea": "idea",
}

// walkGoSources calls fn for every non-test Go file under root, skipping vendor,
// node_modules and dot directories; rel is slash-separated and relative to root.
func walkGoSources(root string, fn func(path, rel string, src []byte) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return fn(path, filepath.ToSlash(rel), src)
	})
}

// legacyPostTypeUses returns the legacy post types a parsed file names through a
// PostType{Problem,Question,Idea} identifier or a string literal equal to the type, with
// their lines. Comments are not parsed; a struct tag or an import path is never equal to a
// bare type, and neither is prose such as "problem not found".
func legacyPostTypeUses(fset *token.FileSet, file *ast.File) (types []string, lines []int) {
	typeSet, lineSet := map[string]bool{}, map[int]bool{}
	record := func(pos token.Pos, typ string) {
		typeSet[typ] = true
		lineSet[fset.Position(pos).Line] = true
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if typ, ok := legacyPostTypeConsts[x.Name]; ok {
				record(x.Pos(), typ)
			}
		case *ast.BasicLit:
			if x.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(x.Value); err == nil {
				for _, typ := range legacyPostTypeConsts {
					if v == typ {
						record(x.Pos(), typ)
					}
				}
			}
		}
		return true
	})
	for typ := range typeSet {
		types = append(types, typ)
	}
	for line := range lineSet {
		lines = append(lines, line)
	}
	sort.Strings(types)
	sort.Ints(lines)
	return types, lines
}

// ScanLegacyPostTypeReferences returns a typeref:<file> dependency for every non-test Go
// file under root that names a legacy post type in code. A file whose SQL is already a
// code:<file> dependency is owned by it and inherits that disposition.
func ScanLegacyPostTypeReferences(root string) ([]LegacyDependency, error) {
	var deps []LegacyDependency
	err := walkGoSources(root, func(path, rel string, src []byte) error {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		types, lines := legacyPostTypeUses(fset, file)
		if len(types) == 0 {
			return nil
		}
		nums := make([]string, len(lines))
		for i, l := range lines {
			nums[i] = strconv.Itoa(l)
		}
		word := "line"
		if len(lines) > 1 {
			word = "lines"
		}
		d := LegacyDependency{Kind: "typeref", Key: "typeref:" + rel,
			Detail: fmt.Sprintf("%s at %s %s", strings.Join(types, ","), word, strings.Join(nums, ","))}
		if text := string(src); legacySQLTableRe.MatchString(text) || legacySQLTypeRe.MatchString(text) {
			d.Owner = "code:" + rel
		}
		deps = append(deps, d)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan legacy post type references: %w", err)
	}
	return deps, nil
}
