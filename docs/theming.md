# <img src="assets/gomukit-icon.svg" alt="" width="28" align="center"> Theming

gomukit uses a two-layer design-token system as CSS custom properties,
prefixed `--gomu-*`, scoped under `.gomu-root`.

## How a color is resolved

Every semantic token defaults to the **host-injected MCP Apps variable**
with a library fallback:

```css
--gomu-color-text: var(--color-text-primary, var(--gomu-p-body));
```

MCP Apps hosts (Claude, ChatGPT, …) deliver `hostContext.styles.variables`
(`--color-background-primary`, `--font-sans`, `--border-radius-md`, semantic
danger/success colors, …) during `ui/initialize`; the runtime sets them as
inline style on the document root. Result: **widgets automatically match the
host's look** without configuration, and still render sensibly with no host.

## Dark mode

- The host's `theme: "light" | "dark"` sets `data-gomu-theme` on the root
  element; token overrides react to it.
- Without a host theme, `@media (prefers-color-scheme: dark)` provides the
  fallback.
- `:root { color-scheme }` starts at `light dark`, so host-provided
  `light-dark(...)` values self-adapt, and is then pinned to the host's theme
  once `hostContext` arrives. That pin is what keeps the iframe canvas
  transparent (see below), so do not override it to `normal`.
- Live `ui/notifications/host-context-changed` updates re-style the widget
  without reload.

## Embedding: hiding the iframe

The widget shell has no border and no corner radius by default: hosts embed
widgets in a chat bubble or panel that already carries its own frame, and a
second outlined box inside it reads as a box in a box. `Framed: true` restores
the card chrome (1px border, `RadiusL` corners).

A widget renders inside the host's iframe. By default it paints a page fill
plus an 8px gutter, so the frame reads as a panel of its own. `Transparent`
removes both, leaving only the widget's own surface on the host:

```go
table := &gomukit.Table{ /* ... */, Theme: &theme.Theme{Transparent: true}}
```

Card, tile, input and dropdown fills are untouched, so text contrast never
depends on what the host draws behind the frame. `ColorPage` and `PagePad` set
the same two knobs individually.

Three facts govern whether this actually looks embedded:

1. **The document canvas is transparent by default.** No attribute is needed;
   `allowtransparency` was an IE thing and is not in the spec. Transparency
   only breaks when something paints: the widget's page fill, or the host.
2. **The host must leave the `<iframe>` element unpainted.** The UA default
   border is `2px inset`, so the host needs `border: 0` and no `background`.
   gomukit cannot influence this from inside.
3. **The root color schemes must match.** Per css-color-adjust, when the
   embedded root element's used color scheme differs from the `<iframe>`
   element's, the UA replaces the transparent canvas with an opaque one in the
   embedded document's `Canvas` color. Chrome does this, Firefox does not, and
   **no author-level `background: transparent` can undo it** — the rule fires
   precisely because the canvas would otherwise be transparent. The runtime
   therefore pins `:root { color-scheme }` to `hostContext.theme` rather than
   leaving it at `light dark`, which would resolve from the OS preference and
   mismatch a host whose theme disagrees with it.

What transparency does **not** buy, because an iframe is still a nested
browsing context:

- Popovers cannot escape the frame. Dropdown panels, tooltips and focus rings
  are clipped by the iframe box; panels are anchored on `.gomu-root` and
  flipped to stay inside it.
- The frame rectangle keeps swallowing pointer events over transparent areas.
- Text selection cannot span host and widget, and `position: fixed` resolves
  against the iframe.
- Host webfonts do not load: the spec CSP has no `font-src`, so `--font-sans`
  resolves only against locally installed families.

`examples/harness` renders every story frameless by default and has a
**Frameless** toggle in the top bar to compare against the framed variant
(`/story/<id>?transparent=0`).

## Overriding tokens: the Theme struct

```go
import "github.com/techthos/gomukit/theme"

t := &theme.Theme{
    ColorPrimary: "#7c3aed",
    RadiusM:      "0.5rem",
    FontFamily:   `"Inter", sans-serif`,
    SpaceUnit:    "0.3rem", // roomier layout
    Extra: map[string]string{
        "--gomu-table-stripe": "rgb(0 0 0 / 4%)",
    },
}
table := &gomukit.Table{ /* ... */, Theme: t}
```

`Theme.CSS()` emits a `.gomu-root { … }` block (preceded by a `:root { … }`
block for the document-level page tokens) appended **after** the base
stylesheet: non-empty fields win over the defaults (including host values);
empty fields keep host-aware behavior. `Extra` keys must start with
`--gomu-`. Values are validated against CSS/HTML breakout
(`Theme.Validate()`).

## Token reference (semantic layer)

The palette, type scale and shape vocabulary come from `DESIGN.md` at the
repo root: warm-cream chrome, one saturated red reserved for the primary
action, 16px as the radius of nearly everything, and no shadow outside the
modal layer. Fields on `Theme` override the semantic tokens; the
library-owned ones below (no host variable, no `Theme` field) take an `Extra`
entry.

| Token | Purpose | Host variable consulted |
|---|---|---|
| `--gomu-color-bg` | canvas: widget shell, modal, text inputs | `--color-background-primary` |
| `--gomu-color-page` | page fill behind the widget (`transparent` hides the frame) | `--color-background-primary` |
| `--gomu-color-surface` | cream: cards, tiles, chips, table header, hovers | `--color-background-secondary` |
| `--gomu-color-secondary` / `-pressed` | secondary button fill and its pressed state | — |
| `--gomu-color-heading` | ink: headings, labels, button text | `--color-text-primary` |
| `--gomu-color-text` | body prose | `--color-text-primary` |
| `--gomu-color-text-muted` | secondary text | `--color-text-secondary` |
| `--gomu-color-link` | inline links (weight, not hue, carries them) | — |
| `--gomu-color-border` | hairline dividers | `--color-border-primary` |
| `--gomu-color-border-strong` | the heavier seam around a control you can type into | — |
| `--gomu-color-primary` / `-pressed` | the one accent: primary buttons | `--color-text-accent` |
| `--gomu-color-danger` | destructive actions, errors | `--color-text-danger` |
| `--gomu-color-success` / `-bg` | success text and its pale chip fill | `--color-text-success` |
| `--gomu-color-warning` | warnings (the one tone `DESIGN.md` does not supply) | `--color-text-warning` |
| `--gomu-color-info` | informational chips — the editorial accent, never the brand red | — |
| `--gomu-color-focus` / `-inner` | the focus ring's blue and the gap inside it | — |
| `--gomu-font` / `--gomu-font-mono` | typography (Inter, then the system stack) | `--font-sans` / `--font-mono` |
| `--gomu-text-body/sm/caption/title` | type scale (16 / 14 / 12 / 18px) | — |
| `--gomu-radius-s/m/l` | corner radii (8 / 16 / 32px) | `--border-radius-sm/md/lg` |
| `--gomu-radius-full` | pill: chips, search bar, icon buttons | — |
| `--gomu-h-btn` / `--gomu-h-input` / `--gomu-h-search` | control heights (40 / 44 / 48px) | — |
| `--gomu-shadow-modal` | the system's only shadow, under the modal | — |
| `--gomu-ring` | focus ring stack | — |
| `--gomu-space-unit` | base spacing unit (0.25rem) | — |
| `--gomu-page-pad` | gutter between widget and iframe edge (8px; set on `:root`) | — |
| `--gomu-card-border-width` / `--gomu-card-radius` | widget shell frame (both `0`; `Framed` sets them) | — |
| `--gomu-card-width` | width of one card in the CardList carousel (17rem) | — |
