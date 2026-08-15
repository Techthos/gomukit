# <img src="assets/gomukit-icon.svg" alt="" width="28" align="center"> Widget reference

Every widget implements `gomukit.Widget`: it renders one self-contained HTML
document (`Document()`), registers as one `ui://` resource
(`Descriptor()`), and links to tools via `ToolMeta()`. With the go-sdk
adapter you rarely call these yourself — see `gomukit/gosdk`.

## Data contract

Widgets read runtime data from the tool result's `structuredContent`:

| Widget | Key (default) | Shape |
|---|---|---|
| Table | `rows` (`RowsKey`) | `[]object` — one JSON object per row |
| CardList | `rows` (`RowsKey`) | `[]object` — one JSON object per card |
| Card | `rows` (`RowsKey`) | `[]object` — renders the first element (`rows[0]`) |
| Form | `values` (`PrefillKey`) | `{field: value}` prefill |
| Form | `errors` (`ErrorsKey`) | `{field: "message"}` server-side errors |
| Confirm | `rows` (`RowsKey`) | `[]object` — the record the operation targets (`rows[0]`) |
| Confirm | `effects` (`EffectsKey`) | `[]object` — side effects: `{text, detail?, value?, severity?}` |
| Choice | `rows` (`RowsKey`) | `[]object` — the record the question is about (`rows[0]`) |
| Choice | `options` (`OptionsKey`) | `[]object` — what is on offer: `{value, label?, summary?, body?, bullets?, details?, badge?, badgeVariant?, default?, disabled?}`, `details` being `[{label, value}]` |
| DatePicker | `value` (`ValueKey`) | `"YYYY-MM-DD"`, or `{start?, end?, min?, max?, disabled?}` — the selection and the window it may move in |
| DatePicker | `rows` (`RowsKey`) | `[]object` — the record the question is about (`rows[0]`) |
| Menu | — | reads nothing; its tiles are authored, not fetched |

`gomukit.RowsOf(slice)` converts typed Go slices to row maps (honors json
tags).

## Table

```go
table := &gomukit.Table{
    URI:   "ui://myapp/users",
    Title: "Users",
    Columns: []gomukit.Column{
        gomukit.Text("name", "Name"),
        gomukit.Number("balance", "Balance", "currency:EUR"),
        gomukit.Date("createdAt", "Created", "date"),
        gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
            "active": gomukit.BadgeSuccess,
            "banned": gomukit.BadgeDanger,
        }),
        gomukit.Link("website", "Website"),
        gomukit.ActionsColumn(
            gomukit.Action{
                Label: "Delete", Tool: "delete_user",
                Variant: gomukit.VariantDanger, Confirm: "Really delete?",
                Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
            },
        ),
    },
    PageSize:    10,
    PageSizes:   []int{10, 25, 50}, // page-size dropdown on the pagination bar
    DefaultSort: &gomukit.SortSpec{Key: "name"},
    Filterable:  true,
    Selection: &gomukit.SelectionConfig{Bulk: []gomukit.Action{{
        Label: "Archive", Tool: "archive_users",
        Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")},
    }}},
    Empty: gomukit.EmptyState{Title: "No users"},
}
```

- **Column types**: `text`, `number`, `date`, `badge`, `link`, `actions`.
  Number formats: `int`, `decimal:<digits>`, `percent`, `currency:<code>`;
  date formats: `date`, `datetime`, `time`, `relative` — all rendered via
  `Intl` in the host's locale/time zone.
- **Sorting/filtering/pagination** are client-side over the delivered rows.
  Text/number/date columns sort by default (`Sortable` overrides). `PageSizes`
  adds a page-size dropdown to the pagination bar; picking a size returns to
  the first page, and the bar stays visible even when everything fits on one.
- **`LoadMore`** replaces the pager with a "Load more" bar: the table starts
  at `PageSize` rows and appends the next `PageSize` per activation (requires
  `PageSize > 0`, excludes `PageSizes`). **`MaxHeight`** (a CSS length, e.g.
  `"20rem"`) caps the rows area: past it the rows scroll under a sticky header
  instead of the widget growing — and combined with `LoadMore`, reaching the
  bottom of that scroll loads the next batch by itself. Filter or sort changes
  start the run over at the first batch.
