package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is the tree check reads, written with the pins this repository
// states today, so each case changes only the one line it is about.
func fixture() map[string]string {
	return map[string]string{
		"deploy/Dockerfile": `# the demo image
FROM golang:1.27-bookworm AS go-build
RUN CGO_ENABLED=0 go build -o /out/memento ./cmd/memento

FROM node:26-bookworm AS web-build
RUN npm ci && npm run build

FROM debian:trixie-slim
COPY --from=go-build /out/memento /app/bin/
`,
		"deploy/merchant/Dockerfile": `FROM golang:1.27-bookworm AS go-build
RUN CGO_ENABLED=0 go build -o /out/merchant-edge ./deploy/merchant

FROM node:26-bookworm AS web-build
RUN npm ci && npm run build
`,
		"agent/frontend/Dockerfile": `FROM oven/bun:1
WORKDIR /app
CMD ["bun", "run", "server/index.ts"]
`,
		"agent/runtime/Dockerfile": `FROM oven/bun:1 AS builder
RUN bun install

FROM oven/bun:1-slim
CMD ["bun", "run", "agent/index.ts"]
`,
		".github/workflows/ci.yml": `steps:
  - uses: actions/setup-go@v5
    with:
      go-version: '1.27.1'

  - uses: actions/setup-node@v4
    with:
      node-version: '26'

  - uses: oven-sh/setup-bun@v2
    with:
      bun-version: '1.3.14'
`,
		"go.mod": `module example.com/x

go 1.27.1
`,
		"deploy/README.md": `## Toolchain pins

| Tool | Pin | Where the images pin it |
|---|---|---|
| Go | 1.27 | ` + "`golang:1.27-bookworm`" + ` |
| Node | 26 | ` + "`node:26-bookworm`" + ` |
| Bun | 1 | ` + "`oven/bun:1`" + ` |
`,
	}
}

// replace rewrites one string in one fixture file, for a case that drifts a
// single pin.
func replace(t *testing.T, files map[string]string, path, old, new string) {
	t.Helper()
	body, ok := files[path]
	if !ok {
		t.Fatalf("fixture has no %s", path)
	}
	if !strings.Contains(body, old) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	files[path] = strings.Replace(body, old, new, 1)
}

