package statemachine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// [*] --> stateName
	reInitial = regexp.MustCompile(`\[\*\]\s*-->\s*(\w+)`)
	// stateA --> stateB: EventName
	reTransition = regexp.MustCompile(`(\w+)\s*-->\s*(\w+)\s*:\s*(\w+)`)
)

// ParseFile parses a single Mermaid stateDiagram markdown file.
// The diagram ID is derived from the filename (without extension).
func ParseFile(path string) (*StateDiagram, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}

	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Parse(id, string(data))
}

// Parse parses Mermaid stateDiagram content with a given ID.
func Parse(id, content string) (*StateDiagram, error) {
	// Extract content inside ```mermaid ... ``` block.
	mermaidContent := extractMermaidBlock(content)
	if mermaidContent == "" {
		return nil, fmt.Errorf("no mermaid stateDiagram block found in %s", id)
	}

	d := &StateDiagram{ID: id}

	// Parse initial state.
	if m := reInitial.FindStringSubmatch(mermaidContent); len(m) > 1 {
		d.InitialState = m[1]
	}

	// Parse transitions.
	matches := reTransition.FindAllStringSubmatch(mermaidContent, -1)
	stateSet := make(map[string]bool)
	if d.InitialState != "" {
		stateSet[d.InitialState] = true
	}

	for _, m := range matches {
		from, to, event := m[1], m[2], m[3]
		d.Transitions = append(d.Transitions, Transition{
			From:  from,
			To:    to,
			Event: event,
		})
		stateSet[from] = true
		stateSet[to] = true
	}

	for s := range stateSet {
		d.States = append(d.States, s)
	}

	if len(d.Transitions) == 0 {
		return nil, fmt.Errorf("no transitions found in stateDiagram %s", id)
	}

	return d, nil
}

// ParseDir parses all *.md files in the given directory.
func ParseDir(dir string) ([]*StateDiagram, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read states dir: %w", err)
	}

	var diagrams []*StateDiagram
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		d, err := ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		diagrams = append(diagrams, d)
	}
	return diagrams, nil
}

// extractMermaidBlock extracts content from the first ```mermaid ... ``` block.
func extractMermaidBlock(content string) string {
	const startMarker = "```mermaid"
	const endMarker = "```"

	startIdx := strings.Index(content, startMarker)
	if startIdx < 0 {
		return ""
	}
	after := content[startIdx+len(startMarker):]
	endIdx := strings.Index(after, endMarker)
	if endIdx < 0 {
		return ""
	}
	return after[:endIdx]
}
