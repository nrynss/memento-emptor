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

	"github.com/nrynss/memento-emptor/internal/domain"
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
	case "product_fit.request":
		return &ProductFitRequest{}, true
	case "product_fit.response":
		return &ProductFitResponse{}, true
	case "set_pooling.request":
		return &SetPoolingRequest{}, true
	case "set_pooling.response":
		return &SetPoolingResponse{}, true
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

// TestToolsFrozen pins the surface: eleven tools, exact names, and a schema
// file behind every descriptor.
func TestToolsFrozen(t *testing.T) {
	if len(ToolNames) != 11 {
		t.Fatalf("frozen surface has %d tools, want 11", len(ToolNames))
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

// docExample is one validating example from docs/interface.md: the raw
// block text and its parsed JSON. The raw text is what the byte pin
// compares; the parsed form is what the schema check validates.
type docExample struct {
	raw   string
	value any
}

// docExamples extracts the validating examples from docs/interface.md.
// A validating example is a fenced block opened with
// ```json validate=<name>, where <name> is the testdata/api file without
// its .json suffix.
func docExamples(t *testing.T) map[string]docExample {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "interface.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read docs/interface.md: %v", err)
	}
	blocks := map[string]docExample{}
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
		blocks[*current] = docExample{raw: strings.Join(lines, "\n"), value: value}
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
// validating example in docs/interface.md is byte-identical to its
// testdata/api file, and every example file appears in the document. The
// comparison is on the raw block text, not the parsed JSON, so a
// reformatting of either side fails the build. Only the fence's trailing
// newline is forgiven: the file's single trailing newline has no block
// line to match it.
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
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		want := strings.TrimSuffix(string(raw), "\n")
		if block.raw != want {
			t.Errorf("docs/interface.md block %q differs byte-for-byte from testdata/api/%s", base, name)
		}
		filed := block.value
		schemaName, ok := schemaForExample(name)
		if !ok {
			t.Errorf("%s: cannot map example to schema", name)
			continue
		}
		schema := loadJSON(t, filepath.Join(dir, schemaName))
		if failures := validator.validate(schema, filed, "$"); len(failures) > 0 {
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

// readExample decodes one example file into a wire type.
func readExample(t *testing.T, name string, target any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(apiDir(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
}

// TestPrecedenceSteps pins the five steps and their order: a correction
// wins over own evidence, which wins over the pool, which wins over a
// statement, with nothing last.
func TestPrecedenceSteps(t *testing.T) {
	cases := []struct {
		basis GuidanceBasis
		value string
		rank  int
	}{
		{GuidanceCorrection, "correction", 1},
		{GuidanceOwnEvidence, "own_evidence", 2},
		{GuidancePool, "pool", 3},
		{GuidanceStatement, "statement", 4},
		{GuidanceNone, "none", 5},
		{GuidanceBasis("bogus"), "bogus", 0},
	}
	for _, tc := range cases {
		if string(tc.basis) != tc.value {
			t.Errorf("basis = %q, want %q", tc.basis, tc.value)
		}
		if got := tc.basis.Rank(); got != tc.rank {
			t.Errorf("GuidanceBasis(%q).Rank() = %d, want %d", tc.basis, got, tc.rank)
		}
	}
	if len(GuidanceBases) != 5 {
		t.Fatalf("GuidanceBases holds %d steps, want 5", len(GuidanceBases))
	}
	for i := 1; i < len(GuidanceBases); i++ {
		if !(GuidanceBases[i-1].Rank() < GuidanceBases[i].Rank()) {
			t.Errorf("GuidanceBases is not strongest first at %d: %q then %q",
				i, GuidanceBases[i-1], GuidanceBases[i])
		}
	}
	if GuidanceBelief != GuidanceOwnEvidence {
		t.Errorf("GuidanceBelief = %q, want the own-evidence step", GuidanceBelief)
	}
}

// poolCounts turns the wire counts into the domain counts the status rule
// reads. A direction's outcomes are its returns and its feedback together:
// the rule is about how the size ran, whichever way the shopper said so.
func poolCounts(c PoolCountsView) domain.PoolCounts {
	return domain.PoolCounts{
		TooSmall:         c.Returns.TooSmall + c.Feedback.TooSmall,
		TooLarge:         c.Returns.TooLarge + c.Feedback.TooLarge,
		FitsWell:         c.Feedback.FitsWell,
		ShoppersTooSmall: c.Shoppers.TooSmall,
		ShoppersTooLarge: c.Shoppers.TooLarge,
		Shoppers:         c.Shoppers.Fit,
	}
}

// countFields flattens the counts for the arithmetic checks.
func countFields(c PoolCountsView) map[string]int {
	return map[string]int{
		"settled":                      c.Settled,
		"kept":                         c.Kept,
		"returns.too_small":            c.Returns.TooSmall,
		"returns.too_large":            c.Returns.TooLarge,
		"feedback.fits_well":           c.Feedback.FitsWell,
		"feedback.too_small":           c.Feedback.TooSmall,
		"feedback.too_large":           c.Feedback.TooLarge,
		"reasons.fit_too_small":        c.Reasons.FitTooSmall,
		"reasons.fit_too_large":        c.Reasons.FitTooLarge,
		"reasons.damaged_or_defective": c.Reasons.DamagedOrDefective,
		"reasons.not_arrived":          c.Reasons.NotArrived,
		"reasons.changed_mind":         c.Reasons.ChangedMind,
		"shoppers.fit":                 c.Shoppers.Fit,
		"shoppers.too_small":           c.Shoppers.TooSmall,
		"shoppers.too_large":           c.Shoppers.TooLarge,
	}
}

// checkPoolCounts pins a counts block to its own arithmetic and to the
// status the rule decides from it. The denominator is every settled
// purchase; the return reasons add up to the returns; and the status reads
// Earned or Out of true from the counts it is shown with.
func checkPoolCounts(t *testing.T, name string, pool PoolView) {
	t.Helper()
	all := countFields(pool.Counts)
	seeded := countFields(pool.Seeded)
	live := countFields(pool.Live)
	recent := countFields(pool.Recent)
	for field, total := range all {
		if seeded[field]+live[field] != total {
			t.Errorf("%s: %s: seeded %d + live %d = %d, want %d",
				name, field, seeded[field], live[field], seeded[field]+live[field], total)
		}
		if recent[field] > total {
			t.Errorf("%s: %s: recent %d exceeds the total %d", name, field, recent[field], total)
		}
	}
	returns := pool.Counts.Returns.TooSmall + pool.Counts.Returns.TooLarge
	if returns != pool.Counts.Reasons.FitTooSmall+pool.Counts.Reasons.FitTooLarge {
		t.Errorf("%s: direction counts do not equal the fit reason counts", name)
	}
	if pool.Counts.Reasons.FitTooSmall != pool.Counts.Returns.TooSmall ||
		pool.Counts.Reasons.FitTooLarge != pool.Counts.Returns.TooLarge {
		t.Errorf("%s: fit reason counts differ from the direction counts", name)
	}
	settled := pool.Counts.Returns.TooSmall + pool.Counts.Returns.TooLarge +
		pool.Counts.Reasons.DamagedOrDefective + pool.Counts.Reasons.NotArrived +
		pool.Counts.Reasons.ChangedMind +
		pool.Counts.Feedback.FitsWell + pool.Counts.Feedback.TooSmall + pool.Counts.Feedback.TooLarge +
		pool.Counts.Kept
	if settled != pool.Counts.Settled {
		t.Errorf("%s: settled counts add to %d, want the denominator %d", name, settled, pool.Counts.Settled)
	}
	verdict := domain.DecidePool(poolCounts(pool.Counts), poolCounts(pool.Recent))
	if verdict.Status != pool.Status {
		t.Errorf("%s: status %q, want %q from the counts", name, pool.Status, verdict.Status)
	}
	if pool.Direction == nil {
		if verdict.Direction != domain.FitDirectionNone {
			t.Errorf("%s: direction is null, want %q from the counts", name, verdict.Direction)
		}
	} else if *pool.Direction != verdict.Direction {
		t.Errorf("%s: direction %q, want %q from the counts", name, *pool.Direction, verdict.Direction)
	}
}

// TestPoolFixturesMatchTheRule checks every pooled example against the
// domain rule and against its own arithmetic. The Earned, Claimed and
// Building product_fit examples each carry a status the counts earn, and
// the Claimed and Building counts are the seeded population's own rows
// for their lines, with no live rows yet.
func TestPoolFixturesMatchTheRule(t *testing.T) {
	for _, name := range []string{
		"product_fit.response.example.json",
		"product_fit.response.example-claimed.json",
		"product_fit.response.example-building.json",
	} {
		var fit ProductFitResponse
		readExample(t, name, &fit)
		checkPoolCounts(t, strings.TrimSuffix(name, ".json"), fit.Pool)
	}

	for _, name := range []string{
		"recall_for_purchase.response.example-correction.json",
		"recall_for_purchase.response.example-pool.json",
		"recall_for_purchase.response.example-own.json",
	} {
		var response RecallResponse
		readExample(t, name, &response)
		if response.Pool == nil {
			t.Fatalf("%s: no pool", name)
		}
		checkPoolCounts(t, name, *response.Pool)
	}
}

// TestRequiredFieldsSerialize pins the tightening: the pool, the pool
// lookup and every precedence step serialise even when absent, as null,
// so a missing key is a breach rather than a shape. The schemas require
// the same keys.
func TestRequiredFieldsSerialize(t *testing.T) {
	raw, err := json.Marshal(RecallResponse{Beliefs: []BeliefView{}, Guidance: GuidanceView{Basis: GuidanceNone}})
	if err != nil {
		t.Fatalf("marshal empty recall: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("parse empty recall: %v", err)
	}
	for _, key := range []string{"pool", "pool_lookup"} {
		if _, present := wire[key]; !present {
			t.Errorf("serialised recall omits %q", key)
		}
	}
	raw, err = json.Marshal(BeliefView{Evidence: []EvidenceView{}})
	if err != nil {
		t.Fatalf("marshal empty belief: %v", err)
	}
	wire = map[string]any{}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("parse empty belief: %v", err)
	}
	if _, present := wire["precedence"]; !present {
		t.Error("serialised belief omits precedence")
	}
	raw, err = json.Marshal(PoolView{})
	if err != nil {
		t.Fatalf("marshal empty pool: %v", err)
	}
	wire = map[string]any{}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("parse empty pool: %v", err)
	}
	if _, present := wire["precedence"]; !present {
		t.Error("serialised pool omits precedence")
	}
}

// TestProductFitCarriesNoShopperRows pins the pool's privacy: no shopper
// identity, no purchase row and no free text.
func TestProductFitCarriesNoShopperRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(apiDir(t), "product_fit.response.example.json"))
	if err != nil {
		t.Fatalf("read product_fit example: %v", err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse product_fit example: %v", err)
	}
	forbiddenKeys := map[string]bool{
		"shopper_id": true, "shopper_hash": true, "custom_id": true,
		"order_id": true, "capture_id": true, "refund_id": true,
		"dispute_id": true, "text": true, "reason_text": true,
	}
	var walk func(value any, path string)
	walk = func(value any, path string) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbiddenKeys[key] {
					t.Errorf("product_fit carries %s at %s", key, path)
				}
				walk(child, path+"."+key)
			}
		case []any:
			for i, child := range typed {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		case string:
			if strings.Contains(strings.ToLower(typed), "shopper") {
				t.Errorf("product_fit carries a shopper reference at %s: %q", path, typed)
			}
		}
	}
	walk(doc, "$")
}

