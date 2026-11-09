// Package catalog is the reference agent's product shelf: candidate
// products and variants from real retailers, behind a seam a builder can
// replace.
//
// A belief names a cut — brand, category and the category-specific fit
// handle — and Match returns only the products that share all three, with
// the shared signals listed as the evidence for why each one matches. The
// line name alone never matches: one line can carry divergent fit labels
// per product, so matching on brand and line would repeat the mistake the
// catalog data warns against.
//
// Where the data ends is marked on every candidate. Channel3 carries no
// size chart and no garment measurements, and variants is null on every
// apparel product fetched: each colorway is its own product and sizes live
// only at the retailer. The candidate says so, and size availability is
// recorded out-of-band at the retailer's offer URL. Real-retailer
// fulfilment is labelled simulated: Channel3 supplies product data and
// offer links, not the right to take payment for other retailers.
//
// Tests replay the recordings in testdata/channel3/ through a stub
// transport, offline. Only a single re-verification call is live.
package catalog
