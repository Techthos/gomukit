package gomukit

import (
	"fmt"
	"sort"
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/htmlx"
)

// statusNode renders the shared, runtime-driven status/announcement region.
// It is the last element of every widget, and its bar keeps its height whether
// or not a message is showing, so the widget does not resize (and the host
// iframe does not jump) when work starts or finishes.
func statusNode(b *Brand) g.Node {
	return h.Div(h.Class("gomu-statusbar"),
		brandNode(b),
		h.Div(h.Class("gomu-status"), htmlx.Data("status", ""), g.Attr("hidden"), h.Aria("live", "polite")),
	)
}

// emptyStateNode renders the shared no-data region, hidden until the runtime
// determines there is nothing to show. Title defaults to "No data".
func emptyStateNode(e EmptyState) g.Node {
	title := e.Title
	if title == "" {
		title = "No data"
	}
	nodes := []g.Node{
		h.Class("gomu-empty"),
		htmlx.Data("empty", ""),
		g.Attr("hidden"),
		h.H3(g.Text(title)),
	}
	if e.Body != "" {
		nodes = append(nodes, h.P(g.Text(e.Body)))
	}
	return h.Div(nodes...)
}

// paginationNode renders the shared prev/next pager, hidden until the runtime
// enables it. When the widget offers a choice of page sizes, the chooser leads
// the bar; the runtime keeps the page size it reports in sync with it.
func paginationNode(sizes []int, current int) g.Node {
	nodes := []g.Node{h.Class("gomu-pagination"), htmlx.Data("pagination", ""), g.Attr("hidden")}
	if len(sizes) > 0 {
		nodes = append(nodes, pageSizeNode(sizes, current))
	}
	nodes = append(nodes,
		pagerButton("prev", "Previous page", "M10 4 6 8 10 12"),
		h.Span(h.Class("gomu-page-info"), htmlx.Data("page-info", "")),
		pagerButton("next", "Next page", "M6 4 10 8 6 12"),
	)
	return h.Div(nodes...)
}

// pagerButton renders one step of the pager as a chevron icon button; the
// direction the chevron points is the path, the words are the aria-label.
func pagerButton(dir, label, path string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-page-btn"),
		htmlx.Data("page", dir),
		h.Aria("label", label),
		g.El("svg",
			g.Attr("viewBox", "0 0 16 16"),
			g.Attr("fill", "none"),
			g.Attr("stroke", "currentColor"),
			g.Attr("stroke-width", "1.75"),
			g.Attr("stroke-linecap", "round"),
			g.Attr("stroke-linejoin", "round"),
			g.Attr("aria-hidden", "true"),
			g.El("path", g.Attr("d", path)),
		),
	)
}

// loadMoreNode renders the growing-list alternative to the pagination bar
// (Table.LoadMore), hidden until the runtime knows more rows exist.
func loadMoreNode() g.Node {
	return h.Div(h.Class("gomu-more"), htmlx.Data("more", ""), g.Attr("hidden"),
		h.Button(h.Type("button"), h.Class("gomu-btn gomu-more-btn"), htmlx.Data("reveal", ""), g.Text("Load more")),
		h.Span(h.Class("gomu-more-count"), htmlx.Data("more-count", "")),
	)
}

// pageSizeNode renders the per-page chooser. The runtime upgrades the select
// into a gomukit dropdown, so the caption is a span rather than a label: it
// names the control for the eye, while the select carries the accessible name.
func pageSizeNode(sizes []int, current int) g.Node {
	opts := []g.Node{
		h.Class("gomu-input gomu-page-size-select"),
		htmlx.Data("page-size", ""),
		h.Aria("label", "Items per page"),
	}
	for _, n := range sizes {
		value := strconv.Itoa(n)
		attrs := []g.Node{h.Value(value), g.Text(value)}
		if n == current {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(attrs...))
	}
	return h.Div(h.Class("gomu-page-size"),
		h.Span(g.Text("Per page")),
		h.Select(opts...),
	)
}

// pageSizeOptions is the set of page sizes a pagination bar offers: the
// configured alternatives plus the current page size, deduplicated and
// ascending. Empty when the widget does not paginate or names no
// alternatives, in which case no chooser is rendered at all.
func pageSizeOptions(pageSize int, sizes []int) []int {
	if pageSize <= 0 || len(sizes) == 0 {
		return nil
	}
	seen := map[int]bool{}
	var out []int
	for _, n := range append(append([]int{}, sizes...), pageSize) {
		if n > 0 && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// validatePageSizes checks the PageSizes field shared by the paginated
// widgets. ctx is the caller's error prefix.
func validatePageSizes(ctx string, pageSize int, sizes []int) error {
	if len(sizes) == 0 {
		return nil
	}
	if pageSize <= 0 {
		return fmt.Errorf("%s: PageSizes needs PageSize > 0", ctx)
	}
	for _, n := range sizes {
		if n <= 0 {
			return fmt.Errorf("%s: PageSizes entries must be > 0, got %d", ctx, n)
		}
	}
	return nil
}
