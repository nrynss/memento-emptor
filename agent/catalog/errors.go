package catalog

import "errors"

// ErrEmptyQuery is returned when a search names no query. An empty query
// matches everything, so it matches nothing loudly instead.
var ErrEmptyQuery = errors.New("catalog: search names no query")

// ErrBadLimit is returned when a search limit is not positive. The caller
// names how many products it wants to choose from.
var ErrBadLimit = errors.New("catalog: search limit is not positive")

// ErrMissingAPIKey is returned when a live Channel3 request has no key.
// The key travels in from the environment and is never written to this
// repository; a request without one fails instead of going out anonymous.
var ErrMissingAPIKey = errors.New("catalog: channel3 request carries no api key")

// ErrEmptyProductID is returned when a product fetch names no product.
var ErrEmptyProductID = errors.New("catalog: product fetch names no product id")

// ErrBadResponse is returned when the catalog service answers outside
// 200, or its body does not decode: transport failure, wrong status, or a
// shape the parser no longer recognises. Match it with errors.Is.
var ErrBadResponse = errors.New("catalog: bad catalog response")

// ErrInvalidWant is returned when the match request names no complete
// cut. A new product shares a cut only when brand, category and fit
// handle all agree; matching on a missing part would repeat the
// line-name mistake, so an incomplete want matches nothing loudly
// instead.
var ErrInvalidWant = errors.New("catalog: match want names no complete cut")
