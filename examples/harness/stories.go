package main

import (
	"fmt"

	"github.com/techthos/gomukit"
	"github.com/techthos/gomukit/theme"
)

// story is one entry in the harness catalog: a named widget configuration the
// host page loads into its iframe. The page reads the catalog from
// /stories.json and renders it as a rail grouped by Group.
type story struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Label string `json:"label"`
	Desc  string `json:"desc"`
	Src   string `json:"src"`
	// Payload seeds the push panel with a structuredContent example that
	// makes sense for this story.
	Payload string `json:"payload"`

	build func() gomukit.Widget
}

// Push-panel presets. Kept as literal JSON so the textarea shows it the way a
// tool author would write it.
const (
	pushRows = `{
  "rows": [
    {
      "id": 9, "name": "Pushed Row", "email": "pushed@example.com",
      "balance": 42.5, "createdAt": "2026-07-23T00:00:00Z",
      "status": "invited", "website": "https://example.com/pushed"
    }
  ]
}`
	pushEmptyRows = `{ "rows": [] }`
	pushErrors    = `{
  "errors": { "email": "This email is already taken." }
}`
	pushValues = `{
  "values": {
    "name": "Grace Hopper", "email": "grace@example.com",
    "role": "admin", "notify": false
  }
}`
	// One record with a field per item type, and no "owner" — the item that
	// reads it shows the missing-value dash.
	pushDetails = `{
  "rows": [
    {
      "id": 2, "name": "Grace Hopper", "email": "grace@example.com",
      "plan": "enterprise", "balance": 4820.4, "seats": 48,
      "utilization": 0.71, "renewsAt": "2027-01-31T00:00:00Z",
      "createdAt": "2024-11-02T09:00:00Z", "status": "active",
      "website": "https://example.com/grace"
    }
  ]
}`
	pushEffects = `{
  "rows": [
    {
      "id": 2, "name": "Grace Hopper", "email": "grace@example.com",
      "balance": 815, "createdAt": "2026-02-03T10:30:00Z",
      "status": "active", "website": "https://example.com/grace"
    }
  ],
  "effects": [
    { "text": "Removes the account", "severity": "danger" },
    {
      "text": "Deletes audit records", "detail": "Not recoverable.",
      "value": "128", "severity": "warning"
    },
    { "text": "Notifies the team", "value": "4 people", "severity": "info" }
  ]
}`
	pushOptions = `{
  "rows": [
    { "id": 4471, "reference": "ORD-4471", "name": "Ada Lovelace" }
  ],
  "options": [
    {
      "value": "drone", "label": "Drone", "summary": "within the hour",
      "body": "Flown from the Kreuzberg hub. Weather permitting.",
      "bullets": ["Live tracking", "Rooftop drop only"],
      "details": [
        { "label": "Price", "value": "EUR 39.00" },
        { "label": "Arrives", "value": "today, 15:40" }
      ],
      "badge": "new", "badgeVariant": "info", "default": true
    },
    {
      "value": "bike", "label": "Bike courier", "summary": "same day",
      "body": "A rider collects the parcel at the next pickup round.",
      "details": [{ "label": "Price", "value": "EUR 12.00" }]
    },
    {
      "value": "post", "label": "Post", "summary": "not collecting today",
      "disabled": true
    }
  ]
}`

	// A selection plus the window it may move in: the runtime takes the bounds
	// and the blocked days from the same object as the dates.
	pushDates = `{
  "rows": [
    { "id": 4471, "reference": "BKG-4471", "name": "Ada Lovelace" }
  ],
  "value": {
    "start": "2026-08-20",
    "end": "2026-08-23",
    "min": "2026-08-01",
    "max": "2026-10-31",
    "disabled": ["2026-08-27", "2026-08-28", "2026-08-29"]
  }
}`
)

