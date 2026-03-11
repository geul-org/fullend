package pagination

// Cursor is a generic wrapper for cursor-based paginated responses.
type Cursor[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
	HasNext    bool   `json:"has_next"`
}