// TestRecallMarksOverriddenSteps pins the disagreement rule: both sides
// come back, the winning step decides, and the lower one is marked
// overridden rather than hidden or dropped.
func TestRecallMarksOverriddenSteps(t *testing.T) {
	t.Run("the correction overrides both lower steps", func(t *testing.T) {
		var response RecallResponse
		readExample(t, "recall_for_purchase.response.example-correction.json", &response)
		if response.Guidance.Basis != GuidanceCorrection || response.OverridingCorrection == nil {
			t.Fatal("the correction does not decide")
		}
		if len(response.Beliefs) != 1 {
			t.Fatalf("beliefs = %d, want the contradicted belief kept", len(response.Beliefs))
		}
		precedence := response.Beliefs[0].Precedence
		if precedence == nil || !precedence.Overridden || precedence.OverriddenBy == nil || *precedence.OverriddenBy != GuidanceCorrection {
			t.Errorf("belief precedence = %+v, want own evidence overridden by the correction", precedence)
		}
		if response.Pool == nil || response.Pool.Precedence == nil || !response.Pool.Precedence.Overridden {
			t.Fatal("the pool is not marked overridden")
		}
		if got := *response.Pool.Precedence.OverriddenBy; got != GuidanceCorrection {
			t.Errorf("pool overridden by %q, want correction", got)
		}
		if response.PoolLookup != PoolConsulted {
			t.Errorf("pool lookup = %q, want consulted", response.PoolLookup)
		}
	})
	t.Run("the pool overrides the statement", func(t *testing.T) {
		var response RecallResponse
		readExample(t, "recall_for_purchase.response.example-pool.json", &response)
		if response.Guidance.Basis != GuidancePool {
			t.Fatalf("guidance basis = %q, want pool", response.Guidance.Basis)
		}
		if len(response.Beliefs) != 1 || response.Beliefs[0].Precedence == nil {
			t.Fatal("the statement belief is missing its precedence")
		}
		precedence := response.Beliefs[0].Precedence
		if !precedence.Overridden || precedence.OverriddenBy == nil || *precedence.OverriddenBy != GuidancePool {
			t.Errorf("statement precedence = %+v, want overridden by the pool", precedence)
		}
		if response.Pool == nil || response.Pool.Precedence == nil || response.Pool.Precedence.Overridden {
			t.Fatal("the pool is not the deciding step")
		}
		if response.Pool.Status != domain.StatusEarned {
			t.Errorf("pool status = %q, want Earned", response.Pool.Status)
		}
		if response.Guidance.BeliefID != nil {
			t.Error("pool guidance points at a belief")
		}
	})
	t.Run("own evidence overrides the pool", func(t *testing.T) {
		var response RecallResponse
		readExample(t, "recall_for_purchase.response.example-own.json", &response)
		if response.Guidance.Basis != GuidanceOwnEvidence {
			t.Fatalf("guidance basis = %q, want own_evidence", response.Guidance.Basis)
		}
		if len(response.Beliefs) != 1 || response.Beliefs[0].Precedence == nil {
			t.Fatal("the own belief is missing its precedence")
		}
		precedence := response.Beliefs[0].Precedence
		if precedence.Step != GuidanceOwnEvidence || precedence.Overridden {
			t.Errorf("own belief precedence = %+v, want the deciding own-evidence step", precedence)
		}
		if response.Beliefs[0].Status != domain.StatusBuilding {
			t.Errorf("own belief status = %q, want Building: a label never gates the step", response.Beliefs[0].Status)
		}
		if response.Pool == nil || response.Pool.Precedence == nil || !response.Pool.Precedence.Overridden {
			t.Fatal("the pool is not marked overridden")
		}
		if got := *response.Pool.Precedence.OverriddenBy; got != GuidanceOwnEvidence {
			t.Errorf("pool overridden by %q, want own_evidence", got)
		}
	})
}

