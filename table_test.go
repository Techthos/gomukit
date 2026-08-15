package gomukit

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/techthos/gomukit/theme"
)

var update = flag.Bool("update", false, "update golden files")

// canonicalTable exercises every column type and table feature.
func canonicalTable() *Table {
	return &Table{
		URI:   "ui://demo/users",
		Title: "Users",
		Columns: []Column{
			Text("name", "Name"),
			Number("balance", "Balance", "currency:EUR"),
			Date("createdAt", "Created", "date"),
			Badge("status", "Status", map[string]BadgeVariant{
				"active": BadgeSuccess,
				"banned": BadgeDanger,
			}),
			Link("website", "Website"),
			ActionsColumn(
				Action{Label: "Edit", Tool: "edit_user", Args: map[string]ArgSource{
					"id": FromRow("id"),
				}, Variant: VariantPrimary},
				Action{Label: "Delete", Tool: "delete_user", Confirm: "Really delete?", Args: map[string]ArgSource{
					"id": FromRow("id"),
				}, Variant: VariantDanger},
			),
		},
		PageSize:    10,
		PageSizes:   []int{25, 10, 50},
		DefaultSort: &SortSpec{Key: "name"},
		Filterable:  true,
		Selection: &SelectionConfig{Bulk: []Action{
			{Label: "Archive", Tool: "archive_users", Args: map[string]ArgSource{
				"ids": FromSelection("id"),
			}},
		}},
		Empty: EmptyState{Title: "No users", Body: "Create a user to get started."},
		InitialData: map[string]any{
			"rows": []map[string]any{
				{"id": 1, "name": "Ada", "balance": 12.5, "createdAt": "2026-01-01T00:00:00Z", "status": "active", "website": "https://example.com"},
			},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestTableGolden(t *testing.T) {
	doc, err := canonicalTable().Document()
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden", "table.html")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if doc != string(want) {
		t.Error("document does not match golden file; run `go test -run TestTableGolden -update ./...` and review the diff")
	}

	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gomu-widget="table"`,
		`id="gomu-config"`,
		`id="gomu-data"`,
		`data-gomu-sort="name"`,
		`data-gomu-select-all`,
		`data-gomu-bulk-menu`,
		`data-gomu-page-size`,
		`--gomu-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
}

func TestTableConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalTable().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"table"`,
		`"rowsKey":"rows"`,
		`"rowId":"id"`,
		`"pageSize":10`,
		`"filterable":true`,
		`"defaultSort":{"key":"name"}`,
		`"type":"badge"`,
		`"badge":{"active":"success","banned":"danger"}`,
		`"link":{"hrefKey":"website"}`,
		`"args":{"id":{"row":"id"}}`,
		`"args":{"ids":{"selection":"id"}}`,
		`"confirm":"Really delete?"`,
		`"empty":{"title":"No users","body":"Create a user to get started."}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestTableConfigVisibleWhen(t *testing.T) {
	tbl := canonicalTable()
	tbl.Columns = append(tbl.Columns, ActionsColumn(
		Action{Label: "Activate", Tool: "schedule_manage", VisibleWhen: RowIs("state", "paused")},
		Action{Label: "Pause", Tool: "schedule_manage", VisibleWhen: RowIs("state", "running")},
	))
	if err := tbl.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	b, err := json.Marshal(tbl.config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"label":"Activate"`,
		`"visibleWhen":{"equals":"paused","key":"state"}`,
		`"visibleWhen":{"equals":"running","key":"state"}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestTableBulkRejectsVisibleWhen(t *testing.T) {
	tbl := canonicalTable()
	tbl.Selection = &SelectionConfig{Bulk: []Action{
		{Label: "Archive", Tool: "archive_users", VisibleWhen: RowIs("state", "paused")},
	}}
	err := tbl.Validate()
	if err == nil || !strings.Contains(err.Error(), "only valid on per-record actions") {
		t.Errorf("Validate() = %v, want a bulk/VisibleWhen complaint", err)
	}
}

func TestTableConfigLoadTool(t *testing.T) {
	tbl := canonicalTable()
	tbl.LoadTool = "list_users"
	tbl.LoadArgs = map[string]any{"scope": "all"}
	b, err := json.Marshal(tbl.config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{`"loadTool":"list_users"`, `"loadArgs":{"scope":"all"}`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}

	// Both keys are omitted when LoadTool is unset.
	b2, err := json.Marshal(canonicalTable().config())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), "loadTool") || strings.Contains(string(b2), "loadArgs") {
		t.Errorf("load keys present without LoadTool set: %s", b2)
	}
}

func TestTableToolMetaAndDescriptor(t *testing.T) {
	tbl := canonicalTable()
	meta, err := json.Marshal(tbl.ToolMeta())
	if err != nil {
		t.Fatal(err)
	}
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/users"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	d := tbl.Descriptor()
	if d.URI != "ui://demo/users" || d.Name != "demo-users" || d.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", d)
	}
}

func TestTableValidate(t *testing.T) {
	valid := func() *Table { return canonicalTable() }

	cases := map[string]func(*Table){
		"bad URI scheme":       func(t *Table) { t.URI = "https://x" },
		"no columns":           func(t *Table) { t.Columns = nil },
		"duplicate column key": func(t *Table) { t.Columns = append(t.Columns, Text("name", "Name2")) },
		"text without key":     func(t *Table) { t.Columns = []Column{{Label: "X"}} },
		"link without hrefKey": func(t *Table) { t.Columns = []Column{{Label: "X", Type: ColLink}} },
		"actions empty":        func(t *Table) { t.Columns = []Column{{Type: ColActions}} },
		"tool action no tool":  func(t *Table) { t.Columns = []Column{ActionsColumn(Action{Label: "X"})} },
		"action no label":      func(t *Table) { t.Columns = []Column{ActionsColumn(Action{Tool: "x"})} },
		"selection arg in row action": func(t *Table) {
			t.Columns = []Column{ActionsColumn(Action{Label: "X", Tool: "x", Args: map[string]ArgSource{
				"ids": FromSelection("id"),
			}})}
		},
		"invalid arg source": func(t *Table) {
			t.Columns = []Column{ActionsColumn(Action{Label: "X", Tool: "x", Args: map[string]ArgSource{
				"v": {},
			}})}
		},
		"negative page size":     func(t *Table) { t.PageSize = -1 },
		"page size option zero":  func(t *Table) { t.PageSizes = []int{10, 0} },
		"page sizes unpaginated": func(t *Table) { t.PageSize, t.PageSizes = 0, []int{10} },
		"load more unpaginated": func(t *Table) {
			t.PageSize, t.PageSizes, t.LoadMore = 0, nil, true
		},
		"load more with page sizes": func(t *Table) {
			t.PageSize, t.PageSizes, t.LoadMore = 5, []int{5, 10}, true
		},
		"default sort no key": func(t *Table) {
			t.DefaultSort = &SortSpec{}
		},
		"unsafe theme": func(t *Table) {
			t.Theme = &theme.Theme{ColorText: "red}</style>"}
		},
	}

	if err := valid().Validate(); err != nil {
		t.Fatalf("canonical table must validate, got: %v", err)
	}
	for name, mutate := range cases {
		tbl := valid()
		mutate(tbl)
		if err := tbl.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}

func TestRowsOf(t *testing.T) {
	type user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	rows, err := RowsOf([]user{{1, "Ada"}, {2, "Bob"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["name"] != "Ada" || rows[1]["id"] != float64(2) {
		t.Errorf("rows = %v", rows)
	}
	if _, err := RowsOf("not a slice"); err == nil {
		t.Error("RowsOf on non-slice must error")
	}
}
