package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"
)

// generateMain creates backend/go.mod (if missing) and backend/cmd/main.go.
func generateMain(artifactsDir string, models []string, modulePath string, queueBackend string, serviceFuncs []ssacparser.ServiceFunc) error {
	if modulePath == "" {
		base := filepath.Base(artifactsDir)
		modulePath = base + "/backend"
	}

	// Generate backend/go.mod if it doesn't exist.
	goModPath := filepath.Join(artifactsDir, "backend", "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(artifactsDir, "backend"), 0755); err != nil {
			return err
		}
		goModContent := fmt.Sprintf("module %s\n\ngo 1.22\n", modulePath)
		if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Join(artifactsDir, "backend", "cmd"), 0755); err != nil {
		return err
	}

	// Build model field initialization lines.
	var initLines []string
	for _, m := range models {
		fieldName := ucFirst(lcFirst(m) + "Model")
		initLines = append(initLines, fmt.Sprintf("\t\t%s: model.New%sModel(conn),", fieldName, m))
	}
	initBlock := strings.Join(initLines, "\n")
	if initBlock == "" {
		initBlock = "\t\t// No models detected"
	}

	// Queue code blocks.
	queueImport := ""
	queueInitBlock := ""
	queueSubscribeBlock := ""
	if queueBackend != "" {
		subscribers := collectSubscribers(serviceFuncs)
		if len(subscribers) > 0 || hasPublishSequence(serviceFuncs) {
			queueImport = "\n\t\"context\"\n\t\"encoding/json\"\n\t\"github.com/geul-org/fullend/pkg/queue\""
			queueInitBlock = fmt.Sprintf(`
	if err := queue.Init(context.Background(), %q, conn); err != nil {
		log.Fatalf("queue init failed: %%v", err)
	}
	defer queue.Close()
`, queueBackend)

			var subLines []string
			for _, fn := range subscribers {
				if fn.Param == nil {
					continue
				}
				subLines = append(subLines, fmt.Sprintf(`
	queue.Subscribe(%q, func(ctx context.Context, msg []byte) error {
		var message service.%s
		if err := json.Unmarshal(msg, &message); err != nil {
			return fmt.Errorf("unmarshal: %%w", err)
		}
		return server.%s(ctx, message)
	})`, fn.Subscribe.Topic, fn.Param.TypeName, fn.Name))
			}
			if len(subLines) > 0 {
				queueSubscribeBlock = strings.Join(subLines, "\n") + "\n\n\tgo queue.Start(context.Background())\n"
				// Add fmt import for Errorf.
				queueImport += "\n\t\"fmt\""
			}
		}
	}

	src := fmt.Sprintf(`package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"

	_ "github.com/lib/pq"

	"%s/internal/model"
	"%s/internal/service"%s
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("dsn", "postgres://localhost:5432/app?sslmode=disable", "database connection string")
	dbDriver := flag.String("db", "postgres", "database driver (postgres, mysql)")
	flag.Parse()

	conn, err := sql.Open(*dbDriver, *dsn)
	if err != nil {
		log.Fatalf("database connection failed: %%v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("database ping failed: %%v", err)
	}
%s
	server := &service.Server{
%s
	}
%s
	handler := service.Handler(server)
	log.Printf("server listening on %%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
`, modulePath, modulePath, queueImport, queueInitBlock, initBlock, queueSubscribeBlock)

	path := filepath.Join(artifactsDir, "backend", "cmd", "main.go")
	return os.WriteFile(path, []byte(src), 0644)
}

// collectSubscribers returns service functions that have @subscribe.
func collectSubscribers(funcs []ssacparser.ServiceFunc) []ssacparser.ServiceFunc {
	var subs []ssacparser.ServiceFunc
	for _, fn := range funcs {
		if fn.Subscribe != nil {
			subs = append(subs, fn)
		}
	}
	return subs
}

// hasPublishSequence returns true if any function uses @publish.
func hasPublishSequence(funcs []ssacparser.ServiceFunc) bool {
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Type == "publish" {
				return true
			}
		}
	}
	return false
}
