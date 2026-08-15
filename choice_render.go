package gomukit

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. The question and the buttons are authored, so
// they are rendered here; the options are not. An option list can be replaced
// wholesale by a tool result, so the list, the description panel and the
// detail block are empty hosts the runtime fills.
func (c *Choice) Document() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(c.InitialData) > 0 {
		data = c.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(c.Title, "Choice"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  c.Theme.CSS(),
		Body:      c.shell(),
		Config:    c.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (c *Choice) shell() g.Node {
	chrome := []g.Node{h.Class("gomu-card gomu-choice gomu-choice--" + c.layoutName())}
	if toolbar := c.toolbar(); toolbar != nil {
		chrome = append(chrome, toolbar)
	}
	chrome = append(chrome,
		c.promptNode(),
		// Filled by the runtime from rows[0]; out of the layout until there is
		// a record to describe.
		g.If(!c.Details.empty(), h.Dl(h.Class("gomu-descriptions"), htmlx.Data("descriptions", ""), g.Attr("hidden"))),
		c.optionsNode(),
		emptyStateNode(EmptyState{Title: "Nothing to choose from", Body: "No options are available right now."}),
		h.P(h.Class("gomu-choice-hint"), htmlx.Data("hint", ""), g.Attr("hidden"), h.Aria("live", "polite")),
		c.decisionNode(),
		h.P(h.Class("gomu-choice-outcome"), htmlx.Data("outcome", ""), g.Attr("hidden"), h.Aria("live", "polite")),
		statusNode(c.Brand),
	)
	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "choice"),
		h.Div(chrome...),
	)
}

func (c *Choice) toolbar() g.Node {
	var items []g.Node
	if c.Title != "" {
		items = append(items, h.H2(h.Class("gomu-title"), g.Text(c.Title)))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gomu-toolbar")}, items...)...)
}

func (c *Choice) promptNode() g.Node {
	nodes := []g.Node{
		h.Class("gomu-choice-prompt"),
		h.H3(h.Class("gomu-choice-question"), h.ID("gomu-choice-question"), g.Text(c.Prompt)),
	}
	if c.Body != "" {
		nodes = append(nodes, h.P(h.Class("gomu-choice-lede"), g.Text(c.Body)))
	}
	return h.Div(nodes...)
}

// optionsNode is the two-column frame: the list of options and the panel that
// describes the one in hand. Which of the panel and the per-option blocks is
// used is a runtime decision — see ChoiceLayout — so both hosts are rendered
// and the runtime fills whichever the effective layout calls for.
func (c *Choice) optionsNode() g.Node {
	role := "radiogroup"
	if c.Multiple {
		role = "group"
	}
	return h.Div(h.Class("gomu-choice-body"),
		h.Div(
			h.Class("gomu-choice-list"),
			htmlx.Data("options", ""),
			h.Role(role),
			h.Aria("labelledby", "gomu-choice-question"),
		),
		h.Div(h.Class("gomu-choice-panel"), htmlx.Data("panel", ""), g.Attr("hidden")),
	)
}

// decisionNode is the submit bar. Submit starts disabled: nothing is chosen
// until the runtime mounts, and a widget whose script never runs must not
// offer a call it cannot make. Cancel leads, so the committing button is not
// where the thumb lands first.
func (c *Choice) decisionNode() g.Node {
	nodes := []g.Node{h.Class("gomu-choice-actions"), htmlx.Data("decision", "")}
	if c.Cancel != nil {
		nodes = append(nodes, h.Button(
			h.Type("button"),
			h.Class("gomu-btn"),
			htmlx.Data("cancel", ""),
			g.Text(c.cancelLabel()),
		))
	}
	return h.Div(append(nodes, h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-btn--"+string(c.submitVariant())),
		htmlx.Data("submit", ""),
		h.Disabled(),
		g.Text(c.submitLabel()),
	))...)
}
