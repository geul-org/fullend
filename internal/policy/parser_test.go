package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	content := `package authz

import rego.v1

# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id

default allow := false

allow if {
    input.action == "create"
    input.resource == "course"
}

allow if {
    input.action in {"update", "delete", "publish"}
    input.resource == "course"
    input.user.id == input.resource_owner
}

allow if {
    input.action in {"create", "update", "delete"}
    input.resource == "lesson"
    input.user.id == input.resource_owner
}

allow if {
    input.action == "enroll"
    input.resource == "course"
}

allow if {
    input.action == "delete"
    input.resource == "review"
    input.user.id == input.resource_owner
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "authz.rego")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Ownership mappings.
	if len(p.Ownerships) != 3 {
		t.Fatalf("expected 3 ownership mappings, got %d", len(p.Ownerships))
	}
	// Direct ownership: course → courses.instructor_id
	if p.Ownerships[0].Resource != "course" || p.Ownerships[0].Table != "courses" || p.Ownerships[0].Column != "instructor_id" {
		t.Errorf("ownership[0] = %+v", p.Ownerships[0])
	}
	// Via ownership: lesson → courses.instructor_id via lessons.course_id
	if p.Ownerships[1].JoinTable != "lessons" || p.Ownerships[1].JoinFK != "course_id" {
		t.Errorf("ownership[1] via = %+v", p.Ownerships[1])
	}

	// Allow rules.
	if len(p.Rules) != 5 {
		t.Fatalf("expected 5 allow rules, got %d", len(p.Rules))
	}

	// Rule 0: create course (no owner)
	if p.Rules[0].Actions[0] != "create" || p.Rules[0].Resource != "course" || p.Rules[0].UsesOwner {
		t.Errorf("rule[0] = %+v", p.Rules[0])
	}

	// Rule 1: update/delete/publish course (owner)
	if len(p.Rules[1].Actions) != 3 || !p.Rules[1].UsesOwner {
		t.Errorf("rule[1] = %+v", p.Rules[1])
	}

	// Rule 2: create/update/delete lesson (owner)
	if p.Rules[2].Resource != "lesson" || !p.Rules[2].UsesOwner {
		t.Errorf("rule[2] = %+v", p.Rules[2])
	}

	// ActionResourcePairs.
	pairs := p.ActionResourcePairs()
	if len(pairs) != 9 {
		t.Errorf("expected 9 action-resource pairs, got %d", len(pairs))
	}

	// ResourcesUsingOwner.
	owners := p.ResourcesUsingOwner()
	if len(owners) != 3 {
		t.Errorf("expected 3 resources using owner, got %d: %v", len(owners), owners)
	}
}

func TestParseFileWithRoles(t *testing.T) {
	content := `package authz

import rego.v1

default allow := false

allow if {
    input.action == "publish"
    input.resource == "article"
    input.user.role == "editor"
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "authz.rego")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(p.Rules))
	}
	if !p.Rules[0].UsesRole || p.Rules[0].RoleValue != "editor" {
		t.Errorf("expected role=editor, got %+v", p.Rules[0])
	}
}

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	content := `package authz

import rego.v1

default allow := false

allow if {
    input.action == "create"
    input.resource == "item"
}
`
	if err := os.WriteFile(filepath.Join(dir, "authz.rego"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	policies, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if len(policies[0].Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(policies[0].Rules))
	}
}
