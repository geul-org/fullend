package reporter

// Status represents the outcome of a validation step.
type Status int

const (
	Pass Status = iota
	Fail
	Skip
)

// StepResult holds the outcome of a single validation step.
type StepResult struct {
	Name        string   // "OpenAPI", "DDL", "SSaC", "STML", "Cross"
	Status      Status   // Pass, Fail, Skip
	Summary     string   // "34 endpoints", "12 tables, 47 columns"
	Errors      []string // individual error messages
	Suggestions []string // fix suggestions (parallel to Errors, empty string if none)
}

// Report holds all step results from a validation run.
type Report struct {
	Steps []StepResult
}

// TotalErrors returns the sum of errors across all steps.
func (r *Report) TotalErrors() int {
	n := 0
	for _, s := range r.Steps {
		n += len(s.Errors)
	}
	return n
}

// HasFailure returns true if any step failed.
func (r *Report) HasFailure() bool {
	for _, s := range r.Steps {
		if s.Status == Fail {
			return true
		}
	}
	return false
}
