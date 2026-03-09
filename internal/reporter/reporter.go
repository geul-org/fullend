package reporter

import (
	"fmt"
	"io"
)

// Print writes the formatted validation report to w.
func Print(w io.Writer, r *Report) {
	for _, step := range r.Steps {
		// Separator step.
		if step.Name == "---" {
			fmt.Fprintf(w, "\n── %s ──\n", step.Summary)
			continue
		}

		switch step.Status {
		case Pass:
			fmt.Fprintf(w, "✓ %-12s %s\n", step.Name, step.Summary)
			// Show warnings even on pass.
			for i, e := range step.Errors {
				fmt.Fprintf(w, "    %s\n", e)
				if i < len(step.Suggestions) && step.Suggestions[i] != "" {
					fmt.Fprintf(w, "      → 제안: %s\n", step.Suggestions[i])
				}
			}
		case Fail:
			fmt.Fprintf(w, "✗ %-12s %s\n", step.Name, step.Summary)
			for i, e := range step.Errors {
				fmt.Fprintf(w, "    %s\n", e)
				if i < len(step.Suggestions) && step.Suggestions[i] != "" {
					fmt.Fprintf(w, "      → 제안: %s\n", step.Suggestions[i])
				}
			}
		case Skip:
			fmt.Fprintf(w, "— %-12s %s\n", step.Name, step.Summary)
		}
	}

	fmt.Fprintln(w)

	if r.HasFailure() {
		fmt.Fprintln(w, "FAILED: Fix errors before codegen.")
	} else {
		allSkip := true
		for _, s := range r.Steps {
			if s.Status == Pass {
				allSkip = false
				break
			}
		}
		if allSkip {
			fmt.Fprintln(w, "No SSOT sources found.")
		} else {
			hasSkip := false
			for _, s := range r.Steps {
				if s.Status == Skip {
					hasSkip = true
					break
				}
			}
			if hasSkip {
				fmt.Fprintln(w, "Partial validation passed.")
			} else {
				fmt.Fprintln(w, "All SSOT sources are consistent.")
			}
		}
	}
}
