package contract

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// The gate for the frozen surface: every example in testdata/api validates
// against its schema, every example matches its Go wire type exactly, and
// every validating example in docs/interface.md is byte-identical in
// meaning to its testdata file. A drift between document, schema and code
// fails the build.

// apiDir returns the testdata/api directory beside this package.
func apiDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "testdata", "api")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("testdata/api is missing at %s: %v", dir, err)
	}
	return dir
}

// loadJSON reads and parses one JSON file.
func loadJSON(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return v
}

// schemaValidator checks example documents against the subset of JSON
// Schema the frozen schemas use: $ref to a local file with an optional
// pointer, type (single or union), required, properties,
// additionalProperties, items, enum, const, minimum, maximum, minLength,
// maxLength, minItems and maxItems. Format and annotation keywords are
// accepted and ignored.
type schemaValidator struct {
	dir   string
	cache map[string]any
}

// resolve splits a local reference of the form file.json or
// file.json#/$defs/Name and returns the referenced schema.
func (v *schemaValidator) resolve(ref string) (any, error) {
	file, ptr, _ := strings.Cut(ref, "#")
	if file == "" {
		return nil, fmt.Errorf("empty $ref file in %q", ref)
	}
	doc, ok := v.cache[file]
	if !ok {
		raw, err := os.ReadFile(filepath.Join(v.dir, file))
		if err != nil {
			return nil, fmt.Errorf("read $ref %q: %w", ref, err)
		}
		var parsed any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("parse $ref %q: %w", ref, err)
		}
		doc = parsed
		v.cache[file] = doc
	}
	if ptr == "" {
		return doc, nil
	}
	current := doc
	for _, part := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("pointer %q does not resolve in %q", ref, file)
		}
		next, ok := obj[part]
		if !ok {
			return nil, fmt.Errorf("pointer %q does not resolve in %q", ref, file)
		}
		current = next
	}
	return current, nil
}

// typeName reports the JSON type of a decoded value.
func typeName(v any) string {
	switch value := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		if value == math.Trunc(value) {
			return "integer"
		}
		return "number"
	default:
		return "unknown"
	}
}

