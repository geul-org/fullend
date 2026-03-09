package gluegen

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// generateDummyValue returns a dummy value for a schema field based on type, format, and field name hints.
func generateDummyValue(fieldName string, schema *openapi3.Schema) interface{} {
	if schema == nil {
		return "test_string"
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
	default:
		return fmt.Sprintf("%q", fmt.Sprint(val))
	}
}

// generateRequestBody builds a JSON request body from an OpenAPI schema.
// Returns the JSON string and a map of field→value for assertion generation.
func generateRequestBody(schema *openapi3.Schema) (string, map[string]interface{}) {
	if schema == nil {
		return "{}", nil
	}

	values := make(map[string]interface{})
	var lines []string
	for name, propRef := range schema.Properties {
		prop := propRef.Value
		val := generateDummyValue(name, prop)
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

// getResponseSchema extracts the 200 response schema from an operation.
func getResponseSchema(op *openapi3.Operation) *openapi3.Schema {
	resp := op.Responses.Status(200)
	if resp == nil || resp.Value == nil || resp.Value.Content == nil {
		return nil
	}
	ct := resp.Value.Content.Get("application/json")
	if ct == nil || ct.Schema == nil {
		return nil
	}
	return resolveSchema(ct.Schema)
}

// needsAuth returns true if the operation requires authentication.
func needsAuth(op *openapi3.Operation) bool {
	return op.Security != nil && len(*op.Security) > 0
}

// inferCaptureField finds the response field that contains an ID to capture.
// e.g. response has "course" object with "ID" → capture "course_id" from "$.course.id"
// Uses snake_case field names to match sqlc-generated JSON tags.
func inferCaptureField(respSchema *openapi3.Schema) (varName, jsonPath string) {
	if respSchema == nil {
		return "", ""
	}
	for name, propRef := range respSchema.Properties {
		prop := propRef.Value
		if prop.Type.Slice()[0] != "object" {
			continue
		}
		// Check if this object has an ID field.
		if prop.Properties != nil {
			if _, hasID := prop.Properties["ID"]; hasID {
				return strings.ToLower(name) + "_id", "$." + name + ".id"
			}
		}
	}
	return "", ""
}
