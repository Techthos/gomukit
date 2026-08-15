package gomukit

import (
	"fmt"
	"strings"

	"github.com/techthos/gomukit/theme"
	"github.com/techthos/gomukit/uispec"
)

// Table is an interactive data table widget: typed columns, client-side
// sort/filter/pagination, row selection with bulk actions, and per-row
// actions that call MCP tools.
//
// Rows are string-keyed JSON objects delivered at runtime in the tool
// result's structuredContent (under RowsKey); the embedded runtime renders
// them. InitialData optionally bakes a snapshot into the document for
// instant first paint.
type Table struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Columns defines the table's columns (required, non-empty).
	Columns []Column

	// RowsKey is the structuredContent key holding the rows array.
	// Defaults to "rows".
	RowsKey string
	// RowID is the row field that uniquely identifies a row, used for
	// selection and FromRow/FromSelection args. Defaults to "id".
	RowID string

	// PageSize enables client-side pagination when > 0.
	PageSize int
	// PageSizes offers alternative page sizes in a dropdown on the pagination
	// bar. Entries must be > 0 and PageSize must be set; PageSize is added to
	// the list if it is not among them. Empty renders no chooser.
	PageSizes []int
	// LoadMore turns the table into a growing list instead of a paged one: it
	// starts at PageSize rows and appends the next PageSize each time the
	// reader activates the "Load more" bar that replaces the prev/next
	// pagination bar. With MaxHeight also set, running out of scroll reveals
	// the next batch by itself.
	//
	// Requires PageSize > 0, and cannot be combined with PageSizes: the page
	// size chooser lives on the bar this removes.
	LoadMore bool
	// MaxHeight caps the rows area at a CSS length (e.g. "20rem"). Past it
	// the rows scroll inside the widget under a sticky header row, instead of
	// the widget growing taller with every row.
	MaxHeight string
	// DefaultSort pre-sorts rows on load.
	DefaultSort *SortSpec
	// Filterable adds a client-side text filter box.
	Filterable bool
	// Selection enables row checkboxes and bulk actions.
	Selection *SelectionConfig
	// Empty configures the no-data message.
	Empty EmptyState

	// InitialData is an optional structuredContent-shaped snapshot baked
	// into the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load
	// (after the host handshake) to hydrate the table from fresh data,
	// replacing the baked InitialData snapshot. This keeps a reloaded widget
	// current instead of reverting to the state frozen at render time. The
	// tool must return the rows under RowsKey in its structuredContent.
	LoadTool string
	// LoadArgs are optional static arguments passed to LoadTool.
	LoadArgs map[string]any

	// Brand renders the application logo/name on the widget.
	Brand *Brand
	// Theme overrides gomukit design tokens for this widget.
	Theme *theme.Theme
	// UI overrides resource _meta.ui (CSP, permissions, prefersBorder).
	UI *uispec.ResourceUIMeta
}

// SelectionConfig enables row selection.
type SelectionConfig struct {
	// Bulk actions appear in the toolbar while rows are selected.
	// FromSelection args resolve across all selected rows.
	Bulk []Action
}

// ColumnType selects how a column renders its cells.
type ColumnType string

const (
	ColText    ColumnType = "text" // the zero-value default
	ColNumber  ColumnType = "number"
	ColDate    ColumnType = "date"
	ColBadge   ColumnType = "badge"
	ColLink    ColumnType = "link"
	ColActions ColumnType = "actions"
)

// BadgeVariant colors a badge cell value.
type BadgeVariant string

const (
	BadgeNeutral BadgeVariant = "neutral"
	BadgeInfo    BadgeVariant = "info"
	BadgeSuccess BadgeVariant = "success"
	BadgeWarning BadgeVariant = "warning"
	BadgeDanger  BadgeVariant = "danger"
)

