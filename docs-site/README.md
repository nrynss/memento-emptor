# Builder docs site

The builder documentation, built with Astro Starlight. It is a separate static
site from the ledger web app and has no server.

```sh
npm ci
npm run build
```

The output is `docs-site/dist/`, which is git-ignored. The build makes no network
request: it reads this directory and the repository's own `docs/` files.

## Where each page comes from

| Page | Source |
|---|---|
| `src/content/docs/index.mdx` | Written here: the front page and project links |
| `src/content/docs/what-the-layer-is.md` | Written here, from `product.md` |
| `src/content/docs/interface.md` | Generated from `docs/interface.md` at build time |
| `src/content/docs/architecture.md` | Generated from `docs/architecture.md` at build time |
| `src/content/docs/demo-and-evidence.md` | Written here, from `demo/README.md` |
| `src/content/docs/comparison.md` | Written here, from `bench/README.md`, `bench/round2/README.md` and `product.md` |
| `src/content/docs/submission-summary.md` | A labelled placeholder, waiting for T6.3 |

`scripts/sync-docs.mjs` copies the interface and architecture documents, strips
their source H1 so Starlight's frontmatter title is the only page heading, and
adds a generated-page note. The copies are git-ignored: `docs/` is the only
source, and the site cannot drift from it.

The site is not wired into `tools/check.sh`; that file is owned by another task.
Run its build separately, as above.

## Staged state

The submission summary page is deliberately incomplete. It waits for T6.3,
T6.4 and T6.5. The comparison page carries both rounds: round 1, and round 2
from its own committed run. See the T6.8 note in
`dev-diary/PHASE-6-ship.md` for the resume steps.
