package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// reActionStep matches: KEYWORD METHOD operationId [{JSON}] [→ capture]
	reActionStep = regexp.MustCompile(
		`^(Given|When|Then|And|But)\s+` +
			`(GET|POST|PUT|DELETE)\s+` +
			`(\w+)` +
			`(?:\s+(\{.*\}))?` +
			`(?:\s+→\s+(\w+))?$`,
	)

	// reAssertStatus matches: KEYWORD status == CODE
	reAssertStatus = regexp.MustCompile(
		`^(Then|And|But)\s+status\s*==\s*(\d+)$`,
	)

	// reAssertResponse matches: KEYWORD response.field OP [value]
	reAssertResponse = regexp.MustCompile(
		`^(Then|And|But)\s+response\.(\w+)\s+` +
			`(exists|==|contains|excludes|count)\s*(.*)$`,
	)
)

// ParseFile parses a single .feature file.
func ParseFile(path string) (*Feature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read feature file %s: %w", path, err)
	}
	return Parse(path, string(data))
}

// Parse parses Gherkin content with fixed patterns.
func Parse(file, content string) (*Feature, error) {
	f := &Feature{File: file}

	lines := strings.Split(content, "\n")
	var currentScenario *Scenario
	inBackground := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Tag line
		if strings.HasPrefix(line, "@") {
			tag := strings.TrimSpace(line)
			if tag == "@scenario" || tag == "@invariant" {
				f.Tag = tag
			}
			continue
		}

		// Feature line
		if strings.HasPrefix(line, "Feature:") {
			f.Name = strings.TrimSpace(strings.TrimPrefix(line, "Feature:"))
			continue
		}

		// Background
		if strings.HasPrefix(line, "Background:") {
			f.Background = &Scenario{}
			currentScenario = f.Background
			inBackground = true
			continue
		}

		// Scenario
		if strings.HasPrefix(line, "Scenario:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "Scenario:"))
			f.Scenarios = append(f.Scenarios, Scenario{Name: name})
			currentScenario = &f.Scenarios[len(f.Scenarios)-1]
			inBackground = false
			continue
		}

		// Step lines
		if currentScenario == nil {
			continue
		}

		step, err := parseStep(line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if step != nil {
			if inBackground {
				f.Background.Steps = append(f.Background.Steps, *step)
			} else {
				currentScenario.Steps = append(currentScenario.Steps, *step)
			}
		}
	}

	if f.Name == "" {
		return nil, fmt.Errorf("%s: missing Feature name", file)
	}
	if len(f.Scenarios) == 0 {
		return nil, fmt.Errorf("%s: no Scenario blocks found", file)
	}
	if f.Tag == "" {
		f.Tag = "@scenario" // default
	}

	return f, nil
}

// parseStep parses a single step line.
func parseStep(line string) (*Step, error) {
	// Try action step (METHOD present).
	if m := reActionStep.FindStringSubmatch(line); m != nil {
		return &Step{
			Keyword:     m[1],
			IsAction:    true,
			Method:      m[2],
			OperationID: m[3],
			JSON:        m[4],
			Capture:     m[5],
		}, nil
	}

	// Try status assertion.
	if m := reAssertStatus.FindStringSubmatch(line); m != nil {
		return &Step{
			Keyword: m[1],
			Assertion: Assertion{
				Kind:  AssertStatus,
				Op:    "==",
				Value: m[2],
			},
		}, nil
	}

	// Try response assertion.
	if m := reAssertResponse.FindStringSubmatch(line); m != nil {
		kind := AssertionKind(m[3])
		if m[3] == "==" {
			kind = AssertEquals
		}
		return &Step{
			Keyword: m[1],
			Assertion: Assertion{
				Kind:  kind,
				Field: m[2],
				Op:    m[3],
				Value: strings.TrimSpace(m[4]),
			},
		}, nil
	}

	// Unknown step — not an error for lines like "Given", "When", etc.
	// that don't match our fixed patterns (could be a freeform step).
	return nil, nil
}

// ParseDir parses all *.feature files in the given directory.
func ParseDir(dir string) ([]*Feature, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read scenario dir: %w", err)
	}

	var features []*Feature
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".feature") {
			continue
		}
		f, err := ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		features = append(features, f)
	}
	return features, nil
}

// AllOperationIDs returns all unique operationIDs referenced in action steps.
func AllOperationIDs(features []*Feature) []string {
	seen := make(map[string]bool)
	for _, f := range features {
		collectOps(f.Background, seen)
		for i := range f.Scenarios {
			collectOps(&f.Scenarios[i], seen)
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func collectOps(s *Scenario, seen map[string]bool) {
	if s == nil {
		return
	}
	for _, step := range s.Steps {
		if step.IsAction && step.OperationID != "" {
			seen[step.OperationID] = true
		}
	}
}
