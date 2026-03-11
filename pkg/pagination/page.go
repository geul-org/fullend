package pagination

// Page is a generic wrapper for offset-based paginated responses.
type Page[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}
