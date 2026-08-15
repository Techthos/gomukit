package gomukit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

const logoSVG = `<svg viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="8" r="7"/></svg>`

// 1x1 transparent GIF.
const logoDataURI = "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"

func TestBrandValidate(t *testing.T) {
	ok := map[string]*Brand{
		"nil":              nil,
		"name only":        {Name: "Acme"},
		"svg only":         {LogoSVG: logoSVG},
		"data uri only":    {LogoDataURI: logoDataURI, LogoAlt: "Acme"},
		"linked":           {Name: "Acme", URL: "https://acme.test"},
		"svg with newline": {Name: "Acme", LogoSVG: "\n" + logoSVG + "\n"},
	}
	for name, b := range ok {
		t.Run(name, func(t *testing.T) {
			if err := b.Validate(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	bad := map[string]*Brand{
		"empty":            {},
		"both logos":       {LogoSVG: logoSVG, LogoDataURI: logoDataURI},
		"svg with script":  {LogoSVG: `<svg><script>alert(1)</script></svg>`},
		"svg with handler": {LogoSVG: `<svg onload="alert(1)"><circle r="1"/></svg>`},
		"svg with use":     {LogoSVG: `<svg><use href="#x"/></svg>`},
		"svg fragment":     {LogoSVG: `<circle r="1"/>`},
		"svg trailing":     {LogoSVG: logoSVG + `<img src=x>`},
		"data uri scheme":  {LogoDataURI: "https://acme.test/logo.png"},
		"data uri type":    {LogoDataURI: "data:text/html;base64,PGgxPmhpPC9oMT4="},
		"data uri payload": {LogoDataURI: "data:image/png;base64,<script>"},
		"bad url":          {Name: "Acme", URL: "javascript:alert(1)"},
	}
	for name, b := range bad {
		t.Run(name, func(t *testing.T) {
			if err := b.Validate(); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// widgetsWithBrand returns each widget kind carrying the given brand, so
// placement is checked uniformly across the four documents.
func widgetsWithBrand(b *Brand) map[string]Widget {
	table := canonicalTable()
	table.Brand = b
	form := canonicalForm()
	form.Brand = b
	card := canonicalCard()
	card.Brand = b
	list := canonicalCardList()
	list.Brand = b
	menu := canonicalMenu()
	menu.Brand = b
	return map[string]Widget{"table": table, "form": form, "card": card, "cardlist": list, "menu": menu}
}

func TestBrandRendersInEveryWidget(t *testing.T) {
	brand := &Brand{Name: "Acme", URL: "https://acme.test", LogoSVG: logoSVG}
	for kind, w := range widgetsWithBrand(brand) {
		t.Run(kind, func(t *testing.T) {
			doc, err := w.Document()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
				t.Fatalf("document does not parse: %v", err)
			}
			for _, want := range []string{
				`class="gomu-brand-name">Acme<`,
				`data-gomu-brand="https://acme.test"`,
				`<circle cx="8" cy="8" r="7"/>`,
			} {
				if !strings.Contains(doc, want) {
					t.Errorf("document missing %q", want)
				}
			}
			// The brand leads the status bar: bottom left, ahead of the
			// runtime's message. Matched on the class attribute so the
			// stylesheet's own ".gomu-brand" rule is not mistaken for the
			// markup.
			bar := strings.Index(doc, `class="gomu-statusbar"`)
			mark := strings.Index(doc, `class="gomu-brand`)
			status := strings.Index(doc, `class="gomu-status"`)
			if bar < 0 || mark < bar {
				t.Error("brand is not inside the status bar")
			}
			if status >= 0 && status < mark {
				t.Error("brand must precede the status message")
			}
		})
	}
}

func TestBrandDataURILogoRendersAsImage(t *testing.T) {
	c := canonicalCard()
	c.Brand = &Brand{Name: "Acme", LogoDataURI: logoDataURI, LogoAlt: "Acme logo"}
	doc, err := c.Document()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, `<img class="gomu-brand-logo" src="`+logoDataURI+`" alt="Acme logo">`) {
		t.Error("document missing the data URI logo image")
	}
}

// The brand rides in the status bar, so it shows with no title — and with no
// toolbar at all, where the title was the toolbar's only other content.
func TestBrandShowsWithoutTitle(t *testing.T) {
	brand := &Brand{Name: "Acme"}
	for kind, w := range widgetsWithBrand(brand) {
		t.Run(kind, func(t *testing.T) {
			switch v := w.(type) {
			case *Table:
				v.Title = ""
			case *Form:
				v.Title = ""
			case *Card:
				v.Title = ""
			case *CardList:
				v.Title = ""
			case *Menu:
				v.Title = ""
			}
			doc, err := w.Document()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(doc, `class="gomu-brand"`) {
				t.Error("document missing the brand")
			}
		})
	}

	// And a widget whose toolbar held nothing but the brand now has no
	// toolbar at all — the mark moved to the bar at the foot.
	c := canonicalCard()
	c.Title, c.Brand = "", brand
	doc, err := c.Document()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc, `class="gomu-toolbar"`) {
		t.Error("a titleless card should render no toolbar")
	}
}

// An unlinked brand renders as a div, not a button: nothing to click.
func TestBrandWithoutURLIsNotClickable(t *testing.T) {
	c := canonicalCard()
	c.Brand = &Brand{Name: "Acme"}
	doc, err := c.Document()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc, "data-gomu-brand") {
		t.Error("unlinked brand should not carry the openLink hook")
	}
	if !strings.Contains(doc, `<div class="gomu-brand">`) {
		t.Error("unlinked brand should render as a div")
	}
}

func TestBrandInvalidFailsDocument(t *testing.T) {
	c := canonicalCard()
	c.Brand = &Brand{LogoSVG: `<svg onclick="x()"></svg>`}
	if _, err := c.Document(); err == nil {
		t.Fatal("expected Document to reject an unsafe brand logo")
	}
}

func TestBrandGolden(t *testing.T) {
	l := canonicalCardList()
	l.Brand = &Brand{Name: "Acme", URL: "https://acme.test", LogoSVG: logoSVG}
	doc, err := l.Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "brand.html")
	if *update {
		if err := os.WriteFile(golden, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if doc != string(want) {
		t.Error("document does not match golden file; run `go test -run TestBrandGolden -update ./...` and review the diff")
	}
}