// TestRecallNeverRefuses pins the never-refuses rule: with nothing to go
// on, recall answers with an empty list and says so, rather than failing.
func TestRecallNeverRefuses(t *testing.T) {
	var response RecallResponse
	readExample(t, "recall_for_purchase.response.example-none.json", &response)
	if len(response.Beliefs) != 0 {
		t.Errorf("beliefs = %d, want none", len(response.Beliefs))
	}
	if response.Pool != nil {
		t.Error("a thin pool is returned as a belief")
	}
	if response.PoolLookup != PoolConsulted {
		t.Errorf("pool lookup = %q, want consulted with an empty pool", response.PoolLookup)
	}
	if response.Guidance.Basis != GuidanceNone {
		t.Errorf("guidance basis = %q, want none", response.Guidance.Basis)
	}
	if response.Guidance.Text == "" {
		t.Error("recall refuses to say anything")
	}
}

// TestCorrectionStandsWithoutBelief pins the standing record: a
// correction with no belief carries no against declaration, contradicts
// nothing, and still decides the recommendation.
func TestCorrectionStandsWithoutBelief(t *testing.T) {
	var request RecordCorrectionRequest
	readExample(t, "record_correction.request.example-standing.json", &request)
	if request.ContradictsBeliefID != "" {
		t.Errorf("standing request contradicts %q, want no belief", request.ContradictsBeliefID)
	}
	var response RecordCorrectionResponse
	readExample(t, "record_correction.response.example-standing.json", &response)
	if response.Correction.ContradictsBeliefID != "" {
		t.Errorf("standing correction declares against %q", response.Correction.ContradictsBeliefID)
	}
	if response.BeliefID != "" {
		t.Errorf("standing correction reports belief %q", response.BeliefID)
	}
	if response.Contradicted {
		t.Error("standing correction marks a belief contradicted")
	}
	if response.Status != domain.StatusClaimed || response.Standing != domain.GraphNone {
		t.Errorf("standing correction reads %q/%q, want Claimed/None", response.Status, response.Standing)
	}
	if response.Guidance.Basis != GuidanceCorrection || response.Guidance.BeliefID != nil {
		t.Errorf("standing correction guidance = %+v, want the correction alone", response.Guidance)
	}
}

