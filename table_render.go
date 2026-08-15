package gomukit

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. The rendered shell contains the table chrome
// only; row content is rendered by the embedded runtime from tool-result
// data (and from the optional baked InitialData snapshot).
func (t *Table) Document() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(t.InitialData) > 0 {
		data = t.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(t.Title, "Table"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  t.Theme.CSS(),
		Body:      t.shell(),
		Config:    t.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (t *Table) shell() g.Node {
	wrap := []g.Node{h.Class("gomu-table-wrap")}
	if t.MaxHeight != "" {
		// The cap turns the wrap into the scroll container the sticky header
		// and the runtime's load-on-scroll hang off (see table.css/table.ts).
		wrap = []g.Node{
			h.Class("gomu-table-wrap gomu-table-wrap--scroll"),
			h.Style("max-height:" + t.MaxHeight),
		}
	}
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "table"),
		h.Div(h.Class("gomu-card"),
			t.toolbar(),
			h.Div(append(wrap,
				// The roles are spelled out because the compact tier restacks
				// each row as a block (see table.css), and `display: block`
				// drops a table's implicit roles. Explicit ones survive the
				// change, so the structure a screen reader hears is the same
				// at every width. The runtime writes the matching row/cell
				// roles on the rows it renders.
				h.Table(h.Class("gomu-table"), h.Role("table"),
					h.THead(h.Role("rowgroup"), h.Tr(append([]g.Node{h.Role("row")}, t.headerCells()...)...)),
					h.TBody(h.Role("rowgroup"), htmlx.Data("rows", "")),
				),
			)...),
			emptyStateNode(t.Empty),
			// LoadMore grows the list in place, so there is no page to move
			// between: its bar replaces the pager.
			g.If(!t.LoadMore, paginationNode(pageSizeOptions(t.PageSize, t.PageSizes), t.PageSize)),
			g.If(t.LoadMore, loadMoreNode()),
			statusNode(t.Brand),
		),
	)
}

func (t *Table) toolbar() g.Node {
	var items []g.Node
	if t.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(t.Title)))
	}
	if t.Filterable {
		items = append(items, h.Input(
			h.Type("search"),
			h.Class("gomu-input gomu-filter"),
			htmlx.Data("filter", ""),
			h.Placeholder("Filter…"),
			h.Aria("label", "Filter rows"),
		))
	}
	if sort := t.sortControl(); sort != nil {
		items = append(items, sort)
	}
	if t.Selection != nil && len(t.Selection.Bulk) > 0 {
		items = append(items, h.Div(
			h.Class("gomu-bulk"),
			htmlx.Data("bulk", ""),
			g.Attr("hidden"),
			h.Span(h.Class("gomu-bulk-count"), htmlx.Data("bulk-count", "")),
			bulkMenuButton(),
		))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

// sortControl renders the compact tier's sort control. Below the stacking
// breakpoint the header row is hidden, and with it the sort buttons, so the
// toolbar carries a select over the same sortable columns; CSS shows it only
// there. Option values are "<key>|asc" / "<key>|desc", as for CardList.
func (t *Table) sortControl() g.Node {
	children := []g.Node{
		h.Class("gomu-input gomu-sort-select"),
		htmlx.Data("sort-select", ""),
		h.Aria("label", "Sort rows"),
		h.Option(h.Value(""), g.Text("Sort…")),
	}
	var n int
	for _, c := range t.Columns {
		if !c.sortable() || c.Key == "" {
			continue
		}
		label := c.Label
		if label == "" {
			label = c.Key
		}
		n++
		children = append(children,
			h.Option(h.Value(c.Key+"|asc"), g.Text(label+" ↑")),
			h.Option(h.Value(c.Key+"|desc"), g.Text(label+" ↓")),
		)
	}
	if n == 0 {
		return nil
	}
	return h.Div(h.Class("gomu-table-sort"), h.Select(children...))
}

func (t *Table) headerCells() []g.Node {
	var cells []g.Node
	if t.Selection != nil {
		cells = append(cells, h.Th(h.Class("gomu-th-select"), h.Role("columnheader"),
			checkboxNode(htmlx.Data("select-all", ""), h.Aria("label", "Select all rows")),
		))
	}
	for _, c := range t.Columns {
		attrs := []g.Node{h.Role("columnheader")}
		if c.Width != "" {
			attrs = append(attrs, h.Style("width:"+c.Width))
		}
		if c.Align != "" {
			attrs = append(attrs, h.Class("gomu-align-"+string(c.Align)))
		}
		if c.sortable() {
			attrs = append(attrs,
				h.Aria("sort", "none"),
				htmlx.Data("sort", c.Key),
				h.Button(h.Type("button"), h.Class("gomu-sort-btn"), g.Text(c.Label)),
			)
		} else {
			attrs = append(attrs, g.Text(c.Label))
		}
		cells = append(cells, h.Th(attrs...))
	}
	return cells
}

// bulkMenuButton renders the trigger for the selection's bulk actions. The
// actions themselves are not rendered here: the runtime fills one shared menu
// panel from the config island when the trigger is pressed (see
// ui/src/actionmenu.ts), the same way it does for a row's actions column.
func bulkMenuButton() g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-bulk-menu"),
		htmlx.Data("bulk-menu", ""),
		h.Aria("haspopup", "menu"),
		h.Aria("expanded", "false"),
		g.Text("Actions"),
		chevronIconNode("gomu-bulk-menu-chevron"),
	)
}

// chevronIconNode is the disclosure mark shared by the menu triggers Go
// renders. Mirrored in ui/src/dropdown.ts, which draws the same path.
func chevronIconNode(class string) g.Node {
	return g.El("svg",
		g.Attr("class", class),
		g.Attr("viewBox", "0 0 16 16"),
		g.Attr("fill", "none"),
		g.Attr("stroke", "currentColor"),
		g.Attr("stroke-width", "1.75"),
		g.Attr("stroke-linecap", "round"),
		g.Attr("stroke-linejoin", "round"),
		g.Attr("aria-hidden", "true"),
		g.El("path", g.Attr("d", "M4 6.5 8 10.5 12 6.5")),
	)
}

// actionButton renders a server-side action button (CardList bulk actions;
// per-row buttons are runtime-rendered from the config island).
func actionButton(a Action, attr string, idx int) g.Node {
	class := "gomu-btn"
	if a.Variant != "" {
		class += " gomu-btn--" + string(a.Variant)
	}
	return h.Button(
		h.Type("button"),
		h.Class(class),
		htmlx.Data(attr, strconv.Itoa(idx)),
		g.Text(a.Label),
	)
}

func docTitle(title, fallback string) string {
	if title != "" {
		return title
	}
	return fallback
}
