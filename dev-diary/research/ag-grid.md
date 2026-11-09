# AG Grid inside Chaaya (T0.10)

AG Grid Community 36.3.0, MIT licence, installed from the npm registry. No
account, no key, no spend. The spike lives in `spikes/ag-grid/` with its own
`package.json`; `web/` is untouched.

## Licence tier

Community. The ledger needs a plain grid, custom cell renderers, the Theming
API, cell editing and a select editor. All of those are Community features;
nothing in the spike imports from `ag-grid-enterprise`. No Enterprise licence
is needed, and the spike must stay that way: P4 inherits this decision for the
real ledger.

The Community package carries `LICENSE.txt` (MIT) at its root after install.
Sponsor-prize fit: the prize rewards use of AG Grid in the entry, and the
Community tier is its free, MIT-licensed tier, which is what a hackathon entry
runs on. The exact sponsor-prize wording lives on Devpost; confirming it is
submission work (T6.6), but nothing in this spike needs a purchase.

## What was run

```sh
cd spikes/ag-grid
npm install          # ag-grid-community 36.3.0, @nrynss/chaaya 0.6.0
npm test             # 2 files, 15 tests, all pass
npm run check        # svelte-check, 0 errors 0 warnings
npm run build        # static bundle, BeliefGrid included (chunk-size warning, see below)
npm run dev          # viewable demo with a Chaaya theme toggle
```

Test transcript (headless, jsdom):

- 4 rows render, 20 cells, 4 `.status-pill` elements, one per ledger status.
- `setDataValue` on a note cell reports `{id, field, oldValue, newValue}`
  through `onEdit`; a status change to `Earned` re-renders the pill.
- Switching the grid option between the light and dark Chaaya themes keeps
  all 4 rows and throws nothing.

## Mounting: plain JavaScript API in a Svelte 5 component

`src/grid.ts` calls `createGrid(host, options)` and returns the `GridApi`;
`src/BeliefGrid.svelte` calls it inside one `$effect` and destroys the grid in
the effect teardown. No AG Grid framework wrapper is used, so there is no
framework-adapter version to track.

v36 requires explicit module registration. Without it the grid renders but
row API calls fail (`error #200`, `api.getDisplayedRowCount is not a
function`-class errors). The spike registers once at import:

```ts
ModuleRegistry.registerModules([AllCommunityModule]);
```

## Cell renderers

Custom renderers are plain functions returning HTML strings, typed with
`ICellRendererParams<BeliefRow>` (`src/beliefs.ts`). The status column renders
a pill span; the seeded column renders a mark only for seeded rows. No Svelte
component is mounted inside a cell, so there is no per-cell framework
overhead and no renderer-lifecycle code to write.

## Theming from Chaaya's tokens

`src/gridTheme.ts` builds `themeQuartz.withParams(...)` from the live
stylesheet via `getComputedStyle(document.documentElement)`, mapping:

| AG Grid param | Chaaya token |
|---|---|
| `backgroundColor` | `--surface` |
| `foregroundColor` / `headerTextColor` | `--text` |
| `headerBackgroundColor` | `--raised` |
| `accentColor` | `--accent` |
| `borderColor` | `--line` |
| `fontFamily` | `--font-ui` |

Forced `data-theme` and the system media query both flow through the same
computed style, so the theme builder needs no theme listener of its own.
Dark mode appends the dark colour-scheme part:

```ts
mode === 'dark' ? light.withPart(colorSchemeDark) : light
```

The component (`src/BeliefGrid.svelte`) reads `theme.resolved` through
Chaaya's `reactive(theme)` in a second `$effect` and applies
`api.setGridOption('theme', ...)` there. Toggling the theme never destroys
the grid, so in-flight edits survive it. Status-pill colours come from
Chaaya's state tokens (`--ok`, `--warn`, `--stop`, `--accent-soft`) in the
spike stylesheet, not from grid params, so pills follow the app theme for
free.

## Row editing

`status` is editable through `agSelectCellEditor` with the four ledger
statuses as values; `note` edits as text. Stable row identity comes from
`getRowId: (params) => params.data.id`. Committed edits arrive via
`onCellValueChanged`, translated to `{id, field, oldValue, newValue}` for the
`onEdit` callback the demo prints below the grid.

## Headless-DOM findings (jsdom)

Recorded so P4 does not rediscover them:

- AG Grid needs `ResizeObserver`, `matchMedia` and `requestAnimationFrame`.
  jsdom provides none usable, so `src/setup.ts` stubs them. The stubs are
  test-only; real browsers need nothing.
- At zero measured width, column virtualization renders only the first
  column. The spike sets `suppressColumnVirtualisation: true` with a comment:
  a five-column ledger grid always fits, so virtualization buys nothing.
- Assertions on rendered output must wait for `firstDataRendered`; the test
  helper does this with a timeout fallback so a silent grid fails loudly.
- `setDataValue` updates data and DOM synchronously, but the
  `cellValueChanged` callback was observed after a task flush in this
  environment, so the edit test awaits a short delay before asserting.
- Theme CSS is not observable in jsdom (v36 themes apply through
  `adoptedStyleSheets`, which jsdom does not implement), and Chaaya's
  reference stylesheet is not loaded in tests, so `chaayaToken` reports its
  light fallbacks there. Theming proof is structural (tokens read, dark part
  applied, `setGridOption` clean) plus visual in `npm run dev`.

## Bundle note for P4

The demo bundle is one 1.2 MB minified chunk (340 KB gzip), all AG Grid. The
real ledger page should lazy-load the grid so the landing page does not pay
for it. Not a spike problem; recorded for the P4 agent.

## Chaaya gaps

None blocking. `reactive(theme)` plus `$effect` covers everything: reading
`theme.resolved` subscribes the effect, and `theme.set` from the demo toggle
repaints both app and grid. Chaaya's `attach()` is for stateful targets with
their own `attach()` lifecycle (follow loops); the theme store has none, so
it is correctly unused here. No issue text needed.