// TestSetPoolingExamples pins the consent response: opting out removes the
// rows already written, and opting back in removes none.
func TestSetPoolingExamples(t *testing.T) {
	var request SetPoolingRequest
	readExample(t, "set_pooling.request.example.json", &request)
	if request.Pooling {
		t.Error("the opt-out example does not opt out")
	}
	var response SetPoolingResponse
	readExample(t, "set_pooling.response.example.json", &response)
	if response.Pooling || response.RowsRemoved != 3 {
		t.Errorf("opt-out response = %+v, want pooling false and 3 rows removed", response)
	}
	var optIn SetPoolingResponse
	readExample(t, "set_pooling.response.example-optin.json", &optIn)
	if !optIn.Pooling || optIn.RowsRemoved != 0 {
		t.Errorf("opt-in response = %+v, want pooling true and no rows removed", optIn)
	}
}

// TestFitVerdictVocabularyMatchesDomain pins the pooled row's verdicts to
// the wire's fit verdicts, so the two vocabularies cannot drift.
func TestFitVerdictVocabularyMatchesDomain(t *testing.T) {
	cases := []struct {
		wire FeedbackVerdict
		row  domain.FitVerdict
	}{
		{FeedbackFitsWell, domain.FitVerdictFitsWell},
		{FeedbackTooSmall, domain.FitVerdictTooSmall},
		{FeedbackTooLarge, domain.FitVerdictTooLarge},
	}
	for _, tc := range cases {
		if string(tc.wire) != string(tc.row) {
			t.Errorf("wire verdict %q differs from the fit row's %q", tc.wire, tc.row)
		}
	}
}