// writeTree writes a fixture under a fresh temporary directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("make %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// TestCheckTable proves each failure mode and the pass: a Node, Go or bun pin
// that drifts from the rest, a lost source, and the current pins agreeing.
func TestCheckTable(t *testing.T) {
	cases := []struct {
		name string
		edit func(*testing.T, map[string]string)
		// want is every substring the error must carry: the file that drifted
		// and the version it wrote. An empty want means the pins must agree.
		want []string
	}{
		{
			name: "current pins agree",
			edit: func(*testing.T, map[string]string) {},
		},
		{
			name: "node drifts in the demo image",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/Dockerfile", "node:26-bookworm", "node:22-bookworm")
			},
			want: []string{"node: deploy/Dockerfile", "says 22"},
		},
		{
			name: "node drifts in the merchant image",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/merchant/Dockerfile", "node:26-bookworm", "node:22-bookworm")
			},
			want: []string{"node: deploy/merchant/Dockerfile", "says 22"},
		},
		{
			name: "node drifts in ci",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, ".github/workflows/ci.yml", "node-version: '26'", "node-version: '22'")
			},
			want: []string{"node: .github/workflows/ci.yml", "says 22"},
		},
		{
			name: "node drifts in the readme table",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/README.md", "| Node | 26 |", "| Node | 24 |")
			},
			want: []string{"node: deploy/README.md", "says 24"},
		},
		{
			name: "go drifts in the merchant image",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/merchant/Dockerfile", "golang:1.27-bookworm", "golang:1.26-bookworm")
			},
			want: []string{"go: deploy/merchant/Dockerfile", "says 1.26"},
		},
		{
			name: "go patch drifts between go.mod and ci",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "go.mod", "go 1.27.1", "go 1.27.0")
			},
			want: []string{"go: go.mod", "says 1.27.0", "says 1.27.1"},
		},
		{
			name: "go drifts in the readme table",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/README.md", "| Go | 1.27 |", "| Go | 1.26 |")
			},
			want: []string{"go: deploy/README.md", "says 1.26"},
		},
		{
			name: "go.mod loses its go directive",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "go.mod", "\ngo 1.27.1\n", "\n")
			},
			want: []string{"go.mod: no go directive"},
		},
		{
			name: "bun drifts in the shopper image",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "agent/frontend/Dockerfile", "oven/bun:1", "oven/bun:2")
			},
			want: []string{"bun: agent/frontend/Dockerfile", "says 2"},
		},
		{
			name: "bun drifts in the runtime image's build stage",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "agent/runtime/Dockerfile", "FROM oven/bun:1 AS builder", "FROM oven/bun:2 AS builder")
			},
			want: []string{"bun: agent/runtime/Dockerfile's bun build stage (line 1)", "says 2"},
		},
		{
			name: "bun drifts in ci",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, ".github/workflows/ci.yml", "bun-version: '1.3.14'", "bun-version: '2.0.0'")
			},
			want: []string{"bun: .github/workflows/ci.yml", "says 2.0.0"},
		},
		{
			name: "bun drifts in the readme table",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/README.md", "| Bun | 1 |", "| Bun | 2 |")
			},
			want: []string{"bun: deploy/README.md", "says 2"},
		},
		{
			name: "the table loses its bun row",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/README.md", "| Bun | 1 | `oven/bun:1` |\n", "")
			},
			want: []string{"deploy/README.md: no bun row"},
		},
		{
			name: "a dockerfile that gains a bun stage is read",
			edit: func(t *testing.T, files map[string]string) {
				files["agent/second/Dockerfile"] = "FROM oven/bun:3 AS builder\n"
			},
			want: []string{"bun: agent/second/Dockerfile", "says 3"},
		},
		{
			name: "the two images agree with each other and disagree with ci and the table",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/Dockerfile", "node:26-bookworm", "node:22-bookworm")
				replace(t, files, "deploy/merchant/Dockerfile", "node:26-bookworm", "node:22-bookworm")
			},
			want: []string{
				"node: no single pin is in the majority",
				"deploy/Dockerfile's node build stage",
				"deploy/merchant/Dockerfile's node build stage",
				"setup-node",
				"toolchain table node row",
			},
		},
		{
			name: "the demo image loses its web stage",
			edit: func(t *testing.T, files map[string]string) {
				files["deploy/Dockerfile"] = "FROM golang:1.27-bookworm AS go-build\n"
			},
			want: []string{"deploy/Dockerfile: no base image for node"},
		},
		{
			name: "ci loses setup-node",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, ".github/workflows/ci.yml", "      node-version: '26'\n", "")
			},
			want: []string{"ci.yml: no node-version: line"},
		},
		{
			name: "the table loses its node row",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/README.md", "| Node | 26 | `node:26-bookworm` |\n", "")
			},
			want: []string{"deploy/README.md: no node row"},
		},
		{
			name: "ci loses setup-bun",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, ".github/workflows/ci.yml", "      bun-version: '1.3.14'\n", "")
			},
			want: []string{"ci.yml: no bun-version: line"},
		},
		{
			name: "a dockerfile's bun tag names no version",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "agent/runtime/Dockerfile", "oven/bun:1 AS builder", "oven/bun:latest AS builder")
			},
			want: []string{"agent/runtime/Dockerfile's bun build stage (line 1)", `"latest" is not a numeric version`},
		},
		{
			name: "a dockerfile's bun image loses its tag",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "agent/frontend/Dockerfile", "FROM oven/bun:1", "FROM oven/bun")
			},
			want: []string{"agent/frontend/Dockerfile line 1", "oven/bun states no tag"},
		},
		{
			name: "a dockerfile's bun image is pinned by digest only",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "agent/frontend/Dockerfile", "FROM oven/bun:1", "FROM oven/bun@sha256:6b1d4a2f9c0d")
			},
			want: []string{"agent/frontend/Dockerfile line 1", "states no tag"},
		},
		{
			name: "the tree states no bun image",
			edit: func(t *testing.T, files map[string]string) {
				delete(files, "agent/frontend/Dockerfile")
				delete(files, "agent/runtime/Dockerfile")
			},
			want: []string{"no Dockerfile states a base image for oven/bun"},
		},
		{
			name: "the image tag names no version",
			edit: func(t *testing.T, files map[string]string) {
				replace(t, files, "deploy/Dockerfile", "golang:1.27-bookworm", "golang:latest")
			},
			want: []string{"deploy/Dockerfile's go build stage (line 2)", "names no 2-part version"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := fixture()
			tc.edit(t, files)
			root := writeTree(t, files)

			_, err := check(root)
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("check on agreeing pins: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("check passed, want a failure naming %q", tc.want)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not name %q:\n%v", want, err)
				}
			}
		})
	}
}

// TestCurrentTreeAgrees is the pass the gate relies on: the pins this
// repository actually states, read from the repository itself.
func TestCurrentTreeAgrees(t *testing.T) {
	_, err := check("../..")
	if err != nil {
		t.Fatalf("the current tree's pins disagree: %v", err)
	}
}
