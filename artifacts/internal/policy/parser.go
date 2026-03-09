package policy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reOwnership = regexp.MustCompile(
		`^#\s*@ownership\s+(\w+):\s+(\w+)\.(\w+)(?:\s+via\s+(\w+)\.(\w+))?$`,
	)
	reAction   = regexp.MustCompile(`input\.action\s*(?:==\s*"(\w+)"|in\s*\{([^}]+)\})`)
	reResource = regexp.MustCompile(`input\.resource\s*==\s*"(\w+)"`)
	reOwnerRef = regexp.MustCompile(`input\.resource_owner`)
	reRoleRef  = regexp.MustCompile(`input\.user\.role\s*==\s*"(\w+)"`)
)

// ParseFile parses a single .rego file and extracts policy information.
func ParseFile(path string) (*Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	p := &Policy{File: path}
	scanner := bufio.NewScanner(f)

	// First pass: extract @ownership annotations.
	lineNum := 0
	var lines []string
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)

		if m := reOwnership.FindStringSubmatch(line); m != nil {
			om := OwnershipMapping{
				Resource: m[1],
				Table:    m[2],
				Column:   m[3],
			}
			if m[4] != "" {
				om.JoinTable = m[4]
				om.JoinFK = m[5]
			}
			p.Ownerships = append(p.Ownerships, om)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Second pass: extract allow rules by scanning allow blocks.
	content := strings.Join(lines, "\n")
	extractAllowRules(content, p)

	return p, nil
}

// ParseDir parses all .rego files in a directory.
func ParseDir(dir string) ([]*Policy, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.rego"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no .rego files found in %s", dir)
	}

	var policies []*Policy
	for _, path := range matches {
		p, err := ParseFile(path)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// extractAllowRules extracts (action, resource) pairs from allow blocks.
func extractAllowRules(content string, p *Policy) {
	// Split by "allow if {" blocks.
	parts := strings.Split(content, "allow if {")
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		// Find the closing brace that matches the allow block (depth-aware).
		endIdx := findClosingBrace(block)
		if endIdx < 0 {
			continue
		}
		block = block[:endIdx]

		rule := AllowRule{}

		// Extract action(s).
		if m := reAction.FindStringSubmatch(block); m != nil {
			if m[1] != "" {
				// Single action: input.action == "create"
				rule.Actions = []string{m[1]}
			} else if m[2] != "" {
				// Action set: input.action in {"update", "delete"}
				rule.Actions = parseActionSet(m[2])
			}
		}

		// Extract resource.
		if m := reResource.FindStringSubmatch(block); m != nil {
			rule.Resource = m[1]
		}

		// Check owner reference.
		rule.UsesOwner = reOwnerRef.MatchString(block)

		// Check role reference.
		if m := reRoleRef.FindStringSubmatch(block); m != nil {
			rule.UsesRole = true
			rule.RoleValue = m[1]
		}

		if len(rule.Actions) > 0 && rule.Resource != "" {
			p.Rules = append(p.Rules, rule)
		}
	}
}

// findClosingBrace finds the index of the closing brace that matches the opening
// of an allow block, accounting for nested braces (e.g., action sets).
func findClosingBrace(s string) int {
	depth := 1
	for i, c := range s {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseActionSet parses the inside of an action set: "update", "delete", "publish"
func parseActionSet(s string) []string {
	var actions []string
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "\"")
		if part != "" {
			actions = append(actions, part)
		}
	}
	return actions
}
