package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gomukit"
	"github.com/techthos/gomukit/gosdk"
	"github.com/techthos/gomukit/theme"
	"github.com/techthos/gomukit/uispec"
)

// The gallery is the catalog half of this server: one tool per widget
// variant, each bound to a resource of its own, so an inspector can open any
// rendering the library can produce without going through the app. It mirrors
// the story list in examples/harness, with the fake host replaced by a real
// MCP conversation.
//
// Gallery data is canned and stateless. Actions fired from a gallery widget
// land on the sandbox_* tools, which compute an answer from the same fixture
// instead of touching the scenario store.

// preview is one entry in the catalog.
type preview struct {
	Tool   string
	Group  string
	Label  string
	Desc   string
	Icon   string
	Badge  string
	Widget gomukit.Widget
	// Data is the structuredContent the tool answers with. Nil means the
	// widget needs nothing at runtime: it was fully authored in Go.
	Data func() previewOut
}

// previewOut carries whichever runtime keys a variant needs. Gallery tools
// share one output shape so the catalog stays a table rather than a tool per
// signature.
//
// Rows is a pointer because the runtime reads key presence, not emptiness: a
// result with no "rows" key leaves the widget's data alone, while "rows": []
// clears it. The empty-state variants need the second, which an omitempty
// slice cannot express.
type previewOut struct {
	Rows *[]map[string]any `json:"rows,omitempty"`
	// Records is the same data under a different key, for the variant that
	// renames the contract with RowsKey.
	Records *[]map[string]any `json:"records,omitempty"`
	Values  map[string]any    `json:"values,omitempty"`
	Effects []map[string]any  `json:"effects,omitempty"`
	Options []map[string]any  `json:"options,omitempty"`
	// Value is what a DatePicker reads: the selection, and the window it may
	// move in.
	Value map[string]any `json:"value,omitempty"`
}

// --- fixture data ---

// galleryRows is the record set behind the gallery. It carries the extra
// fields the card and detail variants show.
func galleryRows() []map[string]any {
	return []map[string]any{
		{"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "company": "Analytical Engines",
			"balance": 1200.5, "seats": 48, "utilization": 0.71, "createdAt": "2024-11-02T09:00:00Z",
			"renewsAt": "2027-01-31T00:00:00Z", "status": "active", "website": "https://example.com/ada",
			"bio": "Wrote the first published algorithm; runs the analytical engine team."},
		{"id": 2, "name": "Grace Hopper", "email": "grace@example.com", "company": "Compiler Works",
			"balance": 815, "seats": 12, "utilization": 0.44, "createdAt": "2025-02-03T10:30:00Z",
			"renewsAt": "2026-11-30T00:00:00Z", "status": "active", "website": "https://example.com/grace",
			"bio": "Compiler pioneer. Keeps the nightly build honest."},
		{"id": 3, "name": "Alan Turing", "email": "alan@example.com", "company": "Bletchley Labs",
			"balance": 0, "seats": 3, "utilization": 0, "createdAt": "2026-03-19T14:00:00Z",
			"renewsAt": "2027-03-19T00:00:00Z", "status": "invited", "website": "",
			"bio": "Invited last week; has not signed in yet."},
		{"id": 4, "name": "Katherine Johnson", "email": "katherine@example.com", "company": "Orbital Math",
			"balance": 233.1, "seats": 9, "utilization": 0.88, "createdAt": "2025-04-01T08:15:00Z",
			"renewsAt": "2026-10-01T00:00:00Z", "status": "active", "website": "https://example.com/katherine",
			"bio": "Checks every trajectory by hand before it ships."},
	}
}

// manyRows synthesizes n records for the long-list and load-more variants.
func manyRows(n int) []map[string]any {
	given := []string{"Ada", "Grace", "Alan", "Katherine", "Barbara", "Edsger", "Margaret", "Donald"}
	family := []string{"Lovelace", "Hopper", "Turing", "Johnson", "Liskov", "Dijkstra", "Hamilton", "Knuth"}
	status := []string{"active", "invited", "archived"}

	rows := make([]map[string]any, 0, n)
	for i := range n {
		rows = append(rows, map[string]any{
			"id":        i + 1,
			"name":      fmt.Sprintf("%s %s", given[i%len(given)], family[(i/len(given)+i)%len(family)]),
			"email":     fmt.Sprintf("user%02d@example.com", i+1),
			"company":   fmt.Sprintf("Account %02d", i+1),
			"balance":   float64((i*317)%2400) + 0.5,
			"seats":     (i%12 + 1) * 3,
			"createdAt": fmt.Sprintf("2026-%02d-%02dT09:00:00Z", i%12+1, i%27+1),
			"status":    status[i%len(status)],
			"website":   fmt.Sprintf("https://example.com/user%02d", i+1),
		})
	}
	return rows
}

func rowsOnly(rows []map[string]any) func() previewOut {
	return func() previewOut { return previewOut{Rows: &rows} }
}

// --- shared gallery pieces ---

func galleryDeleteAction() gomukit.Action {
	return gomukit.Action{Label: "Delete", Tool: "sandbox_delete", Variant: gomukit.VariantDanger,
		Confirm: "Really delete?", Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}}
}

func galleryRowActions() gomukit.Column {
	return gomukit.ActionsColumn(
		gomukit.Action{Label: "Open console", Kind: gomukit.ActionLink, HrefKey: "website"},
		// One argument read from the row, one fixed by the author.
		gomukit.Action{Label: "Send invite", Tool: "sandbox_invite",
			Args: map[string]gomukit.ArgSource{
				"id":      gomukit.FromRow("id"),
				"channel": gomukit.Static("email"),
			}},
		galleryDeleteAction(),
	)
}

// galleryStateActions are the actions of a roster whose buttons depend on the
// record: the two directions of the invite/suspend switch never appear on the
// same record, and the reminder belongs to the ones still waiting. The
// predicates read the raw "status" field, never the badge's words.
func galleryStateActions() []gomukit.Action {
	arg := map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}
	// The complement: everything but a closed record can still be deleted,
	// which leaves a closed one with no actions at all.
	del := galleryDeleteAction()
	del.VisibleWhen = gomukit.RowNot(gomukit.RowIs("status", "closed"))
	return []gomukit.Action{
		{Label: "Send reminder", Tool: "sandbox_invite", Args: arg,
			VisibleWhen: gomukit.RowIs("status", "invited")},
		{Label: "Suspend", Tool: "sandbox_archive_one", Args: arg,
			VisibleWhen: gomukit.RowIs("status", "active")},
		{Label: "Reinstate", Tool: "sandbox_invite", Args: arg,
			VisibleWhen: gomukit.RowIs("status", "archived")},
		del,
	}
}

