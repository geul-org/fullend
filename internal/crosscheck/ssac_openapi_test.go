package crosscheck

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
)

func TestCheckErrStatus_DefaultDefined(t *testing.T) {
	// @empty with default 404, OpenAPI has 404 response → no error.
	doc := buildErrStatusDoc("GetGig", "404")

	funcs := []ssacparser.ServiceFunc{{
		Name:     "GetGig",
		FileName: "gig.ssac",
		Sequences: []ssacparser.Sequence{{
			Type:   "empty",
			Target: "gig",
		}},
	}}

	errs := checkErrStatus(funcs, doc)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %+v", len(errs), errs)
	}
}

func TestCheckErrStatus_DefaultMissing(t *testing.T) {
	// @empty with default 404, OpenAPI has no 404 response → error.
	doc := buildErrStatusDoc("GetGig", "200")

	funcs := []ssacparser.ServiceFunc{{
		Name:     "GetGig",
		FileName: "gig.ssac",
		Sequences: []ssacparser.Sequence{{
			Type:   "empty",
			Target: "gig",
		}},
	}}

	errs := checkErrStatus(funcs, doc)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(errs), errs)
	}
	if !contains(errs[0].Message, "404") {
		t.Errorf("expected 404 in message, got: %s", errs[0].Message)
	}
}

func TestCheckErrStatus_CustomDefined(t *testing.T) {
	// @empty with custom 402, OpenAPI has 402 response → no error.
	doc := buildErrStatusDoc("ExecuteWorkflow", "402")

	funcs := []ssacparser.ServiceFunc{{
		Name:     "ExecuteWorkflow",
		FileName: "workflow.ssac",
		Sequences: []ssacparser.Sequence{{
			Type:      "empty",
			Target:    "org",
			ErrStatus: 402,
		}},
	}}

	errs := checkErrStatus(funcs, doc)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %+v", len(errs), errs)
	}
}

func TestCheckErrStatus_CustomMissing(t *testing.T) {
	// @empty with custom 402, OpenAPI has no 402 response → error.
	doc := buildErrStatusDoc("ExecuteWorkflow", "404")

	funcs := []ssacparser.ServiceFunc{{
		Name:     "ExecuteWorkflow",
		FileName: "workflow.ssac",
		Sequences: []ssacparser.Sequence{{
			Type:      "empty",
			Target:    "org",
			ErrStatus: 402,
		}},
	}}

	errs := checkErrStatus(funcs, doc)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(errs), errs)
	}
	if !contains(errs[0].Message, "402") {
		t.Errorf("expected 402 in message, got: %s", errs[0].Message)
	}
}

func buildErrStatusDoc(opID string, responseCode string) *openapi3.T {
	resp := openapi3.NewResponse().WithDescription("response")
	responses := openapi3.NewResponses()
	responses.Set(responseCode, &openapi3.ResponseRef{Value: resp})

	op := &openapi3.Operation{
		OperationID: opID,
		Responses:   responses,
	}

	paths := openapi3.NewPaths()
	paths.Set("/test", &openapi3.PathItem{Post: op})

	return &openapi3.T{Paths: paths}
}