// catalog returns every story, in rail order. Src is filled in by stories().
func catalog() []story {
	return []story{
		{
			ID: "table-default", Group: "Table", Label: "Full featured",
			Desc:    "Filter, pagination, selection with a bulk action, and a per-row delete behind a confirm.",
			Payload: pushRows, build: func() gomukit.Widget { return table() },
		},
		{
			ID: "table-plain", Group: "Table", Label: "Read only",
			Desc:    "No filter, pagination, selection or actions — just typed columns.",
			Payload: pushRows, build: func() gomukit.Widget { return tablePlain() },
		},
		{
			ID: "table-long", Group: "Table", Label: "Long list",
			Desc:    "24 records, page size 8, pre-sorted by balance.",
			Payload: pushRows, build: func() gomukit.Widget { return tableLong() },
		},
		{
			ID: "table-loadmore", Group: "Table", Label: "Load more",
			Desc:    "24 records, eight at a time in a scroll-capped list — reaching the bottom loads the next batch.",
			Payload: pushRows, build: func() gomukit.Widget { return tableLoadMore() },
		},
		{
			ID: "table-empty", Group: "Table", Label: "Empty state",
			Desc:    "Ships with no snapshot; push a result to fill it.",
			Payload: pushRows, build: func() gomukit.Widget { return tableEmpty() },
		},
		{
			ID: "table-visibility", Group: "Table", Label: "State-dependent actions",
			Desc:    "VisibleWhen: each row's menu carries only the actions its status admits.",
			Payload: pushRows, build: func() gomukit.Widget { return tableVisibility() },
		},
		{
			ID: "cards-default", Group: "CardList", Label: "Carousel",
			Desc:    "Paged card carousel with filter, sort and bulk archive.",
			Payload: pushRows, build: func() gomukit.Widget { return cardList() },
		},
		{
			ID: "cards-loadmore", Group: "CardList", Label: "Load more",
			Desc:    "24 records, six at a time — the strip grows instead of paging.",
			Payload: pushRows, build: func() gomukit.Widget { return cardListLoadMore() },
		},
		{
			ID: "cards-empty", Group: "CardList", Label: "Empty state",
			Desc:    "No records baked in — exercises the empty message.",
			Payload: pushRows, build: func() gomukit.Widget { return cardListEmpty() },
		},
		{
			ID: "cards-visibility", Group: "CardList", Label: "State-dependent actions",
			Desc:    "VisibleWhen on card buttons: an invited record is reminded, an active one suspended.",
			Payload: pushRows, build: func() gomukit.Widget { return cardListVisibility() },
		},
		{
			ID: "card-default", Group: "Card", Label: "Single record",
			Desc:    "One record rendered from rows[0], with a footer action.",
			Payload: pushRows, build: func() gomukit.Widget { return card() },
		},
		{
			ID: "card-sections", Group: "Card", Label: "All sections",
			Desc:    "Header with a button instead of a badge, body prose, footer note.",
			Payload: pushRows, build: func() gomukit.Widget { return cardSections() },
		},
		{
			ID: "card-empty", Group: "Card", Label: "Empty state",
			Desc:    "Waiting for a record; push a result to load one.",
			Payload: pushRows, build: func() gomukit.Widget { return cardEmpty() },
		},
		{
			ID: "card-inputs", Group: "Card", Label: "A card that asks",
			Desc:    "Content items rendering controls; what they hold travels with the footer button.",
			Payload: pushRows, build: func() gomukit.Widget { return cardInputs() },
		},
		{
			ID: "descriptions-types", Group: "Descriptions", Label: "All item types",
			Desc:    "Every item type in one list. Step the width down: the grid drops a column at a time and ends up stacked.",
			Payload: pushDetails, build: func() gomukit.Widget { return descriptionsTypes() },
		},
		{
			ID: "descriptions-dense", Group: "Descriptions", Label: "Narrower columns",
			Desc:    "The same list with --gomu-desc-min at 8rem, so it keeps more columns for longer.",
			Payload: pushDetails, build: func() gomukit.Widget { return descriptionsDense() },
		},
		{
			ID: "form-edit", Group: "Form", Label: "Edit record",
			Desc:    "Prefilled from a baked snapshot. Tick the field-error switch, then submit.",
			Payload: pushValues, build: func() gomukit.Widget { return form() },
		},
		{
			ID: "form-create", Group: "Form", Label: "All field types",
			Desc:    "Text, textarea, number, date, time, select, multiselect, checkbox, readonly.",
			Payload: pushErrors, build: func() gomukit.Widget { return formCreate() },
		},
		{
			ID: "form-layout", Group: "Form", Label: "Grouped fields",
			Desc:    "Two columns, three field sets, spans. Drag the width down: the grid drops to one column.",
			Payload: pushValues, build: func() gomukit.Widget { return formLayout() },
		},
		{
			ID: "form-columns", Group: "Form", Label: "Columns only",
			Desc:    "No groups: three columns and two spans. Past 46rem it drops to two, past 34rem to one.",
			Payload: pushValues, build: func() gomukit.Widget { return formColumns() },
		},
		{
			ID: "form-sets", Group: "Form", Label: "Field sets only",
			Desc:    "One column, three groups: plain, boxed, and one that overrides the form's columns.",
			Payload: pushValues, build: func() gomukit.Widget { return formSets() },
		},
		{
			ID: "menu-default", Group: "Menu", Label: "Launcher",
			Desc:    "Tiles with icons and badges; each tile fires a tools/call.",
			Payload: pushEmptyRows, build: func() gomukit.Widget { return menu() },
		},
		{
			ID: "menu-plain", Group: "Menu", Label: "Plain tiles",
			Desc:    "No icons, badges or descriptions — the minimum a Menu needs.",
			Payload: pushEmptyRows, build: func() gomukit.Widget { return menuPlain() },
		},
		{
			ID: "menu-prompt", Group: "Menu", Label: "Chat tiles",
			Desc:    "Prompt items post a ui/message user turn instead of calling the tool.",
			Payload: pushEmptyRows, build: func() gomukit.Widget { return menuPrompt() },
		},
		{
			ID: "confirm-danger", Group: "Confirm", Label: "Destructive",
			Desc:    "Details, side effects, and both guards: tick the box and type ada@example.com.",
			Payload: pushEffects, build: func() gomukit.Widget { return confirmDanger() },
		},
		{
			ID: "confirm-plain", Group: "Confirm", Label: "Plain question",
			Desc:    "No guards, no details — a prompt, two effects and two buttons.",
			Payload: pushEffects, build: func() gomukit.Widget { return confirmPlain() },
		},
		{
			ID: "confirm-runtime", Group: "Confirm", Label: "Runtime effects",
			Desc:    "Ships with no snapshot; push a result to fill the details and effects.",
			Payload: pushEffects, build: func() gomukit.Widget { return confirmRuntime() },
		},
		{
			ID: "choice-auto", Group: "Choice", Label: "One of several",
			Desc:    "Auto layout: drag the width past 34rem and the description moves into the side panel.",
			Payload: pushOptions, build: func() gomukit.Widget { return choiceAuto() },
		},
		{
			ID: "choice-stacked", Group: "Choice", Label: "Always stacked",
			Desc:    "The description stays under the option it belongs to, whatever the width.",
			Payload: pushOptions, build: func() gomukit.Widget { return choiceStacked() },
		},
		{
			ID: "choice-multi", Group: "Choice", Label: "Multiple, bounded",
			Desc:    "Pick 2 to 3 add-ons: the hint tracks the count and the rest disable at the cap.",
			Payload: pushOptions, build: func() gomukit.Widget { return choiceMulti() },
		},
		{
			ID: "choice-runtime", Group: "Choice", Label: "Runtime options",
			Desc:    "Ships with no options; push a result to load what is on offer.",
			Payload: pushOptions, build: func() gomukit.Widget { return choiceRuntime() },
		},
		{
			ID: "datepicker-single", Group: "Date picker", Label: "One date",
			Desc:    "A single date with presets, a bounded window and two blocked days.",
			Payload: pushDates, build: func() gomukit.Widget { return datePicker() },
		},
		{
			ID: "datepicker-range", Group: "Date picker", Label: "A range",
			Desc:    "Two months side by side, week numbers, and quick ranges beside the grid.",
			Payload: pushDates, build: func() gomukit.Widget { return datePickerRange() },
		},
		{
			ID: "datepicker-dropdowns", Group: "Date picker", Label: "Month and year",
			Desc:    "Caption dropdowns for a date far from today, with weekends blocked.",
			Payload: pushDates, build: func() gomukit.Widget { return datePickerDropdowns() },
		},
		{
			ID: "datepicker-inputs", Group: "Date picker", Label: "Asks for more than the date",
			Desc:    "Details that ask as well as state: guests, a bed choice and a late-arrival box travel with the dates.",
			Payload: pushDates, build: func() gomukit.Widget { return datePickerInputs() },
		},
		{
			ID: "datepicker-runtime", Group: "Date picker", Label: "Runtime availability",
			Desc:    "Ships with no selection; push a result to load the window and the days already taken.",
			Payload: pushDates, build: func() gomukit.Widget { return datePickerRuntime() },
		},
	}
}

// --- Date picker stories ---

