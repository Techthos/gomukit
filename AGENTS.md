# <img src="docs/assets/gomukit-icon.svg" alt="" width="28" align="center"> AGENTS.md — Working with `gomukit`

This document is the complete reference an LLM (or any agent) needs to **use**
the `gomukit` library. For repo-contribution guidance (build commands, asset
pipeline, invariants when editing this codebase) see `CLAUDE.md`; deeper design
docs live in `docs/architecture.md`, `docs/widgets.md`, `docs/theming.md`.

- **Module**: `github.com/techthos/gomukit` (Go >= 1.25)
- **What it is**: prebuilt, parameterized, interactive HTML widgets (**Table**,
  **CardList**, **Card**, **Form**, **Menu**, **Confirm**, **Choice**,
  **DatePicker**) for **MCP Apps** — the official MCP UI extension
  (`io.modelcontextprotocol/ui`, spec `2026-01-26`). Widgets render as fully
  self-contained HTML documents (inline CSS + JS, no external references)
  served as `ui://` template resources from a Go MCP server. Hosts (Claude,
  ChatGPT, VS Code, Cursor, Goose, Postman, …) render them in a sandboxed
  iframe inside the chat.
- **Status**: pre-release; APIs are not stable yet.
- **License**: MIT

---

## 1. Mental model (read this first)

The MCP Apps spec uses a **template model**: the HTML resource is registered
once and cannot contain per-call data. Data arrives at runtime. `gomukit`
therefore splits rendering:

1. **Go renders structure** (registration time): the widget shell (table
   chrome, form fields with native validation attributes, card chrome) plus a
   `#gomu-config` JSON island describing columns/fields/action bindings, and
   an optional `#gomu-data` snapshot (`InitialData`).
