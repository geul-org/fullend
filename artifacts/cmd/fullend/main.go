package main

import (
	"fmt"
	"os"

	"github.com/geul-org/fullend/artifacts/internal/orchestrator"
	"github.com/geul-org/fullend/artifacts/internal/reporter"
)

const usage = `Usage: fullend <command> [arguments]

Commands:
  validate <specs-dir>                 Validate SSOT specs
  gen      <specs-dir> <artifacts-dir> Generate code from specs
  status   <specs-dir>                 Show SSOT status summary
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: fullend validate <specs-dir>")
			os.Exit(2)
		}
		runValidate(os.Args[2])
	case "gen":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: fullend gen <specs-dir> <artifacts-dir>")
			os.Exit(2)
		}
		runGen(os.Args[2], os.Args[3])
	case "status":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: fullend status <specs-dir>")
			os.Exit(2)
		}
		runStatus(os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Print(usage)
		os.Exit(2)
	}
}

func runValidate(specsDir string) {
	detected, err := orchestrator.DetectSSOTs(specsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	report := orchestrator.Validate(specsDir, detected)
	reporter.Print(os.Stdout, report)

	if report.HasFailure() {
		os.Exit(1)
	}
}

func runStatus(specsDir string) {
	detected, err := orchestrator.DetectSSOTs(specsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	lines := orchestrator.Status(specsDir, detected)
	orchestrator.PrintStatus(os.Stdout, lines)
}

func runGen(specsDir, artifactsDir string) {
	report, ok := orchestrator.Gen(specsDir, artifactsDir)
	reporter.Print(os.Stdout, report)

	if !ok {
		os.Exit(1)
	}
}
