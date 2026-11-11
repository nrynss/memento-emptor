// Command pincheck reads every toolchain pin the repository states for Go and
// Node and fails when two of them disagree. It is the gate's mechanical guard
// for the drift T9.3g and T9.3h fixed by hand: an image built on a Node major
// CI does not use builds green in both places and ships the wrong one.
//
// The sources are the two images' build stages (`deploy/Dockerfile` and
// `deploy/merchant/Dockerfile`), CI's `setup-go` and `setup-node`,
// `go.mod`'s Go directive, and the pins `deploy/README.md`'s table states.
// Each file must state its pin: a source that has lost one is a failure, not a
// shorter list, so the check cannot pass by reading less.
//
// The comparison granularity is the granularity each file carries. Node is
// compared by major, because an image tag names a major and nothing in the
// tree pins a Node patch. Go is compared by major.minor for the same reason --
// the image tag floats the patch -- and the files that do name a patch,
// `go.mod` and CI, must also agree with each other exactly.
//
// It reads files and prints; it has no network, no key and no dependency
// outside the standard library. The gate runs it as
//
//	go run ./tools/pincheck
//
// from the repository root.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// kind is the toolchain a pin speaks for.
type kind string

const (
	kindGo   kind = "go"
	kindNode kind = "node"
)

// pin is one version one file states for one toolchain.
type pin struct {
	kind    kind
	version string // the tag's numeric part, or the value the file writes
	source  string // the file and, inside it, what states the pin
}

// check reads the pins under root and returns them in reading order, or an
// error naming every pair that disagrees.
func check(root string) ([]pin, error) {
	var pins []pin
	for _, rel := range []string{"deploy/Dockerfile", "deploy/merchant/Dockerfile"} {
		got, err := dockerPins(root, rel)
		if err != nil {
			return nil, err
		}
		pins = append(pins, got...)
	}
	for _, reader := range []func(string) ([]pin, error){ciPins, goModPins, readmePins} {
		got, err := reader(root)
		if err != nil {
			return nil, err
		}
		pins = append(pins, got...)
	}
	return pins, disagreements(pins)
}

// dockerPins reads the base image of every `FROM` line in one Dockerfile that
// names Go or Node. Each image in the deploy tree builds a Go service and a
// frontend, so a file missing either stage is a failure: a stage deleted by
// accident must not drop its pin silently.
func dockerPins(root, rel string) ([]pin, error) {
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var pins []pin
	for i, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "FROM") {
			continue
		}
		for _, field := range fields[1:] {
			k, image, ok := imageKind(field)
			if !ok {
				continue
			}
			version, err := imageVersion(image)
			if err != nil {
				return nil, fmt.Errorf("%s line %d: %w", rel, i+1, err)
			}
			source := fmt.Sprintf("%s's %s build stage (line %d)", rel, k, i+1)
			if err := requireParts(version, minParts(k), source); err != nil {
				return nil, err
			}
			pins = append(pins, pin{kind: k, version: version, source: source})
		}
	}
	for _, k := range []kind{kindGo, kindNode} {
		found := false
		for _, p := range pins {
			found = found || p.kind == k
		}
		if !found {
			return nil, fmt.Errorf("%s: no base image for %s (want %s:)", rel, k, imageName(k))
		}
	}
	return pins, nil
}

// imageKind reports whether an image reference is the Go or the Node base
// image, and returns the toolchain and the reference's tag.
func imageKind(field string) (kind, string, bool) {
	for _, image := range []struct {
		k    kind
		name string
	}{
		{kindGo, "golang"},
		{kindNode, "node"},
	} {
		if rest, ok := strings.CutPrefix(field, image.name+":"); ok {
			return image.k, rest, true
		}
	}
	return "", "", false
}

