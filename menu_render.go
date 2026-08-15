package gomukit

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. A menu is authored, not fetched: the tiles are
// rendered here in full. The runtime only turns a click into the tool call
// named in the config island.
func (m *Menu) Document() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(m.Title, "Menu"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  m.Theme.CSS(),
		Body:      m.shell(),
		Config:    m.config(),
		RuntimeJS: assets.RuntimeJS,
	})
}

func (m *Menu) shell() g.Node {
	chrome := []g.Node{h.Class("gomu-card"), m.toolbar()}
	if m.Intro != "" {
		chrome = append(chrome, h.P(h.Class("gomu-menu-intro"), g.Text(m.Intro)))
	}
	chrome = append(chrome, m.grid(), statusNode(m.Brand))
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "menu"),
		h.Div(chrome...),
	)
}

func (m *Menu) toolbar() g.Node {
	var items []g.Node
	if m.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(m.Title)))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

func (m *Menu) grid() g.Node {
	// One tile's icon/badge row would otherwise push its label below the
	// labels of the plain tiles beside it, so the row is either present on
	// every tile of the menu or on none.
	tops := false
	for _, item := range m.Items {
		if item.IconSVG != "" || item.Badge != "" {
			tops = true
			break
		}
	}
	nodes := []g.Node{h.Class("gomu-menu"), htmlx.Data("menu", "")}
	for i, item := range m.Items {
		nodes = append(nodes, menuTile(item, i, tops))
	}
	return h.Div(nodes...)
}

// menuTile renders one entry. The index is the runtime's lookup into the
// config island's items array, so tile order and config order must match.
// Validation has already run, so an unsafe icon is dropped rather than
// half-rendered.
func menuTile(item MenuItem, idx int, top bool) g.Node {
	nodes := []g.Node{
		h.Type("button"),
		h.Class("gomu-menu-item"),
		htmlx.Data("menu-item", strconv.Itoa(idx)),
	}
	if top {
		nodes = append(nodes, menuTileTop(item))
	}
	nodes = append(nodes, h.Span(h.Class("gomu-menu-label"), g.Text(item.label())))
	if item.Description != "" {
		nodes = append(nodes, h.Span(h.Class("gomu-menu-desc"), g.Text(item.Description)))
	}
	return h.Button(nodes...)
}

// menuTileTop is the icon/badge row above the label. It keeps its height when
// the item has neither, so labels line up across the grid.
func menuTileTop(item MenuItem) g.Node {
	var parts []g.Node
	if item.IconSVG != "" {
		if svg, err := htmlx.RawSVG(item.IconSVG); err == nil {
			parts = append(parts, h.Span(h.Class("gomu-menu-icon"), h.Aria("hidden", "true"), svg))
		}
	}
	if item.Badge != "" {
		class := "gomu-badge gomu-menu-badge"
		if item.BadgeVariant != "" && item.BadgeVariant != BadgeNeutral {
			class += " gomu-badge--" + string(item.BadgeVariant)
		}
		parts = append(parts, h.Span(h.Class(class), g.Text(item.Badge)))
	}
	return h.Span(append([]g.Node{h.Class("gomu-menu-top")}, parts...)...)
}
