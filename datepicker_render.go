package gomukit

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. The question and the buttons are authored, so
// they are rendered here; the grid is not. Which days a month holds, which of
// them are still free, and what the reader has picked are all runtime state —
// and the month names come from the host's locale, which is not known until
// the handshake — so the calendar is an empty host the runtime fills.
func (d *DatePicker) Document() (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(d.InitialData) > 0 {
		data = d.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(d.Title, "Date picker"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  d.Theme.CSS(),
		Body:      d.shell(),
		Config:    d.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (d *DatePicker) shell() g.Node {
	chrome := []g.Node{h.Class("gomu-card gomu-datepicker gomu-datepicker--" + d.modeName())}
	if toolbar := d.toolbar(); toolbar != nil {
		chrome = append(chrome, toolbar)
	}
	chrome = append(chrome,
		d.promptNode(),
		// Filled by the runtime from rows[0]; out of the layout until there is
		// a record to describe.
		g.If(!d.Details.empty(), h.Dl(h.Class("gomu-descriptions"), htmlx.Data("descriptions", ""), g.Attr("hidden"))),
		h.Div(h.Class("gomu-datepicker-body"),
			h.Div(h.Class("gomu-cal"), htmlx.Data("calendar", ""), h.Aria("labelledby", "gomu-datepicker-question")),
		),
		h.P(h.Class("gomu-datepicker-summary"), htmlx.Data("summary", ""), g.Attr("hidden"), h.Aria("live", "polite")),
		d.decisionNode(),
		h.P(h.Class("gomu-datepicker-outcome"), htmlx.Data("outcome", ""), g.Attr("hidden"), h.Aria("live", "polite")),
		statusNode(d.Brand),
	)
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "datepicker"),
		h.Div(chrome...),
	)
}

func (d *DatePicker) toolbar() g.Node {
	var items []g.Node
	if d.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(d.Title)))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

func (d *DatePicker) promptNode() g.Node {
	nodes := []g.Node{
		h.Class("gomu-datepicker-prompt"),
		h.H3(h.Class("gomu-datepicker-question"), h.ID("gomu-datepicker-question"), g.Text(d.Prompt)),
	}
	if d.Body != "" {
		nodes = append(nodes, h.P(h.Class("gomu-datepicker-lede"), g.Text(d.Body)))
	}
	return h.Div(nodes...)
}

// decisionNode is the submit bar. Submit starts disabled: a date is not picked
// until the runtime mounts, and a widget whose script never runs must not offer
// a call it cannot make. Cancel leads, so the committing button is not where
// the thumb lands first.
func (d *DatePicker) decisionNode() g.Node {
	nodes := []g.Node{h.Class("gomu-datepicker-actions"), htmlx.Data("decision", "")}
	if d.Cancel != nil {
		nodes = append(nodes, h.Button(
			h.Type("button"),
			h.Class("gomu-btn"),
			htmlx.Data("cancel", ""),
			g.Text(d.cancelLabel()),
		))
	}
	return h.Div(append(nodes, h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-btn--"+string(d.submitVariant())),
		htmlx.Data("submit", ""),
		h.Disabled(),
		g.Text(d.submitLabel()),
	))...)
}
