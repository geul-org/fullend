package scenario

// Feature represents a parsed .feature file.
type Feature struct {
	File       string     // source file path
	Tag        string     // "@scenario" or "@invariant"
	Name       string     // Feature name
	Background *Scenario  // optional Background section
	Scenarios  []Scenario // one or more Scenario sections
}

// Scenario represents a single Scenario (or Background) block.
type Scenario struct {
	Name  string // Scenario name (empty for Background)
	Steps []Step
}

// Step represents a single step line inside a Scenario.
type Step struct {
	Keyword string // Given, When, Then, And, But
	IsAction bool  // true = HTTP request, false = assertion

	// Action fields (IsAction == true)
	Method      string // GET, POST, PUT, DELETE
	OperationID string // PascalCase operationId
	JSON        string // raw JSON body (may contain var refs)
	Capture     string // variable name after →

	// Assertion fields (IsAction == false)
	Assertion Assertion
}

// Assertion represents a Then/And assertion step.
type Assertion struct {
	Kind  AssertionKind // status, exists, equals, contains, excludes, count
	Field string        // e.g. "courses" for response.courses
	Op    string        // "==", ">", "<", ">=" etc. (for status/count)
	Value string        // expected value
}

// AssertionKind classifies assertion types.
type AssertionKind string

const (
	AssertStatus   AssertionKind = "status"
	AssertExists   AssertionKind = "exists"
	AssertEquals   AssertionKind = "equals"
	AssertContains AssertionKind = "contains"
	AssertExcludes AssertionKind = "excludes"
	AssertCount    AssertionKind = "count"
)
