package contract

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"

	"github.com/geul-org/fullend/internal/statemachine"
)

// HashServiceFunc computes a contract hash for an SSaC service function.
// The hash is derived from: operationId + sequence types + request fields + response fields.
func HashServiceFunc(sf ssacparser.ServiceFunc) string {
	var parts []string
	parts = append(parts, sf.Name)

	// sequence types in order
	var seqTypes []string
	for _, seq := range sf.Sequences {
		seqTypes = append(seqTypes, "@"+seq.Type)
	}
	parts = append(parts, strings.Join(seqTypes, ","))

	// request args (fields from request source)
	var reqFields []string
	for _, seq := range sf.Sequences {
		for _, arg := range seq.Args {
			if arg.Source == "request" {
				reqFields = append(reqFields, arg.Field)
			}
		}
	}
	sort.Strings(reqFields)
	parts = append(parts, strings.Join(reqFields, ","))

	// response fields
	var respFields []string
	for _, seq := range sf.Sequences {
		if seq.Type == "response" && seq.Fields != nil {
			keys := make([]string, 0, len(seq.Fields))
			for k := range seq.Fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				respFields = append(respFields, k+":"+seq.Fields[k])
			}
		}
	}
	parts = append(parts, strings.Join(respFields, ","))

	return Hash7(strings.Join(parts, "|"))
}

// HashModelMethod computes a contract hash for a model implementation method.
// Based on: method name + parameter types + return types.
func HashModelMethod(name string, params []string, returns []string) string {
	parts := []string{name}
	parts = append(parts, strings.Join(params, ","))
	parts = append(parts, strings.Join(returns, ","))
	return Hash7(strings.Join(parts, "|"))
}

// HashStateDiagram computes a contract hash for a state machine.
// Based on: sorted states + sorted transitions (from:event:to).
func HashStateDiagram(sd *statemachine.StateDiagram) string {
	var parts []string

	states := make([]string, len(sd.States))
	copy(states, sd.States)
	sort.Strings(states)
	parts = append(parts, strings.Join(states, ","))

	var transitions []string
	for _, t := range sd.Transitions {
		transitions = append(transitions, t.From+":"+t.Event+":"+t.To)
	}
	sort.Strings(transitions)
	parts = append(parts, strings.Join(transitions, ","))

	return Hash7(strings.Join(parts, "|"))
}

// HashClaims computes a contract hash for middleware claims (CurrentUser).
// Based on: sorted claim name:type pairs.
func HashClaims(claims map[string]string) string {
	keys := make([]string, 0, len(claims))
	for k := range claims {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+":"+claims[k])
	}
	return Hash7(strings.Join(parts, ","))
}

// hash7 returns the first 7 hex characters of SHA256.
// Hash7 computes a 7-character SHA-256 hash.
func Hash7(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:7]
}