// validate checks a value against a decoded schema, naming failures by
// JSON path.
func (v *schemaValidator) validate(schema any, value any, path string) []string {
	if pass, ok := schema.(bool); ok {
		if pass {
			return nil
		}
		return []string{fmt.Sprintf("%s: schema is false", path)}
	}
	def, ok := schema.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("%s: schema is not an object", path)}
	}
	if ref, ok := def["$ref"].(string); ok {
		resolved, err := v.resolve(ref)
		if err != nil {
			return []string{fmt.Sprintf("%s: %v", path, err)}
		}
		merged := map[string]any{}
		if resolvedMap, ok := resolved.(map[string]any); ok {
			for key, val := range resolvedMap {
				merged[key] = val
			}
		} else {
			merged["$ref-resolved"] = resolved
			return v.validate(resolved, value, path)
		}
		for key, val := range def {
			if key == "$ref" {
				continue
			}
			merged[key] = val
		}
		return v.validate(merged, value, path)
	}
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, path+": "+fmt.Sprintf(format, args...))
	}
	if rawType, present := def["type"]; present {
		var names []string
		switch typed := rawType.(type) {
		case string:
			names = []string{typed}
		case []any:
			for _, entry := range typed {
				name, ok := entry.(string)
				if !ok {
					return []string{fmt.Sprintf("%s: type entry is not a string", path)}
				}
				names = append(names, name)
			}
		default:
			return []string{fmt.Sprintf("%s: type is not a string or array", path)}
		}
		actual := typeName(value)
		matched := false
		for _, name := range names {
			if actual == name || (actual == "integer" && name == "number") {
				matched = true
				break
			}
		}
		if !matched {
			return []string{fmt.Sprintf("%s: expected type %v, got %s", path, names, actual)}
		}
		if value == nil {
			return nil
		}
	}
	if rawEnum, ok := def["enum"].([]any); ok && value != nil {
		matched := false
		for _, option := range rawEnum {
			if reflect.DeepEqual(option, value) {
				matched = true
				break
			}
		}
		if !matched {
			fail("value %v is not one of %v", value, rawEnum)
		}
	}
	if constant, ok := def["const"]; ok {
		if !reflect.DeepEqual(constant, value) {
			fail("value %v does not equal const %v", value, constant)
		}
	}
	switch actual := typeName(value); actual {
	case "object":
		obj := value.(map[string]any)
		if rawRequired, ok := def["required"].([]any); ok {
			for _, entry := range rawRequired {
				name, ok := entry.(string)
				if !ok {
					return []string{fmt.Sprintf("%s: required entry is not a string", path)}
				}
				if _, present := obj[name]; !present {
					fail("missing required property %q", name)
				}
			}
		}
		props := map[string]any{}
		if rawProps, ok := def["properties"].(map[string]any); ok {
			props = rawProps
		}
		for key, propSchema := range props {
			if field, present := obj[key]; present {
				failures = append(failures, v.validate(propSchema, field, path+"."+key)...)
			}
		}
		if closed, ok := def["additionalProperties"].(bool); ok && !closed {
			for key := range obj {
				if _, declared := props[key]; !declared {
					fail("undeclared property %q", key)
				}
			}
		}
	case "array":
		items := value.([]any)
		if rawMin, ok := def["minItems"].(float64); ok {
			if float64(len(items)) < rawMin {
				fail("array has %d items, needs at least %v", len(items), rawMin)
			}
		}
		if rawMax, ok := def["maxItems"].(float64); ok {
			if float64(len(items)) > rawMax {
				fail("array has %d items, allows at most %v", len(items), rawMax)
			}
		}
		if itemSchema, ok := def["items"]; ok {
			for i, item := range items {
				failures = append(failures, v.validate(itemSchema, item, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case "string":
		text := value.(string)
		if rawMin, ok := def["minLength"].(float64); ok {
			if float64(utf8.RuneCountInString(text)) < rawMin {
				fail("string is shorter than minLength %v", rawMin)
			}
		}
		if rawMax, ok := def["maxLength"].(float64); ok {
			if float64(utf8.RuneCountInString(text)) > rawMax {
				fail("string is longer than maxLength %v", rawMax)
			}
		}
	case "integer", "number":
		number := value.(float64)
		if rawMin, ok := def["minimum"].(float64); ok {
			if number < rawMin {
				fail("number %v is below minimum %v", number, rawMin)
			}
		}
		if rawMax, ok := def["maximum"].(float64); ok {
			if number > rawMax {
				fail("number %v is above maximum %v", number, rawMax)
			}
		}
	}
	return failures
}

// schemaForExample maps an example file to its schema file by cutting at
// the .example marker: record_reason.response.example-unclear.json reads
// record_reason.response.schema.json.
func schemaForExample(name string) (string, bool) {
	if !strings.Contains(name, ".example") || !strings.HasSuffix(name, ".json") {
		return "", false
	}
	base := name[:strings.Index(name, ".example")]
	return base + ".schema.json", true
}

// goValueFor maps an example file to its Go wire type for the strict
// round trip: unknown fields fail the build.
func goValueFor(name string) (any, bool) {
	core := name
	if i := strings.Index(core, ".example"); i >= 0 {
		core = core[:i]
	}
	switch core {
	case "recall_for_purchase.request":
		return &RecallRequest{}, true
	case "recall_for_purchase.response":
		return &RecallResponse{}, true
	case "record_correction.request":
		return &RecordCorrectionRequest{}, true
	case "record_correction.response":
		return &RecordCorrectionResponse{}, true
	case "pending_questions.request":
		return &PendingQuestionsRequest{}, true
	case "pending_questions.response":
		return &PendingQuestionsResponse{}, true
	case "record_reason.request":
		return &RecordReasonRequest{}, true
	case "record_reason.response":
		return &RecordReasonResponse{}, true
	case "record_feedback.request":
		return &RecordFeedbackRequest{}, true
	case "record_feedback.response":
		return &RecordFeedbackResponse{}, true
	case "explain_belief.request":
		return &ExplainBeliefRequest{}, true
	case "explain_belief.response":
		return &ExplainBeliefResponse{}, true
	case "ledger_beliefs.request":
		return &LedgerBeliefsRequest{}, true
	case "ledger_beliefs.response":
		return &LedgerBeliefsResponse{}, true
	case "ledger_belief.request":
		return &LedgerBeliefRequest{}, true
	case "ledger_belief.response":
		return &LedgerBeliefResponse{}, true
	case "ledger_evidence.request":
		return &LedgerEvidenceRequest{}, true
	case "ledger_evidence.response":
		return &LedgerEvidenceResponse{}, true
	default:
		return nil, false
	}
}

// exampleFiles lists every example file in testdata/api, sorted.
func exampleFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.Contains(name, ".example") || !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no example files in testdata/api")
	}
	return names
}

// TestToolsFrozen pins the surface: nine tools, exact names, and a schema
// file behind every descriptor.
func TestToolsFrozen(t *testing.T) {
	if len(ToolNames) != 9 {
		t.Fatalf("frozen surface has %d tools, want 9", len(ToolNames))
	}
	seen := map[ToolName]bool{}
	for _, name := range ToolNames {
		if seen[name] {
			t.Errorf("duplicate tool name %q", name)
		}
		seen[name] = true
	}
	dir := apiDir(t)
	for _, tool := range Tools() {
		if !seen[tool.Name] {
			t.Errorf("tool %q has a descriptor but no frozen name", tool.Name)
		}
		for _, file := range []string{tool.RequestSchema, tool.ResponseSchema} {
			if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
				t.Errorf("tool %q: schema %s is missing: %v", tool.Name, file, err)
			}
		}
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
	}
}