func datePicker() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://harness/datepicker",
		Title:  "Delivery",
		Prompt: "When should we deliver order ORD-4471?",
		Body:   "The depot needs one working day's notice.",
		Calendar: &gomukit.Calendar{
			Min:      "2026-08-01",
			Max:      "2026-10-31",
			Disabled: []string{"2026-08-14", "2026-08-15"},
			Presets: []gomukit.DatePreset{
				{Label: "Today", Span: gomukit.SpanToday},
				{Label: "Tomorrow", Span: gomukit.SpanTomorrow},
			},
		},
		Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
			{Label: "Order", Key: "reference"},
			{Label: "Recipient", Key: "name"},
		}},
		Submit: gomukit.DateSubmit{
			Tool:           "schedule_delivery",
			Label:          "Book it",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "Booked.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Decide later", Message: "Nothing was booked."},
		InitialData: map[string]any{
			"rows": []map[string]any{{"id": 4471, "reference": "ORD-4471", "name": "Ada Lovelace"}},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

func datePickerRange() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://harness/datepicker-range",
		Title:  "Booking",
		Prompt: "Which nights should we hold the suite?",
		Mode:   gomukit.DateRange,
		Calendar: &gomukit.Calendar{
			Min:         "2026-08-01",
			Max:         "2026-12-31",
			Disabled:    []string{"2026-08-27", "2026-08-28", "2026-08-29"},
			WeekNumbers: true,
			Presets: []gomukit.DatePreset{
				{Label: "This week", Span: gomukit.SpanThisWeek},
				{Label: "Next 7 days", Span: gomukit.SpanNext7Days},
				{Label: "This month", Span: gomukit.SpanThisMonth},
				{Label: "Trade fair", Start: "2026-09-07", End: "2026-09-11"},
			},
		},
		Default:    "2026-08-20",
		DefaultEnd: "2026-08-23",
		Submit: gomukit.DateSubmit{
			Tool:           "hold_room",
			Label:          "Hold it",
			ValueArg:       "from",
			EndArg:         "until",
			SuccessMessage: "Held.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Cancel", Message: "Nothing was held."},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

func datePickerDropdowns() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://harness/datepicker-dropdowns",
		Prompt: "When does the contract start?",
		Body:   "Weekends are not working days.",
		Calendar: &gomukit.Calendar{
			Min:             "2020-01-01",
			Max:             "2030-12-31",
			DisableWeekends: true,
			MonthDropdowns:  true,
			WeekStart:       gomukit.WeekStartMonday,
			StartOn:         "2027-03-01",
		},
		Submit: gomukit.DateSubmit{Tool: "set_start_date", Label: "Set the date"},
		Cancel: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

// datePickerInputs is the picker asking the questions that come with the date:
// the answers ride along in the same call.
func datePickerInputs() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://harness/datepicker-inputs",
		Title:  "Booking",
		Prompt: "Which nights should we hold the suite?",
		Body:   "Rates are per night and include breakfast.",
		Mode:   gomukit.DateRange,
		Calendar: &gomukit.Calendar{
			Min:         "2026-08-01",
			Max:         "2026-12-31",
			WeekNumbers: true,
		},
		Default:    "2026-08-20",
		DefaultEnd: "2026-08-23",
		Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
			{Label: "Booking", Key: "reference"},
			{Label: "Guests", Key: "guests", Input: &gomukit.Input{
				Name:       "guests",
				Type:       gomukit.InputNumber,
				Default:    2,
				Required:   true,
				Validation: &gomukit.Validation{Min: num(1), Max: num(6), Step: num(1), Message: "Between one and six guests."},
			}},
			{Label: "Bed", Input: &gomukit.Input{
				Name:        "bed",
				Type:        gomukit.InputSelect,
				Placeholder: "Pick one",
				Options:     []gomukit.Option{{Value: "double", Label: "One double"}, {Value: "twin", Label: "Two singles"}},
			}},
			{Label: "Arriving after 22:00", Input: &gomukit.Input{Name: "late", Type: gomukit.InputCheckbox}},
			{Label: "Anything else?", Input: &gomukit.Input{Name: "notes", Placeholder: "Allergies, a cot, a late checkout…"}},
		}},
		Submit: gomukit.DateSubmit{
			Tool:           "hold_room",
			Label:          "Hold it",
			ValueArg:       "from",
			EndArg:         "until",
			SuccessMessage: "Held.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Cancel", Message: "Nothing was held."},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

func datePickerRuntime() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:      "ui://harness/datepicker-runtime",
		Title:    "Availability",
		Prompt:   "Which nights are you staying?",
		Mode:     gomukit.DateRange,
		Calendar: &gomukit.Calendar{WeekNumbers: true},
		Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
			{Label: "Booking", Key: "reference"},
		}},
		Submit: gomukit.DateSubmit{Tool: "hold_room", ValueArg: "from", EndArg: "until"},
		Cancel: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

// stories returns the catalog with Src derived from each ID.
func stories() []story {
	list := catalog()
	for i := range list {
		list[i].Src = "/story/" + list[i].ID
	}
	return list
}

// --- shared building blocks ---

// harnessRows is a small record set with the extra fields (email, website)
// the card widgets display; reused across stories.
func harnessRows() []map[string]any {
	return []map[string]any{
		{"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "balance": 1200.5, "createdAt": "2026-01-12T09:00:00Z", "status": "active", "website": "https://example.com/ada", "bio": "Wrote the first published algorithm; runs the analytical engine team."},
		{"id": 2, "name": "Grace Hopper", "email": "grace@example.com", "balance": 815, "createdAt": "2026-02-03T10:30:00Z", "status": "active", "website": "https://example.com/grace", "bio": "Compiler pioneer. Keeps the nightly build honest."},
		{"id": 3, "name": "Alan Turing", "email": "alan@example.com", "balance": 0, "createdAt": "2026-03-19T14:00:00Z", "status": "invited", "website": "", "bio": "Invited last week; has not signed in yet."},
		{"id": 4, "name": "Katherine Johnson", "email": "katherine@example.com", "balance": 233.1, "createdAt": "2026-04-01T08:15:00Z", "status": "active", "website": "https://example.com/katherine", "bio": "Checks every trajectory by hand before it ships."},
	}
}

// manyRows synthesizes n records for the long-list story.
func manyRows(n int) []map[string]any {
	given := []string{"Ada", "Grace", "Alan", "Katherine", "Barbara", "Edsger", "Margaret", "Donald"}
	family := []string{"Lovelace", "Hopper", "Turing", "Johnson", "Liskov", "Dijkstra", "Hamilton", "Knuth"}
	status := []string{"active", "invited", "archived"}

	rows := make([]map[string]any, 0, n)
	for i := range n {
		name := fmt.Sprintf("%s %s", given[i%len(given)], family[(i/len(given)+i)%len(family)])
		rows = append(rows, map[string]any{
			"id":        i + 1,
			"name":      name,
			"email":     fmt.Sprintf("user%02d@example.com", i+1),
			"balance":   float64((i*317)%2400) + 0.5,
			"createdAt": fmt.Sprintf("2026-%02d-%02dT09:00:00Z", i%12+1, i%27+1),
			"status":    status[i%len(status)],
			"website":   fmt.Sprintf("https://example.com/user%02d", i+1),
		})
	}
	return rows
}

func statusBadge() gomukit.Column {
	return gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
		"active": gomukit.BadgeSuccess, "invited": gomukit.BadgeInfo, "archived": gomukit.BadgeNeutral,
	})
}

