package gomukit

import (
	"fmt"
	"strings"

	"github.com/techthos/gomukit/internal/htmlx"
)

// Brand identifies the application a widget belongs to: an optional logo, a
// name, and an optional link. It renders at the bottom left of the widget
// chrome, leading the status bar, and is shared by pointer across widgets.
//
// Documents must stay self-contained, so a logo is never a URL: supply either
// inline SVG markup (LogoSVG, recommended — it needs nothing from the host's
// CSP) or a base64 data URI (LogoDataURI, which renders as an <img> and
// therefore depends on the host allowing img-src data:).
type Brand struct {
	// Name is the application name shown next to the logo. Required unless a
	// logo is set.
	Name string
	// URL is an optional http(s) link opened through the host (ui/openLink);
	// it makes the whole brand clickable.
	URL string
	// LogoSVG is inline <svg> markup. It is author-trusted input, checked
	// only against script-bearing and resource-loading constructs.
	LogoSVG string
	// LogoDataURI is a "data:image/...;base64,..." alternative to LogoSVG.
	LogoDataURI string
	// LogoAlt is the alt text for LogoDataURI. Defaults to Name.
	LogoAlt string
}

// Validate reports whether the brand is usable and safe to embed.
func (b *Brand) Validate() error {
	if b == nil {
		return nil
	}
	if b.LogoSVG != "" && b.LogoDataURI != "" {
		return fmt.Errorf("brand: set only one of LogoSVG and LogoDataURI")
	}
	if b.Name == "" && b.LogoSVG == "" && b.LogoDataURI == "" {
		return fmt.Errorf("brand: needs a Name or a logo")
	}
	if b.LogoSVG != "" {
		if _, err := htmlx.RawSVG(b.LogoSVG); err != nil {
			return fmt.Errorf("brand: %w", err)
		}
	}
	if b.LogoDataURI != "" {
		if err := validateImageDataURI(b.LogoDataURI); err != nil {
			return fmt.Errorf("brand: LogoDataURI: %w", err)
		}
	}
	if b.URL != "" && !isHTTPURL(b.URL) {
		return fmt.Errorf("brand: URL must be http(s), got %q", b.URL)
	}
	return nil
}

// dataURIMediaTypes are the image types a logo data URI may declare. SVG is
// allowed here because an <img> renders it in a sandboxed image context where
// scripts do not execute.
var dataURIMediaTypes = []string{
	"data:image/png;base64,",
	"data:image/jpeg;base64,",
	"data:image/gif;base64,",
	"data:image/webp;base64,",
	"data:image/svg+xml;base64,",
}

func validateImageDataURI(uri string) error {
	payload := ""
	ok := false
	for _, prefix := range dataURIMediaTypes {
		if strings.HasPrefix(uri, prefix) {
			payload, ok = uri[len(prefix):], true
			break
		}
	}
	if !ok {
		return fmt.Errorf("must start with one of %s", strings.Join(dataURIMediaTypes, ", "))
	}
	if payload == "" {
		return fmt.Errorf("empty payload")
	}
	for _, r := range payload {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=':
		default:
			return fmt.Errorf("payload is not base64")
		}
	}
	return nil
}

func isHTTPURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}