// TestExamplesValidateAgainstSchemas checks every example request and
// response in testdata/api against its schema.
func TestExamplesValidateAgainstSchemas(t *testing.T) {
	dir := apiDir(t)
	validator := &schemaValidator{dir: dir, cache: map[string]any{}}
	for _, name := range exampleFiles(t, dir) {
		schemaName, ok := schemaForExample(name)
		if !ok {
			t.Errorf("%s: cannot map example to schema", name)
			continue
		}
		schema := loadJSON(t, filepath.Join(dir, schemaName))
		value := loadJSON(t, filepath.Join(dir, name))
		if failures := validator.validate(schema, value, "$"); len(failures) > 0 {
			t.Errorf("%s fails %s:\n  %s", name, schemaName, strings.Join(failures, "\n  "))
		}
	}
}

// TestExamplesMatchGoTypes unmarshals every example into its Go wire type
// with unknown fields rejected, so schema and code cannot drift apart.
func TestExamplesMatchGoTypes(t *testing.T) {
	dir := apiDir(t)
	for _, name := range exampleFiles(t, dir) {
		target, ok := goValueFor(name)
		if !ok {
			t.Errorf("%s: no Go wire type", name)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			t.Errorf("%s does not match its Go type: %v", name, err)
		}
	}
}

// docExamples extracts the validating examples from docs/interface.md.
// A validating example is a fenced block opened with
// ```json validate=<name>, where <name> is the testdata/api file without
// its .json suffix.
func docExamples(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "interface.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read docs/interface.md: %v", err)
	}
	blocks := map[string]any{}
	var current *string
	var lines []string
	flush := func() {
		if current == nil {
			return
		}
		var value any
		if err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &value); err != nil {
			t.Fatalf("docs/interface.md block %q is not JSON: %v", *current, err)
		}
		if _, duplicate := blocks[*current]; duplicate {
			t.Fatalf("docs/interface.md validates %q twice", *current)
		}
		blocks[*current] = value
		current = nil
		lines = nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if current != nil {
			if strings.TrimSpace(line) == "```" {
				flush()
				continue
			}
			lines = append(lines, line)
			continue
		}
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "```json validate="); ok {
			name = strings.TrimSpace(name)
			if name == "" {
				t.Fatal("docs/interface.md has an empty validate name")
			}
			current = &name
			lines = nil
		}
	}
	if current != nil {
		t.Fatalf("docs/interface.md block %q never closes", *current)
	}
	if len(blocks) == 0 {
		t.Fatal("docs/interface.md has no validating examples")
	}
	return blocks
}