// imageVersion reads the version out of an image tag, dropping the distribution
// suffix and any digest. An unpinned or floating reference is an error: it
// names no version to compare.
func imageVersion(tag string) (string, error) {
	if i := strings.Index(tag, "@"); i >= 0 {
		tag = tag[:i]
	}
	if i := strings.Index(tag, "-"); i >= 0 {
		tag = tag[:i]
	}
	if tag == "" {
		return "", errors.New("image tag names no version")
	}
	return tag, nil
}

// ciPins reads the versions CI installs, one `go-version:` and one
// `node-version:`. Two of either would leave the check comparing against
// whichever came first, so a duplicate is an error.
func ciPins(root string) ([]pin, error) {
	const rel = ".github/workflows/ci.yml"
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	kinds := []struct {
		k     kind
		key   string
		step  string
		found bool
	}{
		{kindGo, "go-version:", "setup-go", false},
		{kindNode, "node-version:", "setup-node", false},
	}
	var pins []pin
	for i, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		for j := range kinds {
			value, ok := strings.CutPrefix(trimmed, kinds[j].key)
			if !ok {
				continue
			}
			source := fmt.Sprintf("%s's %s (line %d)", rel, kinds[j].step, i+1)
			if kinds[j].found {
				return nil, fmt.Errorf("%s: more than one %s line", source, kinds[j].key)
			}
			kinds[j].found = true
			value = strings.Trim(strings.TrimSpace(value), `'"`)
			if err := requireParts(value, minParts(kinds[j].k), source); err != nil {
				return nil, err
			}
			pins = append(pins, pin{kind: kinds[j].k, version: value, source: source})
		}
	}
	for j := range kinds {
		if !kinds[j].found {
			return nil, fmt.Errorf("%s: no %s line, so CI states no %s version", rel, kinds[j].key, kinds[j].k)
		}
	}
	return pins, nil
}

// goModPins reads the Go directive that sets the language version the module
// requires.
func goModPins(root string) ([]pin, error) {
	const rel = "go.mod"
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	for i, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "go" {
			continue
		}
		source := fmt.Sprintf("%s's go directive (line %d)", rel, i+1)
		if err := requireParts(fields[1], minParts(kindGo), source); err != nil {
			return nil, err
		}
		return []pin{{kind: kindGo, version: fields[1], source: source}}, nil
	}
	return nil, fmt.Errorf("%s: no go directive, so the module states no Go version", rel)
}

// readmePins reads the Go and Node rows of the toolchain table in
// deploy/README.md, the table that records the pins for a reader.
func readmePins(root string) ([]pin, error) {
	const rel = "deploy/README.md"
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var pins []pin
	for i, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) < 3 {
			continue
		}
		k := kind(strings.ToLower(strings.TrimSpace(cells[1])))
		if k != kindGo && k != kindNode {
			continue
		}
		source := fmt.Sprintf("%s's toolchain table %s row (line %d)", rel, k, i+1)
		version := strings.Trim(strings.TrimSpace(cells[2]), "`")
		if err := requireParts(version, minParts(k), source); err != nil {
			return nil, err
		}
		for _, p := range pins {
			if p.kind == k {
				return nil, fmt.Errorf("%s: more than one %s row", source, k)
			}
		}
		pins = append(pins, pin{kind: k, version: version, source: source})
	}
	for _, k := range []kind{kindGo, kindNode} {
		found := false
		for _, p := range pins {
			found = found || p.kind == k
		}
		if !found {
			return nil, fmt.Errorf("%s: no %s row in the toolchain table, so the table states no %s pin", rel, k, k)
		}
	}
	return pins, nil
}

