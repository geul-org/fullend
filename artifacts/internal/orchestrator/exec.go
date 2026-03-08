package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const execTimeout = 30 * time.Second

// ExecResult holds the outcome of an external command execution.
type ExecResult struct {
	Skipped bool   // true if the tool is not installed
	Err     error  // non-nil if the command failed
	Stderr  string // captured stderr on failure
}

// RunExec runs an external command with a timeout.
// If the command is not found in PATH, it returns Skipped=true.
func RunExec(name string, args ...string) ExecResult {
	_, err := exec.LookPath(name)
	if err != nil {
		return ExecResult{Skipped: true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return ExecResult{
			Err:    fmt.Errorf("%s failed: %w", name, err),
			Stderr: stderr.String(),
		}
	}

	return ExecResult{}
}
