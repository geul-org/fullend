package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/artifacts/internal/scenario"
)

// GenerateScenarioHurl generates scenario-*.hurl and invariant-*.hurl from .feature files.
func GenerateScenarioHurl(features []*scenario.Feature, doc *openapi3.T, outDir string) error {
	if doc == nil || len(features) == 0 {
		return nil
	}

	testsDir := filepath.Join(outDir, "tests")
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		return fmt.Errorf("create tests dir: %w", err)
	}

	// Build operationId → (method, path) map.
	opMap := buildOperationMap(doc)

	for _, f := range features {
		prefix := "scenario"
		if f.Tag == "@invariant" {
			prefix = "invariant"
		}

		// Derive filename from feature file.
		base := filepath.Base(f.File)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		outFile := filepath.Join(testsDir, fmt.Sprintf("%s-%s.hurl", prefix, base))

		content, err := renderFeatureHurl(f, doc, opMap)
		if err != nil {
			return fmt.Errorf("render %s: %w", f.File, err)
		}

		if err := os.WriteFile(outFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", outFile, err)
		}
	}

	return nil
}

type operationInfo struct {
	Method string
	Path   string
	Op     *openapi3.Operation
}

func buildOperationMap(doc *openapi3.T) map[string]operationInfo {
	m := make(map[string]operationInfo)
	if doc.Paths == nil {
		return m
	}
	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op != nil && op.OperationID != "" {
				m[op.OperationID] = operationInfo{Method: method, Path: path, Op: op}
			}
		}
	}
	return m
}

func renderFeatureHurl(f *scenario.Feature, doc *openapi3.T, opMap map[string]operationInfo) (string, error) {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Auto-generated from %s — do not edit.\n\n", filepath.Base(f.File)))

	for i, sc := range f.Scenarios {
		if i > 0 {
			buf.WriteString("\n")
		}
		if sc.Name != "" {
			buf.WriteString(fmt.Sprintf("# === %s ===\n\n", sc.Name))
		}

		// Merge Background + Scenario steps.
		var steps []scenario.Step
		if f.Background != nil {
			steps = append(steps, f.Background.Steps...)
		}
		steps = append(steps, sc.Steps...)

		// Track captures and token state.
		captures := make(map[string]bool)
		hasToken := false

		idx := 0
		for idx < len(steps) {
			step := steps[idx]
			if step.IsAction {
				// Look ahead: collect trailing assertions and find status code.
				statusCode := "200"
				var trailingAssertions []scenario.Step
				j := idx + 1
				for j < len(steps) && !steps[j].IsAction {
					if steps[j].Assertion.Kind == scenario.AssertStatus {
						statusCode = steps[j].Assertion.Value
					} else {
						trailingAssertions = append(trailingAssertions, steps[j])
					}
					j++
				}
				writeActionHurlV2(&buf, step, statusCode, trailingAssertions, opMap, doc, captures, &hasToken)
				idx = j
			} else {
				// Standalone assertion (shouldn't happen normally, but handle gracefully).
				idx++
			}
		}
	}

	return buf.String(), nil
}