// galleryStateRows carries one record per status, so a state-dependent action
// list can be read against every branch at once — including a closed record no
// action applies to, which is drawn without a menu trigger.
func galleryStateRows() []map[string]any {
	rows := galleryRows()
	rows[3]["status"] = "archived"
	return append(rows, map[string]any{
		"id": 5, "name": "Barbara Liskov", "email": "barbara@example.com",
		"company": "Abstraction Ltd", "balance": 0, "seats": 0, "utilization": 0,
		"createdAt": "2026-05-04T11:00:00Z", "renewsAt": "2026-05-04T00:00:00Z",
		"status": "closed", "website": "",
		"bio": "Account closed; nothing left to do here.",
	})
}

func galleryBulk() *gomukit.SelectionConfig {
	return &gomukit.SelectionConfig{Bulk: []gomukit.Action{
		{Label: "Archive", Tool: "sandbox_archive",
			Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")}},
		{Label: "Delete", Tool: "sandbox_delete_many", Variant: gomukit.VariantDanger,
			Confirm: "Delete them?", Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")}},
	}}
}

func galleryCardTemplate() gomukit.CardTemplate {
	return gomukit.CardTemplate{
		Header: gomukit.CardHeader{
			TitleKey:       "name",
			DescriptionKey: "email",
			Badge:          customerStatusBadge(),
		},
		Content: gomukit.CardContent{
			Items: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
				{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
				{Label: "Joined", Key: "createdAt", Type: gomukit.ColDate, Format: "relative"},
				{Label: "Console", Key: "website", Type: gomukit.ColLink,
					Link: &gomukit.LinkSpec{HrefKey: "website"}},
			}},
		},
		Footer: gomukit.CardFooter{Actions: []gomukit.Action{galleryDeleteAction()}},
	}
}

func galleryConfirmDetails() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "User", Key: "name"},
		{Label: "Email", Key: "email"},
		{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
		{Label: "Member since", Key: "createdAt", Type: gomukit.ColDate, Format: "date"},
		{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: customerStatusVariants()},
		{Label: "Profile", Type: gomukit.ColLink, Link: &gomukit.LinkSpec{HrefKey: "website", Text: "Open profile"}},
		{Label: "Region", Text: "eu-central-1"},
	}}
}

func galleryChoiceDetails() gomukit.Descriptions {
	return gomukit.Descriptions{Items: []gomukit.DescriptionItem{
		{Label: "Order", Key: "reference"},
		{Label: "Recipient", Key: "name"},
		{Label: "Destination", Text: "Berlin, DE"},
	}}
}

func galleryOrderRow() []map[string]any {
	return []map[string]any{{"id": 4471, "reference": "ORD-4471", "name": "Ada Lovelace"}}
}

// galleryShippingOptions are authored options: the same offer the scenario
// prices at call time, fixed here so the variant renders on its own.
func galleryShippingOptions() []gomukit.ChoiceOption {
	price := gomukit.DescriptionItem{Label: "Price", Key: "price", Type: gomukit.ColNumber, Format: "currency:EUR"}
	arrives := gomukit.DescriptionItem{Label: "Arrives", Key: "eta", Type: gomukit.ColDate, Format: "date"}
	return []gomukit.ChoiceOption{
		{
			Value: "standard", Label: "Standard", Summary: "3 to 5 business days",
			Body:    "Handed to the postal service tonight and tracked as far as the local depot.",
			Bullets: []string{"Tracked to the depot", "No signature on delivery", "Insured to EUR 50"},
			Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{price, arrives}},
			Data:    map[string]any{"price": 4.9, "eta": "2026-08-03T10:00:00Z"},
			Default: true,
		},
		{
			Value: "express", Label: "Express", Summary: "next business day, before 12:00",
			Body:         "Collected by courier this afternoon and delivered to the door tomorrow morning.",
			Bullets:      []string{"Tracked end to end", "Signature required", "Insured to EUR 500"},
			Details:      gomukit.Descriptions{Items: []gomukit.DescriptionItem{price, arrives}},
			Data:         map[string]any{"price": 14.9, "eta": "2026-07-28T12:00:00Z"},
			Badge:        "fastest",
			BadgeVariant: gomukit.BadgeSuccess,
		},
		{
			Value: "pickup", Label: "Depot pickup", Summary: "no depot near this address",
			Body:     "The nearest depot is 40 km away, so this address cannot use pickup.",
			Disabled: true,
		},
	}
}

func gallerySubmit() gomukit.ChoiceSubmit {
	return gomukit.ChoiceSubmit{
		Tool:           "sandbox_ship",
		Label:          "Ship it",
		ValueArg:       "method",
		Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
		SuccessMessage: "On its way.",
	}
}

// --- the catalog ---

