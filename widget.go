// Package gomukit provides prebuilt, parameterized, interactive HTML widgets
// for MCP Apps (the official Model Context Protocol UI extension,
// io.modelcontextprotocol/ui). Widgets render as fully self-contained HTML
// documents — inline CSS, inline JavaScript, no external references — ready
// to serve as ui:// template resources from any Go MCP server.
//
// The core packages are SDK-agnostic; package gosdk adapts widgets to the
// official modelcontextprotocol/go-sdk.
package gomukit

import "github.com/techthos/gomukit/uispec"

// Widget is a renderable MCP Apps UI template. The widget's ui:// URI is
// available as Descriptor().URI.
type Widget interface {
	// Document renders the complete self-contained HTML document.
	Document() (string, error)
	// Descriptor returns registration data for the template resource.
	Descriptor() uispec.ResourceDescriptor
	// ToolMeta returns the _meta map linking a tool to this widget:
	// {"ui": {"resourceUri": ...}}.
	ToolMeta() map[string]any
	// Validate checks the widget configuration.
	Validate() error
}

// Align positions cell or field content.
type Align string

const (
	AlignStart  Align = "start"
	AlignCenter Align = "center"
	AlignEnd    Align = "end"
)

// SortSpec is a default sort order for a table.
type SortSpec struct {
	Key  string `json:"key"`
	Desc bool   `json:"desc,omitempty"`
}

// EmptyState configures the message shown when a widget has no data.
type EmptyState struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	// Immediate shows the empty state on first paint. By default a widget
	// rendered without an InitialData snapshot is treated as not loaded yet:
	// it shows a loading skeleton and reaches the empty state only once data
	// has resolved — from a tool result, a LoadTool call, or a short wait for
	// a host that pushes neither. Set Immediate for a widget that is genuinely
	// empty at render time and has no data coming.
	Immediate bool `json:"immediate,omitempty"`
}
