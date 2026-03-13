package gluegen

import (
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/getkin/kin-openapi/openapi3"
)

// generateDummyValue returns a dummy value for a schema field based on type, format, and field name hints.
func generateDummyValue(fieldName string, schema *openapi3.Schema, checkEnums map[string][]string) interface{} {
	if schema == nil {
		return "test_string"
	}

	// Use first enum value if available (OpenAPI enum).
	if len(schema.Enum) > 0 {
		return fmt.Sprint(schema.Enum[0])
	}

	// Use DDL CHECK constraint enum values.
	if vals, ok := checkEnums[fieldName]; ok && len(vals) > 0 {
		return vals[0]
	}

	// Field name hints (checked before type-based defaults).
	lower := strings.ToLower(fieldName)
	switch {
	case strings.Contains(lower, "password"):
		return "Password1234!"
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount"):
		return 10000
	case strings.Contains(lower, "rating"):
		return 5
	case strings.Contains(lower, "url"):
		return "https://example.com/test"
	}

	switch schema.Type.Slice()[0] {
	case "string":
		switch schema.Format {
		case "email":
			return "test@example.com"
		case "date-time":
			return "2025-01-01T00:00:00Z"
		default:
			return "test_string"
		}
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	case "object":
		return map[string]interface{}{}
	case "array":
		return []interface{}{}
	default:
		return "test_string"
	}
}

// formatDummyValue formats a dummy value as a JSON literal string.
func formatDummyValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case map[string]interface{}:
		return "{}"
	case []interface{}:
		return "[]"
	default:
		return fmt.Sprintf("%q", fmt.Sprint(val))
	}
}

// generateRequestBody builds a JSON request body from an OpenAPI schema.
// Returns the JSON string and a map of field→value for assertion generation.
func generateRequestBody(schema *openapi3.Schema, checkEnums map[string][]string) (string, map[string]interface{}) {
	if schema == nil {
		return "{}", nil
	}

	values := make(map[string]interface{})
	var lines []string
	for name, propRef := range schema.Properties {
		prop := propRef.Value
		val := generateDummyValue(name, prop, checkEnums)
		values[name] = val
		lines = append(lines, fmt.Sprintf("  %s: %s", formatDummyValue(name), formatDummyValue(val)))
	}

	// Sort for deterministic output.
	sortedLines := sortStringSlice(lines)
	return "{\n" + strings.Join(sortedLines, ",\n") + "\n}", values
}

// generateResponseAssertions builds jsonpath assertions from a response schema.
// Response field names are converted to snake_case to match sqlc-generated JSON tags.
func generateResponseAssertions(schema *openapi3.Schema, sentValues map[string]interface{}) []string {
	if schema == nil {
		return nil
	}

	var asserts []string
	for name, propRef := range schema.Properties {
		prop := propRef.Value
		prefix := "$." + name

		if prop.Type.Slice()[0] == "array" {
			asserts = append(asserts, fmt.Sprintf("jsonpath %q isCollection", prefix))
		} else if prop.Type.Slice()[0] == "object" {
			asserts = append(asserts, fmt.Sprintf("jsonpath %q exists", prefix))
			// Check nested ID field (use snake_case for sqlc JSON tags).
			if nested := prop.Properties; nested != nil {
				if _, hasID := nested["ID"]; hasID {
					asserts = append(asserts, fmt.Sprintf("jsonpath %q exists", prefix+".id"))
				}
			}
		} else {
			asserts = append(asserts, fmt.Sprintf("jsonpath %q exists", prefix))
		}
	}

	return sortStringSlice(asserts)
}

