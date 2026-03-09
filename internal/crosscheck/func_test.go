package crosscheck

import (
	"testing"

	"github.com/geul-org/fullend/internal/funcspec"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
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

	// 3 params but 2 request fields → ERROR.
	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:    "call",
			Func:    "verifyPassword",
			Package: "auth",
			Params: []ssacparser.Param{
				{Name: "user.PasswordHash", Source: "user"},
				{Name: "Password", Source: "request"},
				{Name: "Extra", Source: "request"},
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
			Type:    "call",
			Func:    "verifyPassword",
			Package: "auth",
			Params: []ssacparser.Param{
				{Name: "user.PasswordHash", Source: "user"},
				{Name: "Password", Source: "request"},
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
		RequestFields:  []funcspec.Field{{Name: "ID", Type: "int64"}},
		ResponseFields: []funcspec.Field{}, // no response fields
		HasBody:        true,
	}}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{{
			Type:    "call",
			Func:    "issueToken",
			Package: "auth",
			Params:  []ssacparser.Param{{Name: "user.ID", Source: "user"}},
			Result:  &ssacparser.Result{Var: "token", Type: "Token"}, // has result
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
			Type:    "call",
			Func:    "doSomething",
			Package: "auth",
			Params:  nil,
			Result:  nil, // no result
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
			Type:    "call",
			Func:    "verifyPassword",
			Package: "auth",
			Params: []ssacparser.Param{
				{Name: "user.PasswordHash", Source: "user"},
				{Name: "Password", Source: "request"},
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
				Type:    "call",
				Func:    "verifyPassword",
				Package: "auth",
				Params: []ssacparser.Param{
					{Name: "user.PasswordHash", Source: "user"},
					{Name: "Password", Source: "request"},
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

func TestCheckFuncs_PositionalTypeMatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "verifyPassword",
		RequestFields: []funcspec.Field{
			{Name: "PasswordHash", Type: "string"},
			{Name: "Password", Type: "string"},
		},
		HasBody: true,
	}}

	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"User": {
				Columns: map[string]string{
					"PasswordHash": "string",
					"Email":        "string",
					"ID":           "int64",
				},
			},
		},
	}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "user", Type: "User"},
			},
			{
				Type:    "call",
				Func:    "verifyPassword",
				Package: "auth",
				Params: []ssacparser.Param{
					{Name: "user.PasswordHash", Source: "user"},
					{Name: "Password", Source: "request"},
				},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, st, nil)
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "타입 불일치") {
			t.Errorf("unexpected type mismatch: %s", e.Message)
		}
	}
}

func TestCheckFuncs_PositionalTypeMismatch(t *testing.T) {
	specs := []funcspec.FuncSpec{{
		Package: "auth",
		Name:    "issueToken",
		RequestFields: []funcspec.Field{
			{Name: "ID", Type: "string"}, // wrong: should be int64
			{Name: "Email", Type: "string"},
		},
		HasBody: true,
	}}

	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"User": {
				Columns: map[string]string{
					"ID":    "int64",
					"Email": "string",
				},
			},
		},
	}

	sfs := []ssacparser.ServiceFunc{{
		Name: "Login",
		Sequences: []ssacparser.Sequence{
			{
				Type:   "get",
				Result: &ssacparser.Result{Var: "user", Type: "User"},
			},
			{
				Type:    "call",
				Func:    "issueToken",
				Package: "auth",
				Params: []ssacparser.Param{
					{Name: "user.ID", Source: "user"},     // DDL: int64
					{Name: "user.Email", Source: "user"},  // DDL: string
				},
				Result: &ssacparser.Result{Var: "token", Type: "Token"},
			},
		},
	}}

	errs := CheckFuncs(sfs, specs, nil, st, nil)
	found := false
	for _, e := range errs {
		if e.Level == "ERROR" && contains(e.Message, "타입 불일치") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type mismatch ERROR, got: %+v", errs)
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
