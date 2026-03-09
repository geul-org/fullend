package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/geul-org/fullend/artifacts/internal/orchestrator"
	"github.com/geul-org/fullend/artifacts/internal/reporter"
)

const usage = `Usage: fullend <command> [arguments]

Commands:
  validate [--skip kind,...] <specs-dir>                 Validate SSOT specs
  gen      [--skip kind,...] <specs-dir> <artifacts-dir> Generate code from specs
  status   <specs-dir>                                   Show SSOT status summary

Skip kinds: openapi, ddl, ssac, model, stml, states, policy, terraform
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		skipKinds, args := parseSkipFlag(os.Args[2:])
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: fullend validate [--skip kind,...] <specs-dir>")
			os.Exit(2)
		}
		runValidate(args[0], skipKinds)
	case "gen":
		skipKinds, args := parseSkipFlag(os.Args[2:])
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: fullend gen [--skip kind,...] <specs-dir> <artifacts-dir>")
			os.Exit(2)
		}
		runGen(args[0], args[1], skipKinds)
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

// parseSkipFlag extracts --skip flag and returns (skipKinds, remainingArgs).
func parseSkipFlag(args []string) (map[orchestrator.SSOTKind]bool, []string) {
	skipKinds := make(map[orchestrator.SSOTKind]bool)
	var remaining []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--skip" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--skip requires a comma-separated list of SSOT kinds")
				os.Exit(2)
			}
			i++
			for _, s := range strings.Split(args[i], ",") {
				s = strings.TrimSpace(s)
				kind, ok := orchestrator.KindFromString(s)
				if !ok {
					fmt.Fprintf(os.Stderr, "unknown SSOT kind: %q\nvalid kinds: openapi, ddl, ssac, model, stml, states, policy, terraform\n", s)
					os.Exit(2)
				}
				skipKinds[kind] = true
			}
		} else {
			remaining = append(remaining, args[i])
		}
	}

	return skipKinds, remaining
}

func runValidate(specsDir string, skipKinds map[orchestrator.SSOTKind]bool) {
	detected, err := orchestrator.DetectSSOTs(specsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	report := orchestrator.Validate(specsDir, detected, skipKinds)
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

func runGen(specsDir, artifactsDir string, skipKinds map[orchestrator.SSOTKind]bool) {
	report, ok := orchestrator.Gen(specsDir, artifactsDir, skipKinds)
	reporter.Print(os.Stdout, report)

	if !ok {
		os.Exit(1)
	}
}
