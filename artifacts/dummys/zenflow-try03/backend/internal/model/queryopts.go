package model

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// QueryOptsConfig defines allowed values for parsing query parameters.
type QueryOptsConfig struct {
	Pagination *PaginationConfig
	Sort       *SortConfig
	Filter     *FilterConfig
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

// QueryOpts holds parsed query parameters for pagination, sort, and filter.
type QueryOpts struct {
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
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

		// Cursor mode: fixed sort, no runtime switching.
		if cfg.Pagination != nil && cfg.Pagination.Style == "cursor" {
			// Sort is fixed to cfg.Sort.Default + direction. Ignore query params.
		} else {
			if sortBy := c.Query("sortBy"); sortBy != "" {
				if containsStr(cfg.Sort.Allowed, sortBy) {
					opts.SortCol = sortBy
				}
			}
			if sortDir := c.Query("sortDir"); sortDir == "asc" || sortDir == "desc" {
				opts.SortDir = sortDir
			}
		}
	} else if cfg.Pagination != nil && cfg.Pagination.Style == "cursor" {
		// Cursor without x-sort: default to id DESC.
		opts.SortCol = "id"
		opts.SortDir = "desc"
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

	// Cursor WHERE clause
	if opts.Cursor != "" {
		cursorCol := opts.SortCol
		if cursorCol == "" {
			cursorCol = "id"
		}
		op := "<" // DESC: fetch rows less than cursor
		if opts.SortDir == "asc" {
			op = ">"
		}
		if baseWhere == "" && len(args) == 0 {
			sql += " WHERE "
		} else {
			sql += " AND "
		}
		sql += fmt.Sprintf("%s %s $%d", cursorCol, op, argIdx)
		args = append(args, opts.Cursor)
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

	// Pagination
	if opts.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
		argIdx++
	}
	// OFFSET — only for non-cursor mode
	if opts.Offset > 0 && opts.Cursor == "" {
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
