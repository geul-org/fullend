package scenario

import (
	"testing"
)

func TestParseActionStep(t *testing.T) {
	tests := []struct {
		line    string
		method  string
		opID    string
		json    string
		capture string
	}{
		{
			`Given POST Register {"Email": "a@b.com", "Password": "Pass1234!", "Name": "Test"} → user`,
			"POST", "Register", `{"Email": "a@b.com", "Password": "Pass1234!", "Name": "Test"}`, "user",
		},
		{
			`And POST Login {"Email": "a@b.com", "Password": "Pass1234!"} → token`,
			"POST", "Login", `{"Email": "a@b.com", "Password": "Pass1234!"}`, "token",
		},
		{
			`When PUT PublishCourse {"CourseID": course.ID}`,
			"PUT", "PublishCourse", `{"CourseID": course.ID}`, "",
		},
		{
			`Then GET ListCourses → courses`,
			"GET", "ListCourses", "", "courses",
		},
		{
			`When DELETE DeleteCourse {"CourseID": course.ID}`,
			"DELETE", "DeleteCourse", `{"CourseID": course.ID}`, "",
		},
	}

	for _, tt := range tests {
		step, err := parseStep(tt.line)
		if err != nil {
			t.Fatalf("parseStep(%q): %v", tt.line, err)
		}
		if step == nil {
			t.Fatalf("parseStep(%q): returned nil", tt.line)
		}
		if !step.IsAction {
			t.Errorf("expected action step for %q", tt.line)
		}
		if step.Method != tt.method {
			t.Errorf("method: got %q, want %q", step.Method, tt.method)
		}
		if step.OperationID != tt.opID {
			t.Errorf("opID: got %q, want %q", step.OperationID, tt.opID)
		}
		if step.JSON != tt.json {
			t.Errorf("json: got %q, want %q", step.JSON, tt.json)
		}
		if step.Capture != tt.capture {
			t.Errorf("capture: got %q, want %q", step.Capture, tt.capture)
		}
	}
}

func TestParseAssertionSteps(t *testing.T) {
	tests := []struct {
		line string
		kind AssertionKind
		val  string
	}{
		{`Then status == 200`, AssertStatus, "200"},
		{`And status == 401`, AssertStatus, "401"},
		{`And response.enrollment exists`, AssertExists, ""},
		{`And response.courses contains course.ID`, AssertContains, "course.ID"},
		{`And response.courses excludes course.ID`, AssertExcludes, "course.ID"},
	}

	for _, tt := range tests {
		step, err := parseStep(tt.line)
		if err != nil {
			t.Fatalf("parseStep(%q): %v", tt.line, err)
		}
		if step == nil {
			t.Fatalf("parseStep(%q): returned nil", tt.line)
		}
		if step.IsAction {
			t.Errorf("expected assertion step for %q", tt.line)
		}
		if step.Assertion.Kind != tt.kind {
			t.Errorf("kind: got %q, want %q", step.Assertion.Kind, tt.kind)
		}
		if tt.val != "" && step.Assertion.Value != tt.val {
			t.Errorf("value: got %q, want %q", step.Assertion.Value, tt.val)
		}
	}
}

func TestParseFeature(t *testing.T) {
	content := `@scenario
Feature: Instructor creates and publishes a course

  Scenario: Full course lifecycle
    Given POST Register {"Email": "inst@test.com", "Password": "Pass1234!", "Name": "Instructor"} → user
    And POST Login {"Email": "inst@test.com", "Password": "Pass1234!"} → token
    When POST CreateCourse {"Title": "Go 101", "Category": "dev", "Level": "beginner", "Price": 10000} → course
    And PUT PublishCourse {"CourseID": course.ID}
    Then GET ListCourses → courses
    And response.courses contains course.ID
    And status == 200
`

	f, err := Parse("test.feature", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Tag != "@scenario" {
		t.Errorf("tag: got %q", f.Tag)
	}
	if f.Name != "Instructor creates and publishes a course" {
		t.Errorf("name: got %q", f.Name)
	}
	if len(f.Scenarios) != 1 {
		t.Fatalf("scenarios: got %d", len(f.Scenarios))
	}

	sc := f.Scenarios[0]
	if sc.Name != "Full course lifecycle" {
		t.Errorf("scenario name: got %q", sc.Name)
	}

	// 5 action + 2 assertion = 7 steps
	if len(sc.Steps) != 7 {
		t.Fatalf("steps: got %d, want 7", len(sc.Steps))
	}

	// First step: POST Register
	if sc.Steps[0].Method != "POST" || sc.Steps[0].OperationID != "Register" {
		t.Errorf("step 0: got %s %s", sc.Steps[0].Method, sc.Steps[0].OperationID)
	}
	if sc.Steps[0].Capture != "user" {
		t.Errorf("step 0 capture: got %q", sc.Steps[0].Capture)
	}

	// Assertion step: contains
	if sc.Steps[5].Assertion.Kind != AssertContains {
		t.Errorf("step 5 kind: got %q", sc.Steps[5].Assertion.Kind)
	}
}

func TestParseFeatureWithBackground(t *testing.T) {
	content := `@scenario
Feature: Student enrolls in a published course

  Background:
    Given POST Register {"Email": "inst@test.com", "Password": "Pass1234!", "Name": "Inst"} → instructor
    And POST Login {"Email": "inst@test.com", "Password": "Pass1234!"} → token
    And POST CreateCourse {"Title": "Go 101", "Category": "dev", "Level": "beginner", "Price": 10000} → course
    And PUT PublishCourse {"CourseID": course.ID}

  Scenario: Successful enrollment
    Given POST Register {"Email": "student@test.com", "Password": "Pass1234!", "Name": "Student"} → student
    And POST Login {"Email": "student@test.com", "Password": "Pass1234!"} → token
    When POST EnrollCourse {"CourseID": course.ID, "PaymentMethod": "card"} → enrollment
    Then status == 200
`

	f, err := Parse("test.feature", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Background == nil {
		t.Fatal("expected Background")
	}
	if len(f.Background.Steps) != 4 {
		t.Errorf("background steps: got %d, want 4", len(f.Background.Steps))
	}
	if len(f.Scenarios) != 1 {
		t.Fatalf("scenarios: got %d", len(f.Scenarios))
	}
	if len(f.Scenarios[0].Steps) != 4 {
		t.Errorf("scenario steps: got %d, want 4", len(f.Scenarios[0].Steps))
	}
}

func TestAllOperationIDs(t *testing.T) {
	content := `@scenario
Feature: Test

  Scenario: S1
    Given POST Register {"Email": "a@b.com"} → user
    And POST Login {"Email": "a@b.com"} → token
    When GET ListCourses → courses
`

	f, err := Parse("test.feature", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ids := AllOperationIDs([]*Feature{f})
	if len(ids) != 3 {
		t.Errorf("operationIDs: got %d, want 3", len(ids))
	}
}
