package crosscheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ssacvalidator "github.com/geul-org/ssac/validator"
)

// sensitivePatterns are column name substrings that suggest sensitive data.
var sensitivePatterns = []string{"password", "secret", "hash", "token"}

// CheckSensitiveColumns warns when DDL column names match sensitive patterns
// but lack an @sensitive annotation.
func CheckSensitiveColumns(st *ssacvalidator.SymbolTable, sensitiveCols, noSensitiveCols map[string]map[string]bool) []CrossError {
	var errs []CrossError

	for tableName, table := range st.DDLTables {
		for _, colName := range table.ColumnOrder {
			// Skip if already marked @sensitive.
			if sensitiveCols != nil {
				if cols, ok := sensitiveCols[tableName]; ok && cols[colName] {
					continue
				}
			}
			// Skip if explicitly marked @nosensitive.
			if noSensitiveCols != nil {
				if cols, ok := noSensitiveCols[tableName]; ok && cols[colName] {
					continue
				}
			}

			lower := strings.ToLower(colName)
			for _, p := range sensitivePatterns {
				if strings.Contains(lower, p) {
					errs = append(errs, CrossError{
						Rule:       "DDL @sensitive",
						Context:    fmt.Sprintf("%s.%s", tableName, colName),
						Message:    fmt.Sprintf("column %q matches sensitive pattern %q but has no @sensitive annotation — will be exposed in JSON responses", colName, p),
						Level:      "WARNING",
						Suggestion: fmt.Sprintf("add -- @sensitive to the column definition in db/%s.sql to generate json:\"-\" tag", tableName),
					})
					break
				}
			}
		}
	}

	return errs
}

// ParseSensitive parses DDL .sql files in dbDir for @sensitive and @nosensitive tags.
// Returns two maps: sensitive (table → column → true) and nosensitive (table → column → true).
func ParseSensitive(dbDir string) (sensitive, nosensitive map[string]map[string]bool, err error) {
	sensitive = make(map[string]map[string]bool)
	nosensitive = make(map[string]map[string]bool)

	entries, err := os.ReadDir(dbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return sensitive, nosensitive, nil
		}
		return nil, nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dbDir, entry.Name()))
		if err != nil {
			return nil, nil, err
		}

		parseSensitiveSQL(string(data), sensitive, nosensitive)
	}

	return sensitive, nosensitive, nil
}

func parseSensitiveSQL(content string, sensitive, nosensitive map[string]map[string]bool) {
	lines := strings.Split(content, "\n")
	var currentTable string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// CREATE TABLE
		if strings.HasPrefix(upper, "CREATE TABLE") {
			parts := strings.Fields(trimmed)
			for i, p := range parts {
				if strings.ToUpper(p) == "TABLE" && i+1 < len(parts) {
					currentTable = strings.Trim(parts[i+1], "( ")
					break
				}
			}
			continue
		}

		// End of table definition.
		if strings.HasPrefix(trimmed, ")") {
			currentTable = ""
			continue
		}

		if currentTable == "" {
			continue
		}

		colParts := strings.Fields(trimmed)
		if len(colParts) < 2 {
			continue
		}
		colName := colParts[0]
		upperFirst := strings.ToUpper(colName)
		if upperFirst == "PRIMARY" || upperFirst == "UNIQUE" || upperFirst == "CHECK" ||
			upperFirst == "CONSTRAINT" || upperFirst == "FOREIGN" || upperFirst == "--" {
			continue
		}

		if strings.Contains(line, "@nosensitive") {
			if nosensitive[currentTable] == nil {
				nosensitive[currentTable] = make(map[string]bool)
			}
			nosensitive[currentTable][colName] = true
		} else if strings.Contains(line, "@sensitive") {
			if sensitive[currentTable] == nil {
				sensitive[currentTable] = make(map[string]bool)
			}
			sensitive[currentTable][colName] = true
		}
	}
}