// galleryCatalog is every variant, in the order the index menu lists them.
func galleryCatalog() []preview {
	rows := galleryRows()
	long := manyRows(24)

	return []preview{
		// Table
		{
			Tool: "preview_table_full", Group: "Table", Label: "Table, full featured", Icon: iconTable,
			Desc:   "Sort, filter, pagination with a page-size chooser, row actions and bulk actions.",
			Widget: galleryTable(), Data: rowsOnly(rows),
		},
		{
			Tool: "preview_table_readonly", Group: "Table", Label: "Table, read only", Icon: iconTable,
			Desc:   "Typed columns and nothing else: no actions, no selection, data baked in as a snapshot.",
			Widget: galleryTableReadonly(),
		},
		{
			Tool: "preview_table_long", Group: "Table", Label: "Table, long list", Icon: iconTable,
			Desc:   "Twenty-four records, pre-sorted, paginated at eight per page.",
			Widget: galleryTableLong(), Data: rowsOnly(long),
		},
		{
			Tool: "preview_table_loadmore", Group: "Table", Label: "Table, load more", Icon: iconTable,
			Desc:   "A growing list in a scroll-capped rows area: reaching the bottom loads the next batch.",
			Widget: galleryTableLoadMore(), Data: rowsOnly(long),
		},
		{
			Tool: "preview_table_empty", Group: "Table", Label: "Table, empty state", Icon: iconTable,
			Desc:   "The no-data message a table shows when the result carries no rows.",
			Widget: galleryTableEmpty(), Data: rowsOnly([]map[string]any{}),
		},

		{
			Tool: "preview_table_visibility", Group: "Table", Label: "Table, state-dependent actions", Icon: iconTable,
			Desc:   "VisibleWhen: each row's menu carries only the actions its status admits.",
			Widget: galleryTableVisibility(), Data: rowsOnly(galleryStateRows()),
		},

		{
			Tool: "preview_table_keys", Group: "Table", Label: "Table, renamed data keys", Icon: iconTable,
			Desc:   "RowsKey and RowID moved off their defaults: the rows arrive under \"records\", keyed by \"code\".",
			Widget: galleryTableKeys(),
			Data: func() previewOut {
				records := keyedRows()
				return previewOut{Records: &records}
			},
		},

		// Card list
		{
			Tool: "preview_cards_carousel", Group: "CardList", Label: "Cards, carousel", Icon: iconCards,
			Desc:   "Records as a paged card strip with sort, filter and bulk actions.",
			Widget: galleryCards(), Data: rowsOnly(rows),
		},
		{
			Tool: "preview_cards_loadmore", Group: "CardList", Label: "Cards, load more", Icon: iconCards,
			Desc:   "A growing strip: the pagination bar is replaced by a load-more tile.",
			Widget: galleryCardsLoadMore(), Data: rowsOnly(long),
		},
		{
			Tool: "preview_cards_visibility", Group: "CardList", Label: "Cards, state-dependent actions", Icon: iconCards,
			Desc:   "The same predicates on card buttons: a record no action applies to keeps no action bar.",
			Widget: galleryCardsVisibility(), Data: rowsOnly(galleryStateRows()),
		},
		{
			Tool: "preview_cards_empty", Group: "CardList", Label: "Cards, empty state", Icon: iconCards,
			Desc:   "The card list with nothing to show.",
			Widget: galleryCardsEmpty(), Data: rowsOnly([]map[string]any{}),
		},

		// Card
		{
			Tool: "preview_card_record", Group: "Card", Label: "Card, single record", Icon: iconCard,
			Desc:   "One record: header with badge, typed detail list, footer action.",
			Widget: galleryCard(), Data: rowsOnly(rows[:1]),
		},
		{
			Tool: "preview_card_sections", Group: "Card", Label: "Card, every section", Icon: iconCard,
			Desc:   "Header action instead of a badge, prose above the details, a footer note beside the actions.",
			Widget: galleryCardSections(),
		},
		{
			Tool: "preview_card_empty", Group: "Card", Label: "Card, empty state", Icon: iconCard,
			Desc:   "The card with no record in hand.",
			Widget: galleryCardEmpty(), Data: rowsOnly([]map[string]any{}),
		},

		// Descriptions
		{
			Tool: "preview_descriptions_types", Group: "Descriptions", Label: "Descriptions, all item types", Icon: iconList,
			Desc:   "Text, badge, three number formats, two date formats, a link, authored text and a missing value.",
			Widget: galleryDescriptions(), Data: rowsOnly([]map[string]any{detailsRow()}),
		},
		{
			Tool: "preview_descriptions_dense", Group: "Descriptions", Label: "Descriptions, narrower columns", Icon: iconList,
			Desc:   "The same block with the one layout token it exposes lowered.",
			Widget: galleryDescriptionsDense(), Data: rowsOnly([]map[string]any{detailsRow()}),
		},

		// Form
		{
			Tool: "preview_form_edit", Group: "Form", Label: "Form, edit record", Icon: iconForm,
			Desc:   "A short form prefilled at runtime. Submitting taken@example.com returns a field error.",
			Widget: galleryForm(),
			Data: func() previewOut {
				return previewOut{Values: map[string]any{
					"id": "1", "name": "Ada Lovelace", "email": "ada@example.com", "role": "admin", "notify": true,
				}}
			},
		},
		{
			Tool: "preview_form_fields", Group: "Form", Label: "Form, every field type", Icon: iconForm,
			Desc:   "Readonly, text, select, multi-select, number, date, time, textarea, checkbox and hidden.",
			Widget: galleryFormFields(),
		},
		{
			Tool: "preview_form_layout", Group: "Form", Label: "Form, grouped fields", Icon: iconForm,
			Desc:   "Two columns, three field sets — one boxed, one single-column — and fields spanning the row.",
			Widget: galleryFormLayout(),
		},

		// Menu
		{
			Tool: "preview_menu_launcher", Group: "Menu", Label: "Menu, launcher", Icon: iconPalette,
			Desc:   "Tiles with icons, descriptions and badges. Each one calls another preview tool.",
			Widget: galleryMenu(),
		},
		{
			Tool: "preview_menu_plain", Group: "Menu", Label: "Menu, plain tiles", Icon: iconPalette,
			Desc:   "The same widget with no icons, no descriptions, and one tile falling back to the tool name.",
			Widget: galleryMenuPlain(),
		},

		// Confirm
		{
			Tool: "preview_confirm_danger", Group: "Confirm", Label: "Confirm, destructive", Icon: iconTrash,
			Desc:   "Severity, detail summary, authored effects, an acknowledgement box and a type-to-confirm phrase.",
			Widget: galleryConfirmDanger(), Data: rowsOnly(rows[:1]),
		},
		{
			Tool: "preview_confirm_plain", Group: "Confirm", Label: "Confirm, plain question", Icon: iconQuestn,
			Desc:   "No record, no title: a question, its consequences, and two buttons.",
			Widget: galleryConfirmPlain(),
		},
		{
			Tool: "preview_confirm_runtime", Group: "Confirm", Label: "Confirm, runtime effects", Icon: iconQuestn,
			Desc:   "The record and the consequences arrive with the tool result and replace the authored list.",
			Widget: galleryConfirmRuntime(),
			Data: func() previewOut {
				runtime := rows[1:2]
				return previewOut{
					Rows: &runtime,
					Effects: []map[string]any{
						{"text": "Removes the account", "severity": "danger"},
						{"text": "Deletes audit records", "detail": "Not recoverable.", "value": "128", "severity": "warning"},
						{"text": "Notifies the team", "value": "4 people", "severity": "info"},
					},
				}
			},
		},

		// Choice
		{
			Tool: "preview_choice_single", Group: "Choice", Label: "Choice, one of several", Icon: iconTruck,
			Desc:   "Radio options with a description panel; the layout follows the width the host gives it.",
			Widget: galleryChoice(), Data: rowsOnly(galleryOrderRow()),
		},
		{
			Tool: "preview_choice_stacked", Group: "Choice", Label: "Choice, always stacked", Icon: iconTruck,
			Desc:   "The same options with the description forced under the option in hand.",
			Widget: galleryChoiceStacked(), Data: rowsOnly(galleryOrderRow()),
		},
		{
			Tool: "preview_choice_multi", Group: "Choice", Label: "Choice, multiple and bounded", Icon: iconTruck,
			Desc:   "Checkboxes with a floor and a ceiling: once the maximum is ticked the rest disable.",
			Widget: galleryChoiceMulti(),
		},
		{
			Tool: "preview_choice_runtime", Group: "Choice", Label: "Choice, runtime options", Icon: iconTruck,
			Desc:   "The offer arrives with the tool result, typed and formatted in the host's locale.",
			Widget: galleryChoiceRuntime(),
			Data: func() previewOut {
				row, options, _ := newStore().shippingFor(4471)
				return previewOut{Rows: &[]map[string]any{row}, Options: options}
			},
		},

		// Date picker
		{
			Tool: "preview_date_single", Group: "Date picker", Label: "Date picker, one date", Icon: iconCal,
			Desc:   "An inline calendar with presets, a bounded window and two days already taken.",
			Widget: galleryDatePicker(), Data: rowsOnly(galleryOrderRow()),
		},
		{
			Tool: "preview_date_range", Group: "Date picker", Label: "Date picker, a range", Icon: iconCal,
			Desc:   "Two months side by side, ISO week numbers, and quick ranges beside the grid.",
			Widget: galleryDatePickerRange(),
		},
		{
			Tool: "preview_date_dropdowns", Group: "Date picker", Label: "Date picker, month and year", Icon: iconCal,
			Desc:   "Caption dropdowns for a date far from today, with every weekend blocked.",
			Widget: galleryDatePickerDropdowns(),
		},
		{
			Tool: "preview_date_inputs", Group: "Date picker", Label: "Date picker, asking for more", Icon: iconCal,
			Desc:   "Details that ask as well as state: a count, a choice and a checkbox travel with the dates.",
			Widget: galleryDatePickerInputs(), Data: rowsOnly(galleryOrderRow()),
		},
		{
			Tool: "preview_date_runtime", Group: "Date picker", Label: "Date picker, runtime availability", Icon: iconCal,
			Desc:   "The window and the nights already booked arrive with the tool result.",
			Widget: galleryDatePickerRuntime(),
			Data: func() previewOut {
				row := galleryOrderRow()
				return previewOut{Rows: &row, Value: map[string]any{
					"min":      "2026-08-01",
					"max":      "2026-10-31",
					"disabled": []string{"2026-08-27", "2026-08-28", "2026-08-29"},
				}}
			},
		},

		// Theming and chrome
		{
			Tool: "preview_theme_tokens", Group: "Theme", Label: "Theme, token overrides", Icon: iconPalette,
			Desc:   "Colors, fonts, radii, spacing unit and a raw custom property, all overridden at once.",
			Widget: galleryThemeTokens(), Data: rowsOnly(rows),
		},
		{
			Tool: "preview_theme_transparent", Group: "Theme", Label: "Theme, frameless", Icon: iconPalette,
			Desc:   "Transparent page and no gutter, so only the widget card sits on the host surface.",
			Widget: galleryThemeTransparent(), Data: rowsOnly(rows),
		},
		{
			Tool: "preview_brand_datauri", Group: "Theme", Label: "Brand, data-URI logo", Icon: iconPalette,
			Desc:   "The other logo path: a base64 image instead of inline SVG, and prefersBorder set on the resource.",
			Widget: galleryBrandDataURI(), Data: rowsOnly(rows[:1]),
		},
	}
}

