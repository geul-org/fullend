package crosscheck

import (
	"fmt"

	ssacparser "github.com/geul-org/ssac/parser"
)

// defaultCheckRequestFields are the fields of the default pkg/authz CheckRequest struct.
var defaultCheckRequestFields = map[string]bool{
	"Action":     true,
	"Resource":   true,
	"UserID":     true,
	"Role":       true,
	"ResourceID": true,
}

// CheckAuthz validates @auth inputs against the authz CheckRequest fields.
func CheckAuthz(funcs []ssacparser.ServiceFunc, authzPackage string) []CrossError {
	var errs []CrossError

	// Determine expected fields.
	// If custom authz package is set, skip validation (we don't have its source to check).
	if authzPackage != "" {
		return errs
	}

	expectedFields := defaultCheckRequestFields

	for _, fn := range funcs {
		for seqIdx, seq := range fn.Sequences {
			if seq.Type != "auth" {
				continue
			}
			ctx := fmt.Sprintf("%s seq[%d] @auth", fn.Name, seqIdx)

			for key := range seq.Inputs {
				if !expectedFields[key] {
					errs = append(errs, CrossError{
						Rule:    "Authz ↔ SSaC",
						Context: ctx,
						Message: fmt.Sprintf("@auth input 필드 %q가 CheckRequest에 없음 (가능: Action, Resource, UserID, Role, ResourceID)", key),
						Level:   "ERROR",
					})
				}
			}
		}
	}

	return errs
}
