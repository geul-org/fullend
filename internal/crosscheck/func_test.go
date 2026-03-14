package crosscheck

import (
	"testing"

	"github.com/geul-org/fullend/internal/funcspec"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
)

func TestCheckFuncs_ParamCount(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: true,
	}}

	// 3 inputs but 2 request fields → ERROR.
	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "auth.VerifyPassword",
			Inputs: map[string]string{
				"PasswordHash": "user.PasswordHash",
				"Password":     "request.Password",
				"Extra":        "request.Extra",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "불일치") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param count mismatch ERROR, got: %+v", errs)
	}
}

func TestCheckFuncs_ParamCountMatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: true,
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "auth.VerifyPassword",
			Inputs: map[string]string{
				"PasswordHash": "user.PasswordHash",
				"Password":     "request.Password",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "불일치") {
			t.Errorf("unexpected param count ERROR: %s", e.Message)
		}
	}
}

func TestCheckFuncs_ResultResponseMismatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package:        "auth",
		Name:           "issueToken",
		RequestFields:  []funcspec.Field{{Name: "UserID", Type: "int64"}},
		ResponseFields: []funcspec.Field{}, // no response fields
		HasBody:        true,
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "auth.IssueToken",
			Inputs: map[string]string{
				"UserID": "user.ID",
			},
			Result: &ssacparser.Result{Var: "token", Type: "Token"}, // has result
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "Response 필드 없음") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected result/response mismatch ERROR, got: %+v", errs)
	}
}

func TestCheckFuncs_ResponseIgnoredWarning(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package:        "auth",
		Name:           "doSomething",
		RequestFields:  []funcspec.Field{},
		ResponseFields: []funcspec.Field{{Name: "Value", Type: "string"}},
		HasBody:        true,
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Handler",
		Sequences: []ssacparser.Sequence{{
			Type:   "call",
			Model:  "auth.DoSomething",
			Inputs: nil,
			Result: nil, // no result
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "WARNING" && contains(e.Message, "반환값 무시") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected response ignored WARNING, got: %+v", errs)
	}
}

func TestCheckFuncs_SourceVarUndefined(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: true,
	}}

	// No prior @result defining "user" variable.
	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "auth.VerifyPassword",
			Inputs: map[string]string{
				"PasswordHash": "user.PasswordHash",
				"Password":     "request.Password",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "WARNING" && contains(e.Message, "미정의") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected source var undefined WARNING, got: %+v", errs)
	}
}

func TestCheckFuncs_SourceVarDefined(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: true,
	}}

	// Prior @result defines "user" variable.
	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "user", Type: "User"},
			},
			{
				Type:  "call",
				Model: "auth.VerifyPassword",
				Inputs: map[string]string{
					"PasswordHash": "user.PasswordHash",
					"Password":     "request.Password",
				},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	for _, e := range errs {
		if e.Level == "WARNING" && contains(e.Message, "미정의") {
			t.Errorf("unexpected source var WARNING: %s", e.Message)
		}
	}
}

func TestCheckFuncs_InputFieldNameMismatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "issueToken",
		RequestFields: []funcspec.Field{
			{Name: "UserID", Type: "int64"},
			{Name: "Email", Type: "string"},
		},
		HasBody: true,
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "user", Type: "User"},
			},
			{
				Type:  "call",
				Model: "auth.IssueToken",
				Inputs: map[string]string{
					"ID":    "user.ID",    // wrong: should be UserID
					"Email": "user.Email", // correct
				},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "Request에 없음") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected field name mismatch ERROR, got: %+v", errs)
	}
}

func TestCheckFuncs_InputTypeMismatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "billing",
		Name:    "holdEscrow",
		RequestFields: []funcspec.Field{
			{Name: "GigID", Type: "int64"},
			{Name: "Amount", Type: "int"}, // DDL budget is int64
			{Name: "ClientID", Type: "int64"},
		},
		HasBody: true,
	}}

	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {
				Columns: map[string]string{
					"id":        "int64",
					"budget":    "int64",
					"client_id": "int64",
				},
			},
		},
	}

	sfs := []ssacparser.ServiceFunc{{
		Name: "AcceptProposal",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "gig", Type: "Gig"},
			},
			{
				Type:  "call",
				Model: "billing.HoldEscrow",
				Inputs: map[string]string{
					"GigID":    "gig.ID",
					"Amount":   "gig.Budget",   // int64 vs int → mismatch
					"ClientID": "gig.ClientID", // int64 vs int64 → ok
				},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, st, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "타입 불일치") && contains(e.Message, "Amount") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type mismatch ERROR for Amount, got: %+v", errs)
	}
}

