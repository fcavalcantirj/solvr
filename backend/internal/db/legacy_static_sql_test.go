package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every SQL statement written as a compile-time string is extracted with its file and line:
// string literals, raw strings, and concatenations of literals and package constants (also
// constants declared in another file of the same package). The constant prefix of a
// concatenation with a runtime value is still extracted; tests, vendor and non-SQL strings
// are not.
func TestExtractStaticSQL_FoldsLiteralsAndPackageConstants(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/a.go", "package db\n"+
		"\n"+
		"const cols = \"id, \" + \"title\"\n"+
		"\n"+
		"func a(where string) {\n"+
		"\tq1 := \"SELECT \" + cols + \" FROM approaches WHERE id = $1\"\n"+
		"\tq2 := `\n"+
		"\t\tINSERT INTO replies (post_id)\n"+
		"\t\tVALUES ($1)`\n"+
		"\tmsg := \"hello SELECT world\"\n"+
		"\tq3 := \"SELECT id FROM posts WHERE \" + where\n"+
		"\tq4 := fmt.Sprintf(\"UPDATE posts SET %s = $1\", col)\n"+
		"\terr := \"update failed\"\n"+
		"}\n")
	writeSource(t, root, "internal/db/b.go", "package db\n"+
		"\n"+
		"func b() { run(\"WITH x AS (SELECT \" + cols + \" FROM answers) SELECT * FROM x\") }\n")
	writeSource(t, root, "cmd/tool/main.go", "package main\n"+
		"\n"+
		"const cols = \"other\"\n"+
		"\n"+
		"func main() { run(\"DELETE FROM comments WHERE \" + \"id = $1\") }\n")
	writeSource(t, root, "internal/db/a_test.go", "package db\n\nconst q = \"SELECT id FROM comments\"\n")
	writeSource(t, root, "vendor/x/x.go", "package x\n\nconst q = \"SELECT id FROM comments\"\n")

	stmts, err := ExtractStaticSQL(root)
	require.NoError(t, err)

	assert.Equal(t, []StaticSQLStatement{
		{File: "cmd/tool/main.go", Line: 5, SQL: "DELETE FROM comments WHERE id = $1"},
		{File: "internal/db/a.go", Line: 6, SQL: "SELECT id, title FROM approaches WHERE id = $1"},
		{File: "internal/db/a.go", Line: 7, SQL: "\n\t\tINSERT INTO replies (post_id)\n\t\tVALUES ($1)"},
		{File: "internal/db/a.go", Line: 11, SQL: "SELECT id FROM posts WHERE "},
		{File: "internal/db/a.go", Line: 12, SQL: "UPDATE posts SET %s = $1"},
		{File: "internal/db/b.go", Line: 3, SQL: "WITH x AS (SELECT id, title FROM answers) SELECT * FROM x"},
	}, stmts)
}

// The extractor reports a file it cannot parse instead of silently skipping its SQL.
func TestExtractStaticSQL_ReportsUnparsableSource(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/broken.go", "package db\nfunc {\n")

	_, err := ExtractStaticSQL(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal/db/broken.go")
}
