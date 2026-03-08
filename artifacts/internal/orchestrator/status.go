package orchestrator

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
	stmlparser "github.com/geul-org/stml/parser"
)

// StatusLine holds one SSOT's status info.
type StatusLine struct {
	Kind    SSOTKind
	Path    string // relative path for display
	Summary string
}

// Status collects SSOT stats and returns lines to display.
func Status(root string, detected []DetectedSSOT) []StatusLine {
	var lines []StatusLine

	absRoot, _ := filepath.Abs(root)

	for _, d := range detected {
		relPath, err := filepath.Rel(absRoot, d.Path)
		if err != nil || relPath == "" {
			relPath = d.Path
		}

		switch d.Kind {
		case KindOpenAPI:
			lines = append(lines, statusOpenAPI(relPath, d.Path))
		case KindDDL:
			lines = append(lines, statusDDL(relPath, root))
		case KindSSaC:
			lines = append(lines, statusSSaC(relPath, d.Path))
		case KindSTML:
			lines = append(lines, statusSTML(relPath, d.Path))
		case KindTerraform:
			lines = append(lines, statusTerraform(relPath, d.Path))
		case KindModel:
			// Model is auxiliary; skip in status display.
		}
	}

	return lines
}

// PrintStatus writes the status lines to w.
func PrintStatus(w io.Writer, lines []StatusLine) {
	if len(lines) == 0 {
		fmt.Fprintln(w, "No SSOTs found.")
		return
	}

	fmt.Fprintln(w, "SSOT Status:")
	for _, l := range lines {
		fmt.Fprintf(w, "  %-12s %-30s %s\n", l.Kind, l.Path, l.Summary)
	}
}

func statusOpenAPI(relPath, absPath string) StatusLine {
	summary := "?"
	doc, err := openapi3.NewLoader().LoadFromFile(absPath)
	if err == nil {
		count := 0
		for _, pi := range doc.Paths.Map() {
			for range pi.Operations() {
				count++
			}
		}
		summary = fmt.Sprintf("%d endpoints", count)
	}
	return StatusLine{Kind: KindOpenAPI, Path: relPath, Summary: summary}
}

func statusDDL(relPath, root string) StatusLine {
	summary := "?"
	st, err := ssacvalidator.LoadSymbolTable(root)
	if err == nil {
		tables := len(st.DDLTables)
		cols := 0
		for _, t := range st.DDLTables {
			cols += len(t.Columns)
		}
		summary = fmt.Sprintf("%d tables, %d columns", tables, cols)
	}
	return StatusLine{Kind: KindDDL, Path: relPath, Summary: summary}
}

func statusSSaC(relPath, dir string) StatusLine {
	summary := "?"
	funcs, err := ssacparser.ParseDir(dir)
	if err == nil {
		summary = fmt.Sprintf("%d functions", len(funcs))
	}
	return StatusLine{Kind: KindSSaC, Path: relPath, Summary: summary}
}

func statusSTML(relPath, dir string) StatusLine {
	summary := "?"
	pages, err := stmlparser.ParseDir(dir)
	if err == nil {
		summary = fmt.Sprintf("%d pages", len(pages))
	}
	return StatusLine{Kind: KindSTML, Path: relPath, Summary: summary}
}

func statusTerraform(relPath, dir string) StatusLine {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tf"))
	summary := fmt.Sprintf("%d files", len(matches))
	return StatusLine{Kind: KindTerraform, Path: relPath, Summary: summary}
}