// disagreements compares the pins of each toolchain and returns an error
// listing every pin that is out of step, naming the file that writes it and
// both versions. The expected value is the one most of the files state, so the
// message blames the file that drifted rather than the first one read; when no
// value has a majority, every pin of that toolchain is named. Go pins that
// state a patch must also agree exactly, since the image tag and the table
// float where go.mod and CI fix.
func disagreements(pins []pin) error {
	var lines []string
	for _, k := range []kind{kindGo, kindNode} {
		var group []pin
		for _, p := range pins {
			if p.kind == k {
				group = append(group, p)
			}
		}
		if len(group) == 0 {
			return fmt.Errorf("no %s pin was read, so there is nothing to compare", k)
		}

		var keys []string
		buckets := map[string][]pin{}
		for _, p := range group {
			key := comparisonKey(p.version, minParts(k))
			if _, seen := buckets[key]; !seen {
				keys = append(keys, key)
			}
			buckets[key] = append(buckets[key], p)
		}
		expected := keys[0]
		for _, key := range keys {
			if len(buckets[key]) > len(buckets[expected]) {
				expected = key
			}
		}
		if hasTie(buckets, keys, expected) {
			var named []string
			for _, p := range group {
				named = append(named, fmt.Sprintf("%s says %s", p.source, p.version))
			}
			lines = append(lines, fmt.Sprintf("%s: no single pin is in the majority: %s", k, strings.Join(named, "; ")))
			continue
		}

		ref := buckets[expected][0]
		for _, p := range group {
			if comparisonKey(p.version, minParts(k)) == expected {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s says %s, but %s says %s",
				k, p.source, p.version, ref.source, ref.version))
		}

		var patched []pin
		for _, p := range buckets[expected] {
			if parts(p.version) >= 3 {
				patched = append(patched, p)
			}
		}
		if len(patched) < 2 {
			continue
		}
		for _, p := range patched[1:] {
			if p.version != patched[0].version {
				lines = append(lines, fmt.Sprintf("%s: %s says %s, but %s says %s",
					k, p.source, p.version, patched[0].source, patched[0].version))
			}
		}
	}
	if len(lines) > 0 {
		return fmt.Errorf("toolchain pins disagree:\n  %s", strings.Join(lines, "\n  "))
	}
	return nil
}

// hasTie reports whether another value is stated as often as the expected one,
// which leaves no majority to call the expected value.
func hasTie(buckets map[string][]pin, keys []string, expected string) bool {
	for _, key := range keys {
		if key != expected && len(buckets[key]) == len(buckets[expected]) {
			return true
		}
	}
	return false
}

// imageName is the repository an image of a toolchain comes from, as the
// Dockerfile writes it.
func imageName(k kind) string {
	if k == kindGo {
		return "golang"
	}
	return "node"
}

// minParts is how many version components a toolchain's pin must carry: Go a
// major and a minor, Node a major.
func minParts(k kind) int {
	if k == kindGo {
		return 2
	}
	return 1
}

// comparisonKey is the part of a version two files are held to: major.minor
// for Go, the major for Node.
func comparisonKey(version string, n int) string {
	split := strings.Split(version, ".")
	if len(split) > n {
		split = split[:n]
	}
	return strings.Join(split, ".")
}

// parts counts the numeric components of a version that already passed
// requireParts.
func parts(version string) int {
	return len(strings.Split(version, "."))
}

// requireParts fails unless a version is made of at least min numeric
// components, so a tag that names no comparable version cannot slip through as
// an empty key.
func requireParts(version string, min int, source string) error {
	split := strings.Split(version, ".")
	if len(split) < min {
		return fmt.Errorf("%s: %q names no %d-part version to compare", source, version, min)
	}
	for _, part := range split {
		if _, err := strconv.Atoi(part); err != nil {
			return fmt.Errorf("%s: %q is not a numeric version", source, version)
		}
	}
	return nil
}

func main() {
	root := flag.String("root", ".", "repository root to read the pins from")
	flag.Parse()

	pins, err := check(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	agree := map[kind]string{}
	for _, p := range pins {
		agree[p.kind] = comparisonKey(p.version, minParts(p.kind))
	}
	fmt.Printf("go %s and node %s agree across %d declarations\n", agree[kindGo], agree[kindNode], len(pins))
}
