// The live-recall comparison: every Memento turn's served recall_for_purchase
// response is compared with bench/round2/recall.go's frozen expectation for
// the same shopper and phase. A difference is reported, never repaired.
//
// Ids are excluded on purpose: the fixture mints deterministic belief,
// evidence and correction ids and a live layer mints its own, so no run may
// compare ids (FROZEN.md section 10). The guidance prose is excluded too:
// the prompt never carries it (ProjectRecall drops it), so the comparison
// keeps only its basis and which overrides it names. Everything else compares
// field for field: belief values, statuses, standings, evidence rows,
// their reasons and counts, the pool block and its counts, the overriding
// correction, the pending count and the pool lookup.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/nrynss/memento-emptor/bench/round2"
)

// recallDiff is one turn's comparison.
type recallDiff struct {
	RunID       string   `json:"run_id"`
	ShopperID   string   `json:"shopper_id"`
	Arm         int      `json:"arm"`
	Turn        string   `json:"turn"`
	Phase       string   `json:"phase"`
	Differences []string `json:"differences,omitempty"`
}

// recallReport is the committed comparison file.
type recallReport struct {
	// Note states what is compared and what is excluded.
	Note string `json:"note"`
	// Runs carries one entry per Memento turn.
	Runs []recallDiff `json:"runs"`
}

// compareRecall compares every committed Memento turn against the frozen
// expectation for its phase.
func (d *driver) compareRecall() (*recallReport, error) {
	populations := d.population
	paths, err := d.runFilePaths()
	if err != nil {
		return nil, err
	}
	report := &recallReport{
		Note: "Live recall against bench/round2/recall.go's expectation, per shopper and phase. " +
			"Ids, the absolute event times and the guidance prose are excluded; every other field compares. " +
			"The expectation dates its rows from the freeze's fixed anchor, while the live layer is seeded with the same relative spans in real past event time " +
			"(a future event time never ages, so promotion would never happen); only the order of the times is part of the freeze. " +
			"A difference is reported, not repaired.",
	}
	for _, path := range paths {
		record, ok, err := readRunFile(path)
		if err != nil {
			return nil, err
		}
		if !ok || (record.Arm != int(round2.ArmMementoShopperOnly) && record.Arm != int(round2.ArmMementoFull)) {
			continue
		}
		shopper, ok := d.shoppers[record.ShopperID]
		if !ok {
			continue
		}
		phase := round2.TurnPhase(record.Turn)
		expected, err := round2.ExpectedRecall(shopper, populations, phase)
		if err != nil {
			return nil, fmt.Errorf("t104: expected recall for %s: %w", record.ShopperID, err)
		}
		differences, err := diffRecall(expected, []byte(record.MemoryResponse))
		if err != nil {
			return nil, fmt.Errorf("t104: compare recall for %s: %w", record.RunID, err)
		}
		report.Runs = append(report.Runs, recallDiff{
			RunID:       record.RunID,
			ShopperID:   record.ShopperID,
			Arm:         record.Arm,
			Turn:        string(record.Turn),
			Phase:       string(phase),
			Differences: differences,
		})
	}
	return report, nil
}

// diffRecall normalizes both responses and returns the differences.
func diffRecall(expected, live []byte) ([]string, error) {
	want, err := normalizedRecall(expected)
	if err != nil {
		return nil, fmt.Errorf("expected: %w", err)
	}
	got, err := normalizedRecall(live)
	if err != nil {
		return nil, fmt.Errorf("live: %w", err)
	}
	var out []string
	diffValues("", want, got, &out)
	const limit = 30
	if len(out) > limit {
		extra := fmt.Sprintf("… and %d more differences", len(out)-limit)
		out = append(out[:limit:limit], extra)
	}
	return out, nil
}

// normalizedRecall decodes one recall response and canonicalizes it for
// comparison: ids to a placeholder, guidance to its basis and override
// presence, belief and evidence arrays sorted so a presentation order is not
// a difference.
func normalizedRecall(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return canonical(value, ""), nil
}

// idKeys are the fields whose values are minted per layer and never compare.
var idKeys = map[string]bool{
	"belief_id":             true,
	"evidence_id":           true,
	"correction_id":         true,
	"contradicts_belief_id": true,
	"trace_id":              true,
}

