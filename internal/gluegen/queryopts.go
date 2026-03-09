package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateQueryOpts creates model/queryopts.go with parseQueryOpts, buildSelectQuery, buildCountQuery.
func generateQueryOpts(modelDir string) error {
	src := `package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// QueryOptsConfig defines allowed values for parsing query parameters.
type QueryOptsConfig struct {
	Pagination *PaginationConfig
	Sort       *SortConfig
	Filter     *FilterConfig
	Include    *IncludeConfig
}

// PaginationConfig defines pagination behavior.
type PaginationConfig struct {
	Style        string // "offset" or "cursor"
	DefaultLimit int
	MaxLimit     int
}

// SortConfig defines allowed sort columns.
type SortConfig struct {
	Allowed   []string
	Default   string
	Direction string // default direction: "asc" or "desc"
}

// FilterConfig defines allowed filter columns.
type FilterConfig struct {
	Allowed []string
}

// IncludeConfig defines allowed include resources.
type IncludeConfig struct {
	Allowed []string
}

// ParseQueryOpts extracts QueryOpts from a gin context, validated against config.
func ParseQueryOpts(c *gin.Context, cfg QueryOptsConfig) QueryOpts {
	var opts QueryOpts

	// Pagination
	if cfg.Pagination != nil {
		opts.Limit = cfg.Pagination.DefaultLimit
		if limitStr := c.Query("limit"); limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
				opts.Limit = v
			}
		}
		if cfg.Pagination.MaxLimit > 0 && opts.Limit > cfg.Pagination.MaxLimit {
			opts.Limit = cfg.Pagination.MaxLimit
		}

		if cfg.Pagination.Style == "cursor" {
			opts.Cursor = c.Query("cursor")
		} else {
			if offsetStr := c.Query("offset"); offsetStr != "" {
				if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
					opts.Offset = v
				}
			}
		}
	}

	// Sort
	if cfg.Sort != nil {
		opts.SortCol = cfg.Sort.Default
		opts.SortDir = cfg.Sort.Direction
		if opts.SortDir == "" {
			opts.SortDir = "asc"
		}

		if sortBy := c.Query("sortBy"); sortBy != "" {
			if containsStr(cfg.Sort.Allowed, sortBy) {
				opts.SortCol = sortBy
			}
		}
		if sortDir := c.Query("sortDir"); sortDir == "asc" || sortDir == "desc" {
			opts.SortDir = sortDir
		}
	}

	// Filter
	if cfg.Filter != nil {
		opts.Filters = make(map[string]string)
		for _, col := range cfg.Filter.Allowed {
			if val := c.Query(col); val != "" {
				opts.Filters[col] = val
			}
		}
	}

	// Include
	if cfg.Include != nil {
		if incStr := c.Query("include"); incStr != "" {
			for _, inc := range strings.Split(incStr, ",") {
				inc = strings.TrimSpace(inc)
				if containsStr(cfg.Include.Allowed, inc) {
					opts.Includes = append(opts.Includes, inc)
				}
			}
		}
	}

	return opts
}

// BuildSelectQuery constructs a dynamic SELECT with pagination, sort, filter.
// baseArgCount is the number of $N params already used in baseWhere.
func BuildSelectQuery(table, baseWhere string, baseArgCount int, opts QueryOpts) (string, []interface{}) {
	var args []interface{}
	argIdx := baseArgCount + 1

	sql := fmt.Sprintf("SELECT * FROM %s", table)
	if baseWhere != "" {
		sql += " WHERE " + baseWhere
	}

	// Filter
	for col, val := range opts.Filters {
		if baseWhere == "" && len(args) == 0 {
			sql += " WHERE "
		} else {
			sql += " AND "
		}
		sql += fmt.Sprintf("%s = $%d", col, argIdx)
		args = append(args, val)
		argIdx++
	}

	// Sort
	if opts.SortCol != "" {
		dir := "ASC"
		if opts.SortDir == "desc" {
			dir = "DESC"
		}
		sql += fmt.Sprintf(" ORDER BY %s %s", opts.SortCol, dir)
	}

	// Pagination (offset)
	if opts.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
		argIdx++
	}
	if opts.Offset > 0 {
		sql += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, opts.Offset)
		argIdx++
	}

	return sql, args
}

// BuildCountQuery constructs a COUNT query with the same filters.
func BuildCountQuery(table, baseWhere string, baseArgCount int, opts QueryOpts) (string, []interface{}) {
	var args []interface{}
	argIdx := baseArgCount + 1

	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if baseWhere != "" {
		sql += " WHERE " + baseWhere
	}

	for col, val := range opts.Filters {
		if baseWhere == "" && len(args) == 0 {
			sql += " WHERE "
		} else {
			sql += " AND "
		}
		sql += fmt.Sprintf("%s = $%d", col, argIdx)
		args = append(args, val)
		argIdx++
	}

	return sql, args
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
`
	return os.WriteFile(filepath.Join(modelDir, "queryopts.go"), []byte(src), 0644)
}

// extractBaseWhere extracts the WHERE clause from a SQL query string, stripping ORDER BY.
// e.g. "SELECT * FROM courses WHERE published = TRUE ORDER BY created_at DESC;" → "published = TRUE"
// e.g. "SELECT * FROM enrollments WHERE user_id = $1 ORDER BY created_at DESC;" → "user_id = $1"
func extractBaseWhere(sql string) (where string, paramCount int) {
	upper := strings.ToUpper(sql)

	// Find WHERE clause.
	whereIdx := strings.Index(upper, "WHERE ")
	if whereIdx < 0 {
		return "", 0
	}
	rest := sql[whereIdx+6:]

	// Strip ORDER BY and everything after.
	if orderIdx := strings.Index(strings.ToUpper(rest), "ORDER BY"); orderIdx >= 0 {
		rest = rest[:orderIdx]
	}
	// Strip LIMIT.
	if limitIdx := strings.Index(strings.ToUpper(rest), "LIMIT"); limitIdx >= 0 {
		rest = rest[:limitIdx]
	}
	// Strip trailing semicolon and whitespace.
	rest = strings.TrimRight(rest, "; \t\n")

	where = strings.TrimSpace(rest)

	// Count $N params in baseWhere.
	for i := 1; i <= 20; i++ {
		if strings.Contains(where, fmt.Sprintf("$%d", i)) {
			paramCount = i
		}
	}

	return where, paramCount
}