// --- gallery widgets ---

func galleryTable() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
			customerStatusBadge(),
			galleryRowActions(),
		},
		PageSize:   3,
		PageSizes:  []int{3, 5, 10},
		Filterable: true,
		Selection:  galleryBulk(),
		Brand:      appBrand(),
		Theme:      appTheme(),
	}
}

// galleryTableVisibility shows one actions column over records in different
// states: a row's menu holds only what that row's status admits, and a row
// nothing admits keeps no trigger.
func galleryTableVisibility() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table-visibility",
		Title: "Roster",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			customerStatusBadge(),
			gomukit.ActionsColumn(galleryStateActions()...),
		},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

// galleryTableReadonly is the one variant that ships its data baked in: the
// document paints before any tool result arrives.
func galleryTableReadonly() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table-readonly",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
		},
		InitialData: map[string]any{"rows": galleryRows()},
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

func galleryTableLong() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table-long",
		Title: "Directory",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Number("seats", "Seats", "int"),
			gomukit.Date("createdAt", "Created", "date"),
			customerStatusBadge(),
			gomukit.ActionsColumn(galleryDeleteAction()),
		},
		PageSize:    8,
		PageSizes:   []int{8, 16, 24},
		DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   galleryBulk(),
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

func galleryTableLoadMore() *gomukit.Table {
	t := galleryTableLong()
	t.URI = "ui://preview/gallery-table-loadmore"
	t.PageSizes = nil
	t.LoadMore = true
	t.MaxHeight = "20rem"
	return t
}

func galleryTableEmpty() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table-empty",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			customerStatusBadge(),
		},
		Filterable: true,
		Empty: gomukit.EmptyState{Title: "No users yet",
			Body: "Call preview_table_full to see the same widget with rows."},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

// galleryTableKeys proves the data contract is a default, not a rule: the
// widget reads a different array key and identifies records by a different
// field, and every action arg follows.
func galleryTableKeys() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://preview/gallery-table-keys",
		Title: "Warehouses",
		Columns: []gomukit.Column{
			gomukit.Text("code", "Code"),
			gomukit.Text("city", "City"),
			gomukit.Number("capacity", "Capacity", "int"),
			gomukit.ActionsColumn(gomukit.Action{
				Label: "Delete", Tool: "sandbox_delete", Variant: gomukit.VariantDanger,
				Confirm: "Really delete?",
				Args:    map[string]gomukit.ArgSource{"code": gomukit.FromRow("code")},
			}),
		},
		RowsKey:    "records",
		RowID:      "code",
		Filterable: true,
		Selection: &gomukit.SelectionConfig{Bulk: []gomukit.Action{
			{Label: "Archive", Tool: "sandbox_archive",
				Args: map[string]gomukit.ArgSource{"codes": gomukit.FromSelection("code")}},
		}},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

func keyedRows() []map[string]any {
	return []map[string]any{
		{"code": "BER-1", "city": "Berlin", "capacity": 12000},
		{"code": "MUC-2", "city": "Munich", "capacity": 8400},
		{"code": "HAM-3", "city": "Hamburg", "capacity": 15600},
	}
}

