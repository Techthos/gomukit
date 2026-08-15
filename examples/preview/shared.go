package main

import (
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gomukit"
	"github.com/techthos/gomukit/theme"
)

// --- chrome shared by every widget in this server ---

// appBrand is the inline-SVG logo path: no URL, nothing for the host CSP to
// allow.
func appBrand() *gomukit.Brand {
	return &gomukit.Brand{
		Name:    "Acme Dispatch",
		URL:     "https://example.com",
		LogoSVG: `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M1.5 5.5 8 2l6.5 3.5v5L8 14 1.5 10.5z"/><path d="M1.5 5.5 8 9l6.5-3.5M8 9v5"/></svg>`,
	}
}

// dataURIBrand is the other logo path: a base64 image, which needs the host
// to allow img-src data:. One preview uses it so the difference is visible.
func dataURIBrand() *gomukit.Brand {
	const dot = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNiAxNiI+PGNpcmNsZSBjeD0iOCIgY3k9IjgiIHI9IjciIGZpbGw9IiMwZDlhODgiLz48L3N2Zz4="
	return &gomukit.Brand{Name: "Acme Dispatch", LogoDataURI: dot, LogoAlt: "Acme mark"}
}

// appTheme runs frameless: the host leaves the iframe unpainted, so dropping
// the page fill and the gutter puts the card straight on the host surface.
func appTheme() *theme.Theme {
	return &theme.Theme{Transparent: true}
}

// --- shared column and action building blocks ---

func customerStatusBadge() gomukit.Column {
	return gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
		"active":   gomukit.BadgeSuccess,
		"invited":  gomukit.BadgeInfo,
		"archived": gomukit.BadgeNeutral,
	})
}

func orderStatusBadge() gomukit.Column {
	return gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
		"packed":  gomukit.BadgeInfo,
		"shipped": gomukit.BadgeSuccess,
		"held":    gomukit.BadgeWarning,
	})
}

func customerStatusVariants() map[string]gomukit.BadgeVariant {
	return map[string]gomukit.BadgeVariant{
		"active":   gomukit.BadgeSuccess,
		"invited":  gomukit.BadgeInfo,
		"archived": gomukit.BadgeNeutral,
	}
}

// customerDetails is the typed detail list the card, the account view and the
// delete confirmation all show. Every item type the block supports appears in
// it: text, badge, three number formats, two date formats, a link, an
// authored constant, and a key no record carries.
func customerDetails() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "Account", Key: "name"},
		{Label: "Email", Key: "email"},
		{Label: "Company", Key: "company"},
		{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: customerStatusVariants()},
		{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
		{Label: "Seats", Key: "seats", Type: gomukit.ColNumber, Format: "int"},
		{Label: "Utilization", Key: "utilization", Type: gomukit.ColNumber, Format: "percent"},
		{Label: "Customer since", Key: "createdAt", Type: gomukit.ColDate, Format: "date"},
		{Label: "Renews", Key: "renewsAt", Type: gomukit.ColDate, Format: "relative"},
		{Label: "Console", Type: gomukit.ColLink, Link: &gomukit.LinkSpec{HrefKey: "website", Text: "Open console"}},
		{Label: "Region", Text: "eu-central-1"},
		{Label: "Owner", Key: "owner"},
	}}
}

// --- inline icons: a document references nothing external ---

const (
	iconTable   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 10v10"/></svg>`
	iconCards   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="8" height="14" rx="2"/><rect x="13" y="5" width="8" height="14" rx="2"/></svg>`
	iconCard    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 10h6M7 14h10"/></svg>`
	iconPencil  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>`
	iconPlus    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg>`
	iconTrash   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/></svg>`
	iconTruck   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M1 6h12v9H1zM13 9h4l4 3v3h-8z"/><circle cx="6" cy="18" r="2"/><circle cx="17" cy="18" r="2"/></svg>`
	iconBox     = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1zM10 12h4"/></svg>`
	iconList    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/></svg>`
	iconQuestn  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.7"/><path d="M12 17h.01"/></svg>`
	iconPalette = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 3a9 9 0 1 0 0 18c1 0 1.5-.7 1.5-1.5S12.8 18 12.8 17c0-.8.7-1.5 1.5-1.5H16A5 5 0 0 0 21 10c0-3.9-4-7-9-7z"/><circle cx="7.5" cy="11" r="1"/><circle cx="10.5" cy="7.5" r="1"/><circle cx="14.5" cy="7.5" r="1"/></svg>`
	iconCal     = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M8 3v4M16 3v4M3 10h18"/></svg>`
	iconForm    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 8h10M7 12h10M7 16h6"/></svg>`
)

// --- tool-result helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// Structured result shapes. Every widget reads its data from one of these
// keys; the tools below pick the one their widget expects.
type (
	noArgs struct{}
	noOut  struct{}
	idArg  struct {
		ID int `json:"id" jsonschema:"the record id"`
	}
	idsArg struct {
		IDs []int `json:"ids" jsonschema:"the record ids"`
	}
	rowsOut struct {
		Rows []map[string]any `json:"rows"`
	}
	valuesOut struct {
		Values map[string]any `json:"values"`
	}
	errorsOut struct {
		Errors map[string]string `json:"errors,omitempty"`
	}
	confirmOut struct {
		Rows    []map[string]any `json:"rows"`
		Effects []map[string]any `json:"effects"`
	}
	choiceOut struct {
		Rows    []map[string]any `json:"rows"`
		Options []map[string]any `json:"options"`
	}
	// dateOut is what a DatePicker reads: the record the question is about,
	// plus the selection and the window it may move in.
	dateOut struct {
		Rows  []map[string]any `json:"rows"`
		Value map[string]any   `json:"value"`
	}
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