- **RowID** (default `"id"`) identifies rows for selection and args.
- **Actions**: `Kind` tool (default) calls an MCP tool; `Kind` link opens
  `HrefKey` via `ui/open-link`. Arg sources: `Static(v)`, `FromRow(field)`,
  `FromSelection(field)` (bulk only). `Confirm` opens a confirmation modal
  over the frame (native `confirm()` doesn't work in sandboxed iframes).
  An actions column renders one "⋯" trigger per row and bulk actions render an
  "Actions" trigger in the toolbar, each opening a menu of the labels — an
  actions column costs the same width whether it holds one action or five, and
  the confirmation is asked on the menu item.
- **`VisibleWhen`** limits an action to the records it applies to, so a switch
  shows one direction at a time instead of both:

  ```go
  gomukit.Action{
      Label: "Activate", Tool: "schedule_manage",
      VisibleWhen: gomukit.RowIs("state", "paused"),
      Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
  }
  ```

  `RowIs(key, value)` tests one value, `RowIn(key, values...)` a set, and
  `RowNot(p)` the complement; an absent predicate shows the action everywhere.
  The rows arrive after the document is rendered, so the test travels into the
  config island and runs per row in the browser — against raw record values,
  compared strictly by JSON type. Write predicates against machine values
  (a state code, a flag), never a display label, which the application
  localises. A row every action excludes renders no "⋯" trigger at all, and
  hiding a button never changes what the buttons beside it fire. Bulk actions
  stand over a selection rather than one record and reject the field.
- If an action's result contains `RowsKey`, the table re-renders with the
  returned rows and clears the selection — return the updated list from
  mutating tools.
- **`Prompt`** switches an action to the chat path, for hosts that answer a
  view-initiated `tools/call` without opening the widget behind it — see
  [The chat path](#the-chat-path) below.

## The chat path

An action calls its tool from the view and the widget handles the result. That
works when the tool answers with data. When it answers with a widget of its own
— an edit form opened from a row, a detail view opened from a menu tile —
opening it is the host's job, and a host that runs the call out of band opens
nothing: the tool runs and the user sees no change.

Setting a prompt routes that action through the host's chat: the view sends
`ui/message` with the text as a user turn, the model calls the tool, and the
widget arrives as that call's result.

```go
// Table row / card action
gomukit.Action{Label: "Edit", Tool: "edit_user",
    Prompt: "Open the edit form for this user"}

// Menu tile
gomukit.MenuItem{Tool: "list_customers", Label: "Customers",
    Prompt: "Show me the customer list"}

// Confirm, Choice, DatePicker — named ChatPrompt, since Prompt is the
// question already put to the reader
gomukit.AcceptSpec{Tool: "delete_user",
    ChatPrompt: "Delete the account for Ada"}
```

- Write it as the request a user would type; the model decides which tool
  answers, so `Tool` documents what the action opens and stays required.
- `Args` are dropped: the model chooses the arguments. The text is fixed and
  carries no row values.
- There is no result to inspect, so the widget only reports that the turn was
  accepted, or shows an error if the host refused it.
- Link actions may not set `Prompt` — a link already navigates on its own.
- `Choice` and `DatePicker` append the reader's decision to the text, since a
  chat turn has no argument to carry it: `"Ship order ORD-4471 — chose:
  Express"`, `"Book the room — chose: 2026-07-20 to 2026-07-24"`. Dates go as
  ISO whatever the grid rendered; options go by label, since a person reads it.

## Form

```go
form := &gomukit.Form{
    URI:   "ui://myapp/user-form",
    Title: "Edit user",
    Fields: []gomukit.Field{
        {Name: "id", Type: gomukit.FHidden},
        {Name: "name", Label: "Name", Required: true},
        {Name: "email", Label: "Email", Required: true,
            Validation: &gomukit.Validation{
                Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email.",
            }},
        {Name: "role", Label: "Role", Type: gomukit.FSelect, Required: true,
            Options: []gomukit.Option{gomukit.Opt("user"), gomukit.Opt("admin")}},
        {Name: "active", Label: "Active", Type: gomukit.FCheckbox, Default: true},
    },
    Submit: gomukit.SubmitSpec{Tool: "save_user", SuccessMessage: "Saved."},
    Cancel: &gomukit.CancelSpec{},
}
```

- **Field types**: `text`, `textarea`, `number`, `checkbox`, `select`,
  `multiselect`, `date`, `daterange`, `time`, `hidden`, `readonly`.
- **Groups and layout**: `FieldSets` are titled groups (`Title`,
  `Description`, `Fields`, and `Boxed` for a bordered panel), rendered after
  the form's own ungrouped `Fields`. `Columns` puts several fields on a row —
  1 to 4, set on the form and overridable per field set — and `Field.Span`
  lets one field take several of its group's columns. The grid gives columns
  back as the widget narrows (three or four drop to two under 46rem,
  everything to one under 34rem), so the same document reads in a chat pane
  and in a wide panel. Groups change nothing about submission: a grouped
  field shares the same name namespace and submits exactly like an ungrouped
  one.
- **Dropdowns**: `select` and `multiselect` render as the gomukit dropdown —
  the runtime upgrades the `<select>` into a trigger plus popup listbox
  (arrow keys, Home/End, typeahead, Escape, check marks) and keeps the select
  as the value holder, so validation and submitted value types are unchanged.
  `Placeholder` becomes the trigger's empty-state text.
- **Date fields**: `FDate` and `FDateRange` render the gomukit calendar — the
  runtime upgrades the native date input into a trigger showing the date in the
  host's locale, with the grid in a popover, and keeps the input as the value
  holder (so validation and the submitted value are unchanged, and a document
  whose script never runs still has a working date control). `Field.Calendar`
  configures the grid exactly as it configures the `DatePicker` widget: bounds,
  blocked days, presets, month travel — see [DatePicker](#datepicker).
  A range sends two flat arguments: `Name` carries the start and `EndName`
  (default `Name + "_end"`) the end, both `"YYYY-MM-DD"`. `Default` takes
  `[]string{start, end}`. An optional field's popover offers a Clear button.
- **Client validation** renders as native HTML attributes (`required`,
  `pattern`, `min`/`max`/`step`, `minlength`/`maxlength`) and is enforced
  before submit, with inline error messages (`Validation.Message` overrides
  the browser text).
- **Submit** is driven by the submit button's click, not by native form
  submission: hosts sandbox the widget iframe without `allow-forms`, which
  blocks a real submit before its event fires. Enter in a single-line field
  submits too. It calls `Submit.Tool` with `{field: value}` merged over
  `StaticArgs`. Value types: checkbox → bool, number → number (omitted when
  empty), multiselect → []string, everything else → string (hidden fields
  included — parse server-side).
- **Server-side errors**: return `{ErrorsKey: {"field": "message"}}` in
  `structuredContent`; they render inline and mark the form failed.
- **Edit mode**: a model-invoked tool linked to the form (e.g. `edit_user`)
  returns `{PrefillKey: {...}}`; the form prefills from the tool result.

### Field sets and columns

```go
form := &gomukit.Form{
    URI: "ui://myapp/employee", Title: "New employee",
    Columns: 2, // two fields per row, everywhere below unless overridden
    Fields: []gomukit.Field{
        // Ungrouped fields come first. Span 2 = the whole row.
        {Name: "workspace", Label: "Workspace", Type: gomukit.FReadonly, Span: 2},
    },
    FieldSets: []gomukit.FieldSet{
        {
            Title:       "Person",
            Description: "How the record reads everywhere it appears.",
            Fields: []gomukit.Field{
                {Name: "first", Label: "First name", Required: true},
                {Name: "last", Label: "Last name", Required: true},
                {Name: "email", Label: "Email", Required: true, Span: 2},
            },
        },
        {
            Title: "Employment", Boxed: true, // a bordered panel
            Fields: []gomukit.Field{
                {Name: "role", Label: "Role", Type: gomukit.FSelect,
                    Options: []gomukit.Option{gomukit.Opt("engineer"), gomukit.Opt("designer")}},
                {Name: "hours", Label: "Weekly hours", Type: gomukit.FNumber},
            },
        },
        {
            Title: "Notes", Columns: 1, // one column, whatever the form says
            Fields: []gomukit.Field{
                {Name: "notes", Label: "Anything the team should know", Type: gomukit.FTextarea},
            },
        },
    },
    Submit: gomukit.SubmitSpec{Tool: "create_employee", Label: "Create employee"},
}
```

## Card and CardList

`Card` renders one record; `CardList` renders many records as cards in a
horizontally scrolling strip (a carousel) with the same client-side machinery
as `Table` (filter, sort, pagination, selection + bulk actions, per-card
actions). The strip is what makes a collection usable in a chat pane, where a
table overflows and a card grid turns into a long vertical scroll. Both share a
`CardTemplate` and read the same `rows` contract as `Table` — so the same
list/mutation tools drive either.

```go
tmpl := gomukit.CardTemplate{
    Header: gomukit.CardHeader{
        TitleKey:       "name",
        DescriptionKey: "email",
        Badge: gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
            "active": gomukit.BadgeSuccess,
            "banned": gomukit.BadgeDanger,
        }),
    },
    Content: gomukit.CardContent{
        TextKey: "bio",
        Items: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
            {Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
            {Label: "Joined", Key: "createdAt", Type: gomukit.ColDate, Format: "relative"},
            {Label: "Website", Key: "website", Type: gomukit.ColLink,
                Link: &gomukit.LinkSpec{HrefKey: "website"}},
        }},
    },
    Footer: gomukit.CardFooter{
        Text: "Balances update hourly.",
        Actions: []gomukit.Action{
            {Label: "Edit", Tool: "edit_user", Variant: gomukit.VariantPrimary,
                Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")}},
        },
    },
}

cards := &gomukit.CardList{
    URI:         "ui://myapp/users",
    Title:       "Users",
    Template:    tmpl,
    PageSize:    12,
    PageSizes:   []int{12, 24, 48},
    DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
    Filterable:  true,
    Selection: &gomukit.SelectionConfig{Bulk: []gomukit.Action{{
        Label: "Archive", Tool: "archive_users",
        Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")},
    }}},
    Empty: gomukit.EmptyState{Title: "No users"},
}

card := &gomukit.Card{URI: "ui://myapp/user", Title: "User", Template: tmpl}
```

- **Template**: three sections, rendered in order.
  - `Header` — `TitleKey` (required) and `DescriptionKey`/`Description` name
    the record, and one end slot holds either `Badge` (a badge column, for a
    status pill) or `Action` (a button). Not both: it is one slot.
  - `Content` — `TextKey`/`Text` is a paragraph of body prose, and `Items` is
    the shared `Descriptions` block (same types and `Format` strings as table
    columns; a value the record lacks renders as `—`).
  - `Footer` — `TextKey`/`Text` is a note beside the buttons in `Actions`.

  A section with nothing in it is not rendered, so a title-and-actions card
  carries no empty chrome.
- **CardList** reuses `RowsKey`/`RowID`, `PageSize`, `PageSizes`,
  `DefaultSort`, `Filterable`, and `Selection` exactly as `Table`. The sort
  control is a dropdown auto-derived from the sortable content items;
  filtering matches the title, description, body text and item values.
- **Carousel**: prev/next controls appear only when the cards overflow the
  available width and disable at each end. The strip drags with the mouse,
  swipes on touch, scrolls with the arrow keys when focused, and hides its
  scrollbar. `PageSize` still applies and bounds how many cards sit in the
  strip at once. Card width is the `--gomu-card-width`
  token (default `17rem`), overridable per widget:
  `Theme: &theme.Theme{Extra: map[string]string{"--gomu-card-width": "22rem"}}`.
- **Card** renders `rows[0]`; use it as a detail view. Both support
  `InitialData` and `LoadTool`/`LoadArgs` load-time hydration, and re-render
  when an action or tool result returns `RowsKey`.
- **Actions** behave exactly as in tables (`Static`/`FromRow` args, inline
  `Confirm`, `Variant`, `VisibleWhen`); `FromSelection` is bulk-only via
  `CardList.Selection`. A record whose every action is hidden by its predicate
  renders no action bar, and no footer at all when the footer holds nothing
  else.

## Menu

A launcher: a responsive grid of tiles, one per tool the server exposes with a
UI. Choosing a tile calls that tool and the host opens the widget bound to it,
so a menu is the entry point an app hands the user before any record is in
view.

```go
menu := &gomukit.Menu{
    URI:   "ui://myapp/menu",
    Title: "Acme users",
    Intro: "Pick where to start.",
    Items: []gomukit.MenuItem{
        {
            Tool:         "list_users",
            Label:        "User table",
            Description:  "Sortable, filterable directory.",
            IconSVG:      `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 10v10"/></svg>`,
            Badge:        "read",
            BadgeVariant: gomukit.BadgeInfo,
        },
        {Tool: "edit_user", Args: map[string]any{"id": 1}, Label: "Edit Ada"},
    },
    Brand: brand,
}

