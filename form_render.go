package gomukit

import (
	"fmt"
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gomukit/internal/assets"
	"github.com/techthos/gomukit/internal/htmlx"
)

// Document implements Widget. Field structure is fully SSR'd (it is static
// config); prefill values and server-side field errors are runtime state.
func (f *Form) Document() (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(f.InitialData) > 0 {
		data = f.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(f.Title, "Form"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  f.Theme.CSS(),
		Body:      f.shell(),
		Config:    f.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (f *Form) shell() g.Node {
	var body []g.Node
	if f.Title != "" {
		body = append(body, h.Div(h.Class("gomu-toolbar"),
			h.H2(h.Class("gomu-title"), g.Text(f.Title)),
		))
	}
	// novalidate: the runtime runs checkValidity itself and renders inline
	// errors; native validation would swallow the submit event and rely on
	// browser bubbles that hosts' sandboxed iframes may not show.
	formClass := "gomu-form"
	// The form's own width follows its widest grid: one column is a reading
	// measure, four need the room to be four.
	if widest := f.widestGroup(); widest > 1 {
		formClass += " gomu-form--cols-" + strconv.Itoa(widest)
	}
	formChildren := []g.Node{h.Class(formClass), htmlx.Data("form", ""), g.Attr("novalidate")}
	for i, gr := range f.groups() {
		formChildren = append(formChildren, groupNode(gr, i))
	}

	submitLabel := f.Submit.Label
	if submitLabel == "" {
		submitLabel = "Submit"
	}
	actions := []g.Node{h.Class("gomu-form-actions")}
	if f.Cancel != nil {
		cancelLabel := f.Cancel.Label
		if cancelLabel == "" {
			cancelLabel = "Cancel"
		}
		actions = append(actions, h.Button(h.Type("button"), h.Class("gomu-btn"), htmlx.Data("cancel", ""), g.Text(cancelLabel)))
	}
	// type=button, not submit: hosts sandbox the widget iframe without
	// allow-forms, which blocks native form submission outright (the submit
	// event never fires). The runtime drives submission from this click.
	actions = append(actions, h.Button(
		h.Type("button"),
		h.Class("gomu-btn gomu-btn--primary"),
		htmlx.Data("submit", ""),
		g.Text(submitLabel),
	))
	formChildren = append(formChildren, h.Div(actions...))

	body = append(body, h.Form(formChildren...), statusNode(f.Brand))

	return h.Div(h.Class("gomu-root"), htmlx.Data("widget", "form"),
		h.Div(append([]g.Node{h.Class("gomu-card")}, body...)...),
	)
}

// groupNode renders one block of fields: a bare grid for the form's ungrouped
// fields, or a <fieldset> around the same grid for a FieldSet.
//
// The title is a heading the fieldset points at with aria-labelledby rather
// than a <legend>: a legend is laid out inside the fieldset's border by the
// UA, which no grid or panel chrome survives, while aria-labelledby names the
// group just as well.
func groupNode(gr fieldGroup, i int) g.Node {
	grid := gridNode(gr)
	if gr.set == nil {
		return grid
	}
	titleID := fmt.Sprintf("gomu-fs-%d", i)
	class := "gomu-fieldset"
	if gr.set.Boxed {
		class += " gomu-fieldset--boxed"
	}
	head := []g.Node{
		h.Class("gomu-fieldset-head"),
		h.H3(h.Class("gomu-fieldset-title"), h.ID(titleID), g.Text(gr.set.Title)),
	}
	if gr.set.Description != "" {
		head = append(head, h.P(h.Class("gomu-fieldset-desc"), g.Text(gr.set.Description)))
	}
	return h.FieldSet(
		h.Class(class),
		h.Aria("labelledby", titleID),
		h.Div(head...),
		grid,
	)
}

// gridNode lays a group's fields out in its columns. The column count travels
// as a class rather than an inline style: there are four of them, and the
// narrow tiers below override the same classes.
func gridNode(gr fieldGroup) g.Node {
	class := "gomu-form-grid"
	if gr.cols > 1 {
		class += " gomu-cols-" + strconv.Itoa(gr.cols)
	}
	children := []g.Node{h.Class(class)}
	for _, fd := range gr.fields {
		children = append(children, fieldNode(fd, gr.cols))
	}
	return h.Div(children...)
}

func fieldNode(fd Field, cols int) g.Node {
	ft := fd.fieldType()
	id := "gomu-f-" + fd.Name

	if ft == FHidden {
		return h.Input(h.Type("hidden"), h.Name(fd.Name), h.Value(defaultString(fd.Default)))
	}

	var control g.Node
	switch ft {
	case FTextarea:
		rows := fd.Rows
		if rows <= 0 {
			rows = 3
		}
		control = h.Textarea(append(controlAttrs(fd, id),
			h.Rows(strconv.Itoa(rows)),
			g.Text(defaultString(fd.Default)),
		)...)
	case FSelect, FMultiSelect:
		attrs := controlAttrs(fd, id)
		if ft == FMultiSelect {
			attrs = append(attrs, h.Multiple())
		}
		selected := selectedSet(fd.Default)
		for _, opt := range fd.Options {
			optAttrs := []g.Node{h.Value(opt.Value), g.Text(opt.Label)}
			if selected[opt.Value] {
				optAttrs = append(optAttrs, h.Selected())
			}
			attrs = append(attrs, h.Option(optAttrs...))
		}
		control = h.Select(attrs...)
	case FDateRange:
		control = dateRangeNode(fd, id)
	case FCheckbox:
		// Not controlAttrs: the box draws itself (ui/css/check.css) rather than
		// wearing .gomu-input, and none of the text/number validation
		// attributes mean anything on a checkbox.
		attrs := []g.Node{h.ID(id), h.Name(fd.Name)}
		if fd.Required {
			attrs = append(attrs, h.Required())
		}
		if b, ok := fd.Default.(bool); ok && b {
			attrs = append(attrs, h.Checked())
		}
		control = checkboxNode(attrs...)
	default: // text, number, date, time, readonly
		attrs := controlAttrs(fd, id)
		if ft == FDate {
			attrs = append(attrs, dateBounds(fd.Calendar)...)
		}
		attrs = append(attrs, h.Type(inputType(ft)))
		if ft == FReadonly {
			attrs = append(attrs, h.ReadOnly())
		}
		if v := defaultString(fd.Default); v != "" {
			attrs = append(attrs, h.Value(v))
		}
		control = h.Input(attrs...)
	}

	labelText := fd.Label
	if labelText == "" {
		labelText = fd.Name
	}
	var labelChildren []g.Node
	labelChildren = append(labelChildren, h.For(id), g.Text(labelText))
	if fd.Required {
		labelChildren = append(labelChildren, h.Span(h.Class("gomu-required"), h.Aria("hidden", "true"), g.Text(" *")))
	}

	class := "gomu-field gomu-field--" + string(ft)
	if span := fd.span(cols); span > 1 {
		class += " gomu-span-" + strconv.Itoa(span)
	}
	nodes := []g.Node{h.Class(class)}
	if ft == FCheckbox {
		// Checkbox: control first, label after.
		nodes = append(nodes, control, h.Label(labelChildren...))
	} else {
		nodes = append(nodes, h.Label(labelChildren...), control)
	}
	if fd.Description != "" {
		nodes = append(nodes, h.P(h.Class("gomu-field-desc"), g.Text(fd.Description)))
	}
	nodes = append(nodes, h.P(h.Class("gomu-field-error"), htmlx.Data("error-for", fd.Name), g.Attr("hidden")))
	return h.Div(nodes...)
}

// dateRangeNode renders a range field as its two value holders: a native date
// input per end, named after the two tool arguments. The runtime hides them
// behind one trigger and a range calendar (ui/src/calendar.ts) — but they stay
// in the DOM, so the field keeps native constraint validation, and a document
// whose script never runs still asks for both dates.
func dateRangeNode(fd Field, id string) g.Node {
	start, end := fd.rangeDefaults()
	bounds := dateBounds(fd.Calendar)

	label := fd.Label
	if label == "" {
		label = fd.Name
	}

	part := func(id, name, value, class, aria string) g.Node {
		attrs := []g.Node{
			h.Type("date"),
			h.Name(name),
			h.Class("gomu-input " + class),
			h.Aria("label", aria),
		}
		if id != "" {
			attrs = append(attrs, h.ID(id))
		}
		if fd.Required {
			attrs = append(attrs, h.Required())
		}
		if value != "" {
			attrs = append(attrs, h.Value(value))
		}
		return h.Input(append(attrs, bounds...)...)
	}

	// The field's own label addresses the start input by id, and the runtime
	// moves that id onto the trigger it builds — the same handover the
	// dropdown performs over a <select>. Each input also names itself, since
	// without the script the two are all the reader gets.
	return h.Div(
		h.Class("gomu-daterange"),
		htmlx.Data("daterange", fd.Name),
		part(id, fd.Name, start, "gomu-daterange-start", label+" start date"),
		part("", fd.endName(), end, "gomu-daterange-end", label+" end date"),
	)
}

// dateBounds are the native min/max attributes a calendar's window implies.
// The grid enforces the same window, but the attributes make the fallback
// control and form.checkValidity() agree with it.
func dateBounds(c *Calendar) []g.Node {
	if c == nil {
		return nil
	}
	var attrs []g.Node
	if c.Min != "" {
		attrs = append(attrs, h.Min(c.Min))
	}
	if c.Max != "" {
		attrs = append(attrs, h.Max(c.Max))
	}
	return attrs
}

// controlAttrs renders the shared attributes incl. native validation.
func controlAttrs(fd Field, id string) []g.Node {
	attrs := []g.Node{h.ID(id), h.Name(fd.Name), h.Class("gomu-input")}
	if fd.Placeholder != "" {
		attrs = append(attrs, h.Placeholder(fd.Placeholder))
	}
	if fd.Required {
		attrs = append(attrs, h.Required())
	}
	if v := fd.Validation; v != nil {
		if v.Pattern != "" {
			attrs = append(attrs, h.Pattern(v.Pattern))
		}
		if v.Min != nil {
			attrs = append(attrs, h.Min(formatFloat(*v.Min)))
		}
		if v.Max != nil {
			attrs = append(attrs, h.Max(formatFloat(*v.Max)))
		}
		if v.Step != nil {
			attrs = append(attrs, h.Step(formatFloat(*v.Step)))
		}
		if v.MinLen != nil {
			attrs = append(attrs, h.MinLength(strconv.Itoa(*v.MinLen)))
		}
		if v.MaxLen != nil {
			attrs = append(attrs, h.MaxLength(strconv.Itoa(*v.MaxLen)))
		}
	}
	return attrs
}

func inputType(ft FieldType) string {
	switch ft {
	case FNumber:
		return "number"
	case FDate:
		return "date"
	case FTime:
		return "time"
	default:
		return "text"
	}
}

func defaultString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func selectedSet(v any) map[string]bool {
	out := map[string]bool{}
	switch x := v.(type) {
	case string:
		if x != "" {
			out[x] = true
		}
	case []string:
		for _, s := range x {
			out[s] = true
		}
	}
	return out
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