// sortStringSlice returns a sorted copy of the string slice.
func sortStringSlice(ss []string) []string {
	result := make([]string, len(ss))
	copy(result, ss)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// resolveSchema follows $ref to get the actual schema.
func resolveSchema(ref *openapi3.SchemaRef) *openapi3.Schema {
	if ref == nil {
		return nil
	}
	return ref.Value
}

// getRequestSchema extracts the request body schema from an operation.
func getRequestSchema(op *openapi3.Operation) *openapi3.Schema {
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	ct := op.RequestBody.Value.Content.Get("application/json")
	if ct == nil || ct.Schema == nil {
		return nil
	}
	return resolveSchema(ct.Schema)
}

// getResponseSchema extracts the 2xx success response schema from an operation.
func getResponseSchema(op *openapi3.Operation) *openapi3.Schema {
	if op.Responses == nil {
		return nil
	}
	// Try explicit 2xx codes first, then fall back to 200.
	for code, respRef := range op.Responses.Map() {
		if len(code) == 3 && code[0] == '2' && respRef != nil && respRef.Value != nil && respRef.Value.Content != nil {
			ct := respRef.Value.Content.Get("application/json")
			if ct != nil && ct.Schema != nil {
				return resolveSchema(ct.Schema)
			}
		}
	}
	return nil
}

// getSuccessHTTPCode returns the numeric 2xx success code string for an operation (e.g. "200", "201", "204").
// Falls back to "200" if no explicit 2xx is found.
func getSuccessHTTPCode(op *openapi3.Operation) string {
	if op.Responses != nil {
		for code := range op.Responses.Map() {
			if len(code) == 3 && code[0] == '2' {
				return code
			}
		}
	}
	return "200"
}

// needsAuth returns true if the operation requires authentication.
func needsAuth(op *openapi3.Operation) bool {
	return op.Security != nil && len(*op.Security) > 0
}

// findTokenJSONPath finds the jsonpath to a string token field in the response schema.
// Handles both flat ("$.access_token") and nested ("$.token.access_token") structures.
func findTokenJSONPath(respSchema *openapi3.Schema) string {
	if respSchema == nil || respSchema.Properties == nil {
		return "token"
	}

	// First pass: look for a string-typed token field at the top level.
	for name := range respSchema.Properties {
		lname := strings.ToLower(name)
		prop := respSchema.Properties[name].Value
		if prop == nil {
			continue
		}
		if (strings.Contains(lname, "token") || strings.Contains(lname, "accesstoken")) &&
			len(prop.Type.Slice()) > 0 && prop.Type.Slice()[0] == "string" {
			return name
		}
	}

	// Second pass: if a token field is an object, look inside for a string token field.
	for name := range respSchema.Properties {
		lname := strings.ToLower(name)
		prop := respSchema.Properties[name].Value
		if prop == nil {
			continue
		}
		if !strings.Contains(lname, "token") {
			continue
		}
		if len(prop.Type.Slice()) > 0 && prop.Type.Slice()[0] == "object" && prop.Properties != nil {
			for innerName := range prop.Properties {
				innerLname := strings.ToLower(innerName)
				innerProp := prop.Properties[innerName].Value
				if innerProp == nil {
					continue
				}
				if (strings.Contains(innerLname, "token") || strings.Contains(innerLname, "access")) &&
					len(innerProp.Type.Slice()) > 0 && innerProp.Type.Slice()[0] == "string" {
					return name + "." + innerName
				}
			}
		}
	}

	// Fallback: return first token-like field regardless of type.
	for name := range respSchema.Properties {
		lname := strings.ToLower(name)
		if strings.Contains(lname, "token") || strings.Contains(lname, "accesstoken") {
			return name
		}
	}

	return "token"
}

// inferCaptureField finds the response field that contains an ID to capture.
// e.g. response has "gig" object with "id" → capture "gig_id" from "$.gig.id"
// Checks both "id" (OpenAPI convention) and "ID" (Go convention) property names.
func inferCaptureField(respSchema *openapi3.Schema) (varName, jsonPath string) {
	if respSchema == nil {
		return "", ""
	}
	for name, propRef := range respSchema.Properties {
		prop := propRef.Value
		if prop.Type.Slice()[0] != "object" {
			continue
		}
		// Check if this object has an id/ID field.
		if prop.Properties != nil {
			if _, hasID := prop.Properties["id"]; hasID {
				return strcase.ToSnake(name) + "_id", "$." + name + ".id"
			}
			if _, hasID := prop.Properties["ID"]; hasID {
				return strcase.ToSnake(name) + "_id", "$." + name + ".ID"
			}
		}
	}
	return "", ""
}
