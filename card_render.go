package gomukit

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// --- Card (single record) ---

// Document implements Widget. The shell contains the card chrome only; the
// record's title/subtitle/fields are rendered by the embedded runtime from
// tool-result data (and the optional baked InitialData snapshot).
func (c *Card) Document() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(c.InitialData) > 0 {
		data = c.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(c.Title, "Card"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  c.Theme.CSS(),
		Body:      c.shell(),
		Config:    c.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (c *Card) shell() g.Node {
	var chrome []g.Node
	chrome = append(chrome, h.Class("gomu-card"))
	if c.Title != "" {
		chrome = append(chrome, h.Div(h.Class("gomu-toolbar"),
			h.H2(h.Class("gomu-title"), g.Text(c.Title)),
		))
	}
	chrome = append(chrome,
		// The runtime renders one card element into this host.
		h.Div(h.Class("gomu-card-host"), htmlx.Data("card", "")),
		emptyStateNode(c.Empty),
		statusNode(c.Brand),
	)
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "card"),
		h.Div(chrome...),
	)
}

// --- CardList (collection) ---

// Document implements Widget. The shell contains the list chrome (toolbar,
// filter, sort, selection, pagination); the cards themselves are rendered by
// the embedded runtime from tool-result data and the optional snapshot.
func (l *CardList) Document() (string, error) {
	if err := l.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(l.InitialData) > 0 {
		data = l.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(l.Title, "Cards"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  l.Theme.CSS(),
		Body:      l.shell(),
		Config:    l.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (l *CardList) shell() g.Node {
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "cardlist"),
		h.Div(h.Class("gomu-card"),
			l.toolbar(),
			carouselNode(),
			emptyStateNode(l.Empty),
			// LoadMore grows the strip in place, so there is no page to move
			// between and no bar to move with — the runtime appends its tile
			// to the strip instead.
			g.If(!l.LoadMore, paginationNode(pageSizeOptions(l.PageSize, l.PageSizes), l.PageSize)),
			statusNode(l.Brand),
		),
	)
}

// carouselNode renders the horizontally scrolling card strip and its
// prev/next controls. The runtime fills the strip and toggles the controls
// from the strip's scroll geometry; both start hidden.
func carouselNode() g.Node {
	return h.Div(h.Class("gomu-carousel"),
		carouselNavButton("prev", "Previous cards", "‹"),
		h.Div(
			h.Class("gomu-card-strip"),
			htmlx.Data("cards", ""),
			h.Role("group"),
			h.TabIndex("0"),
			h.Aria("label", "Cards"),
		),
		carouselNavButton("next", "Next cards", "›"),
	)
}

func carouselNavButton(dir, label, glyph string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-carousel-nav"),
		htmlx.Data("scroll", dir),
		h.Aria("label", label),
		g.Attr("hidden"),
		g.Text(glyph),
	)
}

func (l *CardList) toolbar() g.Node {
	var items []g.Node
	if l.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(l.Title)))
	}
	if l.Selection != nil {
		items = append(items, h.Label(h.Class("gomu-cards-selectall"),
			checkboxNode(htmlx.Data("select-all", ""), h.Aria("label", "Select all cards")),
			g.Text("Select all"),
		))
	}
	if l.Filterable {
		items = append(items, h.Input(
			h.Type("search"),
			h.Class("gomu-input gomu-filter"),
			htmlx.Data("filter", ""),
			h.Placeholder("Filter…"),
			h.Aria("label", "Filter cards"),
		))
	}
	if sort := l.sortControl(); sort != nil {
		items = append(items, sort)
	}
	if l.Selection != nil && len(l.Selection.Bulk) > 0 {
		bulk := []g.Node{
			h.Class("gomu-bulk"),
			htmlx.Data("bulk", ""),
			g.Attr("hidden"),
			h.Span(h.Class("gomu-bulk-count"), htmlx.Data("bulk-count", "")),
		}
		for i, a := range l.Selection.Bulk {
			bulk = append(bulk, actionButton(a, "bulk-action", i))
		}
		items = append(items, h.Div(bulk...))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

// sortControl renders a select over the template's sortable fields; the
// runtime keeps its value in sync with the current sort and reads changes.
// Option values are "<key>|asc" / "<key>|desc".
func (l *CardList) sortControl() g.Node {
	opts := l.Template.sortOptions()
	if len(opts) == 0 {
		return nil
	}
	children := []g.Node{
		h.Class("gomu-input gomu-sort-select"),
		htmlx.Data("sort-select", ""),
		h.Aria("label", "Sort cards"),
		h.Option(h.Value(""), g.Text("Sort…")),
	}
	for _, o := range opts {
		key, _ := o["key"].(string)
		label, _ := o["label"].(string)
		children = append(children,
			h.Option(h.Value(key+"|asc"), g.Text(label+" ↑")),
			h.Option(h.Value(key+"|desc"), g.Text(label+" ↓")),
		)
	}
	return h.Select(children...)
}
