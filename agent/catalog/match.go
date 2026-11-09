package catalog

import (
	"fmt"
	"strings"
)

// FulfilmentSimulated labels every candidate's offer link. Channel3 gives
// real product data and buy links, not the right to take payment for other
// retailers, so real-retailer fulfilment in the demo is simulated.
const FulfilmentSimulated = "Real-retailer fulfilment is simulated: Channel3 supplies product data and offer links, not the right to take payment for other retailers."

// Want is the cut a belief names: the brand, the category and the
// category-specific fit handle a returned item wore. A candidate shares
// the cut only when all three agree; ExcludeIDs skips SKUs the shopper
// already returned so the agent recommends a different one.
type Want struct {
	// BrandID identifies the brand the belief is about.
	BrandID string
	// Category is the product category slug, such as dress-shirts.
	Category string
	// FitAttribute is the category-specific fit handle, such as top-fit
	// on shirts or footwear-cut on shoes.
	FitAttribute string
	// FitValues are the handle's values the returned items wore, such as
	// Regular. The candidate's values must equal this set: one line can
	// carry divergent labels per product, so a subset is not a match.
	FitValues []string
	// ExcludeIDs skips products already behind the belief, such as the
	// returned SKUs, so the candidate is a new one.
	ExcludeIDs []string
}

// Validate reports whether the want names a complete cut. An incomplete
// want never matches: matching on a missing part would repeat the
// line-name mistake the catalog data warns against.
func (w Want) Validate() error {
	if strings.TrimSpace(w.BrandID) == "" ||
		strings.TrimSpace(w.Category) == "" ||
		strings.TrimSpace(w.FitAttribute) == "" ||
		len(w.FitValues) == 0 {
		return fmt.Errorf("%w: brand, category, fit attribute and fit values are all required", ErrInvalidWant)
	}
	for _, v := range w.FitValues {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%w: fit values carry no empty entry", ErrInvalidWant)
		}
	}
	return nil
}

// Candidate is one matching product with the evidence for why it matches
// and the marks for where the data ends. It never moves money: the offer
// URL is a link, and fulfilment through it is simulated.
type Candidate struct {
	// Product is the matching catalog product.
	Product Product
	// Shared lists the cut signals the candidate shares with the
	// returned items: brand, category and the equal fit handle. This is
	// the evidence for why it matches.
	Shared []string
	// AvailableSizes are the InStock variant values the catalog lists,
	// in listing order. Empty when the catalog models no variants: sizes
	// then live only at the retailer.
	AvailableSizes []string
	// SizeNote marks where the size data ends: Channel3 carries no size
	// chart and no measurements, and on apparel no variant sizes at
	// all, so size availability is recorded out-of-band at the
	// retailer's offer URL.
	SizeNote string
	// Fulfilment carries FulfilmentSimulated, so every recommendation
	// built on a candidate repeats the label.
	Fulfilment string
}

// normalizeFit lowercases and trims fit values so Regular equals regular.
func normalizeFit(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			set[v] = true
		}
	}
	return set
}

// equalFitSets reports whether two fit value lists name the same set. A
// subset is not a match: Slim plus Tailored on one product is a different
// label than Slim alone, even under the same line name.
func equalFitSets(a, b []string) bool {
	sa, sb := normalizeFit(a), normalizeFit(b)
	if len(sa) != len(sb) {
		return false
	}
	for v := range sa {
		if !sb[v] {
			return false
		}
	}
	return true
}

// sizeNote marks where the size data ends for one product. When the
// catalog models no variants the sizes live only at the retailer; when it
// does, the listed sizes still need the retailer's page to confirm, so
// both notes point out-of-band at the offer URL.
func sizeNote(p Product) string {
	url := ""
	if len(p.Offers) > 0 {
		url = p.Offers[0].URL
	}
	sizes := p.InStockSizes()
	if len(sizes) == 0 {
		note := "Channel3 carries no size values for this product: size availability lives at the retailer offer URL, recorded out-of-band."
		if url != "" {
			note += " Offer: " + url
		}
		return note
	}
	note := "Channel3 lists sizes " + strings.Join(sizes, ", ") + "; confirm availability at the retailer offer URL, recorded out-of-band."
	if url != "" {
		note += " Offer: " + url
	}
	return note
}

// Match returns the products that share the want's cut: same brand, same
// category, equal fit-handle values, none excluded. Each candidate
// carries the shared signals as its evidence and the marks for where the
// data ends. An invalid want matches nothing. Order follows the input.
func Match(want Want, products []Product) []Candidate {
	if err := want.Validate(); err != nil {
		return nil
	}
	excluded := make(map[string]bool, len(want.ExcludeIDs))
	for _, id := range want.ExcludeIDs {
		excluded[id] = true
	}
	var out []Candidate
	for _, p := range products {
		if excluded[p.ID] {
			continue
		}
		if p.BrandID != want.BrandID || p.Category != want.Category {
			continue
		}
		got, ok := p.Fit[want.FitAttribute]
		if !ok || !equalFitSets(got, want.FitValues) {
			continue
		}
		out = append(out, Candidate{
			Product: p,
			Shared: []string{
				"brand " + p.BrandID + " (" + p.BrandName + ")",
				"category " + p.Category,
				want.FitAttribute + ": " + strings.Join(got, ", "),
			},
			AvailableSizes: p.InStockSizes(),
			SizeNote:       sizeNote(p),
			Fulfilment:     FulfilmentSimulated,
		})
	}
	return out
}
