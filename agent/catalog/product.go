package catalog

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VariantValue is one selectable value of a variant option, such as one
// shoe size. Exists tells whether the retailer lists it at all;
// Available carries the stock state, such as InStock, when listed.
type VariantValue struct {
	// Label is the shopper-facing value, such as 10 for a shoe size.
	Label string
	// Exists is false for values the retailer does not list.
	Exists bool
	// Available is the stock state, such as InStock, or empty when unlisted.
	Available string
}

// VariantOption is one variant dimension, such as Size, with its values.
type VariantOption struct {
	// Name is the dimension, such as Size.
	Name string
	// Values are the selectable values in listing order.
	Values []VariantValue
}

// Offer is one retailer's standing offer for a product: its price,
// availability and the buy link. Fulfilment through the link is simulated
// in the demo; see FulfilmentSimulated.
type Offer struct {
	// Availability is the stock state, such as InStock.
	Availability string
	// Condition is the item condition, such as new.
	Condition string
	// Domain is the retailer's domain, such as us.sunspel.com.
	Domain string
	// URL is the retailer's offer page, where size availability lives.
	URL string
	// Currency is the price currency, such as USD.
	Currency string
	// Price is the current price in currency units.
	Price float64
}

// Product is one catalog product from a real retailer: its identity, the
// cut signals Match compares, its offers, and its variants when the
// catalog models any. Variants is nil on every apparel product fetched —
// each colorway is its own product and sizes live only at the retailer —
// so a nil Variants is data, not a gap.
type Product struct {
	// ID is the catalog's product id, such as tbnrl46.
	ID string
	// Title is the product title, such as Men's Button Down Oxford Shirt.
	Title string
	// Description is the free-text description, which usually states the
	// fit in words behind the structured handle.
	Description string
	// Gender is the catalog's gender marker, such as male.
	Gender string
	// BrandID identifies the brand. Fit beliefs are keyed per brand, so
	// matching starts here.
	BrandID string
	// BrandName is the brand's display name, such as Sunspel.
	BrandName string
	// Category is the product category slug, such as dress-shirts.
	Category string
	// Fit carries the category-specific fit handles from the catalog's
	// structured attributes, such as top-fit on shirts or footwear-cut
	// on shoes, each with its values.
	Fit map[string][]string
	// Materials lists the fabric contents, such as 100% Cotton.
	Materials []string
	// Offers are the retailer offers standing for this product.
	Offers []Offer
	// Variants are the modelled variant dimensions, or nil when the
	// catalog models none and sizes live only at the retailer.
	Variants []VariantOption
	// Selected echoes a narrowed variant selection, such as Size 10, as
	// dimension to label. Empty when no selection was requested.
	Selected map[string]string
	// ImageURL is the main image URL, scrubbed of tracking tokens.
	ImageURL string
}

// InStockSizes lists the variant values the retailer lists as InStock, in
// listing order. It is empty when the catalog models no variants: sizes
// then live only at the retailer's offer URL, recorded out-of-band.
func (p Product) InStockSizes() []string {
	var out []string
	for _, opt := range p.Variants {
		for _, v := range opt.Values {
			if v.Exists && v.Available == "InStock" {
				out = append(out, v.Label)
			}
		}
	}
	return out
}

// channel3Product is the wire shape of one product in search results and
// product detail. Unknown fields are ignored so the catalog can grow new
// attributes without breaking the parse.
type channel3Product struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Gender      string `json:"gender"`
	Brands      []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"brands"`
	Category struct {
		Slug string `json:"slug"`
	} `json:"category"`
	StructuredAttributes map[string][]string `json:"structured_attributes"`
	Materials            []string            `json:"materials"`
	Offers               []struct {
		Availability string `json:"availability"`
		Condition    string `json:"condition"`
		Domain       string `json:"domain"`
		URL          string `json:"url"`
		Price        struct {
			Currency string  `json:"currency"`
			Price    float64 `json:"price"`
		} `json:"price"`
	} `json:"offers"`
	Variants *struct {
		Options []struct {
			Name   string `json:"name"`
			Values []struct {
				Label        string  `json:"label"`
				Exists       bool    `json:"exists"`
				Available    *string `json:"available"`
				ThumbnailURL *string `json:"thumbnail_url"`
				ProductID    *string `json:"product_id"`
			} `json:"values"`
		} `json:"options"`
		Selected []struct {
			Name  string `json:"name"`
			Label string `json:"label"`
		} `json:"selected"`
	} `json:"variants"`
	Images []struct {
		URL         string `json:"url"`
		IsMainImage bool   `json:"is_main_image"`
	} `json:"images"`
}

// fitKey reports whether a structured-attribute key names a fit handle:
// the category-specific cut label the match compares. The vocabulary is
// per category — top-fit on shirts, footwear-cut and shoe-collar-height
// on footwear — so the test is a substring over the known fit words,
// never one global field name.
func fitKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "fit") ||
		strings.Contains(lower, "cut") ||
		strings.Contains(lower, "collar")
}

// decodeProduct parses one Channel3 product body into a Product. A body
// without an id is not a product and fails loudly instead of inventing
// one.
func decodeProduct(raw []byte) (Product, error) {
	var wire channel3Product
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Product{}, fmt.Errorf("%w: decode product: %v", ErrBadResponse, err)
	}
	if strings.TrimSpace(wire.ID) == "" {
		return Product{}, fmt.Errorf("%w: product carries no id", ErrBadResponse)
	}
	p := Product{
		ID:          wire.ID,
		Title:       wire.Title,
		Description: wire.Description,
		Gender:      wire.Gender,
		Category:    wire.Category.Slug,
		Materials:   append([]string(nil), wire.Materials...),
		Fit:         map[string][]string{},
		Selected:    map[string]string{},
	}
	if len(wire.Brands) > 0 {
		p.BrandID = wire.Brands[0].ID
		p.BrandName = wire.Brands[0].Name
	}
	for key, values := range wire.StructuredAttributes {
		if fitKey(key) {
			p.Fit[key] = append([]string(nil), values...)
		}
	}
	for _, o := range wire.Offers {
		p.Offers = append(p.Offers, Offer{
			Availability: o.Availability,
			Condition:    o.Condition,
			Domain:       o.Domain,
			URL:          o.URL,
			Currency:     o.Price.Currency,
			Price:        o.Price.Price,
		})
	}
	if wire.Variants != nil {
		for _, opt := range wire.Variants.Options {
			vo := VariantOption{Name: opt.Name}
			for _, v := range opt.Values {
				vv := VariantValue{Label: v.Label, Exists: v.Exists}
				if v.Available != nil {
					vv.Available = *v.Available
				}
				vo.Values = append(vo.Values, vv)
			}
			p.Variants = append(p.Variants, vo)
		}
		for _, s := range wire.Variants.Selected {
			p.Selected[s.Name] = s.Label
		}
	}
	for _, img := range wire.Images {
		if img.IsMainImage {
			p.ImageURL = img.URL
			break
		}
	}
	if p.ImageURL == "" && len(wire.Images) > 0 {
		p.ImageURL = wire.Images[0].URL
	}
	return p, nil
}