func galleryCards() *gomukit.CardList {
	return &gomukit.CardList{
		URI:         "ui://preview/gallery-cards",
		Title:       "Users",
		Template:    galleryCardTemplate(),
		PageSize:    3,
		PageSizes:   []int{3, 6, 12},
		DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   galleryBulk(),
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

func galleryCardsLoadMore() *gomukit.CardList {
	l := galleryCards()
	l.URI = "ui://preview/gallery-cards-loadmore"
	l.Title = "Directory"
	l.PageSize = 6
	l.PageSizes = nil
	l.LoadMore = true
	return l
}

// galleryCardsVisibility gives the cards the state-dependent buttons the
// roster table puts in its menus.
func galleryCardsVisibility() *gomukit.CardList {
	l := galleryCards()
	l.URI = "ui://preview/gallery-cards-visibility"
	l.Title = "Roster"
	l.PageSize = 0
	l.PageSizes = nil
	l.Selection = nil
	l.Template.Footer.Actions = galleryStateActions()
	return l
}

func galleryCardsEmpty() *gomukit.CardList {
	l := galleryCards()
	l.URI = "ui://preview/gallery-cards-empty"
	l.Empty = gomukit.EmptyState{Title: "Nothing to show", Body: "Call preview_cards_carousel for the populated variant."}
	return l
}

func galleryCard() *gomukit.Card {
	return &gomukit.Card{
		URI:      "ui://preview/gallery-card",
		Title:    "User",
		Template: galleryCardTemplate(),
		Empty:    gomukit.EmptyState{Title: "No user", Body: "Call the tool again with a record."},
		Brand:    appBrand(),
		Theme:    appTheme(),
	}
}

func galleryCardSections() *gomukit.Card {
	c := galleryCard()
	c.URI = "ui://preview/gallery-card-sections"
	c.Template.Header.Badge = gomukit.Column{}
	c.Template.Header.Action = &gomukit.Action{Label: "Open console", Kind: gomukit.ActionLink, HrefKey: "website"}
	c.Template.Content.TextKey = "bio"
	c.Template.Footer.Text = "Balances update hourly."
	c.InitialData = map[string]any{"rows": galleryRows()[:1]}
	return c
}

func galleryCardEmpty() *gomukit.Card {
	c := galleryCard()
	c.URI = "ui://preview/gallery-card-empty"
	return c
}

// detailsRow is the record behind the description variants: one field per
// item type, and no "owner" key, so the item reading it shows the dash a
// missing value renders as.
func detailsRow() map[string]any {
	return map[string]any{
		"id": 2, "name": "Grace Hopper", "email": "grace@example.com", "company": "Compiler Works",
		"plan": "enterprise", "balance": 4820.40, "seats": 48, "utilization": 0.71,
		"renewsAt": "2027-01-31T00:00:00Z", "createdAt": "2024-11-02T09:00:00Z",
		"status": "active", "website": "https://example.com/grace",
	}
}

func galleryDescriptions() *gomukit.Card {
	return &gomukit.Card{
		URI:   "ui://preview/gallery-descriptions",
		Title: "Account details",
		Template: gomukit.CardTemplate{
			Header:  gomukit.CardHeader{TitleKey: "name", DescriptionKey: "plan"},
			Content: gomukit.CardContent{Items: customerDetails()},
		},
		Empty: gomukit.EmptyState{Title: "No record", Body: "Call the tool again with a record."},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

func galleryDescriptionsDense() *gomukit.Card {
	c := galleryDescriptions()
	c.URI = "ui://preview/gallery-descriptions-dense"
	c.Theme = &theme.Theme{
		ColorPrimary: "#7c3aed",
		Extra:        map[string]string{"--gomu-desc-min": "8rem"},
	}
	return c
}

func galleryForm() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://preview/gallery-form",
		Title: "Edit user",
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden, Default: "1"},
			{Name: "name", Label: "Name", Required: true},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gomukit.FSelect,
				Options: []gomukit.Option{gomukit.Opt("user"), gomukit.Opt("admin")}},
			{Name: "notify", Label: "Send notifications", Type: gomukit.FCheckbox, Default: true},
		},
		Submit: gomukit.SubmitSpec{Tool: "sandbox_save", SuccessMessage: "Saved."},
		Cancel: &gomukit.CancelSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryFormFields() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://preview/gallery-form-fields",
		Title: "New user",
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden, Default: "0"},
			{Name: "account", Label: "Workspace", Type: gomukit.FReadonly, Default: "acme-eu"},
			{Name: "name", Label: "Name", Required: true, Placeholder: "Ada Lovelace",
				Description: "Shown everywhere the record appears.",
				Validation:  &gomukit.Validation{MinLen: ptrInt(2), MaxLen: ptrInt(60)}},
			{Name: "email", Label: "Email", Required: true, Placeholder: "ada@example.com",
				Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gomukit.FSelect, Default: "user",
				Options: []gomukit.Option{gomukit.Opt("user"), gomukit.Opt("admin"), gomukit.Opt("auditor")}},
			{Name: "scopes", Label: "Scopes", Type: gomukit.FMultiSelect, Default: []string{"read"},
				Options: []gomukit.Option{gomukit.Opt("read"), gomukit.Opt("write"), gomukit.Opt("billing")}},
			{Name: "seats", Label: "Seats", Type: gomukit.FNumber, Default: "3",
				Validation: &gomukit.Validation{Min: ptr(1.0), Max: ptr(50.0), Step: ptr(1.0)}},
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
				Placeholder: "Anything the team should know",
				Validation:  &gomukit.Validation{MaxLen: ptrInt(280)}},
			{Name: "notify", Label: "Send notifications", Type: gomukit.FCheckbox, Default: true},
		},
		Submit: gomukit.SubmitSpec{Tool: "sandbox_save", Label: "Create user", SuccessMessage: "User created."},
		Cancel: &gomukit.CancelSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryFormLayout() *gomukit.Form {
	return &gomukit.Form{
		URI:     "ui://preview/gallery-form-layout",
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
					{Name: "hours", Label: "Weekly hours", Type: gomukit.FNumber, Default: "40",
						Validation: &gomukit.Validation{Min: ptr(1.0), Max: ptr(40.0), Step: ptr(1.0)}},
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
		Submit: gomukit.SubmitSpec{Tool: "sandbox_save", Label: "Create employee", SuccessMessage: "Employee created."},
		Cancel: &gomukit.CancelSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryMenu() *gomukit.Menu {
	return &gomukit.Menu{
		URI:   "ui://preview/gallery-menu",
		Title: "Launcher",
		Intro: "Each tile calls a tool; the host opens the widget bound to it.",
		Items: []gomukit.MenuItem{
			{Tool: "preview_table_full", Label: "User table", IconSVG: iconTable,
				Description: "Sortable, filterable directory.", Badge: "read", BadgeVariant: gomukit.BadgeInfo},
			{Tool: "preview_form_edit", Label: "Edit user", IconSVG: iconPencil,
				Description: "Open the edit form for one user.", Badge: "write", BadgeVariant: gomukit.BadgeWarning},
			{Tool: "preview_confirm_danger", Label: "Delete user", IconSVG: iconTrash,
				Description: "A confirmation with everything turned on.", Badge: "danger", BadgeVariant: gomukit.BadgeDanger},
		},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

func galleryMenuPlain() *gomukit.Menu {
	return &gomukit.Menu{
		URI:   "ui://preview/gallery-menu-plain",
		Title: "Launcher",
		Items: []gomukit.MenuItem{
			{Tool: "preview_table_full", Label: "Users"},
			{Tool: "preview_form_fields", Label: "New user"},
			// No label: the tile falls back to the tool name.
			{Tool: "preview_cards_carousel"},
		},
		Brand: appBrand(),
	}
}

func galleryConfirmDanger() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:      "ui://preview/gallery-confirm",
		Title:    "Delete user",
		Prompt:   "Delete Ada Lovelace?",
		Body:     "The account and everything attached to it is removed for good.",
		Severity: gomukit.BadgeDanger,
		Details:  galleryConfirmDetails(),
		Effects: []gomukit.Effect{
			{Text: "Removes the account", Detail: "Sign-in stops working immediately.", Severity: gomukit.BadgeDanger},
			{Text: "Deletes audit records", Value: "128", Severity: gomukit.BadgeWarning},
			{Text: "Frees the seat", Value: "1 seat", Severity: gomukit.BadgeSuccess},
		},
		Acknowledge:   "I understand this cannot be undone.",
		TypeToConfirm: "ada@example.com",
		Accept: gomukit.AcceptSpec{
			Tool:           "sandbox_delete",
			Label:          "Delete user",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "User deleted.",
		},
		Reject: &gomukit.RejectSpec{Label: "Keep user", Message: "Nothing was deleted."},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryConfirmPlain() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:      "ui://preview/gallery-confirm-plain",
		Prompt:   "Archive 4 users?",
		Body:     "Archived users keep their data but cannot sign in.",
		Severity: gomukit.BadgeWarning,
		Effects: []gomukit.Effect{
			{Text: "Revokes active sessions", Value: "4", Severity: gomukit.BadgeWarning},
			{Text: "Keeps every record", Severity: gomukit.BadgeNeutral},
		},
		Accept: gomukit.AcceptSpec{Tool: "sandbox_archive_all", Label: "Archive", SuccessMessage: "Users archived."},
		Reject: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryConfirmRuntime() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:     "ui://preview/gallery-confirm-runtime",
		Title:   "Delete user",
		Prompt:  "Delete this user?",
		Body:    "The record and the consequences below came with the tool result.",
		Details: galleryConfirmDetails(),
		Effects: []gomukit.Effect{
			{Text: "Authored at registration time, replaced by any runtime list.", Severity: gomukit.BadgeNeutral},
		},
		Accept: gomukit.AcceptSpec{
			Tool:           "sandbox_delete",
			Label:          "Delete",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "User deleted.",
		},
		Reject: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryChoice() *gomukit.Choice {
	return &gomukit.Choice{
		URI:     "ui://preview/gallery-choice",
		Title:   "Shipping",
		Prompt:  "How should we ship order ORD-4471?",
		Body:    "The parcel is packed and leaves the warehouse today either way.",
		Details: galleryChoiceDetails(),
		Options: galleryShippingOptions(),
		Submit:  gallerySubmit(),
		Cancel:  &gomukit.RejectSpec{Label: "Decide later", Message: "Nothing was shipped."},
		Brand:   appBrand(),
		Theme:   appTheme(),
	}
}

func galleryChoiceStacked() *gomukit.Choice {
	c := galleryChoice()
	c.URI = "ui://preview/gallery-choice-stacked"
	c.Layout = gomukit.ChoiceStacked
	c.Details = gomukit.Descriptions{}
	return c
}

func galleryChoiceMulti() *gomukit.Choice {
	return &gomukit.Choice{
		URI:      "ui://preview/gallery-choice-multi",
		Title:    "Add-ons",
		Prompt:   "Which extras should this shipment carry?",
		Body:     "Choose two or three; they are billed with the shipping cost.",
		Layout:   gomukit.ChoiceSplit,
		Multiple: true,
		Min:      2,
		Max:      3,
		Options: []gomukit.ChoiceOption{
			{Value: "insurance", Label: "Extra insurance", Summary: "up to EUR 5,000",
				Body:    "Covers the declared value against loss and damage in transit.",
				Bullets: []string{"Claims within 30 days", "Proof of value required"},
				Default: true},
			{Value: "signature", Label: "Signature on delivery", Summary: "hand to the recipient",
				Body: "The courier hands the parcel over in person and records the signature."},
			{Value: "saturday", Label: "Saturday delivery", Summary: "weekend slot",
				Body:         "Delivered on Saturday morning instead of the next business day.",
				Badge:        "surcharge",
				BadgeVariant: gomukit.BadgeWarning},
			{Value: "carbon", Label: "Carbon offset", Summary: "adds EUR 0.40",
				Body: "Buys certified offsets for the leg between the depot and the door."},
		},
		Submit: gomukit.ChoiceSubmit{Tool: "sandbox_extras", Label: "Add extras",
			ValueArg: "extras", SuccessMessage: "Extras added."},
		Cancel: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryChoiceRuntime() *gomukit.Choice {
	return &gomukit.Choice{
		URI:     "ui://preview/gallery-choice-runtime",
		Title:   "Shipping",
		Prompt:  "How should we ship this order?",
		Body:    "The options below were priced by the tool that opened this view.",
		Details: galleryChoiceDetails(),
		Submit:  gallerySubmit(),
		Cancel:  &gomukit.RejectSpec{},
		Brand:   appBrand(),
		Theme:   appTheme(),
	}
}

func galleryDatePicker() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://preview/gallery-date",
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
		Details: galleryChoiceDetails(),
		Submit: gomukit.DateSubmit{
			Tool:           "sandbox_schedule",
			Label:          "Book it",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "Booked.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Decide later", Message: "Nothing was booked."},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryDatePickerRange() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://preview/gallery-date-range",
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
		Submit: gomukit.DateSubmit{Tool: "sandbox_schedule", Label: "Hold it",
			ValueArg: "from", EndArg: "until", SuccessMessage: "Held."},
		Cancel: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryDatePickerDropdowns() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://preview/gallery-date-dropdowns",
		Prompt: "When does the contract start?",
		Body:   "Weekends are not working days.",
		Calendar: &gomukit.Calendar{
			Min:             "2020-01-01",
			Max:             "2030-12-31",
			DisableWeekends: true,
			MonthDropdowns:  true,
			StartOn:         "2027-03-01",
		},
		Submit: gomukit.DateSubmit{Tool: "sandbox_schedule", Label: "Set the date"},
		Cancel: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// galleryDatePickerInputs asks the questions that come with the dates: what
// the controls collect is merged into the same submit call.
func galleryDatePickerInputs() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://preview/gallery-date-inputs",
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
			{Label: "Order", Key: "reference"},
			{Label: "Guests", Input: &gomukit.Input{
				Name:       "guests",
				Type:       gomukit.InputNumber,
				Default:    2,
				Required:   true,
				Validation: &gomukit.Validation{Min: ptr(1.0), Max: ptr(6.0), Step: ptr(1.0), Message: "Between one and six guests."},
			}},
			{Label: "Bed", Input: &gomukit.Input{
				Name:        "bed",
				Type:        gomukit.InputSelect,
				Placeholder: "Pick one",
				Options: []gomukit.Option{
					{Value: "double", Label: "One double"},
					{Value: "twin", Label: "Two singles"},
				},
			}},
			{Label: "Arriving after 22:00", Input: &gomukit.Input{Name: "late", Type: gomukit.InputCheckbox}},
			{Label: "Anything else?", Input: &gomukit.Input{Name: "notes", Placeholder: "Allergies, a cot, a late checkout…"}},
		}},
		Submit: gomukit.DateSubmit{Tool: "sandbox_schedule", Label: "Hold it",
			ValueArg: "from", EndArg: "until", SuccessMessage: "Held."},
		Cancel: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

func galleryDatePickerRuntime() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:      "ui://preview/gallery-date-runtime",
		Title:    "Availability",
		Prompt:   "Which nights are you staying?",
		Mode:     gomukit.DateRange,
		Calendar: &gomukit.Calendar{WeekNumbers: true},
		Details:  galleryChoiceDetails(),
		Submit: gomukit.DateSubmit{Tool: "sandbox_schedule", ValueArg: "from", EndArg: "until",
			Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
		Cancel: &gomukit.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// galleryThemeTokens overrides most of what a Theme can override at once, so
// the difference from the default palette is unmistakable.
func galleryThemeTokens() *gomukit.Table {
	t := galleryTable()
	t.URI = "ui://preview/gallery-theme"
	t.Title = "Themed table"
	t.Theme = &theme.Theme{
		ColorBackground:  "#0b1120",
		ColorSurface:     "#111c33",
		ColorText:        "#e2e8f0",
		ColorTextMuted:   "#94a3b8",
		ColorBorder:      "#1e293b",
		ColorPrimary:     "#22d3ee",
		ColorPrimaryText: "#04212a",
		ColorDanger:      "#fb7185",
		ColorSuccess:     "#4ade80",
		ColorWarning:     "#fbbf24",
		FontFamily:       "ui-serif, Georgia, serif",
		FontFamilyMono:   "ui-monospace, SFMono-Regular, monospace",
		RadiusS:          "2px",
		RadiusM:          "10px",
		RadiusL:          "18px",
		SpaceUnit:        "0.3rem",
		ColorPage:        "#060b16",
		PagePad:          "16px",
		Extra:            map[string]string{"--gomu-desc-min": "10rem"},
	}
	return t
}

// galleryThemeTransparent drops the page fill and the gutter: the frame
// disappears and only the card sits on whatever the host paints behind it.
func galleryThemeTransparent() *gomukit.Table {
	t := galleryTable()
	t.URI = "ui://preview/gallery-theme-transparent"
	t.Title = "Frameless table"
	t.Theme = &theme.Theme{ColorPrimary: "#7c3aed", Transparent: true}
	return t
}

// galleryBrandDataURI uses the base64 logo path and asks the host for a
// border through the resource's _meta.ui.
func galleryBrandDataURI() *gomukit.Card {
	c := galleryCard()
	c.URI = "ui://preview/gallery-brand-datauri"
	c.Title = "Data-URI brand"
	c.Brand = dataURIBrand()
	c.UI = &uispec.ResourceUIMeta{PrefersBorder: ptr(true)}
	return c
}

// --- registration ---

// registerGallery installs one model-visible tool per variant, the index menu
// that lists them, and the sandbox tools the gallery widgets fire.
func registerGallery(s *mcp.Server) {
	catalog := galleryCatalog()

	must(gosdk.AddWidgetToolFor(s, galleryIndex(catalog),
		&mcp.Tool{Name: "preview_index", Description: "Show the gallery of every gomukit widget variant."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, noOut, error) {
			return textResult("Showing the widget gallery."), noOut{}, nil
		}))

	for _, p := range catalog {
		data := p.Data
		label := p.Label
		must(gosdk.AddWidgetToolFor(s, p.Widget,
			&mcp.Tool{Name: p.Tool, Description: p.Group + " preview: " + p.Desc},
			func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, previewOut, error) {
				if data == nil {
					return textResult("Showing " + label + "."), previewOut{}, nil
				}
				return nil, data(), nil
			}))
	}

	registerSandbox(s, catalog[0].Widget, galleryForm())
}

// galleryIndex is the front door to the catalog: one tile per variant.
//
// Every tile carries a Prompt. A gallery tool answers with a widget rather
// than with data, so the host has to open it, and a host that runs a
// view-initiated tools/call out of band opens nothing. Routing through the
// chat makes the model place the call, which is the path that works
// everywhere. The label is echoed verbatim because it is what the tool's own
// description leads with, so the model has an unambiguous target among the
// catalog's several dozen near-identical entries.
func galleryIndex(catalog []preview) *gomukit.Menu {
	items := make([]gomukit.MenuItem, 0, len(catalog))
	for _, p := range catalog {
		items = append(items, gomukit.MenuItem{
			Tool:         p.Tool,
			Prompt:       "Show me the gallery preview for " + p.Label,
			Label:        p.Label,
			Description:  p.Desc,
			IconSVG:      p.Icon,
			Badge:        p.Group,
			BadgeVariant: gomukit.BadgeNeutral,
		})
	}
	return &gomukit.Menu{
		URI:   "ui://preview/gallery-index",
		Title: "Widget gallery",
		Intro: "Every variant this library renders. Each tile asks the chat to open it.",
		Items: items,
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

// registerSandbox installs the app-only tools gallery widgets call. They are
// stateless: each answer is computed from the fixture, so the gallery never
// disturbs the scenario store. A tool's _meta points at the widget that fires
// it; several widgets share the record tools, so those anchor on the table.
func registerSandbox(s *mcp.Server, anchor gomukit.Widget, formAnchor *gomukit.Form) {
	add := func(t *mcp.Tool, h func(ctx context.Context, req *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error)) {
		gosdk.AppOnly(t, anchor)
		must(gosdk.AddWidgetToolFor(s, anchor, t, h))
	}

	add(&mcp.Tool{Name: "sandbox_delete", Description: "Preview-only delete: answers with the fixture minus that record."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			// The renamed-keys variant identifies its records by code, so the
			// answer has to come back under the key that variant reads.
			if in.Code != "" {
				records := withoutCodes(in.Code)
				return textResult("Deleted (preview only)."), previewOut{Records: &records}, nil
			}
			rows := withoutIDs(in.ID)
			return textResult("Deleted (preview only)."), previewOut{Rows: &rows}, nil
		})

	add(&mcp.Tool{Name: "sandbox_delete_many", Description: "Preview-only bulk delete."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			rows := withoutIDs(in.IDs...)
			return textResult(fmt.Sprintf("Deleted %d records (preview only).", len(in.IDs))),
				previewOut{Rows: &rows}, nil
		})

	add(&mcp.Tool{Name: "sandbox_archive", Description: "Preview-only bulk archive."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			if len(in.Codes) > 0 {
				records := withoutCodes(in.Codes...)
				return textResult(fmt.Sprintf("Archived %d warehouses (preview only).", len(in.Codes))),
					previewOut{Records: &records}, nil
			}
			rows := archived(in.IDs...)
			return textResult(fmt.Sprintf("Archived %d records (preview only).", len(in.IDs))),
				previewOut{Rows: &rows}, nil
		})

	add(&mcp.Tool{Name: "sandbox_archive_all", Description: "Preview-only archive of every fixture record."},
		func(context.Context, *mcp.CallToolRequest, sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			rows := archived(1, 2, 3, 4)
			return textResult("Archived every record (preview only)."), previewOut{Rows: &rows}, nil
		})

	add(&mcp.Tool{Name: "sandbox_invite", Description: "Preview-only invitation."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			rows := galleryRows()
			return textResult(fmt.Sprintf("Invitation sent to record %d by %s (preview only).", in.ID, in.Channel)),
				previewOut{Rows: &rows}, nil
		})

	add(&mcp.Tool{Name: "sandbox_ship", Description: "Preview-only shipping decision."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			return textResult("Shipped by " + in.Method + " (preview only)."), previewOut{}, nil
		})

	add(&mcp.Tool{Name: "sandbox_schedule", Description: "Preview-only date or date-range decision."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			switch {
			case in.From != "" && in.Until != "":
				return textResult("Held " + in.From + " to " + in.Until + " (preview only)."), previewOut{}, nil
			case in.Date != "":
				return textResult("Booked for " + in.Date + " (preview only)."), previewOut{}, nil
			}
			return textResult("No date was sent (preview only)."), previewOut{}, nil
		})

	add(&mcp.Tool{Name: "sandbox_extras", Description: "Preview-only add-on selection."},
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxInput) (*mcp.CallToolResult, previewOut, error) {
			return textResult("Added " + strings.Join(in.Extras, ", ") + " (preview only)."), previewOut{}, nil
		})

	// The gallery forms submit here. The validation is real, so the error
	// path a form renders is reachable: submit taken@example.com.
	saveTool := &mcp.Tool{Name: "sandbox_save", Description: "Preview-only form submit with server-side validation."}
	gosdk.AppOnly(saveTool, formAnchor)
	must(gosdk.AddWidgetToolFor(s, formAnchor, saveTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in sandboxSave) (*mcp.CallToolResult, errorsOut, error) {
			errs := map[string]string{}
			if strings.TrimSpace(in.Name) == "" {
				errs["name"] = "Name must not be empty."
			}
			if strings.EqualFold(in.Email, "taken@example.com") {
				errs["email"] = "This email is already taken."
			}
			if in.Seats > 50 {
				errs["seats"] = "This plan tops out at 50 seats."
			}
			if len(errs) > 0 {
				return nil, errorsOut{Errors: errs}, nil
			}
			return textResult("Saved (preview only)."), errorsOut{}, nil
		}))
}

// sandboxInput is the union of what the gallery widgets send. One shape keeps
// the sandbox tools to a single signature.
type sandboxInput struct {
	ID      int      `json:"id,omitempty"`
	Channel string   `json:"channel,omitempty"`
	IDs     []int    `json:"ids,omitempty"`
	Method  string   `json:"method,omitempty"`
	Extras  []string `json:"extras,omitempty"`
	// The date pickers send a day, or the two ends of a span.
	Date  string `json:"date,omitempty"`
	From  string `json:"from,omitempty"`
	Until string `json:"until,omitempty"`
	// The renamed-keys variant identifies records by code, so its actions
	// send these instead.
	Code  string   `json:"code,omitempty"`
	Codes []string `json:"codes,omitempty"`
}

// sandboxSave is what both gallery forms submit; each sends the subset of
// fields it renders.
type sandboxSave struct {
	ID        string   `json:"id,omitempty"`
	Account   string   `json:"account,omitempty"`
	Name      string   `json:"name,omitempty"`
	Email     string   `json:"email,omitempty"`
	Role      string   `json:"role,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	Seats     float64  `json:"seats,omitempty"`
	StartsOn  string   `json:"startsOn,omitempty"`
	TrialFrom string   `json:"trialFrom,omitempty"`
	TrialTo   string   `json:"trialTo,omitempty"`
	DigestAt  string   `json:"digestAt,omitempty"`
	Notes     string   `json:"notes,omitempty"`
	Notify    bool     `json:"notify,omitempty"`
}

func withoutIDs(ids ...int) []map[string]any {
	drop := map[int]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	out := []map[string]any{}
	for _, row := range galleryRows() {
		if id, ok := row["id"].(int); ok && drop[id] {
			continue
		}
		out = append(out, row)
	}
	return out
}

// withoutCodes is withoutIDs for the record set keyed by code.
func withoutCodes(codes ...string) []map[string]any {
	drop := map[string]bool{}
	for _, c := range codes {
		drop[c] = true
	}
	out := []map[string]any{}
	for _, row := range keyedRows() {
		if code, ok := row["code"].(string); ok && drop[code] {
			continue
		}
		out = append(out, row)
	}
	return out
}

func archived(ids ...int) []map[string]any {
	mark := map[int]bool{}
	for _, id := range ids {
		mark[id] = true
	}
	rows := galleryRows()
	for _, row := range rows {
		if id, ok := row["id"].(int); ok && mark[id] {
			row["status"] = "archived"
		}
	}
	return rows
}
