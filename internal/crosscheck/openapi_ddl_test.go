package crosscheck

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

func makeExt(v any) any {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

func TestCheckCursorSort_NoSort_OK(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListGigs",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "cursor", "defaultLimit": 20}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {Columns: map[string]string{"id": "int64"}},
		},
	}
	errs := checkCursorSort(op, st, "GET /gigs (ListGigs)")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckCursorSort_UniqueDefault_OK(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListOrders",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "cursor"}),
			"x-sort": makeExt(map[string]any{
				"allowed":   []string{"order_id"},
				"default":   "order_id",
				"direction": "desc",
			}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"orders": {
				Columns:    map[string]string{"id": "int64", "order_id": "string"},
				PrimaryKey: []string{"id"},
				Indexes: []ssacvalidator.Index{
					{Name: "uniq_order_id", Columns: []string{"order_id"}, IsUnique: true},
				},
			},
		},
	}
	errs := checkCursorSort(op, st, "GET /orders (ListOrders)")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckCursorSort_NonUniqueDefault_ERROR(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListGigs",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "cursor"}),
			"x-sort": makeExt(map[string]any{
				"allowed":   []string{"status"},
				"default":   "status",
				"direction": "desc",
			}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {
				Columns: map[string]string{"id": "int64", "status": "string"},
			},
		},
	}
	errs := checkCursorSort(op, st, "GET /gigs (ListGigs)")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "UNIQUE") {
		t.Errorf("expected UNIQUE error, got %q", errs[0].Message)
	}
}

func TestCheckCursorSort_MultipleAllowed_ERROR(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListGigs",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "cursor"}),
			"x-sort": makeExt(map[string]any{
				"allowed":   []string{"created_at", "budget"},
				"default":   "created_at",
				"direction": "desc",
			}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {Columns: map[string]string{"id": "int64"}},
		},
	}
	errs := checkCursorSort(op, st, "GET /gigs (ListGigs)")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "런타임 정렬 전환") {
		t.Errorf("expected runtime sort error, got %q", errs[0].Message)
	}
}

func TestCheckCursorSort_OffsetIgnored(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListGigs",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "offset"}),
			"x-sort": makeExt(map[string]any{
				"allowed":   []string{"created_at", "budget"},
				"default":   "created_at",
				"direction": "desc",
			}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {Columns: map[string]string{"id": "int64"}},
		},
	}
	errs := checkCursorSort(op, st, "GET /gigs (ListGigs)")
	if len(errs) != 0 {
		t.Errorf("expected no errors for offset mode, got %v", errs)
	}
}

func TestCheckCursorSort_PKColumn_OK(t *testing.T) {
	op := &openapi3.Operation{
		OperationID: "ListGigs",
		Extensions: map[string]any{
			"x-pagination": makeExt(map[string]any{"style": "cursor"}),
			"x-sort": makeExt(map[string]any{
				"allowed":   []string{"id"},
				"default":   "id",
				"direction": "desc",
			}),
		},
	}
	st := &ssacvalidator.SymbolTable{
		DDLTables: map[string]ssacvalidator.DDLTable{
			"gigs": {
				Columns:    map[string]string{"id": "int64", "title": "string"},
				PrimaryKey: []string{"id"},
			},
		},
	}
	errs := checkCursorSort(op, st, "GET /gigs (ListGigs)")
	if len(errs) != 0 {
		t.Errorf("expected no errors for PK column, got %v", errs)
	}
}