func writeActionHurlV2(buf *strings.Builder, step scenario.Step, statusCode string, assertions []scenario.Step, opMap map[string]operationInfo, doc *openapi3.T, captures map[string]bool, hasToken *bool) {
	info, ok := opMap[step.OperationID]
	if !ok {
		buf.WriteString(fmt.Sprintf("# SKIP: %s %s (operationId not found)\n\n", step.Method, step.OperationID))
		return
	}

	buf.WriteString(fmt.Sprintf("# %s\n", step.OperationID))

	// Build URL: substitute path params from JSON or captures.
	url := buildScenarioURL(info.Path, step.JSON, captures)
	buf.WriteString(fmt.Sprintf("%s {{host}}%s\n", info.Method, url))

	// Auth header if token captured and endpoint needs auth.
	if *hasToken && needsAuth(info.Op) {
		buf.WriteString("Authorization: Bearer {{token}}\n")
	}

	// Request body: render JSON with variable substitution.
	body := buildScenarioBody(step.JSON, info.Path)
	if body != "" {
		buf.WriteString("Content-Type: application/json\n")
		buf.WriteString(body + "\n")
	}

	// Status line.
	buf.WriteString(fmt.Sprintf("\nHTTP %s\n", statusCode))

	// Captures (only for 2xx responses).
	if step.Capture != "" && (statusCode == "200" || statusCode == "201") {
		captures[step.Capture] = true
		if step.Capture == "token" {
			*hasToken = true
			buf.WriteString("[Captures]\n")
			buf.WriteString("token: jsonpath \"$.token.AccessToken\"\n")
		} else {
			// Infer capture from response schema — skip if response is an array.
			captureVar, jsonPath := inferScenarioCapture(step.Capture, info.Op)
			if captureVar != "" {
				buf.WriteString("[Captures]\n")
				buf.WriteString(fmt.Sprintf("%s: jsonpath %q\n", captureVar, jsonPath))
			}
		}
	}

	// Merged [Asserts] block for all trailing assertions.
	if len(assertions) > 0 {
		buf.WriteString("[Asserts]\n")
		for _, a := range assertions {
			writeAssertLineV2(buf, a.Assertion, captures)
		}
	}

	buf.WriteString("\n")
}

func writeAssertLineV2(buf *strings.Builder, a scenario.Assertion, captures map[string]bool) {
	switch a.Kind {
	case scenario.AssertExists:
		buf.WriteString(fmt.Sprintf("jsonpath \"$.%s\" exists\n", a.Field))

	case scenario.AssertEquals:
		val := resolveVarRef(a.Value, captures)
		buf.WriteString(fmt.Sprintf("jsonpath \"$.%s\" == %s\n", a.Field, val))

	case scenario.AssertContains:
		val := resolveVarRef(a.Value, captures)
		_, subField := splitVarRef(a.Value)
		if subField != "" {
			buf.WriteString(fmt.Sprintf("jsonpath \"$.%s[*].%s\" includes %s\n", a.Field, subField, val))
		} else {
			buf.WriteString(fmt.Sprintf("jsonpath \"$.%s\" includes %s\n", a.Field, val))
		}

	case scenario.AssertExcludes:
		val := resolveVarRef(a.Value, captures)
		_, subField := splitVarRef(a.Value)
		if subField != "" {
			buf.WriteString(fmt.Sprintf("jsonpath \"$.%s[*].%s\" not includes %s\n", a.Field, subField, val))
		} else {
			buf.WriteString(fmt.Sprintf("jsonpath \"$.%s\" not includes %s\n", a.Field, val))
		}

	case scenario.AssertCount:
		buf.WriteString(fmt.Sprintf("jsonpath \"$.%s\" count %s %s\n", a.Field, a.Op, a.Value))
	}
}

// buildScenarioURL substitutes path parameters from JSON body and captures.
func buildScenarioURL(pathTemplate, json string, captures map[string]bool) string {
	result := pathTemplate
	i := 0
	for i < len(result) {
		openBrace := strings.Index(result[i:], "{")
		if openBrace < 0 {
			break
		}
		pos := i + openBrace
		closeBrace := strings.Index(result[pos:], "}")
		if closeBrace < 0 {
			break
		}
		paramName := result[pos+1 : pos+closeBrace]

		// Try to find value in JSON body.
		varName := findJSONVarRef(json, paramName)
		if varName != "" {
			hurlVar := varRefToHurl(varName)
			result = result[:pos] + "{{" + hurlVar + "}}" + result[pos+closeBrace+1:]
			i = pos + len(hurlVar) + 4
		} else {
			hurlVar := pascalToSnakeHurl(paramName)
			result = result[:pos] + "{{" + hurlVar + "}}" + result[pos+closeBrace+1:]
			i = pos + len(hurlVar) + 4
		}
	}
	return result
}

