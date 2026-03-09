package statemachine

import (
	"testing"
)

func TestParseCourseStateDiagram(t *testing.T) {
	content := `# CourseState

` + "```mermaid" + `
stateDiagram-v2
    [*] --> unpublished
    unpublished --> published: PublishCourse
    published --> deleted: DeleteCourse
    unpublished --> deleted: DeleteCourse
` + "```" + `
`

	d, err := Parse("course", content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if d.ID != "course" {
		t.Errorf("ID = %q, want %q", d.ID, "course")
	}

	if d.InitialState != "unpublished" {
		t.Errorf("InitialState = %q, want %q", d.InitialState, "unpublished")
	}

	if len(d.Transitions) != 3 {
		t.Fatalf("Transitions count = %d, want 3", len(d.Transitions))
	}

	// Check transitions.
	expected := []Transition{
		{From: "unpublished", To: "published", Event: "PublishCourse"},
		{From: "published", To: "deleted", Event: "DeleteCourse"},
		{From: "unpublished", To: "deleted", Event: "DeleteCourse"},
	}
	for i, want := range expected {
		got := d.Transitions[i]
		if got.From != want.From || got.To != want.To || got.Event != want.Event {
			t.Errorf("Transition[%d] = %+v, want %+v", i, got, want)
		}
	}

	// Check Events().
	events := d.Events()
	if len(events) != 2 {
		t.Errorf("Events count = %d, want 2", len(events))
	}

	// Check ValidFromStates.
	fromStates := d.ValidFromStates("DeleteCourse")
	if len(fromStates) != 2 {
		t.Errorf("ValidFromStates(DeleteCourse) count = %d, want 2", len(fromStates))
	}
}

func TestParseNoMermaidBlock(t *testing.T) {
	_, err := Parse("test", "# No mermaid block here")
	if err == nil {
		t.Error("expected error for missing mermaid block")
	}
}

func TestParseNoTransitions(t *testing.T) {
	content := "```mermaid\nstateDiagram-v2\n    [*] --> draft\n```"
	_, err := Parse("test", content)
	if err == nil {
		t.Error("expected error for no transitions")
	}
}