func TestCheckFuncs_InputTypeMatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "billing",
		Name:    "holdEscrow",
		RequestFields: []funcspec.Field{
			{Name: "GigID", Type: "int64"},
			{Name: "Amount", Type: "int64"},
			{Name: "ClientID", Type: "int64"},
		},
		HasBody: true,
	}}

	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {
				Columns: map[string]string{
					"id":        "int64",
					"budget":    "int64",
					"client_id": "int64",
				},
			},
		},
	}

	sfs := []ssacparser.ServiceFunc{{
		Name: "AcceptProposal",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "gig", Type: "Gig"},
			},
			{
				Type:  "call",
				Model: "billing.HoldEscrow",
				Inputs: map[string]string{
					"GigID":    "gig.ID",
					"Amount":   "gig.Budget",
					"ClientID": "gig.ClientID",
				},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, st, nil)
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "타입 불일치") {
			t.Errorf("unexpected type mismatch ERROR: %s", e.Message)
		}
	}
}

func TestCheckFuncs_StubIsError(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: false, // stub
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "auth.VerifyPassword",
			Inputs: map[string]string{
				"PasswordHash": "user.PasswordHash",
				"Password":     "request.Password",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "TODO") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stub TODO ERROR, got: %+v", errs)
	}
	// Ensure it's NOT WARNING.
	for _, e := range errs {
		if e.Level == "WARNING" && contains(e.Message, "TODO") {
			t.Errorf("stub should be ERROR not WARNING: %+v", e)
		}
	}
}

func TestCheckFuncs_ForbiddenImport(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "bad",
		Name:    "doQuery",
		RequestFields: []funcspec.Field{
			{Name: "Key", Type: "string"},
		},
		HasBody: true,
		Imports: []string{"database/sql", "fmt"},
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Handler",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "bad.DoQuery",
			Inputs: map[string]string{
				"Key": "request.Key",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "database/sql") && contains(e.Message, "I/O 패키지") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected forbidden import ERROR for database/sql, got: %+v", errs)
	}

	// fmt should NOT be flagged.
	for _, e := range errs {
		if contains(e.Message, `"fmt"`) {
			t.Errorf("fmt should not be forbidden: %+v", e)
		}
	}
}

func TestCheckFuncs_ForbiddenImportNetHTTP(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "bad",
		Name:    "callAPI",
		RequestFields: []funcspec.Field{
			{Name: "URL", Type: "string"},
		},
		HasBody: true,
		Imports: []string{"net/http"},
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Handler",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "bad.CallAPI",
			Inputs: map[string]string{
				"URL": "request.URL",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "net/http") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected forbidden import ERROR for net/http, got: %+v", errs)
	}
}

func TestCheckFuncs_AllowedImportsOnly(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "calc",
		Name:    "calculate",
		RequestFields: []funcspec.Field{
			{Name: "Value", Type: "int64"},
		},
		HasBody: true,
		Imports: []string{"math", "strings", "fmt", "time", "encoding/json"},
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Handler",
		Sequences: []ssacparser.Sequence{{
			Type:  "call",
			Model: "calc.Calculate",
			Inputs: map[string]string{
				"Value": "request.Value",
			},
		}},
	}}

	errs := CheckFuncs(sfs, specs, nil, nil, nil)
	for _, e := range errs {
		if contains(e.Message, "I/O 패키지") {
			t.Errorf("unexpected forbidden import error: %+v", e)
		}
	}
}

func TestCheckFuncs_PackageModelSkipDDL(t *testing.T) {
	// 패키지 접두사 모델 (@model session.Session.Get)은
	// DDL 테이블 체크 대상이 아님을 간접 확인.
	// CheckFuncs는 직접 DDL을 안 보지만, 패키지 모델이 @call이 아닌
	// @model인 경우 CheckSSaCDDL/CheckDDLCoverage에서 스킵됨.
	// 여기서는 패키지 모델이 specMap에 없어도 @call이 아니면 에러 안 남을 확인.

	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"users": {Columns: map[string]string{"id": "int64"}},
		},
	}

	sfs := []ssacparser.ServiceFunc{{
		Name: "GetSession",
		Sequences: []ssacparser.Sequence{{
			Type:    "get",
			Package: "session",
			Model:   "Session.Get",
			Inputs:  map[string]string{"token": "request.Token"},
			Result:  &ssacparser.Result{Var: "session", Type: "Session"},
		}},
	}}

	// CheckFuncs only processes seq.Type == "call", so package model @get is ignored.
	errs := CheckFuncs(sfs, nil, nil, st, nil)
	for _, e := range errs {
		if e.Level == "ERROR" {
			t.Errorf("unexpected ERROR for package model: %+v", e)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
