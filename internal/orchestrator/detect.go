package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
)

// SSOTKind identifies a type of SSOT source.
type SSOTKind string

const (
	KindOpenAPI   SSOTKind = "OpenAPI"
	KindDDL       SSOTKind = "DDL"
	KindSSaC      SSOTKind = "SSaC"
	KindModel     SSOTKind = "Model"
	KindSTML      SSOTKind = "STML"
	KindTerraform SSOTKind = "Terraform"
	KindStates    SSOTKind = "States"
	KindPolicy    SSOTKind = "Policy"
	KindScenario  SSOTKind = "Scenario"
	KindFunc      SSOTKind = "Func"
	KindConfig    SSOTKind = "Config"
)

// DetectedSSOT holds the kind and resolved directory path.
type DetectedSSOT struct {
	Kind SSOTKind
	Path string // absolute path to the relevant directory or file
}

// DetectSSOTs scans root for known SSOT directories and returns what exists.
func DetectSSOTs(root string) ([]DetectedSSOT, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &NotDirError{Path: abs}
	}

	var found []DetectedSSOT

	checks := []struct {
		kind    SSOTKind
		pattern string
	}{
		{KindOpenAPI, "api/openapi.yaml"},
		{KindDDL, "db/*.sql"},
		{KindSSaC, "service/*.go"},
		{KindModel, "model/*.go"},
		{KindSTML, "frontend/*.html"},
		{KindTerraform, "terraform/*.tf"},
	}

	for _, c := range checks {
		matches, _ := filepath.Glob(filepath.Join(abs, c.pattern))
		if len(matches) > 0 {
			dir := filepath.Dir(matches[0])
			if c.kind == KindOpenAPI {
				dir = matches[0] // file path, not dir
			}
			found = append(found, DetectedSSOT{Kind: c.kind, Path: dir})
		} else if c.kind == KindSSaC {
			// Also check for domain folder structure: service/{domain}/*.go
			subMatches, _ := filepath.Glob(filepath.Join(abs, "service", "*", "*.go"))
			if len(subMatches) > 0 {
				found = append(found, DetectedSSOT{Kind: c.kind, Path: filepath.Join(abs, "service")})
			}
		}
	}

	// Check for states/ directory (Mermaid stateDiagram files).
	statesDir := filepath.Join(abs, "states")
	if statesMatches, _ := filepath.Glob(filepath.Join(statesDir, "*.md")); len(statesMatches) > 0 {
		found = append(found, DetectedSSOT{Kind: KindStates, Path: statesDir})
	}

	// Check for policy/ directory (OPA Rego files).
	policyDir := filepath.Join(abs, "policy")
	if policyMatches, _ := filepath.Glob(filepath.Join(policyDir, "*.rego")); len(policyMatches) > 0 {
		found = append(found, DetectedSSOT{Kind: KindPolicy, Path: policyDir})
	}

	// Check for scenario/ directory (Gherkin .feature files).
	scenarioDir := filepath.Join(abs, "scenario")
	if scenarioMatches, _ := filepath.Glob(filepath.Join(scenarioDir, "*.feature")); len(scenarioMatches) > 0 {
		found = append(found, DetectedSSOT{Kind: KindScenario, Path: scenarioDir})
	}

	// Check for fullend.yaml (project config).
	configPath := filepath.Join(abs, "fullend.yaml")
	if _, err := os.Stat(configPath); err == nil {
		found = append(found, DetectedSSOT{Kind: KindConfig, Path: configPath})
	}

	// Check for func/ directory (custom func spec files).
	funcDir := filepath.Join(abs, "func")
	if fi, err := os.Stat(funcDir); err == nil && fi.IsDir() {
		// Check for any .go files in subdirectories.
		funcMatches, _ := filepath.Glob(filepath.Join(funcDir, "*", "*.go"))
		if len(funcMatches) > 0 {
			found = append(found, DetectedSSOT{Kind: KindFunc, Path: funcDir})
		}
	}

	return found, nil
}

// AllSSOTKinds returns all SSOT kinds that fullend manages.
func AllSSOTKinds() []SSOTKind {
	return []SSOTKind{
		KindConfig, KindOpenAPI, KindDDL, KindSSaC, KindModel,
		KindSTML, KindStates, KindPolicy, KindScenario, KindFunc, KindTerraform,
	}
}

// kindNames maps CLI --skip values to SSOTKind.
var kindNames = map[string]SSOTKind{
	"openapi":   KindOpenAPI,
	"ddl":       KindDDL,
	"ssac":      KindSSaC,
	"model":     KindModel,
	"stml":      KindSTML,
	"states":    KindStates,
	"policy":    KindPolicy,
	"terraform": KindTerraform,
	"scenario":  KindScenario,
	"func":      KindFunc,
	"config":    KindConfig,
}

// KindFromString parses a CLI --skip value into a SSOTKind.
func KindFromString(s string) (SSOTKind, bool) {
	k, ok := kindNames[strings.ToLower(s)]
	return k, ok
}

// NotDirError is returned when the specs path is not a directory.
type NotDirError struct {
	Path string
}

func (e *NotDirError) Error() string {
	return "not a directory: " + e.Path
}