// TestInterfaceDocExamples pins the document to the fixtures: every
// validating example in docs/interface.md equals its testdata/api file,
// and every example file appears in the document.
func TestInterfaceDocExamples(t *testing.T) {
	dir := apiDir(t)
	blocks := docExamples(t)
	names := exampleFiles(t, dir)
	if len(blocks) != len(names) {
		t.Fatalf("document validates %d examples, testdata/api holds %d", len(blocks), len(names))
	}
	validator := &schemaValidator{dir: dir, cache: map[string]any{}}
	for _, name := range names {
		base := strings.TrimSuffix(name, ".json")
		block, ok := blocks[base]
		if !ok {
			t.Errorf("docs/interface.md never validates %s", name)
			continue
		}
		filed := loadJSON(t, filepath.Join(dir, name))
		if !reflect.DeepEqual(block, filed) {
			t.Errorf("docs/interface.md block %q differs from testdata/api/%s", base, name)
		}
		schemaName, ok := schemaForExample(name)
		if !ok {
			t.Errorf("%s: cannot map example to schema", name)
			continue
		}
		schema := loadJSON(t, filepath.Join(dir, schemaName))
		if failures := validator.validate(schema, block, "$"); len(failures) > 0 {
			t.Errorf("docs/interface.md block %q fails %s:\n  %s", base, schemaName, strings.Join(failures, "\n  "))
		}
	}
}

// TestRecallCarriesBothSides pins the hard requirement: recall returns
// the underlying standing and the warning together, and the overriding
// correction ships both at the top level and inside the belief.
func TestRecallCarriesBothSides(t *testing.T) {
	dir := apiDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "recall_for_purchase.response.example.json"))
	if err != nil {
		t.Fatalf("read recall example: %v", err)
	}
	var response RecallResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("parse recall example: %v", err)
	}
	if len(response.Beliefs) == 0 {
		t.Fatal("recall example carries no beliefs")
	}
	belief := response.Beliefs[0]
	if belief.Status != "Out of true" {
		t.Errorf("belief status = %q, want Out of true", belief.Status)
	}
	if belief.Standing != "Venerable" {
		t.Errorf("belief standing = %q, want Venerable", belief.Standing)
	}
	if !belief.Contradicted || belief.ContradictedBy == nil {
		t.Fatal("contradicted belief carries no inline warning")
	}
	if response.OverridingCorrection == nil {
		t.Fatal("recall carries no top-level overriding correction")
	}
	if response.OverridingCorrection.CorrectionID != belief.ContradictedBy.CorrectionID {
		t.Errorf("top-level correction %q differs from inline %q",
			response.OverridingCorrection.CorrectionID, belief.ContradictedBy.CorrectionID)
	}
	if response.Guidance.Basis != GuidanceCorrection {
		t.Errorf("guidance basis = %q, want correction", response.Guidance.Basis)
	}
	if response.Guidance.CorrectionID == nil || *response.Guidance.CorrectionID != response.OverridingCorrection.CorrectionID {
		t.Error("guidance does not point at the overriding correction")
	}
}

// TestCorrectionLeavesBeliefStanding pins the shopper-wins rule: the
// correction applies at once and the belief keeps its status and
// standing.
func TestCorrectionLeavesBeliefStanding(t *testing.T) {
	dir := apiDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "record_correction.response.example.json"))
	if err != nil {
		t.Fatalf("read correction example: %v", err)
	}
	var response RecordCorrectionResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("parse correction example: %v", err)
	}
	if !response.Contradicted {
		t.Error("correction response does not mark the belief contradicted")
	}
	if response.Status != "Out of true" {
		t.Errorf("belief status = %q, want Out of true beside the kept standing", response.Status)
	}
	if response.Standing != "Venerable" {
		t.Errorf("belief standing = %q, want the kept Venerable standing", response.Standing)
	}
	if response.Guidance.Basis != GuidanceCorrection {
		t.Errorf("guidance basis = %q, want correction", response.Guidance.Basis)
	}
}