// LinkSpec configures a link column. The URL comes from HrefKey; the link
// text comes from TextKey, or the fixed Text, or the URL itself.
type LinkSpec struct {
	HrefKey string `json:"hrefKey"`
	TextKey string `json:"textKey,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Column defines one table column.
type Column struct {
	// Key is the row field this column displays (unused for ColActions).
	Key   string
	Label string
	// Type defaults to ColText.
	Type ColumnType
	// Sortable overrides the default (text/number/date sortable, others not).
	Sortable *bool
	Align    Align
	// Format refines rendering, interpreted by the runtime via Intl:
	// numbers: "int" | "decimal:<digits>" | "percent" | "currency:<code>";
	// dates: "date" | "datetime" | "time" | "relative".
	Format string
	// Badge maps cell values to badge variants (ColBadge).
	Badge map[string]BadgeVariant
	// Link configures ColLink columns.
	Link *LinkSpec
	// Actions renders per-row action buttons (ColActions).
	Actions []Action
	// Width is a CSS width for the column (e.g. "12rem", "20%").
	Width string
}

func (c Column) columnType() ColumnType {
	if c.Type == "" {
		return ColText
	}
	return c.Type
}

func (c Column) sortable() bool {
	if c.Sortable != nil {
		return *c.Sortable
	}
	switch c.columnType() {
	case ColText, ColNumber, ColDate:
		return true
	default:
		return false
	}
}

// --- column sugar ---

// Text returns a text column.
func Text(key, label string) Column { return Column{Key: key, Label: label, Type: ColText} }

// Number returns a number column with an optional format
// ("int", "decimal:<digits>", "percent", "currency:<code>").
func Number(key, label string, format ...string) Column {
	return Column{Key: key, Label: label, Type: ColNumber, Format: first(format), Align: AlignEnd}
}

// Date returns a date column with an optional format
// ("date", "datetime", "time", "relative").
func Date(key, label string, format ...string) Column {
	return Column{Key: key, Label: label, Type: ColDate, Format: first(format)}
}

// Badge returns a badge column mapping values to variants.
func Badge(key, label string, variants map[string]BadgeVariant) Column {
	return Column{Key: key, Label: label, Type: ColBadge, Badge: variants}
}

// Link returns a link column whose URL comes from hrefKey.
func Link(hrefKey, label string) Column {
	return Column{Key: hrefKey, Label: label, Type: ColLink, Link: &LinkSpec{HrefKey: hrefKey}}
}

// ActionsColumn returns a per-row actions column.
func ActionsColumn(actions ...Action) Column {
	return Column{Label: "", Type: ColActions, Actions: actions}
}

func first(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

// --- Widget implementation ---

func (t *Table) rowsKey() string {
	if t.RowsKey == "" {
		return "rows"
	}
	return t.RowsKey
}

func (t *Table) rowID() string {
	if t.RowID == "" {
		return "id"
	}
	return t.RowID
}

// Validate implements Widget.
func (t *Table) Validate() error {
	if err := uispec.ValidateURI(t.URI); err != nil {
		return fmt.Errorf("gomukit: table: %w", err)
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("gomukit: table %s: at least one column is required", t.URI)
	}
	seen := map[string]bool{}
	for i, c := range t.Columns {
		ctx := fmt.Sprintf("gomukit: table %s: column %d (%s)", t.URI, i, c.Label)
		switch c.columnType() {
		case ColText, ColNumber, ColDate, ColBadge:
			if c.Key == "" {
				return fmt.Errorf("%s: key is required", ctx)
			}
		case ColLink:
			if c.Link == nil || c.Link.HrefKey == "" {
				return fmt.Errorf("%s: link columns need Link.HrefKey", ctx)
			}
		case ColActions:
			if len(c.Actions) == 0 {
				return fmt.Errorf("%s: actions columns need at least one action", ctx)
			}
		default:
			return fmt.Errorf("%s: unknown column type %q", ctx, c.Type)
		}
		if c.Key != "" {
			if seen[c.Key] {
				return fmt.Errorf("%s: duplicate column key %q", ctx, c.Key)
			}
			seen[c.Key] = true
		}
		for _, a := range c.Actions {
			if err := a.validate(ctx); err != nil {
				return err
			}
			if a.kind() == ActionTool {
				for name, src := range a.Args {
					if src.selection != "" {
						return fmt.Errorf("%s: action %q: argument %q: FromSelection is only valid in bulk actions", ctx, a.Label, name)
					}
				}
			}
		}
	}
	if t.PageSize < 0 {
		return fmt.Errorf("gomukit: table %s: PageSize must be >= 0", t.URI)
	}
	if err := validatePageSizes(fmt.Sprintf("gomukit: table %s", t.URI), t.PageSize, t.PageSizes); err != nil {
		return err
	}
	if t.LoadMore {
		if t.PageSize <= 0 {
			return fmt.Errorf("gomukit: table %s: LoadMore needs PageSize > 0", t.URI)
		}
		if len(t.PageSizes) > 0 {
			return fmt.Errorf("gomukit: table %s: LoadMore and PageSizes are mutually exclusive", t.URI)
		}
	}
	if t.DefaultSort != nil && t.DefaultSort.Key == "" {
		return fmt.Errorf("gomukit: table %s: DefaultSort.Key is required", t.URI)
	}
	if t.Selection != nil {
		for _, a := range t.Selection.Bulk {
			if err := validateBulkAction(fmt.Sprintf("gomukit: table %s: bulk", t.URI), a); err != nil {
				return err
			}
		}
	}
	if err := t.Brand.Validate(); err != nil {
		return fmt.Errorf("gomukit: table %s: %w", t.URI, err)
	}
	if err := t.Theme.Validate(); err != nil {
		return fmt.Errorf("gomukit: table %s: %w", t.URI, err)
	}
	return nil
}

// Descriptor implements Widget.
func (t *Table) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      t.URI,
		Name:     resourceName(t.URI),
		Title:    t.Title,
		MIMEType: uispec.MIMEType,
		UI:       t.UI,
	}
}

// ToolMeta implements Widget.
func (t *Table) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: t.URI}.MetaMap()
}

// columnConfig serializes one column for the #gomu-config island. Shared
// by Table columns and Card fields so both sides format cells identically.
func columnConfig(c Column) map[string]any {
	col := map[string]any{
		"key":      c.Key,
		"label":    c.Label,
		"type":     string(c.columnType()),
		"sortable": c.sortable(),
	}
	if c.Align != "" {
		col["align"] = string(c.Align)
	}
	if c.Format != "" {
		col["format"] = c.Format
	}
	if len(c.Badge) > 0 {
		col["badge"] = c.Badge
	}
	if c.Link != nil {
		col["link"] = c.Link
	}
	if len(c.Actions) > 0 {
		col["actions"] = actionConfigs(c.Actions)
	}
	return col
}

// config builds the #gomu-config island content.
func (t *Table) config() map[string]any {
	cols := make([]map[string]any, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = columnConfig(c)
	}

	cfg := map[string]any{
		"widget":     "table",
		"rowsKey":    t.rowsKey(),
		"rowId":      t.rowID(),
		"pageSize":   t.PageSize,
		"filterable": t.Filterable,
		"columns":    cols,
	}
	if t.LoadMore {
		cfg["loadMore"] = true
	}
	if t.DefaultSort != nil {
		cfg["defaultSort"] = t.DefaultSort
	}
	if t.Selection != nil {
		cfg["selection"] = map[string]any{"bulk": actionConfigs(t.Selection.Bulk)}
	}
	if t.Empty != (EmptyState{}) {
		cfg["empty"] = t.Empty
	}
	if t.LoadTool != "" {
		cfg["loadTool"] = t.LoadTool
		if len(t.LoadArgs) > 0 {
			cfg["loadArgs"] = t.LoadArgs
		}
	}
	return cfg
}

// resourceName derives a registration name from a ui:// URI
// ("ui://demo/users" -> "demo-users").
func resourceName(uri string) string {
	name := strings.TrimPrefix(uri, uispec.URIScheme+"://")
	name = strings.Trim(name, "/")
	return strings.ReplaceAll(name, "/", "-")
}
