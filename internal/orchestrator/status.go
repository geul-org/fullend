package orchestrator

import (
	"fmt"
	"io"
	"path/filepath"
)

// StatusLine holds one SSOT's status info.
type StatusLine struct {
	Kind    SSOTKind
	Path    string // relative path for display
	Summary string
}

// Status collects SSOT stats and returns lines to display.
func Status(root string, detected []DetectedSSOT) []StatusLine {
	// Parse all SSOTs once.
	parsed := ParseAll(root, detected, nil)

	var lines []StatusLine

	absRoot, _ := filepath.Abs(root)

	for _, d := range detected {
		relPath, err := filepath.Rel(absRoot, d.Path)
		if err != nil || relPath == "" {
			relPath = d.Path
		}

		switch d.Kind {
		case KindOpenAPI:
			summary := "?"
			if parsed.OpenAPIDoc != nil {
				count := 0
				for _, pi := range parsed.OpenAPIDoc.Paths.Map() {
					for range pi.Operations() {
						count++
					}
				}
				summary = fmt.Sprintf("%d endpoints", count)
			}
			lines = append(lines, StatusLine{Kind: KindOpenAPI, Path: relPath, Summary: summary})
		case KindDDL:
			summary := "?"
			if parsed.SymbolTable != nil {
				tables := len(parsed.SymbolTable.DDLTables)
				cols := 0
				for _, t := range parsed.SymbolTable.DDLTables {
					cols += len(t.Columns)
				}
				summary = fmt.Sprintf("%d tables, %d columns", tables, cols)
			}
			lines = append(lines, StatusLine{Kind: KindDDL, Path: relPath, Summary: summary})
		case KindSSaC:
			summary := "?"
			if parsed.ServiceFuncs != nil {
				summary = fmt.Sprintf("%d functions", len(parsed.ServiceFuncs))
			}
			lines = append(lines, StatusLine{Kind: KindSSaC, Path: relPath, Summary: summary})
		case KindSTML:
			summary := "?"
			if parsed.STMLPages != nil {
				summary = fmt.Sprintf("%d pages", len(parsed.STMLPages))
			}
			lines = append(lines, StatusLine{Kind: KindSTML, Path: relPath, Summary: summary})
		case KindStates:
			summary := "?"
			if parsed.StateDiagrams != nil {
				totalTransitions := 0
				for _, d := range parsed.StateDiagrams {
					totalTransitions += len(d.Transitions)
				}
				summary = fmt.Sprintf("%d diagrams, %d transitions", len(parsed.StateDiagrams), totalTransitions)
			}
			lines = append(lines, StatusLine{Kind: KindStates, Path: relPath, Summary: summary})
		case KindPolicy:
			summary := "?"
			if parsed.Policies != nil {
				totalRules := 0
				for _, p := range parsed.Policies {
					totalRules += len(p.Rules)
				}
				summary = fmt.Sprintf("%d files, %d rules", len(parsed.Policies), totalRules)
			}
			lines = append(lines, StatusLine{Kind: KindPolicy, Path: relPath, Summary: summary})
		case KindScenario:
			scenarioHurls, _ := filepath.Glob(filepath.Join(d.Path, "scenario-*.hurl"))
			invariantHurls, _ := filepath.Glob(filepath.Join(d.Path, "invariant-*.hurl"))
			total := len(scenarioHurls) + len(invariantHurls)
			lines = append(lines, StatusLine{Kind: KindScenario, Path: relPath, Summary: fmt.Sprintf("%d hurl files", total)})
		case KindFunc:
			summary := "?"
			if parsed.ProjectFuncSpecs != nil {
				stubs := 0
				for _, s := range parsed.ProjectFuncSpecs {
					if !s.HasBody {
						stubs++
					}
				}
				if stubs > 0 {
					summary = fmt.Sprintf("%d funcs (%d TODO)", len(parsed.ProjectFuncSpecs), stubs)
				} else {
					summary = fmt.Sprintf("%d funcs", len(parsed.ProjectFuncSpecs))
				}
			}
			lines = append(lines, StatusLine{Kind: KindFunc, Path: relPath, Summary: summary})
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