// The tool that shows the menu returns no structured data of its own.
type empty struct{}
gosdk.AddWidgetToolFor(server, menu,
    &mcp.Tool{Name: "main_menu", Description: "Show the app menu."},
    func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
        return nil, empty{}, nil
    })
```

- **Authored, not fetched**: unlike the data widgets, the tiles are rendered
  server-side from `Items`. There is no `InitialData`, no `LoadTool`, and no
  `structuredContent` key — the document is complete the moment it is
  registered.
- **`Args`** are fixed values. A tile has no record behind it, so
  `Static`/`FromRow`/`FromSelection` do not apply.
- **`Prompt`** switches a tile to the chat launch path, for hosts that run a
  view-initiated `tools/call` out of band and so never open the bound widget.
  The view sends `ui/message` with the text as a user turn, the model calls the
  tool, and the widget arrives as that call's result. Write it as the request a
  user would type; `Args` are dropped for such a tile, since the model chooses
  the arguments. Tiles of both kinds can sit in one menu.
- **`Label`** defaults to `Tool`, so a bare `{Tool: "list_users"}` still
  renders a usable tile.
- **`IconSVG`** is inline markup, never a URL, with the same safety checks as
  `Brand.LogoSVG`. `Badge`/`BadgeVariant` add a short marker ("read", "beta")
  in the tile's top right.
- **While a tile is firing**, the whole grid is disabled — a second tile would
  race the first one's view swap — and the status region reads
  "Opening &lt;label&gt;…". A result with `isError` is shown there and the menu
  stays usable; otherwise nothing is rendered, because the host takes over. A
  `Prompt` tile has no result to inspect and clears its status as soon as the
  host accepts the turn.
- **Layout** reflows via CSS `auto-fill`, so one document works in a narrow
  chat pane and a wide panel. Minimum tile width is the
  `--gomu-menu-tile-min` token (default `11rem`), overridable per widget:
  `Theme: &theme.Theme{Extra: map[string]string{"--gomu-menu-tile-min": "14rem"}}`.

## Confirm

An approval view: it states an operation, shows the record it targets and the
side effects it will have, and offers exactly two outcomes. This is the long
form of `Action.Confirm` — that one re-labels a button for a second click and
has room for a few words; use `Confirm` when the consequences have to be
weighed before the reader decides.

```go
confirm := &gomukit.Confirm{
    URI:      "ui://myapp/delete-user",
    Title:    "Delete user",
    Prompt:   "Delete Ada Lovelace?",
    Body:     "The account and everything attached to it is removed for good.",
    Severity: gomukit.BadgeDanger,

    Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
        {Label: "User", Key: "name"},
        {Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
        {Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: map[string]gomukit.BadgeVariant{
            "active": gomukit.BadgeSuccess,
        }},
        {Label: "Region", Text: "eu-central-1"},
    }},
    Effects: []gomukit.Effect{
        {Text: "Removes the account", Severity: gomukit.BadgeDanger},
        {Text: "Deletes audit records", Value: "128", Severity: gomukit.BadgeWarning},
    },

    Acknowledge:   "I understand this cannot be undone.",
    TypeToConfirm: "ada@example.com",

    Accept: gomukit.AcceptSpec{
        Tool: "delete_user", Label: "Delete user",
        Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
        SuccessMessage: "User deleted.",
    },
    Reject: &gomukit.RejectSpec{Label: "Keep user", Message: "Nothing was deleted."},
}
```

- **Severity** (`BadgeInfo` default, `BadgeWarning`, `BadgeDanger`) colors the
  icon and picks the accept button's variant — danger gets the danger button,
  anything else the primary one. `Accept.Variant` overrides that.
- **Details** is a `Descriptions` block bound to `rows[0]`, the same record
  contract as `Card`. **Effects** are the consequences, one row each:
  `Text`, an optional `Detail` line, an optional `Value` shown at the row end
  ("128", "4 people"), and a `Severity` that colors the row's dot.
- **Authored now or delivered later**: the effects written in Go are what the
  widget shows until a tool result arrives with its own under `EffectsKey`,
  which replaces the list wholesale. Use `InitialData` for a baked snapshot and
  `LoadTool`/`LoadArgs` to fetch the record and effects fresh on load — worth
  it here, since the consequences should be current at decision time rather
  than at registration time.
- **Guards** add friction before the accept button enables: `Acknowledge`
  renders a checkbox that must be ticked, `TypeToConfirm` a phrase that must be
  typed exactly. Either, both or neither; the button renders `disabled`
  server-side when any is configured, so the document is correct before the
  runtime mounts.
- **Accept** calls `Accept.Tool` with `Static`/`FromRow` args (`FromSelection`
  is rejected — there is no selection). On success the buttons are replaced by
  `SuccessMessage`, or by the result's own text. A result with `isError` leaves
  the widget usable so a transient failure can be retried.
- **Reject** is optional. Without `Reject.Tool` declining is a local outcome
  the server never hears about; set the tool when the operation needs an
  explicit "no".
- **The decision is final**: once accepted or declined the buttons are gone and
  the outcome stays on screen, even if the host pushes later results.

## Choice

A deciding view: a question, the options answering it, and the case for each
one. Picking is local — nothing is called until the reader submits. Where
`Confirm` asks yes/no about one operation, `Choice` asks *which* operation.

```go
choice := &gomukit.Choice{
    URI:    "ui://myapp/shipping",
    Title:  "Shipping",
    Prompt: "How should we ship order ORD-4471?",
    Body:   "The parcel is packed and leaves the warehouse today either way.",

    Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
        {Label: "Order", Key: "reference"},
        {Label: "Destination", Text: "Berlin, DE"},
    }},

    Options: []gomukit.ChoiceOption{
        {
            Value: "standard", Label: "Standard", Summary: "3-5 business days",
            Body:    "Handed to the postal service tonight.",
            Bullets: []string{"Tracked to the depot", "No signature on delivery"},
            Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
                {Label: "Price", Key: "price", Type: gomukit.ColNumber, Format: "currency:EUR"},
                {Label: "Arrives", Key: "eta", Type: gomukit.ColDate, Format: "date"},
            }},
            Data:    map[string]any{"price": 4.9, "eta": "2026-08-03T10:00:00Z"},
            Default: true,
        },
        {
            Value: "express", Label: "Express", Summary: "next business day",
            Body:         "Arrives before 12:00 tomorrow.",
            Badge:        "fastest",
            BadgeVariant: gomukit.BadgeSuccess,
        },
        {Value: "pickup", Label: "Depot pickup", Summary: "no depot nearby", Disabled: true},
    },

    Submit: gomukit.ChoiceSubmit{
        Tool: "ship_order", Label: "Ship it", ValueArg: "method",
        Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
        SuccessMessage: "On its way.",
    },
    Cancel: &gomukit.RejectSpec{Label: "Decide later", Message: "Nothing was shipped."},
}
```

- **An option argues for itself.** `Label` and `Summary` are the list; `Body`,
  `Bullets` and `Details` are the description block. `Details` items typed with
  a `Key` read the option's own `Data` record, so prices and dates are
  formatted for the host's locale instead of being baked into strings.
- **Layout is about width, not taste.** `ChoiceSplit` puts the description in a
  side panel that follows the option in hand; `ChoiceStacked` unfolds it inside
  the chosen option. The default, `ChoiceAuto`, measures the width the host
  gave the widget — side panel at or above `34rem`, stacked below — and
  re-measures as the pane resizes, so the same document reads in a wide canvas
  and a narrow chat column.
- **One or several.** Without `Multiple` the options are radios and the last
  pick wins. With it they are checkboxes bounded by `Min` (default 1) and `Max`
  (0 = no limit): submit stays disabled until the floor is met, the unticked
  options disable at the cap rather than failing on click, and a hint under the
  list tracks the count.
- **Authored now or delivered later**: the options written in Go are what the
  widget shows until a tool result arrives with its own under `OptionsKey`,
  which replaces the list wholesale and re-applies the new defaults. Options
  from a tool describe themselves in plain values — `details` there is
  `[{label, value}]`, already formatted server-side. `LoadTool`/`LoadArgs`
  fetch them fresh on load, which is what a server pricing shipping at call
  time wants.
- **Submit** calls `Submit.Tool` with its `Static`/`FromRow` args plus
  `ValueArg` (default `"choice"`): the chosen `Value`, or the array of chosen
  values in option order. On success the controls are replaced by
  `SuccessMessage`, or by the result's own text; a result with `isError` leaves
  the widget usable so a transient failure can be retried.
- **`Disabled` options** are never chosen, but pointing at one still shows its
  description — why it cannot be taken is the thing worth reading.
- **The decision is final**: once submitted or cancelled the controls are gone
  and the outcome stays on screen, even if the host pushes later results.

### Descriptions

`Descriptions` is a shared block, not a widget: a label/value detail list used
by `Confirm`, `Choice` (both for the record and per option), `DatePicker` and a
card's content. Each item takes its value from a record field (`Key`, typed and
Intl-formatted exactly like a table cell) or from fixed
text authored in Go (`Text`) — one or the other, never both. An item can also
ask for a value instead of stating one (`Input`, below).

```go
gomukit.Descriptions{Items: []gomukit.DescriptionItem{
    {Label: "User", Key: "name"},
    {Label: "Joined", Key: "createdAt", Type: gomukit.ColDate, Format: "relative"},
    {Label: "Profile", Type: gomukit.ColLink, Link: &gomukit.LinkSpec{HrefKey: "website"}},
    {Label: "Region", Text: "eu-central-1"},
}}
```

- **Item types** are the `Column` types minus `actions`: `text` (default),
  `number`, `date`, `badge`, `link`, with the same `Format` strings.
- **No layout options.** The list takes as many columns as the widget's own
  width allows and collapses to one in a narrow pane. The smallest an item may
  get before a column is dropped is the `--gomu-desc-min` token (default
  `12rem`), overridable per widget:
  `Theme: &theme.Theme{Extra: map[string]string{"--gomu-desc-min": "16rem"}}`.
- **A value the record does not carry renders as an em dash** rather than
  disappearing: a reader deciding on these facts should see which are missing.

#### Items that ask: `Input`

An item with an `Input` renders a control in its value cell, and what the
reader puts in it travels with the widget's own call — the question a widget
asks in passing, rather than a form of its own.

```go
gomukit.Descriptions{Items: []gomukit.DescriptionItem{
    {Label: "Booking", Key: "reference"},
    {Label: "Guests", Key: "guests", Input: &gomukit.Input{
        Name: "guests", Type: gomukit.InputNumber, Default: 2, Required: true,
        Validation: &gomukit.Validation{Min: &one, Max: &six, Message: "Between one and six."},
    }},
    {Label: "Bed", Input: &gomukit.Input{
        Name: "bed", Type: gomukit.InputSelect, Placeholder: "Pick one",
        Options: []gomukit.Option{{Value: "double", Label: "One double"}, {Value: "twin", Label: "Two singles"}},
    }},
    {Label: "Arriving after 22:00", Input: &gomukit.Input{Name: "late", Type: gomukit.InputCheckbox}},
}}
```

- **Four control types**: `InputText` (the default), `InputNumber`,
  `InputSelect` (a gomukit dropdown over `Options`) and `InputCheckbox`. A
  number arrives as a number and a checkbox as a bool; an empty number sends
  no argument at all.
- **Only widgets that own a call take them.** `DatePicker.Details` merges the
  values into the submit call beside the picked date; a card's `Content.Items`
  merge them into every action button of that card (per record — a `CardList`
  keeps each card's answers to itself, and bulk actions get none). An `Input`
  in `Confirm.Details` or a `Choice`'s details is a validation error: nothing
  there would carry it.
- **`Name` is a tool argument** and must not collide with anything else the
  call already builds — the date arguments, `Submit.Args`, an action's `Args`.
- **Prefill order**: the reader's own answer, else the record field named by
  `Key`, else `Default`. Answers survive re-renders, so a tool result landing
  mid-answer replaces the values around the control, not in it.
- **Validation** is native and runs before the call: a required control that is
  empty, or one outside its bounds, blocks it and shows `Validation.Message`
  (or the browser's own text) under the control.

## DatePicker

A date, or the span between two, as the whole question. It is the standalone
form of the same calendar `FDate` and `FDateRange` open in a form: use the
widget when the date *is* the question ("when should this ship?", "which
nights?"), and the field when it is one answer among several.

```go
picker := &gomukit.DatePicker{
    URI:    "ui://myapp/booking",
    Title:  "Booking",
    Prompt: "Which nights should we hold the suite?",
    Mode:   gomukit.DateRange,
    Calendar: &gomukit.Calendar{
        Min:         "2026-08-01",
        Max:         "2026-12-31",
        Disabled:    []string{"2026-08-27", "2026-08-28"},
        WeekNumbers: true,
        Presets: []gomukit.DatePreset{
            {Label: "This week", Span: gomukit.SpanThisWeek},
            {Label: "Trade fair", Start: "2026-09-07", End: "2026-09-11"},
        },
    },
    Submit: gomukit.DateSubmit{
        Tool: "hold_room", Label: "Hold it",
        ValueArg: "from", EndArg: "until", SuccessMessage: "Held.",
    },
    Cancel: &gomukit.RejectSpec{},
}
```

- **The grid is inline**, not behind a trigger: a view whose only job is a
  calendar should not ask to be opened. Picking is local; only the submit
  button calls a tool, and it stays disabled until the selection is something
  the reader could submit (both ends, in a range).
- **Dates are days, not instants.** Everything travels as `"YYYY-MM-DD"`:
  no time, no zone, no offset. The host's time zone is used for exactly one
  thing — deciding which day is today, so a widget read in Auckland does not
  ring yesterday because the server is in Berlin.
- **Submit** calls `Submit.Tool` with its `Static`/`FromRow` args plus
  `ValueArg` (default `"date"`, or `"start"` in a range) and, in a range,
  `EndArg` (default `"end"`). Two flat string arguments, so a tool schema can
  declare two dates and a server can read them without unpacking anything.
- **Runtime state** arrives under `ValueKey` (default `"value"`): either a
  date string, or an object carrying the selection *and* the grid's limits —
  `{start, end, min, max, disabled}`. Which days are still free is exactly the
  kind of thing that changes between registration and the question, so
  `LoadTool`/`LoadArgs` fetch it fresh on load.
- **`Details`** describes the record the question is about (from `rows[0]`),
  the same block `Confirm` and `Choice` use — and, here, one that can ask as
  well as state: an item with an `Input` renders a control above the grid whose
  value travels with the submit call (see Descriptions → Items that ask). With
  `ChatPrompt` set there are no arguments to fill, so the answers are appended
  to the message text instead.
- **The decision is final**: once submitted or cancelled the controls are gone
  and the outcome stays on screen, even if the host pushes later results.

### Calendar

`Calendar` is a shared block, not a widget: the same grid configuration for the
`DatePicker` widget and for a form's `FDate`/`FDateRange` fields. The zero
value (a nil `*Calendar`) is a one-month grid with every day selectable, which
is what most fields want.

| Field | Effect |
|---|---|
| `Min`, `Max` | The selectable window, `"YYYY-MM-DD"`. The grid will not travel past the months holding them, and date fields render them as native `min`/`max` too. |
| `Disabled` | Individual days that cannot be picked — holidays, days already booked. A range may not straddle one. |
| `DisableWeekends` | Blocks every Saturday and Sunday. |
| `Months` | Months shown at once; defaults to 1 for a date and 2 for a range, maximum 4. They sit side by side where there is room and fall onto a second row where there is not — every month asked for is always on screen, so a span across a boundary is one gesture in a chat pane too. |
| `WeekNumbers` | A leading column of ISO 8601 week numbers. |
| `MonthDropdowns` | Month and year dropdowns in place of the caption, bounded by `FromYear`/`ToYear` (defaulting to the years of `Min`/`Max`). For dates of birth and anything else far from today. |
| `WeekStart` | Overrides the first day of the week; defaults to the host locale's own (`WeekStartMonday`, `WeekStartSunday`, `WeekStartSaturday`). |
| `StartOn` | The month the grid opens on while nothing is selected. |
| `Presets` | Named shortcuts: a rail beside the grid where there is room for one, a row above it otherwise. |

`DatePreset` names a window with either a fixed `Start`/`End` or a `Span`
resolved at runtime against the reader's today: `SpanToday`, `SpanYesterday`,
`SpanTomorrow`, `SpanLast7Days`, `SpanLast30Days`, `SpanLast90Days`,
`SpanNext7Days`, `SpanNext30Days`, `SpanThisWeek`, `SpanLastWeek`,
`SpanThisMonth`, `SpanLastMonth`, `SpanThisYear`, `SpanYearToDate`. A span is
the name of a rule rather than a pair of dates because a server cannot name
those dates at registration time: "the last 7 days" is a different week by the
time the widget is read. In a single-date calendar a preset picks the day its
window opens on.

A preset is measured against `Min`/`Max` and the blocked days as they stand when
the widget is read, and is shown as unavailable where it has nothing to offer —
a lit shortcut always does something. A range preset that overlaps the bounds is
trimmed to them: "last 30 days" against a calendar that opens on the 1st picks
the days of it there are. One wholly outside the bounds, or straddling a blocked
day, is switched off, since a shorter window would name something nobody asked
for. A single-date preset is never moved: it picks the day it names or is
switched off, so a "Today" before `Min` does not quietly pick another day.

- **Keyboard**: one tab stop for the whole grid, arrows by day and week,
  PageUp/PageDown by month (with Shift, by year), Home/End to the ends of the
  week, Enter or Space to pick, Escape to close a popover. A blocked day still
  takes focus — passing over it is how you get past it.
- **Locale**: month and weekday names, the first day of the week and the
  formatted value all come from the host's locale, which is why the grid is
  built at runtime rather than server-rendered.

## Branding

Every widget takes an optional `Brand`: a logo, a name, and a link identifying
the application the widget belongs to. One `*Brand` is normally shared by all
of a server's widgets.

```go
brand := &gomukit.Brand{
    Name:    "Acme",
    URL:     "https://acme.example",  // opened through the host
    LogoSVG: `<svg viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="8" r="7"/></svg>`,
}

