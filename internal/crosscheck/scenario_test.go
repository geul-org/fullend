package crosscheck

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/scenario"
	ssacparser "github.com/geul-org/ssac/parser"
)

// helper: build minimal OpenAPI doc with operations.
func buildTestDoc(ops map[string]struct {
	method    string
	path      string
	responses []string
	bodyProps []string
}) *openapi3.T {
	doc := &openapi3.T{
		Paths: openapi3.NewPaths(),
	}
	for opID, info := range ops {
		pi := doc.Paths.Find(info.path)
		if pi == nil {
			pi = &openapi3.PathItem{}
			doc.Paths.Set(info.path, pi)
		}

		op := &openapi3.Operation{OperationID: opID}

		// Responses.
		resps := openapi3.NewResponses()
		for _, code := range info.responses {
			resps.Set(code, &openapi3.ResponseRef{Value: &openapi3.Response{Description: strPtr("ok")}})
		}
		op.Responses = resps

		// Request body.
		if len(info.bodyProps) > 0 {
			schema := openapi3.NewObjectSchema()
			for _, p := range info.bodyProps {
				schema.Properties[p] = &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}
			}
			op.RequestBody = &openapi3.RequestBodyRef{
				Value: openapi3.NewRequestBody().WithJSONSchema(schema),
			}
		}

		switch info.method {
		case "GET":
			pi.Get = op
		case "POST":
			pi.Post = op
		case "PUT":
			pi.Put = op
		case "DELETE":
			pi.Delete = op
		}
	}
	return doc
}

func strPtr(s string) *string { return &s }

// --- Rule 4: Capture reference validity ---

func TestCheckCaptureRefs_Valid(t *testing.T) {
	steps := []scenario.Step{
		{IsAction: true, OperationID: "Login", Method: "POST", Capture: "clientToken"},
		{IsAction: true, OperationID: "CreateGig", Method: "POST", JSON: `{"title": "Build API"}`, Capture: "gigResult"},
		{IsAction: true, OperationID: "PublishGig", Method: "PUT", JSON: `{"id": gigResult.gig.id}`},
	}

	errs := checkCaptureRefs("test.feature", "Happy Path", steps)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckCaptureRefs_UndefinedRef(t *testing.T) {
	steps := []scenario.Step{
		{IsAction: true, OperationID: "PublishGig", Method: "PUT", JSON: `{"id": unknown.gig.id}`},
	}

	errs := checkCaptureRefs("test.feature", "Bad Ref", steps)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Level != "ERROR" {
		t.Errorf("expected ERROR level, got %s", errs[0].Level)
	}
}

func TestCheckCaptureRefs_QuotedValuesIgnored(t *testing.T) {
	steps := []scenario.Step{
		{IsAction: true, OperationID: "Register", Method: "POST", JSON: `{"email": "test@test.com", "role": "client"}`},
	}

	errs := checkCaptureRefs("test.feature", "Register", steps)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for quoted values, got %d: %v", len(errs), errs)
	}
}

// --- Rule 5: Token role matching ---

func TestCheckTokenRoles_Valid(t *testing.T) {
	steps := []scenario.Step{
		{IsAction: true, OperationID: "Register", Method: "POST", JSON: `{"email":"fl@test.com","role": "freelancer"}`},
		{IsAction: true, OperationID: "Login", Method: "POST", JSON: `{"email":"fl@test.com"}`, Capture: "flToken"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
		{IsAction: true, OperationID: "SubmitProposal", Method: "POST"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
	}

	opRoles := map[string][]string{
		"SubmitProposal": {"freelancer"},
	}

	errs := checkTokenRoles("test.feature", "Happy", steps, opRoles)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckTokenRoles_WrongRole(t *testing.T) {
	steps := []scenario.Step{
		{IsAction: true, OperationID: "Register", Method: "POST", JSON: `{"email":"cl@test.com","role": "client"}`},
		{IsAction: true, OperationID: "Login", Method: "POST", JSON: `{"email":"cl@test.com"}`, Capture: "clientToken"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
		{IsAction: true, OperationID: "SubmitProposal", Method: "POST"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
	}

	opRoles := map[string][]string{
		"SubmitProposal": {"freelancer"},
	}

	errs := checkTokenRoles("test.feature", "Wrong Role", steps, opRoles)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Level != "WARNING" {
		t.Errorf("expected WARNING level, got %s", errs[0].Level)
	}
}

func TestCheckTokenRoles_IntentionalRejection(t *testing.T) {
	// Using wrong role but expecting 403 → should NOT warn.
	steps := []scenario.Step{
		{IsAction: true, OperationID: "Register", Method: "POST", JSON: `{"email":"cl@test.com","role": "client"}`},
		{IsAction: true, OperationID: "Login", Method: "POST", JSON: `{"email":"cl@test.com"}`, Capture: "clientToken"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
		{IsAction: true, OperationID: "SubmitProposal", Method: "POST"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "403"}},
	}

	opRoles := map[string][]string{
		"SubmitProposal": {"freelancer"},
	}

	errs := checkTokenRoles("test.feature", "Intentional 403", steps, opRoles)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors (intentional rejection), got %d: %v", len(errs), errs)
	}
}

// --- Rule 6: Status code validity ---

func TestCheckStatusCode_Defined(t *testing.T) {
	doc := buildTestDoc(map[string]struct {
		method    string
		path      string
		responses []string
		bodyProps []string
	}{
		"CreateGig": {method: "POST", path: "/gigs", responses: []string{"200", "400"}, bodyProps: []string{"title"}},
	})
	opMap := buildOpMap(doc)
	info := opMap["CreateGig"]

	steps := []scenario.Step{
		{IsAction: true, OperationID: "CreateGig", Method: "POST"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "200"}},
	}

	errs := checkStatusCode("test.feature", steps, 0, info.op, "CreateGig")
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestCheckStatusCode_Undefined(t *testing.T) {
	doc := buildTestDoc(map[string]struct {
		method    string
		path      string
		responses []string
		bodyProps []string
	}{
		"CreateGig": {method: "POST", path: "/gigs", responses: []string{"200"}, bodyProps: []string{"title"}},
	})
	opMap := buildOpMap(doc)
	info := opMap["CreateGig"]

	steps := []scenario.Step{
		{IsAction: true, OperationID: "CreateGig", Method: "POST"},
		{Assertion: scenario.Assertion{Kind: "status", Value: "500"}},
	}

	errs := checkStatusCode("test.feature", steps, 0, info.op, "CreateGig")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Level != "WARNING" {
		t.Errorf("expected WARNING level, got %s", errs[0].Level)
	}
}

// --- buildOpRoleMap integration ---

func TestBuildOpRoleMap(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name: "SubmitProposal",
			Sequences: []ssacparser.Sequence{
				{Type: ssacparser.SeqAuth, Action: "submit_proposal", Resource: "gig"},
			},
		},
	}
	policies := []*policy.Policy{
		{
			Rules: []policy.AllowRule{
				{Actions: []string{"submit_proposal"}, Resource: "gig", UsesRole: true, RoleValue: "freelancer"},
			},
		},
	}

	result := buildOpRoleMap(funcs, policies)
	roles, ok := result["SubmitProposal"]
	if !ok {
		t.Fatal("expected SubmitProposal in opRoleMap")
	}
	if len(roles) != 1 || roles[0] != "freelancer" {
		t.Errorf("expected [freelancer], got %v", roles)
	}
}