// timeKeys are the fields whose values are event times. The frozen
// expectation dates them from the fixture's fixed anchor; the live layer is
// seeded in real event time (a future event time never ages, so promotion
// would never happen). Only the order of the times is part of the freeze, so
// the comparison excludes the absolute values and the run note says so.
var timeKeys = map[string]bool{
	"updated_at":  true,
	"occurred_at": true,
	"recorded_at": true,
}

// canonical walks one decoded response.
func canonical(node any, field string) any {
	switch value := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			switch {
			case idKeys[key]:
				if child == nil {
					out[key] = nil
				} else {
					out[key] = "<id>"
				}
			case timeKeys[key]:
				out[key] = "<time>"
			case key == "guidance":
				guide, _ := child.(map[string]any)
				out[key] = map[string]any{
					"basis":              guide["basis"],
					"correction_present": guide["correction_id"] != nil,
					"belief_present":     guide["belief_id"] != nil,
				}
			default:
				out[key] = canonical(child, key)
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(value))
		for _, child := range value {
			out = append(out, canonical(child, field))
		}
		// Sort by the identity a reader uses, never by the whole value: a
		// status is what the comparison reports, so sorting on it would
		// pair two different beliefs and hide the real difference.
		sort.SliceStable(out, func(i, j int) bool {
			return arraySortKey(field, out[i]) < arraySortKey(field, out[j])
		})
		return out
	default:
		return value
	}
}

// arraySortKey is the stable identity of one array element.
func arraySortKey(field string, value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return canonicalKey(value)
	}
	switch field {
	case "beliefs":
		return fmt.Sprint(object["kind"], "|", object["value"], "|", subjectKey(object["subject"]))
	case "evidence":
		item, _ := object["item"].(map[string]any)
		outcome, _ := object["outcome"].(map[string]any)
		reason, _ := object["reason"].(map[string]any)
		return fmt.Sprint(item["sku"], "|", item["size"], "|", outcome["kind"], "|", reason["category"], "|", object["counts_toward"])
	default:
		return canonicalKey(value)
	}
}

// subjectKey renders one subject for a sort key.
func subjectKey(value any) string {
	subject, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprint(subject["kind"], "/", subject["id"])
}

// canonicalKey renders one canonical value as a stable sort key.
func canonicalKey(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

// diffValues compares two canonical values and records the differences.
func diffValues(path string, want, got any, out *[]string) {
	switch wantValue := want.(type) {
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: type %T in the expectation, %T live", pathOrRoot(path), want, got))
			return
		}
		keys := map[string]bool{}
		for key := range wantValue {
			keys[key] = true
		}
		for key := range gotValue {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			wantChild, inWant := wantValue[key]
			gotChild, inGot := gotValue[key]
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			switch {
			case !inGot:
				if isEmptyValue(wantChild) {
					continue
				}
				*out = append(*out, fmt.Sprintf("%s: missing live, expected %s", childPath, render(wantChild)))
			case !inWant:
				if isEmptyValue(gotChild) {
					continue
				}
				*out = append(*out, fmt.Sprintf("%s: unexpected live value %s", childPath, render(gotChild)))
			default:
				diffValues(childPath, wantChild, gotChild, out)
			}
		}
	case []any:
		gotValue, ok := got.([]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: type %T in the expectation, %T live", pathOrRoot(path), want, got))
			return
		}
		if len(wantValue) != len(gotValue) {
			*out = append(*out, fmt.Sprintf("%s: %d in the expectation, %d live", pathOrRoot(path), len(wantValue), len(gotValue)))
		}
		for i := 0; i < len(wantValue) && i < len(gotValue); i++ {
			diffValues(fmt.Sprintf("%s[%d]", path, i), wantValue[i], gotValue[i], out)
		}
	default:
		if canonicalKey(want) != canonicalKey(got) {
			*out = append(*out, fmt.Sprintf("%s: expected %s, live %s", pathOrRoot(path), render(want), render(got)))
		}
	}
}

// isEmptyValue reports an absent field and an omitted empty value as the
// same reading: the wire contract omits empty optional ids while the fixture
// spells them as empty strings.
func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && text == ""
}

// pathOrRoot names the comparison root.
func pathOrRoot(path string) string {
	if path == "" {
		return "response"
	}
	return strings.TrimPrefix(path, ".")
}

// render prints one value compactly for a difference line.
func render(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}