// buildScenarioBody renders the JSON body, substituting variable references.
// Path parameters are excluded from the body.
func buildScenarioBody(json, pathTemplate string) string {
	if json == "" {
		return ""
	}

	// Identify path param names.
	pathParams := make(map[string]bool)
	for i := 0; i < len(pathTemplate); i++ {
		if pathTemplate[i] == '{' {
			end := strings.Index(pathTemplate[i:], "}")
			if end > 0 {
				pathParams[pathTemplate[i+1:i+end]] = true
				i += end
			}
		}
	}

	// Parse and filter JSON fields.
	inner := strings.TrimSpace(json)
	inner = strings.TrimPrefix(inner, "{")
	inner = strings.TrimSuffix(inner, "}")

	parts := splitJSONFields(inner)
	var kept []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		colonIdx := strings.Index(part, ":")
		if colonIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:colonIdx])
		key = strings.Trim(key, `"`)
		if pathParams[key] {
			continue // skip path params from body
		}

		// Substitute variable references in values.
		value := strings.TrimSpace(part[colonIdx+1:])
		value = substituteVarRefs(value)
		kept = append(kept, fmt.Sprintf("  %q: %s", key, value))
	}

	if len(kept) == 0 {
		return ""
	}
	return "{\n" + strings.Join(kept, ",\n") + "\n}"
}

// splitJSONFields splits JSON fields by comma, respecting nesting.
func splitJSONFields(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, c := range s {
		switch c {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// substituteVarRefs replaces unquoted var.Field references with {{var_field}} Hurl syntax.
func substituteVarRefs(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, `"`) && strings.Contains(value, ".") {
		return "{{" + varRefToHurl(value) + "}}"
	}
	return value
}

// varRefToHurl converts var.Field to var_field (snake_case).
func varRefToHurl(ref string) string {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 {
		return pascalToSnakeHurl(ref)
	}
	return parts[0] + "_" + strings.ToLower(parts[1])
}

// resolveVarRef converts a value reference to Hurl variable syntax.
func resolveVarRef(value string, captures map[string]bool) string {
	if strings.Contains(value, ".") {
		return "{{" + varRefToHurl(value) + "}}"
	}
	return value
}

// splitVarRef splits "var.Field" into ("var", "Field").
func splitVarRef(ref string) (string, string) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return ref, ""
}

// findJSONVarRef looks for a field in JSON and returns the variable reference.
func findJSONVarRef(json, fieldName string) string {
	if json == "" {
		return ""
	}
	inner := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(json), "}"), "{")
	parts := splitJSONFields(inner)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		colonIdx := strings.Index(part, ":")
		if colonIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:colonIdx])
		key = strings.Trim(key, `"`)
		if key != fieldName {
			continue
		}
		val := strings.TrimSpace(part[colonIdx+1:])
		if !strings.HasPrefix(val, `"`) && strings.Contains(val, ".") {
			return val
		}
	}
	return ""
}

// inferScenarioCapture infers capture variable and jsonpath from response schema.
// Returns empty strings if the response is an array (no meaningful ID capture).
func inferScenarioCapture(captureName string, op *openapi3.Operation) (string, string) {
	respSchema := getResponseSchema(op)
	if respSchema == nil {
		return captureName + "_id", "$." + captureName + ".ID"
	}

	// Look for a response field matching the capture name.
	for name, propRef := range respSchema.Properties {
		if strings.EqualFold(name, captureName) {
			prop := propRef.Value
			if prop == nil {
				continue
			}
			// Skip array responses — no single ID to capture.
			if len(prop.Type.Slice()) > 0 && prop.Type.Slice()[0] == "array" {
				return "", ""
			}
			if prop.Properties != nil {
				if _, hasID := prop.Properties["ID"]; hasID {
					return captureName + "_id", "$." + name + ".ID"
				}
			}
		}
	}

	// Fallback: first object with ID (skip arrays).
	varName, jsonPath := inferCaptureField(respSchema)
	if varName != "" {
		return varName, jsonPath
	}

	return "", ""
}
