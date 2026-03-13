package reporter

import (
	"fmt"
	"io"
)

// ChainLink mirrors orchestrator.ChainLink to avoid circular import.
type ChainLink struct {
	Kind    string
	File    string
	Line    int
	Summary string
}

// PrintChain writes a formatted feature chain to w.
func PrintChain(w io.Writer, operationID string, links []ChainLink) {
	fmt.Fprintf(w, "\n── Feature Chain: %s ──\n\n", operationID)

	if len(links) == 0 {
		fmt.Fprintln(w, "  No SSOT links found.")
		return
	}

	for _, link := range links {
		loc := link.File
		if link.Line > 0 {
			loc = fmt.Sprintf("%s:%d", link.File, link.Line)
		}
		fmt.Fprintf(w, "  %-10s %-45s %s\n", link.Kind, loc, link.Summary)
	}
	fmt.Fprintln(w)
}