table := &gomukit.Table{URI: "ui://myapp/users", Brand: brand /* … */}
```

- **Placement**: always the bottom left of the widget, leading the status bar
  ahead of the runtime's message. It is not part of the toolbar, so a widget
  with a brand and no `Title` renders no toolbar at all.
- **Logo**: prefer `LogoSVG` (inline markup, nothing needed from the host's
  CSP). `LogoDataURI` takes a `data:image/...;base64,...` string and renders an
  `<img>`, which depends on the host allowing `img-src data:` — not guaranteed
  by the spec. External logo URLs are never allowed: documents stay
  self-contained.
- **Safety**: `LogoSVG` is author input, checked against script-bearing and
  resource-loading constructs (`<script>`, `on*=` handlers, `<use>`,
  `<foreignObject>`, `javascript:`, …); a violation fails `Validate()` and
  therefore `Document()`.
- **Link**: a branded widget renders a button, not an anchor — navigation is
  blocked inside the host's sandboxed iframe, so the URL goes to the host via
  `ui/openLink`.

## Wiring with the official Go SDK

```go
server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

// Model-visible tool rendered by the table:
gosdk.AddWidgetToolFor(server, table,
    &mcp.Tool{Name: "list_users", Description: "List users."}, listUsers)

// App-only tool (fired by row actions, hidden from the model):
del := &mcp.Tool{Name: "delete_user", Description: "Delete a user."}
gosdk.AppOnly(del, table)
gosdk.AddWidgetToolFor(server, table, del, deleteUser)
```

`examples/demo` is a complete runnable server; `examples/harness` is a fake
host for manual verification without any MCP client.
