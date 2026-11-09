# Spike: AG Grid inside Chaaya (T0.10)

AG Grid Community mounted through its plain JavaScript API inside a Svelte 5
component on Chaaya. Delete-able: nothing here ships, and `web/` is untouched.

## Run it

```sh
cd spikes/ag-grid
npm ci
npm test      # 15 tests: statuses, renderers, grid mount, editing, themes
npm run dev   # demo page with a theme toggle
npm run build # static bundle, proves the component compiles and links
```

The demo shows four belief-ledger rows, one per status (Claimed, Building,
Earned, Out of true). Status and Shopper note edit in place; the last
committed edit prints below the grid. The header button flips Chaaya's theme
and the grid follows it.

## Files

| File | What it is |
|---|---|
| `src/beliefs.ts` | Statuses, seed rows, column defs, custom cell renderers |
| `src/gridTheme.ts` | AG Grid theme built from Chaaya's CSS tokens, light and dark |
| `src/grid.ts` | `createBeliefGrid`: the plain-JS-API mount the component calls |
| `src/BeliefGrid.svelte` | Svelte 5 wrapper: `createGrid` in `$effect`, theme via `reactive` |
| `src/DemoApp.svelte` | Viewable demo with theme toggle and last-edit readout |
| `src/*.test.ts` | Proof: grid renders, pills render, edits report, themes switch |

## Licence

AG Grid Community 36.3.0, MIT (`node_modules/ag-grid-community/LICENSE.txt`
after install). No Enterprise feature is used, so no Enterprise licence is
needed. Findings and the sponsor-prize note live in
`dev-diary/research/ag-grid.md`.
