package crosscheck

// CrossError represents a cross-validation error between two SSOT layers.
type CrossError struct {
	Rule       string // e.g. "x-sort ↔ DDL", "SSaC @result ↔ DDL"
	Context    string // e.g. operationId or funcName
	Message    string
	Level      string // "ERROR" or "WARNING" (empty = ERROR)
	Suggestion string // fix suggestion (empty if none)
}
