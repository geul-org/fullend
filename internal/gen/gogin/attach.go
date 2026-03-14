package gogin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/geul-org/fullend/internal/contract"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
)

// attachServiceDirectives scans service .go files and injects //fullend:gen directives.
func attachServiceDirectives(intDir string, serviceFuncs []ssacparser.ServiceFunc) error {
	// Build map: .go filename → ServiceFunc
	sfByFile := make(map[string]ssacparser.ServiceFunc)
	for _, sf := range serviceFuncs {
		goFile := strings.TrimSuffix(sf.FileName, ".ssac") + ".go"
		sfByFile[goFile] = sf
	}

	serviceDir := filepath.Join(intDir, "service")

	// Process flat files and domain subdirectories.
	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil // no service dir yet
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Domain subdirectory.
			domainDir := filepath.Join(serviceDir, entry.Name())
			if err := attachDirectivesInDir(domainDir, sfByFile); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".go") {
			// Flat file.
			if err := attachDirectiveToFile(filepath.Join(serviceDir, entry.Name()), sfByFile); err != nil {
				return err
			}
		}
	}

	return nil
}

// attachDirectivesInDir processes all .go files in a directory.
func attachDirectivesInDir(dir string, sfByFile map[string]ssacparser.ServiceFunc) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if err := attachDirectiveToFile(filepath.Join(dir, entry.Name()), sfByFile); err != nil {
			return err
		}
	}
	return nil
}

// attachDirectiveToFile injects a //fullend:gen directive into a single service .go file.
func attachDirectiveToFile(path string, sfByFile map[string]ssacparser.ServiceFunc) error {
	name := filepath.Base(path)

	// Skip infrastructure files.
	if name == "handler.go" || name == "server.go" {
		return nil
	}

	sf, ok := sfByFile[name]
	if !ok {
		return nil
	}

	// Compute SSOT path.
	ssotPath := "service/" + sf.FileName
	if sf.Domain != "" {
		ssotPath = "service/" + sf.Domain + "/" + sf.FileName
	}

	d := &contract.Directive{
		Ownership: "gen",
		SSOT:      ssotPath,
		Contract:  contract.HashServiceFunc(sf),
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := injectFuncDirective(string(src), d)
	return os.WriteFile(path, []byte(content), 0644)
}

// injectFuncDirective inserts a directive before the first func declaration.
func injectFuncDirective(src string, d *contract.Directive) string {
	// Find "func " at the start of a line.
	idx := strings.Index(src, "\nfunc ")
	if idx >= 0 {
		return src[:idx+1] + d.String() + "\n" + src[idx+1:]
	}
	// Try at the very beginning.
	if strings.HasPrefix(src, "func ") {
		return d.String() + "\n" + src
	}
	return src
}

// attachTSXDirectives scans pages/*.tsx files and injects // fullend:gen directive.
func attachTSXDirectives(artifactsDir string) error {
	pagesDir := filepath.Join(artifactsDir, "frontend", "src", "pages")
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		return nil // pages directory doesn't exist yet
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tsx") {
			continue
		}
		path := filepath.Join(pagesDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(src)

		// Skip if directive already present.
		if strings.Contains(content, "fullend:") {
			continue
		}

		// SSOT path: STML file derives from TSX filename.
		stmlName := strings.TrimSuffix(entry.Name(), ".tsx") + ".html"
		ssotPath := "frontend/" + stmlName
		hash := contract.Hash7(content)

		d := &contract.Directive{Ownership: "gen", SSOT: ssotPath, Contract: hash}
		newContent := d.StringJS() + "\n" + content
		os.WriteFile(path, []byte(newContent), 0644)
	}
	return nil
}

// injectFileDirective inserts a file-level directive before the package declaration.
func injectFileDirective(src string, d *contract.Directive) string {
	// Find "package " — skip any "// Code generated" comment.
	lines := strings.SplitN(src, "\n", -1)
	for i, line := range lines {
		if strings.HasPrefix(line, "package ") {
			// Insert directive before package line.
			before := strings.Join(lines[:i], "\n")
			after := strings.Join(lines[i:], "\n")
			if before != "" {
				return before + "\n" + d.String() + "\n" + after
			}
			return d.String() + "\n" + after
		}
	}
	return d.String() + "\n" + src
}
