# Channel3 product data

Service: Channel3 catalog API (`https://api.trychannel3.com/v1`), docs at
`https://channel3.ai/docs`. Account: the hackathon developer tier, 1,000 free
credits per month. Every call below sent the key in the `x-api-key` header read
from the environment (`CHANNEL3_API_KEY` via `tools/env.sh`); the key appears
nowhere in this file or in `testdata/channel3/`.

## Commands run, in order

1. `POST /v1/search` body `{"query": "mens oxford button-down shirt", "limit": 10}`
   → 200, 10 products. Recording: `testdata/channel3/search-oxford-shirts.json`.
2. `GET /v1/products/{id}` for 5 shirt SKUs from that search (3 Sunspel, 2 Bonobos)
   → 200 each. Recordings: `testdata/channel3/product-sunspel-*.json`,
   `testdata/channel3/product-bonobos-*.json`.
3. `GET /v1/products/tbnrl46?option_Size=M` on a product with no variants
   → 200, product unchanged, `variants` still `null`. Recording:
   `testdata/channel3/product-sunspel-oxford-white-sizeM-ignored-tbnrl46.json`.
4. `POST /v1/search` body `{"query": "puma running shoes", "limit": 10}`
   → 200, 10 products, 2 with non-null `variants`. Recording:
   `testdata/channel3/search-puma-running-shoes.json`.
5. `GET /v1/products/VWvTIPF` and `GET /v1/products/VWvTIPF?option_Size=10`
   → 200 each. Recordings: `testdata/channel3/product-puma-darter-pro-2-*.json`.
6. `GET /v1/reporting/usage` (default 30-day window) → 200, `total_requests: 0`.

Scrubbing: buy-link and image URLs carry `aid` (account/affiliate id) and `o`
(click token) query parameters. Both values are replaced with `REDACTED` in
every recording. No key, token, email, or account id remains.

## The question: does the data say enough about a product's cut?

Mostly yes for matching, no for measuring. There is no size chart and no
garment-measurement field anywhere in the schema. What exists is a fit label in
two places:

- `structured_attributes` carries a category-specific fit handle. For shirts it
  is `top-fit`; for footwear it is `footwear-cut` / `shoe-collar-height`.
- The free-text `description` usually states the fit in words ("regular fit",
  "slim-fit").

Concrete examples from the recordings:

- Sunspel `tbnrl46` (Men's Button Down Oxford Shirt): `structured_attributes`
  includes `"top-fit": ["Regular"]`, and the description opens "crafted from
  heavyweight 100% cotton … It features a regular fit". Same for the sibling
  SKUs `VyAkjCA` and `PSW8GeK`: identical `top-fit: Regular`, near-identical
  descriptions, same brand, same category `dress-shirts`, same retailer and
  price. Three SKUs, one shared cut.
- Bonobos `V9BYM3z` (Everyday Oxford Shirt, stripe): `"top-fit": ["Slim"]`,
  description "A slim-fit everyday Oxford shirt". Sibling SKU `m4iRrEM`
  (same line name, indigo): `"top-fit": ["Slim", "Tailored"]`. Same line name,
  slightly different fit labels — so the line name alone does not prove a shared
  cut; `top-fit` must be compared per SKU.
- Puma `VWvTIPF` (Men's Puma Darter Pro 2 Running Shoes, footwear):
  `"footwear-cut": ["Low cut"]`, `"shoe-collar-height": ["Low-top"]`. The fit
  vocabulary is category-specific, not one global "cut" field.

So the candidate match for the demo (product.md step 2, "shares the cut of the
returned items") can use, in order: same `brands[].id` + same category slug +
equal `structured_attributes` fit handle (`top-fit`), backed by `title` and
`description` text. There is nothing to map against a size chart in Channel3's
data — the "size chart maps it to the shopper's measurements" step in the demo
must come from the retailer's page (the offer `url`), not from this API.

## Variants and sizes

`variants` is `null` on every apparel product fetched: each colorway is a
separate product id, and sizes are not modelled. Variant support is rolling out
per the docs and is not live on these shirts. Where variants exist, the shape
works as documented:

- Puma `VWvTIPF` detail returns `variants.options: [{name: "Size", values:
  [{label, exists, available, thumbnail_url, product_id}]}]`; sizes 7.5, 9,
  9.5, 10 and 10.5 exist and are `InStock`, while 8, 8.5 and the rest have
  `exists: false` — the recorded order is 4 to 14, not a contiguous run.
- `GET /v1/products/VWvTIPF?option_Size=10` echoes
  `variants.selected: [{name: "Size", label: "10"}]` and narrows `offers` to
  that selection (1 offer).
- On a product with `variants: null`, the `option_Size=M` parameter is ignored
  silently (same bytes back, still `null`).

Consequence for the demo: Channel3 cannot supply the M/L size options for the
shirt line. The size the agent recommends is chosen at the retailer's page, and
the layer matches on brand + line + `top-fit`, not on a Channel3 size value.
Record the retailer's size availability out-of-band if the demo needs it.

## Brand and line chosen for the demo

Sunspel, "Button Down Oxford Shirt" line: SKUs `tbnrl46` (white),
`VyAkjCA` (light blue), `PSW8GeK` (stripe). All share brand id `SMwk`,
category `dress-shirts`, gender `male`, `top-fit: Regular`,
`fabric: Cotton`, one InStock offer each on `us.sunspel.com` at USD 195.
Three returned size-M shirts can be seeded against these SKUs, and a fourth
SKU from the same line is the step-2 candidate. The Bonobos Everyday Oxford
pair is the negative control: same line name, divergent `top-fit`.

## Credits used

Pricing: `/search` costs 1 credit; `/products/*`, `/brands`, `/websites`,
`/categories`, `/reporting/*` are free. This task used 2 searches (2 credits)
and 9 free detail/usage calls. The usage endpoint returned `total_requests: 0`
for the window, so its counters lag real time; the spend above follows the
pricing table, not that endpoint. Well inside the monthly free allowance —
no crawling was done and none is needed.