2. **The embedded TypeScript runtime renders data** (runtime, inside the
   host's sandboxed iframe): rows, prefill values, errors — first from the
   snapshot, then from every `ui/notifications/tool-result` notification and
   every widget-initiated `tools/call` response. All formatting uses `Intl`
   with the host's locale/time zone.

Consequences for you as a library user:

- You never write HTML, CSS, or JavaScript. You declare a widget struct, link
  tools to it, and return **`structuredContent`-shaped data** from tool
  handlers.
- The widget ↔ server contract is entirely about **which keys appear in the
  tool result's `structuredContent`** (see section 4).
- Sorting, filtering, pagination, and selection are **client-side** over the
  rows delivered — there is no server round-trip for them.

### Package map

| Package | Import path | Role |
|---|---|---|
| `gomukit` | `github.com/techthos/gomukit` | Widget definitions (`Table`, `Form`, `Card`, `CardList`, `Menu`, `Confirm`, `Choice`, `DatePicker`, `Action`, columns, fields) + `RowsOf` |
| `theme` | `github.com/techthos/gomukit/theme` | `Theme` struct → CSS design-token overrides |
| `uispec` | `github.com/techthos/gomukit/uispec` | MCP Apps spec constants and `_meta` types (zero deps) |
| `gosdk` | `github.com/techthos/gomukit/gosdk` | Adapter for the official `github.com/modelcontextprotocol/go-sdk` — the **only** package importing an MCP SDK |

The core is SDK-agnostic: with any other Go MCP implementation, wire widgets
manually via the `Widget` interface (section 8).

---

## 2. Quickstart (official go-sdk)

```go
package main

import (
    "context"
    "net/http"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/techthos/gomukit"
    "github.com/techthos/gomukit/gosdk"
)

func main() {
    table := &gomukit.Table{
        URI:   "ui://myapp/users",
        Title: "Users",
        Columns: []gomukit.Column{
            gomukit.Text("name", "Name"),
            gomukit.Number("balance", "Balance", "currency:EUR"),
            gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
                "active": gomukit.BadgeSuccess,
            }),
        },
        Filterable: true,
        PageSize:   10,
    }

    // EnableUI declares the MCP Apps extension capability.
    server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

    type in struct{}
    type out struct {
        Rows []map[string]any `json:"rows"` // key must match Table.RowsKey (default "rows")
    }
    gosdk.AddWidgetToolFor(server, table,
        &mcp.Tool{Name: "list_users", Description: "List users in a table."},
        func(context.Context, *mcp.CallToolRequest, in) (*mcp.CallToolResult, out, error) {
            rows, _ := gomukit.RowsOf(loadUsers())
            return nil, out{Rows: rows}, nil
        })

    h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
    http.ListenAndServe(":8080", h)
}
```

The typed handler's `out` struct is serialized by the SDK into the tool
result's `structuredContent`; the widget reads its data from there.

---

## 3. Package `gomukit` — full API reference

### 3.1 The `Widget` interface

`*Table`, `*Form`, `*Card`, `*CardList`, `*Menu`, `*Confirm`, `*Choice`, and
`*DatePicker` implement:

```go
type Widget interface {
    Document() (string, error)              // complete self-contained HTML document (calls Validate first)
    Descriptor() uispec.ResourceDescriptor  // registration data for the ui:// template resource
    ToolMeta() map[string]any               // tool _meta linking the tool to this widget: {"ui": {"resourceUri": ...}}
    Validate() error                        // checks the widget configuration
}
```

With `gosdk` you rarely call these yourself; they exist for manual wiring.

### 3.2 Shared types

```go
type Align string        // AlignStart | AlignCenter | AlignEnd

type SortSpec struct {   // default sort order for a table
    Key  string `json:"key"`
    Desc bool   `json:"desc,omitempty"`
}

type EmptyState struct { // no-data message (Table.Empty)
    Title     string `json:"title,omitempty"` // defaults to "No data" when rendered
    Body      string `json:"body,omitempty"`
    Immediate bool   `json:"immediate,omitempty"` // show it on first paint, skip the skeleton
}
```

**Loading vs. empty.** A widget rendered without `InitialData` is *not loaded yet*, not
empty. It shows a loading skeleton and reaches its empty state only once data actually
resolves: a `ui/notifications/tool-result`, a `LoadTool` response (success or failure),
a cancelled call, a handshake that no host answered, or a 1.5s wait for a host that
pushes none of these. Set `Empty.Immediate` for a widget that is genuinely empty at
render time with no data coming, to assert the empty state on first paint instead.
`Form` has no skeleton — its fields are structure, not data — but a form with a
`LoadTool` keeps its controls disabled until the prefill lands or the load fails, so
nothing typed is overwritten.

### 3.3 `Table`

```go
type Table struct {
    URI     string      // REQUIRED. ui:// resource URI, e.g. "ui://myapp/users"
    Title   string      // toolbar heading + document title
    Columns []Column    // REQUIRED, non-empty

    RowsKey string      // structuredContent key holding the rows array. Default "rows"
    RowID   string      // row field uniquely identifying a row (selection, FromRow/FromSelection). Default "id"

    PageSize    int              // > 0 enables client-side pagination (page size); 0 disables; < 0 is invalid
    PageSizes   []int            // alternative page sizes offered in a dropdown on the pagination bar; entries > 0; needs PageSize > 0; PageSize is added if absent; empty renders no chooser
    LoadMore    bool             // grow the list instead of paging it: starts at PageSize rows and appends PageSize more per "Load more" bar, in place of the pagination bar; with MaxHeight set, reaching the bottom of the scroll area also loads the next batch; needs PageSize > 0; cannot be combined with PageSizes
    MaxHeight   string           // CSS length (e.g. "20rem") capping the rows area; past it the rows scroll inside the widget under a sticky header row
    DefaultSort *SortSpec        // pre-sort rows on load (Key required when set)
    Filterable  bool             // adds a client-side text filter box
    Selection   *SelectionConfig // enables row checkboxes + bulk actions
    Empty       EmptyState       // no-data message

    InitialData map[string]any        // optional structuredContent-shaped snapshot baked into the document
    LoadTool    string                 // read tool the runtime calls once on load to re-fetch rows (must return them under RowsKey), replacing the baked snapshot so a reloaded widget shows current data
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand                 // application logo/name, shown bottom left in the status bar
    Theme       *theme.Theme          // design-token overrides for this widget
    UI          *uispec.ResourceUIMeta // overrides resource _meta.ui (CSP, permissions, prefersBorder)
}

type SelectionConfig struct {
    Bulk []Action // toolbar actions shown while rows are selected; FromSelection args resolve across all selected rows
}
```

Rows are string-keyed JSON objects (`[]map[string]any` /
`[]object`) delivered at runtime under `RowsKey`.

### 3.4 `Column`

```go
type Column struct {
    Key      string                  // row field this column displays (REQUIRED except for ColActions)
    Label    string
    Type     ColumnType              // defaults to ColText
    Sortable *bool                   // overrides default sortability
    Align    Align
    Format   string                  // see formats below
    Badge    map[string]BadgeVariant // ColBadge: cell value -> variant
    Link     *LinkSpec               // ColLink config
    Actions  []Action                // ColActions: per-row actions, shown in a "⋯" menu
    Width    string                  // CSS width, e.g. "12rem", "20%"
}
```

**Column types** (`ColumnType`): `ColText` (`"text"`, zero-value default),
`ColNumber`, `ColDate`, `ColBadge`, `ColLink`, `ColActions`.

**Formats** (interpreted by the runtime via `Intl`, host locale/time zone):

| Type | Format values |
|---|---|
| number | `"int"`, `"decimal:<digits>"`, `"percent"`, `"currency:<code>"` (e.g. `"currency:EUR"`) |
| date | `"date"`, `"datetime"`, `"time"`, `"relative"` |

**Sortability defaults**: text/number/date columns are sortable; badge, link,
and actions columns are not. Set `Sortable: &b` to override either way.

**Badge variants** (`BadgeVariant`): `BadgeNeutral`, `BadgeInfo`,
`BadgeSuccess`, `BadgeWarning`, `BadgeDanger`.

**Link columns**:

```go
type LinkSpec struct {
    HrefKey string `json:"hrefKey"`           // REQUIRED: row field holding the URL
    TextKey string `json:"textKey,omitempty"` // row field holding the link text …
    Text    string `json:"text,omitempty"`    // … or a fixed text; else the URL itself is shown
}
```

**Column constructors** (sugar — plain struct literals work too):

```go
gomukit.Text(key, label)                    // text column
gomukit.Number(key, label, format...)       // number column, right-aligned (Align: AlignEnd)
gomukit.Date(key, label, format...)         // date column
gomukit.Badge(key, label, variants)         // badge column
gomukit.Link(hrefKey, label)                // link column (Key and Link.HrefKey both set to hrefKey)
gomukit.ActionsColumn(actions...)           // per-row actions column (empty Label)
```

### 3.5 `Form`

```go
type Form struct {
    URI       string     // REQUIRED. ui:// resource URI
    Title     string     // heading + document title
    Fields    []Field    // ungrouped fields, rendered above FieldSets
    FieldSets []FieldSet // titled groups of fields, rendered in order after Fields
    Columns   int        // fields per row: 1 (default) .. 4
    Submit    SubmitSpec  // REQUIRED (Submit.Tool must be set)
    Cancel    *CancelSpec // when set, adds a reset button

    PrefillKey string  // structuredContent key with {"field": value} prefill. Default "values"
    ErrorsKey  string  // structuredContent key with {"field": "message"} errors. Default "errors"

    InitialData map[string]any         // optional snapshot, e.g. {"values": {...}} for a pre-filled edit form
    LoadTool    string                 // read tool the runtime calls once on load to re-fetch prefill (must return it under PrefillKey), replacing the baked snapshot
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}

type SubmitSpec struct {
    Tool           string         // REQUIRED: MCP tool called with {field: value, ...} merged over StaticArgs
    Label          string         // button label, default "Submit"
    StaticArgs     map[string]any // fixed args merged UNDER the field values (field values win)
    SuccessMessage string         // shown after a successful submit
}

type CancelSpec struct {
    Label string // default "Cancel"
}

type FieldSet struct {
    Title       string  // REQUIRED: names the group
    Description string  // a line under the title
    Fields      []Field // REQUIRED, non-empty
    Columns     int     // overrides Form.Columns for this group (1..4); 0 inherits
    Boxed       bool    // draw as a bordered panel with a filled header
}
```

A form needs at least one field, in `Fields` or in a `FieldSet`. Grouped
fields are fields: they share one name/argument namespace with everything
else in the form, submit identically, prefill identically, and the runtime is
told nothing about the groups. A `FieldSet` renders as a `<fieldset>` named
by its title through `aria-labelledby`.

**Layout.** `Form.Columns` (1..4, default 1) is how many fields share a row;
a `FieldSet` may set its own, and `Field.Span` lets one field take several of
its group's columns (`Span: 2` in a two-column group = the whole row). The
grid narrows with the widget — three and four columns drop to two under
46rem, everything drops to one column (and every span is ignored) under 34rem
— so a multi-column form still reads in a chat pane. The document's own
maximum width follows its widest group (36rem for one column, up to 72rem for
four).

```go
form := &gomukit.Form{
    URI: "ui://myapp/employee", Title: "New employee", Columns: 2,
    Fields: []gomukit.Field{
        {Name: "workspace", Label: "Workspace", Type: gomukit.FReadonly, Span: 2},
    },
    FieldSets: []gomukit.FieldSet{{
        Title: "Person", Description: "How the record reads everywhere it appears.",
        Fields: []gomukit.Field{
            {Name: "first", Label: "First name", Required: true},
            {Name: "last", Label: "Last name", Required: true},
            {Name: "email", Label: "Email", Span: 2},
        },
    }, {
        Title: "Notes", Columns: 1, Boxed: true,
        Fields: []gomukit.Field{{Name: "notes", Label: "Notes", Type: gomukit.FTextarea}},
    }},
    Submit: gomukit.SubmitSpec{Tool: "create_employee"},
}
```

### 3.6 `Field`

```go
type Field struct {
    Name        string      // REQUIRED, unique: the tool-call argument name
    Label       string      // defaults to Name when rendered
    Description string      // help text under the control
    Placeholder string
    Type        FieldType   // defaults to FText
    Required    bool
    Default     any         // initial value: string-like for most; bool for FCheckbox; []string (or string) for FMultiSelect
    Options     []Option    // REQUIRED for FSelect / FMultiSelect
    Validation  *Validation // client-side constraints (date fields take bounds from Calendar instead)
    Rows        int         // textarea height (FTextarea), default 3
    Span        int         // columns the field occupies within its group: 1 (default) .. the group's Columns

    Calendar *Calendar // FDate / FDateRange only: the grid the field opens (see 3.16.1)
    EndName  string    // FDateRange only: argument carrying the range's end. Default Name + "_end"
}
```

**Field types** (`FieldType`): `FText` (`"text"`, zero-value default),
`FTextarea`, `FNumber`, `FCheckbox`, `FSelect`, `FMultiSelect`, `FDate`,
`FDateRange`, `FTime`, `FHidden`, `FReadonly`.

`FSelect` and `FMultiSelect` render as the gomukit dropdown: the runtime
upgrades the `<select>` into a styled trigger and popup listbox (keyboard
navigation, typeahead, check marks on the chosen entries) while the select
itself stays the value holder, so submitted value types are unchanged. Every
other select in the library — the CardList sort control, the pagination bar's
page-size chooser — is the same control.

If a `Placeholder` is set on a select field, it is the empty-state text of the
trigger.

`FDate` and `FDateRange` render the gomukit calendar (the same grid the
`DatePicker` widget renders inline, configured by `Field.Calendar` — see 3.16.1):
the runtime upgrades the native date input into a trigger showing the date in
the host's locale, with the grid in a popover, while the input stays the value
holder. A document whose script never runs still has a working date control.
`FDateRange` renders two date inputs, named `Name` and `EndName`, and submits
two flat `"YYYY-MM-DD"` arguments; its `Default` is `[]string{start, end}`. An
optional date field's popover offers a Clear button. `Placeholder` is the
trigger's empty-state text.

```go
type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
gomukit.Opt("admin") // Option{Value: "admin", Label: "admin"}
```

**Client-side validation** — rendered as native HTML attributes and enforced
by the runtime before submit (inline error messages; `Message` overrides the
browser's text):

```go
type Validation struct {
    Pattern string   // HTML pattern-attribute regex
    Min     *float64 // number/date/time constraints
    Max     *float64
    Step    *float64
    MinLen  *int     // text length constraints
    MaxLen  *int
    Message string   // overrides the browser's validation message
}
```

**Submitted value types** (what your submit tool receives as arguments):

| Field type | Submitted as |
|---|---|
| `FCheckbox` | `bool` |
| `FNumber` | number (**omitted entirely when empty**) |
| `FMultiSelect` | `[]string` |
| `FDate` | `string` `"YYYY-MM-DD"` (`""` when empty) |
| `FDateRange` | two arguments: `Name` = start, `EndName` = end, both `"YYYY-MM-DD"` |
| everything else — including `FHidden` and `FReadonly` | `string` (parse server-side, e.g. hidden numeric IDs arrive as `"3"`) |

### 3.7 `Action`

An `Action` is a user-triggerable operation: a per-row action
(`ActionsColumn`), a bulk action over selected rows (`SelectionConfig.Bulk`),
or a link.

In a `Table`, both kinds are reached through a menu rather than a strip of
buttons: an actions column renders one "⋯" trigger per row, and the bulk bar
renders a single "Actions" trigger beside the selection count. `Confirm` is
asked inside that menu, on the item itself. `Card`/`CardList` actions are
still buttons.

```go
type Action struct {
    Label   string               // REQUIRED
    Kind    ActionKind           // ActionTool (default) | ActionLink
    Tool    string               // MCP tool name (REQUIRED for ActionTool)
    Args    map[string]ArgSource // tool argument name -> value source; ignored when Prompt is set
    Prompt  string               // post this text as a user turn instead of calling Tool (ActionTool only)
    HrefKey string               // row field holding the URL (REQUIRED for ActionLink; opens via ui/open-link)
    Confirm string               // when set: a confirmation modal poses this text over the frame before firing
    Variant ActionVariant        // VariantDefault ("") | VariantPrimary | VariantDanger
    VisibleWhen RowPredicate     // draw the action only on records matching this test; absent = every record
}
```

**Per-record visibility** (`RowPredicate` is opaque — construct ONLY with
these; the zero value is absent):

```go
gomukit.RowIs("state", "paused")               // field equals the value
gomukit.RowIn("state", "paused", "failed")     // field equals any of the values
gomukit.RowNot(gomukit.RowIs("state", "running")) // the complement of a predicate
```

The document is rendered once, before any record exists, so the test travels
into the config island as data (`{"visibleWhen":{"key":"state","equals":"paused"}}`,
`"in":[…]` for a set, plus `"not":true` when negated) and the runtime evaluates
it per record. Consequences:

- Compare **raw record values**, never a display label: the comparison is
  strict (same JSON type and value), and a predicate written against a
  translated word would match in one language and silently hide the button in
  every other. `RowIs("count", 0)` does not match `"0"`.
- A missing field and a missing record both read as `null`, so
  `RowIs("field", nil)` matches records that lack it.
- Hidden buttons keep their **index** in the full action list, so a click always
  fires the tool the button declares.
- In a `Table`, a row whose actions are all hidden renders **no "⋯" trigger**;
  in `Card`/`CardList` such a record renders **no action bar** (and no footer at
  all when the footer carries nothing else).
- Bulk actions (`SelectionConfig.Bulk`) reject `VisibleWhen` (validation error):
  they stand over a selection, not over one record.

**Argument sources** (`ArgSource` is opaque — construct ONLY with these):

```go
gomukit.Static(v)             // fixed value
gomukit.FromRow("field")      // value of the field on the row the action was triggered on
gomukit.FromSelection("field") // values of the field across ALL selected rows — bulk actions ONLY
```

`FromSelection` in a per-row (column) action is a validation error. An
`ArgSource` built any other way (zero value, multiple sources) fails
validation/marshaling.

**Behavior contract**:

- `Confirm` opens a confirmation modal over the frame (the `Confirm` text as
  the question, plus cancel/confirm buttons); the action fires only on a
  deliberate confirm. Native `confirm()` dialogs are silently disabled in
  sandboxed MCP Apps iframes — never rely on them.
- **If a tool called by a table action returns a result whose
  `structuredContent` contains `RowsKey`, the table re-renders with the
  returned rows and clears the selection.** Therefore: mutating tools
  (delete/archive/…) should return the updated full row list.

#### The chat path (`Prompt` / `ChatPrompt`)

An action normally calls its tool from the view, and the widget handles the
result. That works when the tool answers with data. When it answers with a
widget of its own — an edit form, a detail view — opening it is the host's job,
and a host that runs a view-initiated `tools/call` out of band opens nothing:
the tool runs, and the user sees no change.

Setting the prompt field routes that action through the host's chat instead.
The view sends `ui/message` with the text as a user turn, the model calls the
tool, and the widget arrives as that call's result. The field is named `Prompt`
on `Action` and `MenuItem`, and `ChatPrompt` on `AcceptSpec`, `ChoiceSubmit`
and `DateSubmit` — those widgets already use `Prompt` for the question put to
the reader.

Common to all of them:

- Write it as the request a user would type. The model decides which tool
  answers, so `Tool` documents what the action opens and stays required.
- `Args` are dropped from the config island: the model chooses the arguments.
- The text is fixed and carries no row values — the model works out which
  record is meant from the conversation.
- There is no tool result to inspect, so the widget reports only that the turn
  was accepted, and shows an error if the host refused it.
- Link actions may not set `Prompt` (validation error): a link already
  navigates on its own.

`Choice` and `DatePicker` append the reader's decision to the text, because a
chat turn has no argument to carry it and the decision is the whole point of
those widgets:

```go
// Choice, reader picked "Express":
//   "Ship order ORD-4471 — chose: Express"
// DatePicker over a range:
//   "Book the room — chose: 2026-07-20 to 2026-07-24"
```

Dates go as ISO regardless of the locale the grid rendered, and options go by
label rather than value, since a person reads the turn.

### 3.8 `RowsOf`

```go
func RowsOf(slice any) ([]map[string]any, error)
```

Converts a typed slice (e.g. `[]User` or `[]*User`) into row maps via
`encoding/json`, honoring `json` struct tags. Use it to feed typed data into
`Table.InitialData` or a tool result:

```go
rows, err := gomukit.RowsOf(users)
table.InitialData = map[string]any{"rows": rows}
```

Errors if the value doesn't marshal to a JSON array of objects.

### 3.9 `CardTemplate`

Shared by `Card` and `CardList`: describes how one record renders as a card,
in three sections rendered in this order. Only `Header` is required; a section
with nothing in it is not rendered at all.

```go
type CardTemplate struct {
    Header  CardHeader  // REQUIRED (it holds the title)
    Content CardContent // the card body
    Footer  CardFooter  // the bottom section
}

type CardHeader struct {
    TitleKey       string  // REQUIRED: row field shown as the card title
    DescriptionKey string  // row field shown under the title
    Description    string  // fixed text under the title, instead of DescriptionKey
    Badge          Column  // status badge for the header's end slot (build with gomukit.Badge); present when its Key is set — must be a badge column
    Action         *Action // button for the header's end slot, instead of Badge
}

type CardContent struct {
    TextKey string       // row field rendered as a paragraph of body text
    Text    string       // fixed body prose, instead of TextKey
    Items   Descriptions // label/value detail rows (the shared Descriptions block)
}

type CardFooter struct {
    TextKey string   // row field shown as a footer note
    Text    string   // fixed footer note, instead of TextKey
    Actions []Action // footer buttons; FromSelection args are invalid here (bulk actions belong to CardList.Selection)
}
```

- The header has **one** end slot: set `Badge` or `Action`, never both.
- `Content.Items` is the same `Descriptions` block used by `Confirm`, so card
  fields carry the same types, `Format` strings, badge maps and links as a
  table column, and a value the record does not carry renders as `—`.
- Text slots are either/or: `DescriptionKey`/`Description`,
  `Content.TextKey`/`Content.Text` and `Footer.TextKey`/`Footer.Text` each
  reject being set together.
- Action buttons are indexed header-first, then footer — relevant only to the
  runtime, not to callers.

### 3.10 `Card`

Renders a **single record** — the first element of the rows array delivered
under `RowsKey` (same data contract as `Table`/`CardList`).

```go
type Card struct {
    URI      string       // REQUIRED. ui:// resource URI
    Title    string       // toolbar heading + document title
    Template CardTemplate // REQUIRED

    RowsKey string // structuredContent key holding the rows array; renders rows[0]. Default "rows"
    RowID   string // record field used for FromRow action args. Default "id"
    Empty   EmptyState // shown when no record is present

    InitialData map[string]any         // optional snapshot, e.g. {"rows": [{...}]}
    LoadTool    string                 // read tool called once on load to re-fetch the record (under RowsKey), replacing the snapshot
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}
```

### 3.11 `CardList`

Renders a **collection** as cards in a horizontally scrolling strip (a
carousel), with the same client-side runtime as `Table` (filter, sort,
pagination, selection + bulk actions, per-card actions, load-time hydration) —
laid out as cards instead of table rows. The strip is the only layout: it fits
a narrow chat pane, where a table overflows and a card grid collapses into a
long vertical scroll.

```go
type CardList struct {
    URI      string       // REQUIRED. ui:// resource URI
    Title    string       // toolbar heading + document title
    Template CardTemplate // REQUIRED

    RowsKey string // structuredContent key holding the rows array. Default "rows"
    RowID   string // record field identifying a card (selection, FromRow/FromSelection). Default "id"

    PageSize    int              // > 0 enables client-side pagination; 0 disables; < 0 invalid
    PageSizes   []int            // alternative page sizes offered in a dropdown on the pagination bar; entries > 0; needs PageSize > 0; PageSize is added if absent; empty renders no chooser
    LoadMore    bool             // grow the strip instead of paging it: starts at PageSize records and appends PageSize more per "Load more" tile at the end of the strip, in place of the pagination bar; needs PageSize > 0; cannot be combined with PageSizes
    DefaultSort *SortSpec        // pre-sort records on load (Key required when set)
    Filterable  bool             // adds a client-side text filter box (matches title, description, body text, and content item values)
    Selection   *SelectionConfig // per-card checkboxes + bulk actions (FromSelection resolves across selected cards)
    Empty       EmptyState

    InitialData map[string]any
    LoadTool    string                 // read tool called once on load to re-fetch records (under RowsKey), replacing the snapshot
    LoadArgs    map[string]any
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}
```

The sort control is a dropdown over the template's **sortable content items**
(text/number/date `Content.Items` reading a record field; badge/link items,
fixed-text items and the header slots are not offered) — no config needed.
`DefaultSort` sets the initial order and may reference any field key.

Carousel behavior is automatic: prev/next controls appear only when the cards
overflow the available width and disable at each end, the strip is draggable
with the mouse and swipeable on touch, and its scrollbar is hidden. `PageSize`
still applies and bounds how many cards are in the strip at once. Card width comes from the
`--gomu-card-width` token (default `17rem`), overridable per widget through
`theme.Theme.Extra`.

### 3.12 `Menu`

The app's front door: a responsive grid of tiles, one per tool the server
exposes with a UI. Choosing a tile calls that tool, and the host opens the
widget bound to it — a menu item is navigation, not an action with a result of
its own.

Unlike the data widgets, a `Menu` is **fully authored at registration time**:
the tiles are server-rendered from `Items`, the document carries no
`#gomu-data` island, and the menu reads nothing from `structuredContent`.
The config island holds only the tool name and static args behind each tile,
matched positionally to the rendered buttons.

```go
type Menu struct {
    URI   string      // ui:// resource URI (required)
    Title string      // toolbar + document title
    Intro string      // optional lead text above the tiles
    Items []MenuItem  // at least one required

    Brand *Brand                  // application logo/name
    Theme *theme.Theme            // design token overrides
    UI    *uispec.ResourceUIMeta  // resource _meta.ui override
}

type MenuItem struct {
    Tool         string         // MCP tool called when the item is chosen (required)
    Args         map[string]any // static arguments passed to Tool; ignored when Prompt is set
    Prompt       string         // post this text as a user turn instead of calling Tool
    Label        string         // tile heading; defaults to Tool
    Description  string         // supporting line under the label
    IconSVG      string         // inline <svg> markup shown above the label
    Badge        string         // short marker in the tile's top right ("read", "beta")
    BadgeVariant BadgeVariant   // colors the badge; defaults to BadgeNeutral
}
```

```go
menu := &gomukit.Menu{
    URI:   "ui://demo/menu",
    Title: "Acme users",
    Intro: "Pick where to start.",
    Items: []gomukit.MenuItem{
        {Tool: "list_users", Label: "User table",
         Description: "Sortable, filterable directory.",
         Badge: "read", BadgeVariant: gomukit.BadgeInfo},
        {Tool: "edit_user", Args: map[string]any{"id": 1}, Label: "Edit Ada"},
    },
}

// The tool that shows the menu returns no structured data of its own.
type empty struct{}
gosdk.AddWidgetToolFor(server, menu,
    &mcp.Tool{Name: "main_menu", Description: "Show the app menu."},
    func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
        return nil, empty{}, nil
    })
```

`MenuItem.Args` are fixed values, not row lookups: a menu tile has no record
behind it, so `Static`/`FromRow`/`FromSelection` do not apply here.

**`Prompt`: the chat launch path.** A plain tile assumes the host opens the
widget bound to a view-initiated `tools/call`. Not every host does: one that
runs such a call out of band answers it and opens nothing, so the tile shows
its "Opening …" status for the length of the call and then looks inert. Setting
`Prompt` routes that tile through the host's chat instead — the view sends
`ui/message` with the text as a user turn, the model calls the tool, and the
widget arrives as that call's result:

```go
{Tool: "list_customers", Label: "Customers",
 Prompt: "Show me the customer list"},
```

Write `Prompt` as the request a user would type. The model decides which tool
answers it, so `Tool` documents intent and supplies the default `Label`, and
`Args` are dropped from the config island — the model chooses the arguments.
One menu may mix both kinds of tile freely.

Runtime behavior: the whole grid goes inert while a tile's call is in flight
(a second tile would race the first one's view swap), a `loading` status reads
"Opening &lt;label&gt;…", and a tool result that comes back with `isError` is
shown in the status region with the menu left usable. Nothing else is rendered
from the result — the host is expected to take over the view. A `Prompt` tile
carries no tool result to inspect, so it clears its status as soon as the host
accepts the turn. Tile width comes from the `--gomu-menu-tile-min` token
(default `11rem`), overridable per widget through `theme.Theme.Extra`.

Documents are self-contained, so `IconSVG` is inline markup, never a URL — the
same trust level and the same checks as `Brand.LogoSVG`.

### 3.13 `Descriptions`

A label/value detail list. **Not a widget**: no URI, no `Document()`, not
registerable. It is a shared block embedded by value, used by `Confirm`, `Choice` (both
for the record and per option), `DatePicker` and a card's content section. Its
items normally state a value; in the widgets that own a call they can also ask
for one (see Editable items below).

```go
type Descriptions struct {
    Items []DescriptionItem
}

type DescriptionItem struct {
    Label  string                  // required
    Key    string                  // record field holding the value (prefill source on an Input item)
    Text   string                  // fixed authored value, used instead of Key
    Type   ColumnType              // ColText (default), ColNumber, ColDate, ColBadge, ColLink
    Format string                  // same Intl format strings as Column.Format
    Badge  map[string]BadgeVariant // value -> variant (ColBadge)
    Link   *LinkSpec               // ColLink; URL comes from the record
    Align  Align
    Input  *Input                  // renders a control instead of a value
}
```

Exactly one of `Key` and `Text` per item. A `Key` value is read from the
record at runtime and typed/Intl-formatted exactly like a table cell; a `Text`
value is authored in Go and always plain text. `ColActions` is not a valid
item type.

#### Editable items (`Input`)

An item with an `Input` asks instead of states: its value cell holds a control,
and what the reader puts in it is collected at call time and merged into the
arguments of the widget's own call.

```go
type InputType string

const (
    InputText     InputType = "text"     // the zero-value default
    InputNumber   InputType = "number"   // value travels as a number
    InputSelect   InputType = "select"   // dropdown over Options
    InputCheckbox InputType = "checkbox" // value travels as a bool
)

type Input struct {
    Name        string      // tool argument the value travels in (required, unique in the block)
    Type        InputType   // InputText default
    Placeholder string      // empty-state text; a select's unchosen label
    Required    bool        // blocks the widget's call until filled in
    Default     any         // string-like for text/number/select, bool for InputCheckbox
    Options     []Option    // required for InputSelect, invalid otherwise
    Validation  *Validation // as on a form Field; only Message applies to select/checkbox
}
```

Where inputs are accepted, and what carries their values:

| Block | Carries the values |
|---|---|
| `DatePicker.Details` | the submit call, alongside the picked date(s) |
| `Card` / `CardList` / `Carousel` `Content.Items` | every action button of the card the item sits in (per record; bulk actions get nothing) |
| `Confirm.Details`, `Choice.Details`, `ChoiceOption.Details` | nothing — an `Input` there is a validation error |

The control opens on the reader's own answer if it has one, else on the record
field named by `Key`, else on `Default`. Answers survive re-renders: a tool
result landing mid-answer replaces the values around the control, not in it.
Native constraint validation runs before the call and shows `Validation.Message`
(or the browser's own text) under the offending control; a required control
that is empty blocks the call. An empty number input sends no argument at all.

With `DateSubmit.ChatPrompt` set, the picker posts a chat turn instead of
calling the tool, so the answers are appended to the text as
`"; Guests: 3, Bed: double"` rather than sent as arguments.

There are no layout options by design: the list flows into as many columns as
the widget's own width allows and collapses to one in a narrow pane. The item
floor is the `--gomu-desc-min` token (default `12rem`), overridable through
`theme.Theme.Extra`. A data-bound value the record does not carry renders as an
em dash rather than vanishing.

### 3.14 `Confirm`

An approval widget: one question, the record it is about, the side effects of
answering yes, and exactly two outcomes. The long form of `Action.Confirm`.

```go
type Confirm struct {
    URI      string        // ui:// resource URI (required)
    Title    string        // toolbar + document title
    Prompt   string        // headline question (required)
    Body     string        // supporting prose
    Severity BadgeVariant  // BadgeInfo (default) | BadgeWarning | BadgeDanger

    Details Descriptions   // the record, bound to rows[0]
    Effects []Effect       // side effects; runtime EffectsKey replaces them

    Acknowledge   string   // checkbox label that must be ticked to enable accept
    TypeToConfirm string   // phrase that must be typed to enable accept

    Accept AcceptSpec      // required
    Reject *RejectSpec     // nil renders no declining button

    RowsKey    string      // default "rows"
    EffectsKey string      // default "effects"
    RowID      string      // default "id"

    InitialData map[string]any  // baked structuredContent snapshot
    LoadTool    string          // read tool called once on load
    LoadArgs    map[string]any

    Brand *Brand
    Theme *theme.Theme
    UI    *uispec.ResourceUIMeta
}

type Effect struct {
    Text     string        // the consequence (required)
    Detail   string        // secondary line
    Value    string        // magnitude at the row end ("128", "4 people")
    Severity BadgeVariant  // colors the row's dot; defaults to BadgeNeutral
}

type AcceptSpec struct {
    Tool           string               // MCP tool called on accept (required)
    Label          string               // defaults to "Confirm"
    Args           map[string]ArgSource // Static / FromRow only; ignored when ChatPrompt is set
    ChatPrompt     string               // post this text as a user turn instead of calling Tool
    Variant        ActionVariant        // overrides the variant derived from Severity
    SuccessMessage string               // shown in place of the buttons on success
}

type RejectSpec struct {
    Label   string               // defaults to "Cancel"
    Tool    string               // optional; without it the server never hears the "no"
    Args    map[string]ArgSource // Static / FromRow only
    Message string               // terminal text; defaults to "Cancelled."
}
```

```go
confirm := &gomukit.Confirm{
    URI:      "ui://demo/delete-user",
    Prompt:   "Delete Ada Lovelace?",
    Severity: gomukit.BadgeDanger,
    Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
        {Label: "User", Key: "name"},
        {Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
    }},
    Effects: []gomukit.Effect{
        {Text: "Removes the account", Severity: gomukit.BadgeDanger},
        {Text: "Deletes audit records", Value: "128", Severity: gomukit.BadgeWarning},
    },
    TypeToConfirm: "ada@example.com",
    Accept: gomukit.AcceptSpec{Tool: "delete_user", Label: "Delete user",
        Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
        SuccessMessage: "User deleted."},
    Reject: &gomukit.RejectSpec{Label: "Keep user"},
}
```

Runtime behavior:

- **Severity** colors the icon and picks the accept button's variant: danger →
  `VariantDanger`, anything else → `VariantPrimary`, unless `Accept.Variant`
  says otherwise.
- **Effects** authored in Go are shown until a payload carries `EffectsKey`,
  which replaces the list wholesale. An effect severity that is not one of the
  `Badge*` values is ignored rather than styled.
- **Guards** gate the accept button: the acknowledgement must be ticked and the
  phrase typed exactly (trimmed). The button is rendered `disabled`
  server-side whenever a guard is configured, so the document is correct before
  the runtime mounts. Enter in the phrase field accepts.
- **Accepting** calls `Accept.Tool`; on success the buttons are replaced by
  `SuccessMessage` (or the result's text). A result with `isError` re-arms the
  widget so a transient failure can be retried.
- **Rejecting** calls `Reject.Tool` when set, then settles with `Message`.
- **The decision is terminal**: after accepting or declining the buttons stay
  gone, even if the host pushes further results (which still refresh the
  details and effects).

### 3.15 `Choice`

A deciding widget: a question, the options answering it, and the case for each
one. Picking is local — only the submit button calls a tool. Use it where
`Confirm` asks yes/no about one operation and the reader has to choose *which*
operation instead.

```go
type Choice struct {
    URI    string          // ui:// resource URI (required)
    Title  string          // toolbar + document title
    Prompt string          // headline question (required)
    Body   string          // supporting prose

    Layout   ChoiceLayout  // ChoiceAuto (default) | ChoiceSplit | ChoiceStacked
    Multiple bool          // checkboxes instead of radios
    Min      int           // fewest options a multiple choice accepts (default 1)
    Max      int           // most it accepts; 0 = no limit. Multiple only

    Options []ChoiceOption // may be empty when they arrive under OptionsKey
    Details Descriptions   // the record the question is about, bound to rows[0]

    Submit ChoiceSubmit    // required
    Cancel *RejectSpec     // nil renders no declining button

    RowsKey    string      // default "rows"
    OptionsKey string      // default "options"
    RowID      string      // default "id"

    InitialData map[string]any  // baked structuredContent snapshot
    LoadTool    string          // read tool called once on load
    LoadArgs    map[string]any

    Brand *Brand
    Theme *theme.Theme
    UI    *uispec.ResourceUIMeta
}

type ChoiceOption struct {
    Value   string        // sent to the tool (required, unique)
    Label   string        // list heading; defaults to Value
    Summary string        // one supporting line, always visible in the list

    Body    string        // prose in the description block
    Bullets []string      // short points under Body
    Details Descriptions  // label/value list; Key items read Data
    Data    map[string]any // the option's own record

    Badge        string        // short text beside the label
    BadgeVariant BadgeVariant  // defaults to BadgeNeutral; needs Badge

    Default  bool         // preselected (a single choice takes at most one)
    Disabled bool         // on offer, but not choosable now
}

type ChoiceSubmit struct {
    Tool           string               // MCP tool called on submit (required)
    Label          string               // defaults to "Continue"
    ValueArg       string               // argument carrying the decision; defaults to "choice"
    Args           map[string]ArgSource // Static / FromRow only; ignored when ChatPrompt is set
    ChatPrompt     string               // post this plus the decision as a user turn instead of calling Tool
    Variant        ActionVariant        // defaults to VariantPrimary
    SuccessMessage string               // shown in place of the controls on success
}
```

`Cancel` reuses `RejectSpec` (§3.14): `Label` (default "Cancel"), an optional
`Tool` so the server hears the "no", its `Args`, and the terminal `Message`
(default "Cancelled.").

```go
choice := &gomukit.Choice{
    URI:    "ui://demo/shipping",
    Prompt: "How should we ship order ORD-4471?",
    Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
        {Label: "Order", Key: "reference"},
    }},
    Options: []gomukit.ChoiceOption{
        {
            Value: "standard", Label: "Standard", Summary: "3-5 business days",
            Body:    "Handed to the postal service tonight.",
            Bullets: []string{"Tracked to the depot", "No signature"},
            Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
                {Label: "Price", Key: "price", Type: gomukit.ColNumber, Format: "currency:EUR"},
            }},
            Data:    map[string]any{"price": 4.9},
            Default: true,
        },
        {Value: "express", Label: "Express", Summary: "next business day",
         Badge: "fastest", BadgeVariant: gomukit.BadgeSuccess},
    },
    Submit: gomukit.ChoiceSubmit{Tool: "ship_order", Label: "Ship it", ValueArg: "method",
        Args: map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
        SuccessMessage: "On its way."},
    Cancel: &gomukit.RejectSpec{Label: "Decide later"},
}
```

The submit call is `Args` resolved as usual plus `ValueArg`: the chosen
`Value` (single) or the array of chosen values in option order (multiple). So
the example above calls `ship_order` with `{"id": 4471, "method": "express"}`.

Runtime behavior:

- **Layout** decides where an option's description block goes. `ChoiceSplit`
  puts it in a side panel that follows the option in hand; `ChoiceStacked`
  unfolds it inside the chosen option. `ChoiceAuto` measures the width the
  host gave the widget and picks split at or above 34rem, stacked below —
  re-measuring as the pane resizes, so one document reads in a wide canvas and
  a narrow chat column. The server-rendered class is `--auto`, which styles as
  stacked, so a document whose script never ran is still correct.
- **Options** authored in Go are shown until a payload carries `OptionsKey`,
  which replaces the list wholesale and re-applies the new options' own
  defaults. A tool-supplied option describes itself in plain values — its
  `details` entries are `{label, value}` pairs, already formatted server-side,
  where an authored `Details` item is a typed field config resolved against
  `Data`. A `badgeVariant` that is not a `Badge*` value is ignored rather than
  styled.
- **Selection**: a single choice is radios and swaps; a multiple choice is
  checkboxes. Submit stays disabled until `Min` (default 1) is met; at `Max`
  the unticked options disable rather than failing on click, and a hint under
  the list tracks the count. `Disabled` options are never chosen, but pointing
  at one still shows its description — why it cannot be taken is the thing
  worth reading.
- **Submitting** calls `Submit.Tool`; on success the controls are replaced by
  `SuccessMessage` (or the result's text). A result with `isError` re-arms the
  widget so a transient failure can be retried.
- **Cancelling** calls `Cancel.Tool` when set, then settles with `Message`.
- **The decision is terminal**: after submitting or cancelling the controls
  stay gone, even if the host pushes further results (which still refresh the
  details and options).

### 3.16 `DatePicker`

A date, or the span between two, as the whole question. The standalone form of
the calendar a form's `FDate`/`FDateRange` fields open: use the widget when the
date *is* the question, the field when it is one answer among several. Picking
is local — only the submit button calls a tool.

```go
type DatePicker struct {
    URI    string   // ui:// resource URI (required)
    Title  string   // toolbar + document title
    Prompt string   // headline question (required)
    Body   string   // supporting prose under the prompt

    Mode     DateMode  // DateSingle (zero value) or DateRange
    Calendar *Calendar // the grid; nil is the zero value (see 3.16.1)

    Default    string // preselected date "YYYY-MM-DD" (the start, in a range)
    DefaultEnd string // preselected end of the span (DateRange only)

    Details Descriptions // describes the record the question is about (rows[0]);
                         // its items may also ask (Input), and their values
                         // travel with the submit call (see 3.13)

    Submit DateSubmit   // required
    Cancel *RejectSpec  // nil renders no declining button

    ValueKey string // structuredContent key with the selection and runtime bounds. Default "value"
    RowsKey  string // structuredContent key with the context record. Default "rows"
    RowID    string // record field used for FromRow args. Default "id"

    InitialData map[string]any
    LoadTool    string         // read tool called once on load, replacing InitialData
    LoadArgs    map[string]any
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}

type DateSubmit struct {
    Tool           string               // REQUIRED
    Label          string               // default "Continue"
    ValueArg       string               // argument carrying the date. Default "date"; "start" in a range
    EndArg         string               // argument carrying the end. Default "end". DateRange only
    Args           map[string]ArgSource // Static and FromRow only (no FromSelection); ignored when ChatPrompt is set
    ChatPrompt     string               // post this plus the picked date as a user turn instead of calling Tool
    Variant        ActionVariant        // default VariantPrimary
    SuccessMessage string               // shown in place of the controls on success
}

type DateMode string
const (
    DateSingle DateMode = ""      // one date
    DateRange  DateMode = "range" // a start and an end
)
```

- **The grid is inline**, not behind a trigger. Submit renders `disabled`
  server-side and stays disabled until the selection is complete (both ends, in
  a range), so a document whose script never runs offers no call it cannot make.
- **Dates are days, not instants.** Everything travels as `"YYYY-MM-DD"`. The
  host's time zone decides one thing only: which day is today.
- **Submitting** sends `ValueArg` (and `EndArg` in a range) as flat strings
  alongside the `Static`/`FromRow` args. On success the controls are replaced by
  `SuccessMessage` (or the result's text); a result with `isError` re-arms the
  widget so a transient failure can be retried.
- **Cancelling** calls `Cancel.Tool` when set, then settles with `Message`.
- **Runtime state** arrives under `ValueKey`: either `"YYYY-MM-DD"`, or an
  object carrying the selection *and* the grid's limits —
  `{start?, end?, min?, max?, disabled?}`. A selection pushed after the decision
  is ignored; the outcome stays.
- **The decision is terminal**, exactly as in `Confirm` and `Choice`.

#### 3.16.1 `Calendar`

A shared block, not a widget: one grid configuration for the `DatePicker`
widget and for a form's date fields. A nil `*Calendar` is the zero value — one
month (two in a range), every day selectable.

```go
type Calendar struct {
    Min      string   // earliest selectable day "YYYY-MM-DD"
    Max      string   // latest selectable day
    Disabled []string // individual days that cannot be picked
    DisableWeekends bool // blocks every Saturday and Sunday

    Months         int       // months shown at once; default 1 (single) / 2 (range), max 4
    WeekNumbers    bool      // leading column of ISO 8601 week numbers
    MonthDropdowns bool      // month + year dropdowns in place of the caption
    FromYear, ToYear int     // bound the year dropdown; default: the years of Min/Max
    WeekStart      WeekStart // default: the host locale's own first day

    StartOn string // month the grid opens on while nothing is selected

    Presets []DatePreset // named shortcuts, beside the grid or above it
}

type WeekStart string
const (
    WeekStartLocale   WeekStart = "" // the host locale's first day (default)
    WeekStartMonday   WeekStart = "monday"
    WeekStartSunday   WeekStart = "sunday"
    WeekStartSaturday WeekStart = "saturday"
)

type DatePreset struct {
    Label string   // REQUIRED
    Span  DateSpan // a window relative to the reader's today
    Start string   // or a fixed window "YYYY-MM-DD"
    End   string   // DateRange only
}
```

`DateSpan` values, resolved at runtime against the reader's today (a server
cannot name those dates at registration time): `SpanToday`, `SpanYesterday`,
`SpanTomorrow`, `SpanLast7Days`, `SpanLast30Days`, `SpanLast90Days`,
`SpanNext7Days`, `SpanNext30Days`, `SpanThisWeek`, `SpanLastWeek`,
`SpanThisMonth`, `SpanLastMonth`, `SpanThisYear`, `SpanYearToDate`. In a
single-date calendar a preset picks the day its window opens on.

A preset is measured against `Min`/`Max` and the blocked days as they stand when
the widget is read, and is shown as unavailable where it has nothing to offer. A
range preset overlapping the bounds is trimmed to them ("last 30 days" against a
calendar opening on the 1st picks the days of it there are); one wholly outside
them, or straddling a blocked day, is switched off. A single-date preset is
never moved: it picks the day it names or is switched off.

- **Blocked days bound a range too**: a span may not straddle one, so a second
  click across a taken day starts a new range instead.
- **Keyboard**: one tab stop for the grid, arrows by day/week, PageUp/PageDown
  by month (Shift: by year), Home/End across the week, Enter or Space to pick,
  Escape to close a popover. A blocked day still takes focus.
- **Every month asked for is shown**: side by side where the widget has room,
  wrapped under each other where it has not. A range picker whose second month
  is a month's travel away cannot show a span across the boundary at all.
- **Locale**: month and weekday names, the first day of the week and the
  formatted value come from the host's locale, which is why the grid is built at
  runtime and rebuilt when the host context changes.
- In a form, `Min`/`Max` also render as the date inputs' native `min`/`max`.

### 3.17 `Brand`

Identifies the application a widget belongs to. Available on every widget as the
`Brand` field; one `*Brand` is typically
shared across every widget of a server. It always renders at the top left of the
widget chrome, as the first item of the toolbar, before the title.

```go
type Brand struct {
    Name        string // application name; required unless a logo is set
    URL         string // optional http(s) link, opened through the host (ui/openLink)
    LogoSVG     string // inline <svg> markup (recommended)
    LogoDataURI string // "data:image/...;base64,..." alternative to LogoSVG
    LogoAlt     string // alt text for LogoDataURI; defaults to Name
}
```

Documents are self-contained, so a logo is never a URL. Prefer `LogoSVG`: it is
plain markup and needs nothing from the host's CSP. `LogoDataURI` renders as an
`<img>` and therefore depends on the host allowing `img-src data:`, which the
spec does not guarantee. A `Brand` with `URL` renders as a button, not an
anchor — navigation is blocked in the host's sandboxed iframe, so the runtime
hands the URL to `ui/openLink`.

The brand renders at the start of the widget's footer bar (bottom left), ahead of the runtime status message — not in the toolbar, so a widget with a brand and no `Title` renders no toolbar at all.

### 3.18 Validation rules (what `Validate()` / `Document()` reject)

Table:
- `URI` must be a well-formed `ui://` URI with a non-empty path.
- At least one column; no duplicate column `Key`s.
- text/number/date/badge columns: `Key` required.
- link columns: `Link.HrefKey` required.
- actions columns: at least one action.
- `PageSize >= 0`; `PageSizes` entries `> 0` and only with `PageSize > 0`;
  `LoadMore` needs `PageSize > 0` and rejects `PageSizes`;
  `DefaultSort.Key` required when `DefaultSort` is set.
- Actions: `Label` required; `Tool` required for tool kind; `HrefKey` required
  for link kind; all `Args` built with the constructors; `FromSelection` only
  in bulk actions; `VisibleWhen` only on per-record actions, and it needs a
  field name plus at least one value.
- `Theme` must pass `theme.Validate()`.

Form:
- `URI` as above; at least one field (in `Fields` or a `FieldSet`);
  `Submit.Tool` required.
- Field `Name` required and unique across the whole form, grouped or not. An
  `FDateRange` field's `EndName` shares that namespace: it must not collide
  with another field's `Name`, nor with its own.
- `Columns` (form and field set) must be 0..4; `FieldSet.Title` required;
  a `FieldSet` needs at least one field.
- `Field.Span` must be 0..the columns of its group.
- `FSelect`/`FMultiSelect` require non-empty `Options`.
- `Calendar` requires `FDate` or `FDateRange`; `EndName` requires `FDateRange`.
- Date defaults must be `"YYYY-MM-DD"`, must not run backwards, and must be days
  the field's own `Calendar` allows.
- `Calendar` validated as below.
- `Theme` must pass `theme.Validate()`.

Card / CardList (via `CardTemplate`):
- `URI` as above; `Template.Header.TitleKey` required.
- `Header.Badge`, when present (`Key` set), must be a badge column, and cannot
  be combined with `Header.Action` — the header has one end slot.
- Each text slot rejects being filled twice: `Header.DescriptionKey` +
  `Header.Description`, `Content.TextKey` + `Content.Text`, `Footer.TextKey` +
  `Footer.Text`.
- `Content.Items` validated as `Descriptions` (§3.13): `Label` required;
  `Key` xor `Text`; text/number/date/badge need `Key`; link needs
  `Link.HrefKey`; no duplicate item `Key`s.
- `Header.Action` and `Footer.Actions`: validated like any action;
  `FromSelection` is rejected (per-card actions run on one record).
  `VisibleWhen` is allowed here and rejected on `Selection.Bulk`.
- CardList only: `PageSize >= 0`; `PageSizes` entries `> 0` and only with
  `PageSize > 0`; `LoadMore` needs `PageSize > 0` and rejects `PageSizes`;
  `DefaultSort.Key` required when set; bulk actions validated.
- `Theme` must pass `theme.Validate()`.

Menu:
- `URI` as above; at least one item.
- Item `Tool` required.
- Item `IconSVG`, when set, must pass the same `<svg>` checks as
  `Brand.LogoSVG` (below).
- Item `BadgeVariant`, when set, must be one of the `Badge*` constants.
- `Theme` must pass `theme.Validate()`.

Confirm:
- `URI` as above; `Prompt` required; `Accept.Tool` required.
- `Severity`, when set, must be one of the `Badge*` constants.
- `Accept.Args` / `Reject.Args`: built with `Static` or `FromRow`;
  `FromSelection` is rejected (a confirmation has no selection).
- `Reject.Args` require `Reject.Tool`.
- Effects: `Text` required; `Severity`, when set, must be a `Badge*` constant.
- `Details` validated as `Descriptions` (below).
- `Theme` must pass `theme.Validate()`.

Choice:
- `URI` as above; `Prompt` required; `Submit.Tool` required.
- `Layout`, when set, must be `ChoiceSplit` or `ChoiceStacked`.
- `Submit.Args` / `Cancel.Args`: built with `Static` or `FromRow`;
  `FromSelection` is rejected (a choice has no row selection).
- `Submit.Args` cannot contain `Submit.ValueArg` — the decision and a static
  argument cannot share a name.
- `Cancel.Args` require `Cancel.Tool`.
- `Min`/`Max` require `Multiple`, cannot be negative, and `Min <= Max` when
  `Max > 0`.
- Options: `Value` required and unique; `BadgeVariant`, when set, must be a
  `Badge*` constant and needs `Badge` text; no empty `Bullets` entry; a
  `Disabled` option cannot be `Default`; at most one `Default` in a single
  choice, and no more than `Max` in a multiple one.
- Option `Details` and widget `Details` validated as `Descriptions` (below).
- An empty `Options` list is legal: options may arrive at runtime.
- `Theme` must pass `theme.Validate()`.

DatePicker:
- `URI` as above; `Prompt` required; `Submit.Tool` required.
- `Mode`, when set, must be `DateRange`.
- `Submit.Args` / `Cancel.Args`: built with `Static` or `FromRow`;
  `FromSelection` is rejected (a date has no row selection).
- `Submit.Args` cannot contain `Submit.ValueArg` or `Submit.EndArg`, and the two
  cannot be the same name.
- `Submit.EndArg` and `DefaultEnd` require `Mode: DateRange`.
- `Cancel.Args` require `Cancel.Tool`.
- `Default`/`DefaultEnd` must be `"YYYY-MM-DD"`, `DefaultEnd` requires
  `Default`, the span cannot run backwards, and neither day may be outside
  `Calendar.Min`/`Max` or in `Calendar.Disabled`.
- `Details` validated as `Descriptions` (below).
- `Calendar` validated as below.
- `Theme` must pass `theme.Validate()`.

Calendar (wherever embedded):
- `Min`, `Max`, `Disabled` entries and `StartOn` must be `"YYYY-MM-DD"`;
  `Max >= Min`.
- No empty `Disabled` entry, and no `Disabled` day outside `Min`/`Max` — a day
  blocked where nothing can be picked means the two disagree.
- `Months` between 1 and 4 (0 = the default for the mode).
- `WeekStart`, when set, must be a `WeekStart*` constant.
- `FromYear`/`ToYear` must be four-digit years with `ToYear >= FromYear`.
- Presets: `Label` required; exactly one of `Span` and `Start`/`End`; `Span`
  must be a `Span*` constant; `End` requires `Start`, requires a range
  calendar, and cannot precede `Start`.

Descriptions (wherever embedded):
- Item `Label` required.
- Exactly one of `Key` and `Text` per item; a `Text` item must be `ColText`.
- text/number/date/badge items need `Key`; link items need `Link.HrefKey`.
- No duplicate item `Key`s; `ColActions` is rejected.

Descriptions items with an `Input`:
- Rejected outright in `Confirm.Details`, `Choice.Details` and
  `ChoiceOption.Details` (nothing there carries what a control collects).
- `Input.Name` required and unique within the block. Two items may share a
  `Key` when one of them only prefills a control from it.
- `Input.Type` must be an `Input*` constant; `InputSelect` needs `Options` and
  every other type rejects them; an `InputCheckbox` `Default` must be a `bool`.
- An `Input` item cannot also carry `Text`, `Type`, `Format`, `Badge` or `Link`.
- `DatePicker`: an input name cannot repeat the date arguments (`ValueArg`,
  `EndArg`) or any key of `Submit.Args`.
- `Card`/`CardList`: an input name cannot repeat an argument name of the
  header action or any footer action.

Brand (all widgets, when set):
- `Name` or a logo is required.
- `LogoSVG` and `LogoDataURI` are mutually exclusive.
- `LogoSVG` must be a single `<svg>…</svg>` element and is rejected when it
  contains `<script>`, an `on*=` event handler, `<foreignObject>`, `<iframe>`,
  `<embed>`, `<object>`, `<use>`, `<animate>`, `<set>`, `javascript:`, an HTML
  comment, or `</style`.
- `LogoDataURI` must be `data:image/{png,jpeg,gif,webp,svg+xml};base64,` with a
  non-empty base64 payload.
- `URL`, when set, must be `http://` or `https://`.

`Document()` calls `Validate()` first and returns its error, so with `gosdk`
registration you get configuration errors at startup, not at render time.

---

## 4. The runtime data contract (structuredContent keys)

Widgets read all runtime data from the tool result's `structuredContent`:

| Widget | Key (configurable via) | Shape | Meaning |
|---|---|---|---|
| Table | `rows` (`RowsKey`) | `[]object` | rows to render |
| CardList | `rows` (`RowsKey`) | `[]object` | records to render as cards |
| Card | `rows` (`RowsKey`) | `[]object` | renders the first element (`rows[0]`) |
| Form | `values` (`PrefillKey`) | `{field: value}` | prefill (edit flows) |
| Form | `errors` (`ErrorsKey`) | `{field: "message"}` | server-side field errors, rendered inline; marks the submit failed |
| Confirm | `rows` (`RowsKey`) | `[]object` | the record the operation targets (`rows[0]`) |
| Confirm | `effects` (`EffectsKey`) | `[]object` | side effects: `{text, detail?, value?, severity?}`; replaces the authored list |
| Choice | `rows` (`RowsKey`) | `[]object` | the record the question is about (`rows[0]`) |
| Choice | `options` (`OptionsKey`) | `[]object` | what is on offer: `{value, label?, summary?, body?, bullets?, details?, badge?, badgeVariant?, default?, disabled?}` where `details` is `[{label, value}]`; replaces the authored list |
| DatePicker | `value` (`ValueKey`) | `"YYYY-MM-DD"` or `{start?, end?, min?, max?, disabled?}` | the selection, and the window it may move in: bounds and days already taken |
| DatePicker | `rows` (`RowsKey`) | `[]object` | the record the question is about (`rows[0]`) |
| Menu | — | — | reads nothing; tiles are authored and server-rendered |

`Card`/`CardList` share the `rows` contract with `Table`: an action or tool
result whose `structuredContent` contains `RowsKey` re-renders the widget
(`CardList` also clears the selection), so the same `list_users`/`delete_user`
tools drive a table or a card list interchangeably.

### Status messages come from the result's text content

Where a widget shows "the result's text" — the status bar, and the settled
outcome of `Confirm`/`Choice`/`DatePicker` — it takes the first text content
block that reads as a message. Blank blocks and blocks that are a serialized
JSON object or array are skipped, and the widget's own wording is used instead
("Done", "Saved.", `SuccessMessage`, …). That matters because a handler which
returns only structured output gets its `structuredContent` mirrored into a text
block automatically (the spec suggests it; the go-sdk does it whenever the
handler leaves `Content` empty) — that payload is data, not a message, and never
reaches the status bar. To control the wording, set `Content` explicitly with a
short sentence, or use the widget's `SuccessMessage`.

The same holds for a failed call: the error message is shown when it reads as
one, and a blank or JSON-payload message is replaced by the widget's own
wording ("The action failed.", "The request failed.", "Could not open …").

With the go-sdk typed handlers, your `Out` struct's JSON form becomes
`structuredContent` — so match the JSON tags to these keys:

```go
type rowsOut struct { Rows []map[string]any `json:"rows"` }
type editOut struct { Values map[string]any `json:"values"` }
type saveOut struct { Errors map[string]string `json:"errors,omitempty"` }
```

Flows:

- **List**: model calls e.g. `list_users` → result `{"rows": [...]}` → table
  renders.
- **Row/bulk mutation**: widget calls e.g. `delete_user` (app-only) → return
  `{"rows": [...updated list...]}` → table re-renders, selection cleared.
- **Edit form**: model calls e.g. `edit_user` (linked to the Form) → result
  `{"values": {"id": 3, "name": "Ada", ...}}` → form prefills.
- **Submit**: widget calls `Submit.Tool` with `{field: value, ...}` merged
  over `StaticArgs` → return `{"errors": {"email": "taken"}}` to fail with
  inline errors, or an errors-free result to succeed (shows
  `SuccessMessage` if set).

### Embedded per-call delivery (result-embedded widgets)

Besides the spec's template model (register the `ui://` resource once, the
host fetches it and pushes per-call data), a widget can be delivered
**embedded in the tool result**: build it per call with the data baked in
(`InitialData` — the runtime paints it before the handshake completes), give
it a **unique URI per render**, and append the rendered `Document()` to the
result's `content` as an embedded resource:

```
{type:"resource", resource:{uri, mimeType:"text/html;profile=mcp-app", text: doc}}
```

The mimeType MUST be `uispec.MIMEType` (`text/html;profile=mcp-app`) — that
profile is what tells the host to attach the MCP Apps bridge (handshake,
`tools/call`, size reporting) instead of rendering a dead static iframe.
There is no legacy mcp-ui interop: the runtime speaks only the MCP Apps
protocol, and actions in a host without it will fail with a request timeout.

Size reporting starts at first paint (not gated on the handshake) so the
host can grow the iframe immediately; the document resets
`body{margin:0;padding:8px}` so `body.scrollHeight` measures true content
height (margin sits outside it and would clip the bottom edge).

---

## 5. Package `gosdk` — official go-sdk adapter

```go
import "github.com/techthos/gomukit/gosdk"
```

```go
// Declares the MCP Apps extension in server capabilities. Mutates and
// returns opts (nil allocates fresh options) so it composes with
// mcp.NewServer. NOTE: explicitly setting Capabilities disables the SDK's
// historical default of advertising {"logging":{}}.
func EnableUI(opts *mcp.ServerOptions) *mcp.ServerOptions

// Registers w's template as a ui:// resource on s. The document is rendered
// ONCE and served from memory. Idempotent per (server, URI): re-registering
// the same URI is a no-op. Returns render/validation errors.
func AddWidget(s *mcp.Server, w gomukit.Widget) error

// Registers tool t linked to w via _meta (registers w's resource first if
// needed); raw-handler variant.
func AddWidgetTool(s *mcp.Server, w gomukit.Widget, t *mcp.Tool, h mcp.ToolHandler) error

// Same, with the SDK's typed handler: input/output JSON schemas inferred
// from In and Out. Out's JSON form becomes structuredContent.
func AddWidgetToolFor[In, Out any](s *mcp.Server, w gomukit.Widget, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) error

// Marks t app-only (_meta.ui.visibility: ["app"]): callable from the widget
// UI, hidden from the model. Call BEFORE registering the tool (registration
// merges, so the visibility is kept). Use for row-action and submit tools.
func AppOnly(t *mcp.Tool, w gomukit.Widget)

// Merges data into the result's _meta — delivered to the widget but hidden
// from the model (per spec).
func WithAppData(res *mcp.CallToolResult, data map[string]any)

// Reports whether the session's client declared the MCP Apps extension.
// Branching on it is optional — attaching _meta.ui unconditionally is
// spec-legal (hosts ignore unknown metadata).
func ClientSupportsUI(ss *mcp.ServerSession) bool
```

**Canonical wiring pattern** (from `examples/demo`):

```go
server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

// Model-visible tool rendered by the table:
gosdk.AddWidgetToolFor(server, table,
    &mcp.Tool{Name: "list_users", Description: "List users."}, listUsers)

// App-only tool (fired by a row action, hidden from the model):
del := &mcp.Tool{Name: "delete_user", Description: "Delete a user."}
gosdk.AppOnly(del, table)
gosdk.AddWidgetToolFor(server, table, del, deleteUser)
```

Multiple tools may link to the same widget; `AddWidget` runs implicitly and
is idempotent. Serve via `mcp.NewStreamableHTTPHandler` (HTTP) or
`server.Run(ctx, &mcp.StdioTransport{})` (stdio).

---

## 6. Package `theme` — styling overrides

Widgets ship a `--gomu-*` design-token system scoped under `.gomu-root`.
Every semantic token defaults to the **host-injected MCP Apps CSS variable**
(delivered via `hostContext.styles.variables` during `ui/initialize`) with a
built-in fallback — so widgets automatically match Claude/ChatGPT theming with
zero configuration, and dark mode follows the host theme (with a
`prefers-color-scheme` fallback). Only use `Theme` to deliberately override.

```go
type Theme struct {
    ColorBackground  string // canvas: widget shell, modal, text inputs
    ColorSurface     string // cream: cards, tiles, chips, table header, hovers
    ColorText        string
    ColorTextMuted   string
    ColorBorder      string
    ColorPrimary     string // accent: primary buttons, focused controls, links
    ColorPrimaryText string // text on primary background
    ColorDanger      string
    ColorSuccess     string
    ColorWarning     string

    FontFamily     string
    FontFamilyMono string

    RadiusS string
    RadiusM string
    RadiusL string

    SpaceUnit string // base spacing unit (default 0.25rem); all gaps/paddings derive from it

    Framed bool        // draw the widget shell as a card: 1px border + RadiusL corners.
                       // Off by default — hosts already frame the widget in a bubble or panel
    Transparent bool   // drop the page fill and the gutter: the iframe rectangle disappears,
                       // leaving only the widget's own surface on the host UI. Card, control
                       // and overlay fills are untouched. == ColorPage "transparent" + PagePad "0"
    ColorPage   string // page fill alone (cards/controls/overlays keep ColorBackground); ignored when Transparent
    PagePad     string // gutter between widget and iframe edge (default 8px); ignored when Transparent

    Extra map[string]string // extra/override raw custom properties; keys MUST start with "--gomu-"
}

func (t *Theme) CSS() string      // ":root{...}" (page tokens) + ".gomu-root{...}", "" when nothing set
func (t *Theme) Validate() error  // surfaces what CSS() would silently skip
```

- The zero value (and a nil `*Theme`) overrides nothing. Fields hold raw CSS
  values (`"#0f62fe"`, `"0.5rem"`, `"Inter, sans-serif"`). Non-empty fields
  win the cascade over host values; empty fields keep host-aware defaults.
- **Value safety**: values must not contain `{`, `}`, `;`, `</`, or `<!--`
  (CSS/HTML breakout guard). `Extra` keys must start with `--gomu-` and use
  only `[A-Za-z0-9_-]`. Widget `Validate()` calls `Theme.Validate()`.

Token → host variable mapping (for reference): `--gomu-color-bg` ←
`--color-background-primary`, `--gomu-color-surface` ←
`--color-background-secondary`, `--gomu-color-text` ←
`--color-text-primary`, `--gomu-color-text-muted` ←
`--color-text-secondary`, `--gomu-color-border` ←
`--color-border-primary`, `--gomu-color-primary` ← `--color-text-accent`,
danger/success/warning ← `--color-text-danger/success/warning`,
`--gomu-font`/`--gomu-font-mono` ← `--font-sans`/`--font-mono`,
`--gomu-radius-s/m/l` ← `--border-radius-sm/md/lg`.

The default palette, type scale and shape vocabulary come from `DESIGN.md` at
the repo root (warm-cream chrome, one accent reserved for the primary action,
8/16/32px radii, no shadow outside the modal). Tokens without a `Theme` field
— `--gomu-color-heading`, `-link`, `-secondary`, `-border-strong`, `-info`,
`-focus`, the `--gomu-text-*` scale and the `--gomu-h-*` control heights — are
set through `Extra`; `docs/theming.md` lists them all.

### Embedding without a visible frame

The widget shell carries no border and no corner radius by default, so it sits
flush in whatever the host already draws around it; `Framed: true` puts the
card chrome back.

`Transparent: true` makes the widget document paint nothing of its own, so the
host page shows through the iframe and only the widget's own surface reads as
part of the host UI. Two things it depends on:

- The **host** must leave the `<iframe>` element unpainted: `border: 0`
  (the UA default is `2px inset`) and no `background`.
- The embedded document's **root color scheme must match the `<iframe>`
  element's**, or the UA paints an opaque canvas behind the whole document and
  no author `background: transparent` can undo it. The runtime handles this by
  pinning `:root { color-scheme }` to `hostContext.theme`.

Content still cannot escape the iframe box: dropdown panels, tooltips and
focus rings are clipped by the frame, and the frame rectangle keeps swallowing
pointer events over its transparent areas.

---

## 7. Package `uispec` — spec constants and `_meta` types

Dependency-free; use it for manual wiring or advanced `_meta` control.

```go
const (
    ExtensionID = "io.modelcontextprotocol/ui" // capability-negotiation extension id
    SpecVersion = "2026-01-26"                 // targeted MCP Apps spec version
    MIMEType    = "text/html;profile=mcp-app"  // media type of UI template resources
    MetaKey     = "ui"                         // _meta key for UI metadata
    URIScheme   = "ui"                         // ui:// scheme
)

const (
    VisibilityModel = "model" // tool callable by the model
    VisibilityApp   = "app"   // tool callable from the app UI only
)

// Presence marker for a requested sandbox permission. Serializes to {} when
// set (non-nil) and is omitted when nil, per the MCP Apps spec.
type Permission struct{}

// Marker used to request a permission, e.g. Permissions{Camera: uispec.Grant}.
var Grant = &Permission{}

// Browser capabilities a UI resource requests. Serializes to the spec object
// shape, e.g. {"camera":{},"clipboardWrite":{}}.
type Permissions struct {
    Camera         *Permission `json:"camera,omitempty"`
    Microphone     *Permission `json:"microphone,omitempty"`
    Geolocation    *Permission `json:"geolocation,omitempty"`
    ClipboardWrite *Permission `json:"clipboardWrite,omitempty"`
}

// External origins a UI resource needs (hosts default to fully locked-down).
type CSP struct {
    ConnectDomains  []string `json:"connectDomains,omitempty"`
    ResourceDomains []string `json:"resourceDomains,omitempty"`
    FrameDomains    []string `json:"frameDomains,omitempty"`
    BaseURIDomains  []string `json:"baseUriDomains,omitempty"`
}

// _meta.ui on a ui:// resource (set via Table.UI / Form.UI).
type ResourceUIMeta struct {
    CSP           *CSP         `json:"csp,omitempty"`
    Permissions   *Permissions `json:"permissions,omitempty"`
    Domain        string       `json:"domain,omitempty"`
    PrefersBorder *bool        `json:"prefersBorder,omitempty"`
}

// _meta.ui on a tool, linking it to its template resource.
type ToolUIMeta struct {
    ResourceURI string   `json:"resourceUri"`
    Visibility  []string `json:"visibility,omitempty"`
}

// Everything needed to register a widget's template resource, SDK-agnostic.
type ResourceDescriptor struct {
    URI, Name, Title, Description string
    MIMEType                      string // always uispec.MIMEType for gomukit widgets
    UI                            *ResourceUIMeta
}

func (m ResourceUIMeta) MetaMap() map[string]any     // {"ui": {...}}
func (m ToolUIMeta) MetaMap() map[string]any         // {"ui": {"resourceUri": ..., ...}}
func (d ResourceDescriptor) MetaMap() map[string]any // nil when d.UI == nil

// Recursive merge (maps merge, everything else overwrites); nil dst allocated.
func MergeMeta(dst, src map[string]any) map[string]any

// Checks uri is a well-formed ui:// URI (prefix + non-empty path).
func ValidateURI(uri string) error
```

Note: gomukit widgets don't need any `CSP` declarations — documents are fully
self-contained and satisfy the spec's default locked-down policy. Only set
`UI.CSP` if you know a host-specific reason to.

---

## 8. Using gomukit WITHOUT the official go-sdk

The core emits plain spec-shaped values; adapt to any Go MCP implementation:

```go
w := table // or form, card, cardlist, menu, confirm, choice, datepicker

doc, err := w.Document() // render once (validates); serve from memory
d := w.Descriptor()      // d.URI, d.Name (derived: "ui://demo/users" -> "demo-users"),
                         // d.Title, d.MIMEType ("text/html;profile=mcp-app"), d.MetaMap()

// 1. Advertise the extension in server capabilities:
//    capabilities.extensions["io.modelcontextprotocol/ui"] = {"mimeTypes": ["text/html;profile=mcp-app"]}
// 2. Register a resource at d.URI with d.MIMEType (+ d.MetaMap() as _meta if non-nil);
//    resources/read returns doc as text.
// 3. On each linked tool, merge w.ToolMeta() into the tool's _meta.
// 4. Tool results carry widget data in structuredContent (section 4).
```

---

## 9. Constraints, gotchas, and rules for generated code

1. **Match keys exactly.** The most common wiring bug: the handler's output
   JSON key doesn't match `RowsKey`/`PrefillKey`/`ErrorsKey` (defaults:
   `"rows"`, `"values"`, `"errors"`). Nothing renders and nothing errors.
2. **Row identity**: every row should carry the `RowID` field (default
   `"id"`); selection and `FromRow`/`FromSelection` depend on it.
3. **Mutating table/cardlist tools must return the updated row list** under
   `RowsKey`, or the UI keeps showing stale rows (a `Card` re-renders `rows[0]`).
4. **Hidden/readonly/text form fields submit strings.** A hidden numeric ID
   arrives as `"3"` — parse server-side (`strconv.Atoi`). Empty `FNumber`
   fields are omitted from the arguments entirely.
5. **Mark widget-only tools app-only** (`gosdk.AppOnly`) — submit targets and
   row-action tools the model shouldn't invoke directly. Call `AppOnly`
   before registering the tool.
6. **`FromSelection` only in bulk actions**; `FromRow` in per-row actions.
7. **No native dialogs**: `confirm()`/`alert()` don't work in sandboxed MCP
   Apps iframes. Use `Action.Confirm` for destructive confirmation.
8. **Documents must stay self-contained**: no external URLs, CDNs, fonts, or
   images from the network. This is by construction — don't try to inject
   any via `Theme` (values are validated against breakout anyway).
9. **Widgets are registered once, immutably**: `AddWidget` renders the
   document a single time and serves it from memory. Don't mutate a widget
   struct after registration and expect changes; per-call variation belongs
   in tool-result data, not the template.
10. **Use `gomukit.RowsOf`** to convert typed slices; it honors `json` tags,
    so column `Key`s must match the JSON tag names, not Go field names.
11. **`InitialData`** is optional and only an instant-first-paint snapshot; it
    is shaped like `structuredContent` (e.g. `{"rows": [...]}`), and is
    superseded by runtime tool results.
12. **Sort/filter/pagination are client-side** over delivered rows. For big
    datasets, page/filter server-side in the tool and deliver a bounded list.
13. **Theming**: prefer no `Theme` (host-matched look). When overriding,
    values are raw CSS; `Extra` keys must start with `--gomu-`.
14. **Errors surface at startup**: `AddWidget*` returns validation errors
    (bad URI, missing keys, duplicate columns/fields, unsafe theme values) —
    check them (`log.Fatal`/`must`).

---

## 10. Examples and manual testing

- `examples/demo` — complete runnable MCP server (menu launcher, users table
  with row/bulk actions, the same users as a card carousel, edit form with
  server-side validation, prefill and string-ID parsing, a confirmation, and a
  date picker whose selectable window is computed per call).
  `go run ./examples/demo -addr :8080` (streamable HTTP at `/mcp`) or
  `go run ./examples/demo -stdio`. Point MCPJam, Claude custom connectors, or
  any MCP Apps host at `http://localhost:8080/mcp`.
- `examples/preview` — the widest MCP server, for driving from an MCP Apps
  capable inspector: `make preview` (or `make inspect`, which starts the MCP
  Inspector already connected), endpoint `http://localhost:8081/mcp`. Two
  halves: a small stateful application (Acme Dispatch — customers and orders,
  mutable across calls) and a gallery with one tool and resource per widget
  variant; `-mode` picks one or both. Details in `docs/preview.md`.
- `examples/harness` — a fake MCP Apps host in one HTML page, with a story
  browser: a rail of widget variants (table, cardlist, card, descriptions,
  form, menu, confirm, choice, date picker, plus empty states and a long list)
  defined in `examples/harness/stories.go` and served one per route
  (`/story/<id>`, catalog at `/stories.json`). It renders
  the selected story in a sandboxed iframe at a chosen viewport width, answers
  the `ui/initialize` handshake, logs all JSON-RPC traffic (expandable
  entries), follows `ui/notifications/size-changed`, and simulates tool
  results/errors and theme changes (with a stateful in-memory backend so
  row/bulk actions update live). `go run ./examples/harness`, open
  `http://localhost:8090`. Use it to verify widget behavior without any real
  MCP client. Add a story by appending to `catalog()` and writing its builder.

## 11. Development commands (when modifying this repo)

```sh
make test         # go test ./... + vitest
make test-go      # Go tests only
make test-ui      # vitest only
make typecheck    # tsc --noEmit
make vet          # go vet ./...
make assets       # npm ci + rebuild the TS/CSS bundle into internal/assets/dist
make verify-dist  # rebuild assets, fail if committed dist drifted (CI does this)
make build        # build the example servers into ./bin (Go only, no Node)
make clean        # remove ./bin
make harness      # the fake MCP Apps host on :8090
make preview      # the preview MCP server on :8081
make inspect      # the preview server with the MCP Inspector in front of it
make inspect-demo # the same, in front of examples/demo on :8080
make screenshots  # rescreenshot every widget story into docs/assets
```

After editing anything under `ui/` (src or css), run `make assets` and commit
the resulting `internal/assets/dist/` changes — the bundle is committed and
`go:embed`-ed so Go consumers never need Node; CI fails on drift. Golden
files: `go test ./ -update` regenerates `testdata/golden/`. Full contributor
rules: `CLAUDE.md`.
