// Package theme provides global styling overrides for gomukit widgets.
//
// Widgets ship a two-layer design-token system as CSS custom properties
// (--gomu-*). Semantic tokens default to host-injected variables (MCP Apps
// hosts deliver theme variables via hostContext.styles.variables) with
// built-in fallbacks. A Theme overrides those defaults: its CSS() block is
// emitted after the base stylesheet, so non-empty fields win the cascade.
package theme

import (
	"fmt"
	"sort"
	"strings"
)

// Theme overrides gomukit design tokens. The zero value overrides nothing.
// All fields hold raw CSS values (e.g. "#0f62fe", "0.5rem", "ui-sans-serif,
// system-ui"). Empty fields keep the host-aware defaults.
type Theme struct {
	ColorBackground  string // canvas: widget shell, modal, text inputs
	ColorSurface     string // cream: cards, tiles, chips, table header, hovers
	ColorText        string
	ColorTextMuted   string
	ColorBorder      string
	ColorPrimary     string // accent: primary buttons, focused controls, links
	ColorPrimaryText string // text on primary background
	ColorDanger      string
	ColorSuccess     string
	ColorWarning     string

	FontFamily     string
	FontFamilyMono string

	RadiusS string
	RadiusM string
	RadiusL string

	// SpaceUnit is the base spacing unit all gaps/paddings derive from
	// (default 0.25rem). Increase for a roomier layout, decrease for density.
	SpaceUnit string

	// Framed draws the widget shell as a card: a 1px border with RadiusL
	// corners. Off by default — hosts embed widgets in a bubble or panel that
	// already has its own frame, and a second one around the widget reads as a
	// box inside a box.
	Framed bool

	// Transparent removes the page fill and the gutter to the iframe edge, so
	// the frame itself becomes invisible: only the widget's own surface sits on
	// the host UI. Card, tile, control and overlay surfaces are untouched and
	// stay opaque. Equivalent to ColorPage "transparent" plus PagePad "0".
	//
	// It also requires the host to leave the iframe element unpainted (no
	// background, no border); see docs/theming.md.
	Transparent bool

	// ColorPage overrides the page fill alone, leaving cards, controls and
	// overlays on ColorBackground. Ignored when Transparent is set.
	ColorPage string

	// PagePad is the gutter between the widget and the iframe edge
	// (default 8px). Ignored when Transparent is set.
	PagePad string

	// Extra adds or overrides raw custom properties. Keys must start with
	// "--gomu-". Use it for tokens without a dedicated field.
	Extra map[string]string
}

// tokenFields maps Theme fields to custom-property names in emission order.
var tokenFields = []struct {
	name  string
	value func(*Theme) string
}{
	{"--gomu-color-bg", func(t *Theme) string { return t.ColorBackground }},
	{"--gomu-color-page", func(t *Theme) string {
		if t.Transparent {
			return "transparent"
		}
		return t.ColorPage
	}},
	{"--gomu-color-surface", func(t *Theme) string { return t.ColorSurface }},
	{"--gomu-color-text", func(t *Theme) string { return t.ColorText }},
	{"--gomu-color-text-muted", func(t *Theme) string { return t.ColorTextMuted }},
	{"--gomu-color-border", func(t *Theme) string { return t.ColorBorder }},
	{"--gomu-color-primary", func(t *Theme) string { return t.ColorPrimary }},
	{"--gomu-color-primary-text", func(t *Theme) string { return t.ColorPrimaryText }},
	{"--gomu-color-danger", func(t *Theme) string { return t.ColorDanger }},
	{"--gomu-color-success", func(t *Theme) string { return t.ColorSuccess }},
	{"--gomu-color-warning", func(t *Theme) string { return t.ColorWarning }},
	{"--gomu-font", func(t *Theme) string { return t.FontFamily }},
	{"--gomu-font-mono", func(t *Theme) string { return t.FontFamilyMono }},
	{"--gomu-radius-s", func(t *Theme) string { return t.RadiusS }},
	{"--gomu-radius-m", func(t *Theme) string { return t.RadiusM }},
	{"--gomu-radius-l", func(t *Theme) string { return t.RadiusL }},
	{"--gomu-space-unit", func(t *Theme) string { return t.SpaceUnit }},
	{"--gomu-card-border-width", func(t *Theme) string {
		if t.Framed {
			return "1px"
		}
		return ""
	}},
	{"--gomu-card-radius", func(t *Theme) string {
		if t.Framed {
			return "var(--gomu-radius-l)"
		}
		return ""
	}},
}

// rootFields are document-level tokens: they must land on :root because the
// body gutter is outside the widget element.
var rootFields = []struct {
	name  string
	value func(*Theme) string
}{
	{"--gomu-page-pad", func(t *Theme) string {
		if t.Transparent {
			return "0"
		}
		return t.PagePad
	}},
}

// CSS renders the theme as declaration blocks (":root { … }" for
// document-level tokens, ".gomu-root { … }" for widget tokens), or "" when
// nothing is set. Entries that fail Validate are skipped; call Validate to
// surface them as errors.
func (t *Theme) CSS() string {
	if t == nil {
		return ""
	}
	var out strings.Builder
	if decls := collect(t, rootFields); len(decls) > 0 {
		out.WriteString(":root{" + strings.Join(decls, ";") + "}")
	}
	decls := collect(t, tokenFields)
	for _, k := range sortedKeys(t.Extra) {
		v := t.Extra[k]
		if strings.HasPrefix(k, "--gomu-") && safeKey(k) && v != "" && safeValue(v) {
			decls = append(decls, k+":"+v)
		}
	}
	if len(decls) > 0 {
		out.WriteString(".gomu-root{" + strings.Join(decls, ";") + "}")
	}
	return out.String()
}

func collect(t *Theme, fields []struct {
	name  string
	value func(*Theme) string
}) []string {
	var decls []string
	for _, f := range fields {
		if v := f.value(t); v != "" && safeValue(v) {
			decls = append(decls, f.name+":"+v)
		}
	}
	return decls
}

// Validate reports invalid Extra keys and unsafe values that CSS() would
// silently skip.
func (t *Theme) Validate() error {
	if t == nil {
		return nil
	}
	for _, fields := range [][]struct {
		name  string
		value func(*Theme) string
	}{rootFields, tokenFields} {
		for _, f := range fields {
			if v := f.value(t); v != "" && !safeValue(v) {
				return fmt.Errorf("theme: unsafe value for %s: %q", f.name, v)
			}
		}
	}
	for _, k := range sortedKeys(t.Extra) {
		if !strings.HasPrefix(k, "--gomu-") {
			return fmt.Errorf("theme: Extra key %q must start with --gomu-", k)
		}
		if !safeKey(k) {
			return fmt.Errorf("theme: unsafe Extra key %q", k)
		}
		if v := t.Extra[k]; v == "" || !safeValue(v) {
			return fmt.Errorf("theme: unsafe or empty value for Extra key %q: %q", k, v)
		}
	}
	return nil
}

// safeValue rejects values that could escape the declaration block or the
// enclosing <style> element. Legitimate CSS values (colors, lengths, font
// stacks, var()/light-dark() expressions) never contain these sequences.
func safeValue(v string) bool {
	return !strings.ContainsAny(v, "{};") && !strings.Contains(v, "</") && !strings.Contains(v, "<!--")
}

// safeKey rejects custom-property names with characters outside the safe set.
func safeKey(k string) bool {
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
