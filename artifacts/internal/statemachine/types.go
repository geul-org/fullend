package statemachine

// StateDiagram represents a parsed Mermaid stateDiagram.
type StateDiagram struct {
	ID           string       // derived from filename (e.g. "course")
	InitialState string       // state after [*] -->
	States       []string     // all unique state names
	Transitions  []Transition // all state transitions
}

// Transition represents a single state transition.
type Transition struct {
	From  string // source state
	To    string // target state
	Event string // operationId / SSaC function name
}

// Events returns all unique event names in this diagram.
func (d *StateDiagram) Events() []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range d.Transitions {
		if !seen[t.Event] {
			seen[t.Event] = true
			result = append(result, t.Event)
		}
	}
	return result
}

// ValidFromStates returns all states from which the given event is valid.
func (d *StateDiagram) ValidFromStates(event string) []string {
	var result []string
	for _, t := range d.Transitions {
		if t.Event == event {
			result = append(result, t.From)
		}
	}
	return result
}