func deleteAction() gomukit.Action {
	return gomukit.Action{Label: "Delete", Tool: "delete_user", Variant: gomukit.VariantDanger,
		Confirm: "Really?", Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}}
}

// rowActions is a full actions column: enough entries that the row menu has
// something to show, including a link and a confirmed destructive action.
func rowActions() gomukit.Column {
	return gomukit.ActionsColumn(
		gomukit.Action{Label: "Open profile", Kind: gomukit.ActionLink, HrefKey: "website"},
		gomukit.Action{Label: "Send invite", Tool: "invite_user",
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
		deleteAction(),
	)
}

// stateActions are the actions of a roster whose buttons depend on the record:
// the two directions of the invite/suspend switch never appear together, and
// the reminder belongs to the records still waiting. The predicates read the
// raw "status" field, never the badge's words.
func stateActions() []gomukit.Action {
	return []gomukit.Action{
		{Label: "Send reminder", Tool: "invite_user", VisibleWhen: gomukit.RowIs("status", "invited"),
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
		{Label: "Suspend", Tool: "suspend_user", VisibleWhen: gomukit.RowIs("status", "active"),
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
		{Label: "Reinstate", Tool: "reinstate_user", VisibleWhen: gomukit.RowIs("status", "archived"),
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
		// The complement: everything but a closed record can still be edited,
		// which leaves a closed one with no actions and so no menu trigger.
		{Label: "Edit", Tool: "edit_user", VisibleWhen: gomukit.RowNot(gomukit.RowIs("status", "closed")),
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
	}
}

func archiveBulk() *gomukit.SelectionConfig {
	return &gomukit.SelectionConfig{Bulk: []gomukit.Action{
		{Label: "Archive", Tool: "archive_users", Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")}},
		{Label: "Delete", Tool: "delete_users", Variant: gomukit.VariantDanger, Confirm: "Delete them?",
			Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")}},
	}}
}

// demoBrand exercises the inline-SVG logo path.
func demoBrand() *gomukit.Brand {
	return &gomukit.Brand{
		Name:    "Acme",
		URL:     "https://example.com",
		LogoSVG: `<svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="8" r="7"/></svg>`,
	}
}

// The stories run on the library's own palette (DESIGN.md) rather than an
// example accent, so the harness shows what a host gets out of the box.
func demoTheme() *theme.Theme { return nil }

func num(v float64) *float64 { return &v }

// --- Table stories ---

func table() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://harness/table",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
			statusBadge(),
			rowActions(),
		},
		PageSize:    3,
		PageSizes:   []int{3, 5, 10},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// tableVisibility shows the same actions column over records in different
// states: every row's menu holds only what that row's status admits.
func tableVisibility() *gomukit.Table {
	t := table()
	t.URI = "ui://harness/table-visibility"
	t.Title = "Roster"
	t.Columns = []gomukit.Column{
		gomukit.Text("name", "Name"),
		gomukit.Text("email", "Email"),
		statusBadge(),
		gomukit.ActionsColumn(stateActions()...),
	}
	t.PageSize = 0
	t.PageSizes = nil
	t.Selection = nil
	t.InitialData = map[string]any{"rows": mixedStatusRows()}
	return t
}

// mixedStatusRows carries one record per status, so a state-dependent action
// list can be read against every branch at once — including a closed record
// no action applies to, which is drawn without a menu trigger.
func mixedStatusRows() []map[string]any {
	rows := harnessRows()
	rows[3]["status"] = "archived"
	return append(rows, map[string]any{
		"id": 5, "name": "Barbara Liskov", "email": "barbara@example.com", "balance": 0,
		"createdAt": "2026-05-04T11:00:00Z", "status": "closed", "website": "",
		"bio": "Account closed; nothing left to do here.",
	})
}

func tablePlain() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://harness/table-plain",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
		},
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func tableLong() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://harness/table-long",
		Title: "Directory",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
			statusBadge(),
			gomukit.ActionsColumn(deleteAction()),
		},
		PageSize:    8,
		PageSizes:   []int{8, 16, 24},
		DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": manyRows(24)},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// tableLoadMore exercises the growing list: no pagination bar, a "Load more"
// bar instead, and a scroll-capped rows area that loads on scroll.
func tableLoadMore() *gomukit.Table {
	t := tableLong()
	t.URI = "ui://harness/table-loadmore"
	t.Title = "Directory"
	t.PageSizes = nil
	t.LoadMore = true
	t.MaxHeight = "20rem"
	return t
}

func tableEmpty() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://harness/table-empty",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
			statusBadge(),
		},
		Filterable:  true,
		Empty:       gomukit.EmptyState{Title: "No users yet", Body: "Push a tool-result from the panel to fill the table."},
		InitialData: map[string]any{"rows": []map[string]any{}},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// --- Card stories ---

// cardTemplate is shared by the single Card and the CardList.
func cardTemplate() gomukit.CardTemplate {
	return gomukit.CardTemplate{
		Header: gomukit.CardHeader{
			TitleKey:       "name",
			DescriptionKey: "email",
			Badge:          statusBadge(),
		},
		Content: gomukit.CardContent{
			Items: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
				{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
				{Label: "Joined", Key: "createdAt", Type: gomukit.ColDate, Format: "relative"},
				{Label: "Website", Key: "website", Type: gomukit.ColLink,
					Link: &gomukit.LinkSpec{HrefKey: "website"}},
			}},
		},
		Footer: gomukit.CardFooter{Actions: []gomukit.Action{deleteAction()}},
	}
}

func cardList() *gomukit.CardList {
	return &gomukit.CardList{
		URI:         "ui://harness/cards",
		Title:       "Users",
		Template:    cardTemplate(),
		PageSize:    3,
		PageSizes:   []int{3, 6, 12},
		DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// cardListLoadMore exercises the growing strip: no pagination bar, a "Load
// more" tile at the end of the run instead.
func cardListLoadMore() *gomukit.CardList {
	l := cardList()
	l.URI = "ui://harness/cards-loadmore"
	l.Title = "Directory"
	l.PageSize = 6
	l.PageSizes = nil
	l.LoadMore = true
	l.InitialData = map[string]any{"rows": manyRows(24)}
	return l
}

// cardListVisibility gives the cards the same state-dependent buttons the
// roster table puts in its menus — and a record no action applies to keeps no
// action bar at all.
func cardListVisibility() *gomukit.CardList {
	l := cardList()
	l.URI = "ui://harness/cards-visibility"
	l.Title = "Roster"
	l.PageSize = 0
	l.PageSizes = nil
	l.Selection = nil
	l.Template.Footer.Actions = stateActions()
	l.InitialData = map[string]any{"rows": mixedStatusRows()}
	return l
}

func cardListEmpty() *gomukit.CardList {
	l := cardList()
	l.URI = "ui://harness/cards-empty"
	l.Empty = gomukit.EmptyState{Title: "Nothing to show", Body: "Push a tool-result with rows to populate the list."}
	l.InitialData = map[string]any{"rows": []map[string]any{}}
	return l
}

func card() *gomukit.Card {
	return &gomukit.Card{
		URI:         "ui://harness/card",
		Title:       "User",
		Template:    cardTemplate(),
		Empty:       gomukit.EmptyState{Title: "No user", Body: "Push a tool-result to load one."},
		InitialData: map[string]any{"rows": harnessRows()[:1]},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// cardSections fills every section slot: a header whose end slot holds a
// button rather than a badge, prose above the detail list, and a footer note
// beside the actions.
func cardSections() *gomukit.Card {
	c := card()
	c.URI = "ui://harness/card-sections"
	c.Template.Header.Badge = gomukit.Column{}
	c.Template.Header.Action = &gomukit.Action{
		Label: "Open profile", Kind: gomukit.ActionLink, HrefKey: "website",
	}
	c.Template.Content.TextKey = "bio"
	c.Template.Footer.Text = "Balances update hourly."
	return c
}

func cardEmpty() *gomukit.Card {
	c := card()
	c.URI = "ui://harness/card-empty"
	c.InitialData = map[string]any{"rows": []map[string]any{}}
	return c
}

// --- Form stories ---

func form() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://harness/form",
		Title: "Edit user",
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden, Default: "1"},
			{Name: "name", Label: "Name", Required: true},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gomukit.FSelect, Options: []gomukit.Option{gomukit.Opt("user"), gomukit.Opt("admin")}},
			{Name: "notify", Label: "Send notifications", Type: gomukit.FCheckbox, Default: true},
		},
		Submit:      gomukit.SubmitSpec{Tool: "save_user", SuccessMessage: "Saved."},
		Cancel:      &gomukit.CancelSpec{},
		InitialData: map[string]any{"values": map[string]any{"name": "Ada Lovelace", "email": "ada@example.com"}},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func formCreate() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://harness/form-create",
		Title: "New user",
		Fields: []gomukit.Field{
			{Name: "account", Label: "Account", Type: gomukit.FReadonly, Default: "acme-eu"},
			{Name: "name", Label: "Name", Required: true, Placeholder: "Ada Lovelace",
				Description: "Shown everywhere the record appears."},
			{Name: "email", Label: "Email", Required: true, Placeholder: "ada@example.com",
				Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gomukit.FSelect, Default: "user",
				Options: []gomukit.Option{gomukit.Opt("user"), gomukit.Opt("admin"), gomukit.Opt("auditor")}},
			{Name: "scopes", Label: "Scopes", Type: gomukit.FMultiSelect,
				Options: []gomukit.Option{gomukit.Opt("read"), gomukit.Opt("write"), gomukit.Opt("billing")},
				Default: []string{"read"}},
			{Name: "seats", Label: "Seats", Type: gomukit.FNumber, Default: "3",
				Validation: &gomukit.Validation{Min: num(1), Max: num(50), Step: num(1)}},
			{Name: "startsOn", Label: "Starts on", Type: gomukit.FDate, Default: "2026-08-01",
				Calendar: &gomukit.Calendar{Min: "2026-01-01", MonthDropdowns: true, FromYear: 2026, ToYear: 2030}},
			{Name: "trialFrom", Label: "Trial period", Type: gomukit.FDateRange, EndName: "trialTo",
				Description: "The dates the free trial covers.",
				Calendar: &gomukit.Calendar{
					Min: "2026-01-01",
					Presets: []gomukit.DatePreset{
						{Label: "Next 7 days", Span: gomukit.SpanNext7Days},
						{Label: "Next 30 days", Span: gomukit.SpanNext30Days},
					},
				}},
			{Name: "digestAt", Label: "Daily digest", Type: gomukit.FTime, Default: "09:00"},
			{Name: "notes", Label: "Notes", Type: gomukit.FTextarea, Rows: 3,
				Placeholder: "Anything the team should know"},
			{Name: "notify", Label: "Send notifications", Type: gomukit.FCheckbox, Default: true},
		},
		Submit:      gomukit.SubmitSpec{Tool: "save_user", Label: "Create user", SuccessMessage: "User created."},
		Cancel:      &gomukit.CancelSpec{},
		InitialData: map[string]any{},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func formLayout() *gomukit.Form {
	return &gomukit.Form{
		URI:     "ui://harness/form-layout",
		Title:   "New employee",
		Columns: 2,
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden, Default: "0"},
			{Name: "account", Label: "Workspace", Type: gomukit.FReadonly, Default: "acme-eu", Span: 2},
		},
		FieldSets: []gomukit.FieldSet{
			{
				Title:       "Person",
				Description: "How the record reads everywhere it appears.",
				Fields: []gomukit.Field{
					{Name: "first", Label: "First name", Required: true, Placeholder: "Ada"},
					{Name: "last", Label: "Last name", Required: true, Placeholder: "Lovelace"},
					{Name: "email", Label: "Email", Required: true, Span: 2, Placeholder: "ada@example.com",
						Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
					{Name: "phone", Label: "Phone", Placeholder: "+30 …"},
					{Name: "birthday", Label: "Date of birth", Type: gomukit.FDate,
						Calendar: &gomukit.Calendar{Max: "2026-01-01", MonthDropdowns: true, FromYear: 1950, ToYear: 2026}},
				},
			},
			{
				Title:       "Employment",
				Description: "What the contract says.",
				Boxed:       true,
				Fields: []gomukit.Field{
					{Name: "role", Label: "Role", Type: gomukit.FSelect, Default: "engineer",
						Options: []gomukit.Option{gomukit.Opt("engineer"), gomukit.Opt("designer"), gomukit.Opt("support")}},
					{Name: "seats", Label: "Weekly hours", Type: gomukit.FNumber, Default: "40",
						Validation: &gomukit.Validation{Min: num(1), Max: num(40), Step: num(1)}},
					{Name: "startsOn", Label: "Contract period", Type: gomukit.FDateRange, EndName: "endsOn", Span: 2,
						Description: "Leave the end open for a permanent contract.",
						Calendar:    &gomukit.Calendar{Min: "2026-01-01"}},
				},
			},
			{
				Title:   "Notes",
				Columns: 1,
				Fields: []gomukit.Field{
					{Name: "notes", Label: "Anything the team should know", Type: gomukit.FTextarea, Rows: 3},
					{Name: "notify", Label: "Announce in the team channel", Type: gomukit.FCheckbox, Default: true},
				},
			},
		},
		Submit:      gomukit.SubmitSpec{Tool: "save_user", Label: "Create employee", SuccessMessage: "Employee created."},
		Cancel:      &gomukit.CancelSpec{},
		InitialData: map[string]any{},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// formColumns is the layout system with no groups at all: one grid, three
// columns, two fields taking more than one of them.
func formColumns() *gomukit.Form {
	return &gomukit.Form{
		URI:     "ui://harness/form-columns",
		Title:   "Shipping address",
		Columns: 3,
		Fields: []gomukit.Field{
			{Name: "recipient", Label: "Recipient", Required: true, Span: 2, Placeholder: "Ada Lovelace"},
			{Name: "phone", Label: "Phone", Placeholder: "+30 …"},
			{Name: "street", Label: "Street and number", Required: true, Span: 3, Placeholder: "Ermou 12"},
			{Name: "postcode", Label: "Post code", Required: true, Placeholder: "10563"},
			{Name: "city", Label: "City", Required: true, Placeholder: "Athens"},
			{Name: "country", Label: "Country", Type: gomukit.FSelect, Default: "GR",
				Options: []gomukit.Option{
					{Value: "GR", Label: "Greece"},
					{Value: "DE", Label: "Germany"},
					{Value: "NL", Label: "Netherlands"},
				}},
			{Name: "notes", Label: "Delivery notes", Type: gomukit.FTextarea, Rows: 2, Span: 3,
				Description: "Gate codes, floor, where to leave the parcel."},
		},
		Submit:      gomukit.SubmitSpec{Tool: "save_user", Label: "Save address", SuccessMessage: "Address saved."},
		Cancel:      &gomukit.CancelSpec{},
		InitialData: map[string]any{},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// formSets is the grouping with no multi-column layout: the long form read as
// three blocks rather than one column of twelve controls.
func formSets() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://harness/form-sets",
		Title: "Account settings",
		FieldSets: []gomukit.FieldSet{
			{
				Title:       "Profile",
				Description: "What other people see.",
				Fields: []gomukit.Field{
					{Name: "displayName", Label: "Display name", Required: true, Default: "Ada Lovelace"},
					{Name: "bio", Label: "Bio", Type: gomukit.FTextarea, Rows: 2,
						Default: "Mathematician. Writes about engines."},
				},
			},
			{
				Title:       "Notifications",
				Description: "Nothing here changes what other people see.",
				Boxed:       true,
				Fields: []gomukit.Field{
					{Name: "digest", Label: "Email digest", Type: gomukit.FSelect, Default: "daily",
						Options: []gomukit.Option{gomukit.Opt("off"), gomukit.Opt("daily"), gomukit.Opt("weekly")}},
					{Name: "digestAt", Label: "Send at", Type: gomukit.FTime, Default: "09:00"},
					{Name: "mentions", Label: "Email me when I am mentioned", Type: gomukit.FCheckbox, Default: true},
				},
			},
			{
				// This group overrides the form's single column.
				Title:   "Danger zone",
				Columns: 2,
				Fields: []gomukit.Field{
					// A select always holds one of its options, so "nobody" is an
					// option rather than a placeholder the control never shows.
					{Name: "successor", Label: "Hand the workspace to", Type: gomukit.FSelect,
						Options: []gomukit.Option{
							{Value: "", Label: "Nobody"},
							gomukit.Opt("grace@example.com"),
							gomukit.Opt("alan@example.com"),
						}},
					{Name: "closeOn", Label: "Close the account on", Type: gomukit.FDate,
						Calendar: &gomukit.Calendar{Min: "2026-08-01"}},
				},
			},
		},
		Submit:      gomukit.SubmitSpec{Tool: "save_user", SuccessMessage: "Settings saved."},
		Cancel:      &gomukit.CancelSpec{},
		InitialData: map[string]any{},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// --- Menu stories ---

func menu() *gomukit.Menu {
	icon := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 10v10"/></svg>`
	pen := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>`
	box := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1zM10 12h4"/></svg>`
	return &gomukit.Menu{
		URI:   "ui://harness/menu",
		Title: "Harness app",
		Intro: "Each tile fires a tools/call; watch the traffic pane.",
		Items: []gomukit.MenuItem{
			{
				Tool: "list_users", Label: "User table",
				Description:  "Sortable, filterable directory.",
				IconSVG:      icon,
				Badge:        "read",
				BadgeVariant: gomukit.BadgeInfo,
			},
			{
				Tool: "edit_user", Args: map[string]any{"id": 1},
				Label:        "Edit Ada",
				Description:  "Open the edit form for one user.",
				IconSVG:      pen,
				Badge:        "write",
				BadgeVariant: gomukit.BadgeWarning,
			},
			{
				Tool: "archive_users", Label: "Archive all",
				Description:  "Bulk-archive every user.",
				IconSVG:      box,
				Badge:        "danger",
				BadgeVariant: gomukit.BadgeDanger,
			},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

func menuPlain() *gomukit.Menu {
	return &gomukit.Menu{
		URI:   "ui://harness/menu-plain",
		Title: "Harness app",
		Items: []gomukit.MenuItem{
			{Tool: "list_users", Label: "Users"},
			{Tool: "save_user", Label: "New user"},
			// No label: the tile falls back to the tool name.
			{Tool: "archive_users"},
		},
		Brand: demoBrand(),
	}
}

// menuPrompt mixes both launch paths: the first tile calls its tool the usual
// way, the other two hand the request to the host's chat. Hosts that answer a
// view-initiated tools/call out of band open nothing for the first tile and a
// real turn for the rest, which is the difference the story exists to show.
func menuPrompt() *gomukit.Menu {
	return &gomukit.Menu{
		URI:   "ui://harness/menu-prompt",
		Title: "Harness app",
		Intro: "The first tile calls its tool; the others ask the chat to open it.",
		Items: []gomukit.MenuItem{
			{
				Tool: "list_users", Label: "Users (direct)",
				Description:  "Plain tools/call — needs a host that opens the bound widget.",
				Badge:        "call",
				BadgeVariant: gomukit.BadgeInfo,
			},
			{
				Tool: "list_users", Label: "Users (chat)",
				Description:  "Posts a user turn and lets the model open the table.",
				Prompt:       "Show me the user directory",
				Badge:        "chat",
				BadgeVariant: gomukit.BadgeNeutral,
			},
			{
				Tool: "edit_user", Label: "Edit Ada (chat)",
				Description:  "The model picks the arguments, so none are declared here.",
				Prompt:       "Open the edit form for Ada Lovelace",
				Badge:        "chat",
				BadgeVariant: gomukit.BadgeNeutral,
			},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

// cardInputs is the card asking about the record it shows: the controls sit in
// the content list, and the footer button carries what they hold.
func cardInputs() *gomukit.Card {
	return &gomukit.Card{
		URI:         "ui://harness/card-inputs",
		Title:       "Adjust balance",
		InitialData: map[string]any{"rows": harnessRows()[:1]},
		Template: gomukit.CardTemplate{
			Header: gomukit.CardHeader{
				TitleKey:       "name",
				DescriptionKey: "email",
				Badge: gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
					"active":   gomukit.BadgeSuccess,
					"invited":  gomukit.BadgeInfo,
					"archived": gomukit.BadgeNeutral,
				}),
			},
			Content: gomukit.CardContent{
				Items: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
					{Label: "Current balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
					{Label: "New balance", Key: "balance", Input: &gomukit.Input{
						Name:       "amount",
						Type:       gomukit.InputNumber,
						Required:   true,
						Validation: &gomukit.Validation{Min: num(0), Step: num(0.01), Message: "A balance cannot go below zero."},
					}},
					{Label: "Reason", Input: &gomukit.Input{
						Name:        "reason",
						Type:        gomukit.InputSelect,
						Placeholder: "Why?",
						Options: []gomukit.Option{
							{Value: "refund", Label: "Refund"},
							{Value: "credit", Label: "Goodwill credit"},
							{Value: "correction", Label: "Correction"},
						},
					}},
					{Label: "Tell the customer", Input: &gomukit.Input{Name: "notify", Type: gomukit.InputCheckbox, Default: true}},
				}},
			},
			Footer: gomukit.CardFooter{
				Text: "The adjustment is logged against your account.",
				Actions: []gomukit.Action{
					{Label: "Apply", Tool: "set_balance", Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}, Variant: gomukit.VariantPrimary},
				},
			},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

// --- Descriptions stories ---

// detailsRow is the record behind the Descriptions stories: one field per item
// type. It has no "owner" field, so the item reading that key demonstrates the
// missing-value dash.
func detailsRow() map[string]any {
	return map[string]any{
		"id": 2, "name": "Grace Hopper", "email": "grace@example.com",
		"plan": "enterprise", "balance": 4820.40, "seats": 48,
		"utilization": 0.71, "renewsAt": "2027-01-31T00:00:00Z",
		"createdAt": "2024-11-02T09:00:00Z", "status": "active",
		"website": "https://example.com/grace",
	}
}

// detailItems exercises every item type the block supports: the five typed,
// record-bound kinds, a fixed authored value, and a key the record does not
// carry.
func detailItems() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "Account", Key: "name"},
		{Label: "Email", Key: "email"},
		{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: map[string]gomukit.BadgeVariant{
			"active":   gomukit.BadgeSuccess,
			"invited":  gomukit.BadgeInfo,
			"archived": gomukit.BadgeNeutral,
		}},
		// No Align on the numbers: the value sits under its label here, so
		// pushing it to the end of the cell would only strand it.
		{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
		{Label: "Seats", Key: "seats", Type: gomukit.ColNumber, Format: "int"},
		{Label: "Utilization", Key: "utilization", Type: gomukit.ColNumber, Format: "percent"},
		{Label: "Customer since", Key: "createdAt", Type: gomukit.ColDate, Format: "date"},
		{Label: "Renews", Key: "renewsAt", Type: gomukit.ColDate, Format: "relative"},
		{Label: "Console", Type: gomukit.ColLink,
			Link: &gomukit.LinkSpec{HrefKey: "website", Text: "Open console"}},
		// Authored in Go: the same for every record, so no field carries it.
		{Label: "Region", Text: "eu-central-1"},
		// Nothing in the record answers to this key — the value renders as a
		// dash rather than disappearing.
		{Label: "Owner", Key: "owner"},
	}}
}

// descriptionsTypes is the block on its own: a card with nothing but a title
// and the detail list, so the grid is all there is to look at.
func descriptionsTypes() *gomukit.Card {
	return &gomukit.Card{
		URI:   "ui://harness/descriptions",
		Title: "Account details",
		Template: gomukit.CardTemplate{
			Header:  gomukit.CardHeader{TitleKey: "name", DescriptionKey: "plan"},
			Content: gomukit.CardContent{Items: detailItems()},
		},
		Empty:       gomukit.EmptyState{Title: "No record", Body: "Push a tool-result to load one."},
		InitialData: map[string]any{"rows": []map[string]any{detailsRow()}},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// descriptionsDense lowers the one knob the block has: how narrow an item may
// get before the grid drops a column.
func descriptionsDense() *gomukit.Card {
	c := descriptionsTypes()
	c.URI = "ui://harness/descriptions-dense"
	c.Theme = &theme.Theme{
		ColorPrimary: "#7c3aed",
		Extra:        map[string]string{"--gomu-desc-min": "8rem"},
	}
	return c
}

// --- Confirm stories ---

// confirmDetails is the record summary the confirmation stories show: who the
// operation is about, in the same typed cells a table would use.
func confirmDetails() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "User", Key: "name"},
		{Label: "Email", Key: "email"},
		{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
		{Label: "Member since", Key: "createdAt", Type: gomukit.ColDate, Format: "date"},
		{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: map[string]gomukit.BadgeVariant{
			"active":   gomukit.BadgeSuccess,
			"invited":  gomukit.BadgeInfo,
			"archived": gomukit.BadgeNeutral,
		}},
		{Label: "Profile", Type: gomukit.ColLink, Link: &gomukit.LinkSpec{HrefKey: "website", Text: "Open profile"}},
		{Label: "Region", Text: "eu-central-1"},
	}}
}

func confirmDanger() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:      "ui://harness/confirm",
		Title:    "Delete user",
		Prompt:   "Delete Ada Lovelace?",
		Body:     "The account and everything attached to it is removed for good.",
		Severity: gomukit.BadgeDanger,
		Details:  confirmDetails(),
		Effects: []gomukit.Effect{
			{Text: "Removes the account", Detail: "Sign-in stops working immediately.", Severity: gomukit.BadgeDanger},
			{Text: "Deletes audit records", Value: "128", Severity: gomukit.BadgeWarning},
			{Text: "Frees the seat", Value: "1 seat", Severity: gomukit.BadgeSuccess},
		},
		Acknowledge:   "I understand this cannot be undone.",
		TypeToConfirm: "ada@example.com",
		Accept: gomukit.AcceptSpec{
			Tool:           "delete_user",
			Label:          "Delete user",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "User deleted.",
		},
		Reject:      &gomukit.RejectSpec{Label: "Keep user", Message: "Nothing was deleted."},
		InitialData: map[string]any{"rows": harnessRows()[:1]},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func confirmPlain() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:      "ui://harness/confirm-plain",
		Prompt:   "Archive 4 users?",
		Body:     "Archived users keep their data but cannot sign in.",
		Severity: gomukit.BadgeWarning,
		Effects: []gomukit.Effect{
			{Text: "Revokes active sessions", Value: "4", Severity: gomukit.BadgeWarning},
			{Text: "Keeps every record", Severity: gomukit.BadgeNeutral},
		},
		Accept: gomukit.AcceptSpec{Tool: "archive_users", Label: "Archive", SuccessMessage: "Users archived."},
		Reject: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

// confirmRuntime authors nothing but the question: the record and the
// consequences arrive from the tool that opens it, which is how a real server
// reports what this particular call will cost.
func confirmRuntime() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:     "ui://harness/confirm-runtime",
		Title:   "Delete user",
		Prompt:  "Delete this user?",
		Body:    "Push a tool result to load the record and its side effects.",
		Details: confirmDetails(),
		Accept: gomukit.AcceptSpec{
			Tool:           "delete_user",
			Label:          "Delete",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "User deleted.",
		},
		Reject: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

// --- Choice stories ---

// shippingOptions is the offer the choice stories present: three ways to send
// one parcel, each carrying its own record so the prices and dates are
// formatted for the host's locale rather than baked into strings.
func shippingOptions() []gomukit.ChoiceOption {
	price := gomukit.DescriptionItem{Label: "Price", Key: "price", Type: gomukit.ColNumber, Format: "currency:EUR"}
	arrives := gomukit.DescriptionItem{Label: "Arrives", Key: "eta", Type: gomukit.ColDate, Format: "date"}
	return []gomukit.ChoiceOption{
		{
			Value:   "standard",
			Label:   "Standard",
			Summary: "3-5 business days",
			Body:    "Handed to the postal service tonight and tracked as far as the local depot.",
			Bullets: []string{"Tracked to the depot", "No signature on delivery", "Insured to EUR 50"},
			Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{price, arrives}},
			Data:    map[string]any{"price": 4.9, "eta": "2026-08-03T10:00:00Z"},
			Default: true,
		},
		{
			Value:        "express",
			Label:        "Express",
			Summary:      "next business day, before 12:00",
			Body:         "Collected by courier this afternoon and delivered to the door tomorrow morning.",
			Bullets:      []string{"Tracked end to end", "Signature required", "Insured to EUR 500"},
			Details:      gomukit.Descriptions{Items: []gomukit.DescriptionItem{price, arrives}},
			Data:         map[string]any{"price": 14.9, "eta": "2026-07-28T12:00:00Z"},
			Badge:        "fastest",
			BadgeVariant: gomukit.BadgeSuccess,
		},
		{
			Value:    "pickup",
			Label:    "Depot pickup",
			Summary:  "no depot near this address",
			Body:     "The nearest depot is 40 km away, so this address cannot use pickup.",
			Disabled: true,
		},
	}
}

func choiceDetails() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "Order", Key: "reference"},
		{Label: "Recipient", Key: "name"},
		{Label: "Destination", Text: "Berlin, DE"},
	}}
}

func choiceAuto() *gomukit.Choice {
	return &gomukit.Choice{
		URI:     "ui://harness/choice",
		Title:   "Shipping",
		Prompt:  "How should we ship order ORD-4471?",
		Body:    "The parcel is packed and leaves the warehouse today either way.",
		Details: choiceDetails(),
		Options: shippingOptions(),
		Submit: gomukit.ChoiceSubmit{
			Tool:           "ship_order",
			Label:          "Ship it",
			ValueArg:       "method",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "On its way.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Decide later", Message: "Nothing was shipped."},
		InitialData: map[string]any{
			"rows": []map[string]any{{"id": 4471, "reference": "ORD-4471", "name": "Ada Lovelace"}},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

func choiceStacked() *gomukit.Choice {
	c := choiceAuto()
	c.URI = "ui://harness/choice-stacked"
	c.Layout = gomukit.ChoiceStacked
	c.Details = gomukit.Descriptions{}
	return c
}

func choiceMulti() *gomukit.Choice {
	return &gomukit.Choice{
		URI:      "ui://harness/choice-multi",
		Title:    "Add-ons",
		Prompt:   "Which extras should this shipment carry?",
		Body:     "Choose two or three; they are billed with the shipping cost.",
		Layout:   gomukit.ChoiceSplit,
		Multiple: true,
		Min:      2,
		Max:      3,
		Options: []gomukit.ChoiceOption{
			{
				Value: "insurance", Label: "Extra insurance", Summary: "up to EUR 5,000",
				Body:    "Covers the declared value against loss and damage in transit.",
				Bullets: []string{"Claims within 30 days", "Proof of value required"},
				Default: true,
			},
			{
				Value: "signature", Label: "Signature on delivery", Summary: "hand to the recipient",
				Body: "The courier hands the parcel over in person and records the signature.",
			},
			{
				Value: "saturday", Label: "Saturday delivery", Summary: "weekend slot",
				Body:         "Delivered on Saturday morning instead of the next business day.",
				Badge:        "surcharge",
				BadgeVariant: gomukit.BadgeWarning,
			},
			{
				Value: "carbon", Label: "Carbon offset", Summary: "adds EUR 0.40",
				Body: "Buys certified offsets for the leg between the depot and the door.",
			},
		},
		Submit: gomukit.ChoiceSubmit{Tool: "add_extras", Label: "Add extras", ValueArg: "extras", SuccessMessage: "Extras added."},
		Cancel: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}

// choiceRuntime authors the question and nothing else: what is on offer comes
// from the tool that opens it, which is how a server that prices shipping at
// call time would do it.
func choiceRuntime() *gomukit.Choice {
	return &gomukit.Choice{
		URI:     "ui://harness/choice-runtime",
		Title:   "Shipping",
		Prompt:  "How should we ship this order?",
		Body:    "Push a tool result to load the order and the options on offer.",
		Details: choiceDetails(),
		Submit: gomukit.ChoiceSubmit{
			Tool:           "ship_order",
			ValueArg:       "method",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "On its way.",
		},
		Cancel: &gomukit.RejectSpec{},
		Brand:  demoBrand(),
		Theme:  demoTheme(),
	}
}
