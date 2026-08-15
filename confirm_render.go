package gomukit

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. Everything the author wrote — prompt, body,
// guards, buttons — is rendered here; the detail values and the effect list
// are runtime state, so they get hosts the runtime fills from the record and
// from tool results.
func (c *Confirm) Document() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(c.InitialData) > 0 {
		data = c.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(c.Title, "Confirm"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  c.Theme.CSS(),
		Body:      c.shell(),
		Config:    c.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (c *Confirm) shell() g.Node {
	chrome := []g.Node{h.Class("gomu-card gomu-confirm gomu-confirm--" + string(c.severity()))}
	if toolbar := c.toolbar(); toolbar != nil {
		chrome = append(chrome, toolbar)
	}
	chrome = append(chrome,
		c.promptNode(),
		// Filled by the runtime from rows[0] and from the effects the tool
		// result carries; both start empty and stay out of the layout until
		// there is something to show.
		g.If(!c.Details.empty(), h.Dl(h.Class("gomu-descriptions"), htmlx.Data("descriptions", ""), g.Attr("hidden"))),
		h.Ul(h.Class("gomu-effects"), htmlx.Data("effects", ""), g.Attr("hidden")),
		c.guardsNode(),
		c.decisionNode(),
		h.P(h.Class("gomu-confirm-outcome"), htmlx.Data("outcome", ""), g.Attr("hidden"), h.Aria("live", "polite")),
		statusNode(c.Brand),
	)
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "confirm"),
		h.Div(chrome...),
	)
}

func (c *Confirm) toolbar() g.Node {
	var items []g.Node
	if c.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(c.Title)))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

// promptNode is the question itself, marked with the severity icon so the
// weight of the decision is legible before a word is read.
func (c *Confirm) promptNode() g.Node {
	text := []g.Node{h.Class("gomu-confirm-text"), h.H3(h.Class("gomu-confirm-question"), g.Text(c.Prompt))}
	if c.Body != "" {
		text = append(text, h.P(h.Class("gomu-confirm-lede"), g.Text(c.Body)))
	}
	return h.Div(h.Class("gomu-confirm-prompt"),
		h.Span(h.Class("gomu-confirm-icon"), h.Aria("hidden", "true"), severityIcon(c.severity())),
		h.Div(text...),
	)
}

// severityIcon is inline markup, never a URL: the document references nothing
// external. Danger and warning share the alert triangle; anything milder gets
// the informational disc.
func severityIcon(sev BadgeVariant) g.Node {
	path := g.Group([]g.Node{
		g.El("circle", g.Attr("cx", "12"), g.Attr("cy", "12"), g.Attr("r", "9")),
		g.El("path", g.Attr("d", "M12 11v5")),
		g.El("path", g.Attr("d", "M12 8h.01")),
	})
	if sev == BadgeDanger || sev == BadgeWarning {
		path = g.Group([]g.Node{
			g.El("path", g.Attr("d", "M10.3 3.9 1.9 18a2 2 0 0 0 1.7 3h16.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z")),
			g.El("path", g.Attr("d", "M12 9v4")),
			g.El("path", g.Attr("d", "M12 17h.01")),
		})
	}
	return g.El("svg",
		g.Attr("viewBox", "0 0 24 24"),
		g.Attr("fill", "none"),
		g.Attr("stroke", "currentColor"),
		g.Attr("stroke-width", "2"),
		g.Attr("stroke-linecap", "round"),
		g.Attr("stroke-linejoin", "round"),
		path,
	)
}

// guardsNode renders the friction the author asked for: an acknowledgement to
// tick, a phrase to type, or neither. The runtime keeps the accept button
// disabled until every guard present here is satisfied.
func (c *Confirm) guardsNode() g.Node {
	var guards []g.Node
	if c.Acknowledge != "" {
		guards = append(guards, h.Label(h.Class("gomu-confirm-ack"),
			checkboxNode(htmlx.Data("ack", "")),
			h.Span(g.Text(c.Acknowledge)),
		))
	}
	if c.TypeToConfirm != "" {
		const id = "gomu-confirm-phrase"
		guards = append(guards, h.Div(h.Class("gomu-confirm-type"),
			h.Label(h.For(id),
				g.Text("Type "),
				h.Code(g.Text(c.TypeToConfirm)),
				g.Text(" to confirm"),
			),
			h.Input(
				h.ID(id),
				h.Type("text"),
				h.Class("gomu-input"),
				htmlx.Data("phrase", ""),
				h.AutoComplete("off"),
				g.Attr("spellcheck", "false"),
				g.Attr("autocapitalize", "off"),
			),
		))
	}
	if len(guards) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-confirm-guards")}, guards...)...)
}

// decisionNode is the two-outcome bar. Accept starts disabled whenever a
// guard is configured, so the widget is correct before the runtime mounts —
// and the reject button leads, keeping the destructive one away from where
// the thumb lands first.
func (c *Confirm) decisionNode() g.Node {
	nodes := []g.Node{h.Class("gomu-confirm-actions"), htmlx.Data("decision", "")}
	if c.Reject != nil {
		nodes = append(nodes, h.Button(
			h.Type("button"),
			h.Class("gomu-btn"),
			htmlx.Data("reject", ""),
			g.Text(c.rejectLabel()),
		))
	}
	accept := []g.Node{
		h.Type("button"),
		h.Class("gomu-btn gomu-btn--" + string(c.acceptVariant())),
		htmlx.Data("accept", ""),
		g.Text(c.acceptLabel()),
	}
	if c.Acknowledge != "" || c.TypeToConfirm != "" {
		accept = append(accept, h.Disabled())
	}
	return h.Div(append(nodes, h.Button(accept...))...)
}
