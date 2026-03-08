package orchestrator

import (
	"os"
	"path/filepath"
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
		}
	}

	return found, nil
}

// NotDirError is returned when the specs path is not a directory.
type NotDirError struct {
	Path string
}

func (e *NotDirError) Error() string {
	return "not a directory: " + e.Path
}
