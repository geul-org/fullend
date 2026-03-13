package gluegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBaseWhere(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantWhere  string
		wantParams int
	}{
		{
			name:       "no where",
			sql:        "SELECT * FROM gigs ORDER BY created_at DESC;",
			wantWhere:  "",
			wantParams: 0,
		},
		{
			name:       "simple where",
			sql:        "SELECT * FROM enrollments WHERE user_id = $1 ORDER BY created_at DESC;",
			wantWhere:  "user_id = $1",
			wantParams: 1,
		},
		{
			name:       "where with two params",
			sql:        "SELECT * FROM items WHERE owner_id = $1 AND status = $2 ORDER BY id DESC;",
			wantWhere:  "owner_id = $1 AND status = $2",
			wantParams: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, params := extractBaseWhere(tt.sql)
			if where != tt.wantWhere {
				t.Errorf("where = %q, want %q", where, tt.wantWhere)
			}
			if params != tt.wantParams {
				t.Errorf("params = %d, want %d", params, tt.wantParams)
			}
		})
	}
}

func TestGenerateQueryOpts_CursorWhereClause(t *testing.T) {
	dir := t.TempDir()
	if err := generateQueryOpts(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "queryopts.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	// Verify cursor WHERE clause generation.
	checks := []struct {
		name    string
		snippet string
	}{
		{"cursor WHERE block", `if opts.Cursor != ""`},
		{"cursor less-than operator", `op := "<"`},
		{"cursor asc greater-than", `op = ">"`},
		{"cursor column from SortCol", `cursorCol := opts.SortCol`},
		{"cursor default id", `cursorCol = "id"`},
		{"offset skip in cursor mode", `opts.Offset > 0 && opts.Cursor == ""`},
		{"cursor sort fixed comment", "Cursor mode: fixed sort"},
		{"cursor default id DESC", `opts.SortCol = "id"`},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(src, c.snippet) {
				t.Errorf("generated queryopts.go missing %q", c.snippet)
			}
		})
	}
}
