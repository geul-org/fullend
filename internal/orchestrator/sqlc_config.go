package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateSqlcConfig creates sqlc.yaml from detected DDL files.
// Returns the config path and whether it was generated (vs already existing).
func generateSqlcConfig(specsDir, artifactsDir string) (string, error) {
	configPath := filepath.Join(specsDir, "sqlc.yaml")

	// If sqlc.yaml already exists, use it as-is.
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	// Check that queries directory exists.
	queriesDir := filepath.Join(specsDir, "db", "queries")
	if _, err := os.Stat(queriesDir); os.IsNotExist(err) {
		return "", fmt.Errorf("db/queries/ 디렉토리가 없습니다 — sqlc 쿼리 파일을 작성하세요")
	}

	engine := detectDBEngine(specsDir)
	absOut := filepath.Join(artifactsDir, "backend", "internal", "db")
	// sqlc resolves out path relative to sqlc.yaml location (specsDir).
	dbOutDir, err := filepath.Rel(specsDir, absOut)
	if err != nil {
		dbOutDir = absOut
	}

	src := fmt.Sprintf(`version: "2"
sql:
  - engine: "%s"
    schema: "db/"
    queries: "db/queries/"
    gen:
      go:
        package: "db"
        out: "%s"
        sql_package: "database/sql"
        emit_json_tags: true
        emit_empty_slices: true
`, engine, dbOutDir)

	if err := os.WriteFile(configPath, []byte(src), 0644); err != nil {
		return "", fmt.Errorf("sqlc.yaml 생성 실패: %w", err)
	}

	return configPath, nil
}

// detectDBEngine inspects DDL files to determine the database engine.
func detectDBEngine(specsDir string) string {
	dbDir := filepath.Join(specsDir, "db")
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return "postgresql"
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dbDir, entry.Name()))
		if err != nil {
			continue
		}
		content := strings.ToUpper(string(data))
		// MySQL indicators.
		if strings.Contains(content, "AUTO_INCREMENT") ||
			strings.Contains(content, "ENGINE=INNODB") ||
			strings.Contains(content, "ENGINE = INNODB") {
			return "mysql"
		}
	}

	return "postgresql"
}
