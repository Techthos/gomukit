// Applies hostContext to the document: theme variables, fonts, theme
// attribute, locale — and reports content size back to the host.
import { Bridge } from "./bridge";
import { HostContext } from "./protocol";
import { setLocale } from "./format";

const FONT_STYLE_ID = "gomu-host-fonts";

/** Document event fired after a hostContext has been applied. Behaviors
 * re-render on it because Intl formatting depends on host locale/timeZone. */
export const HOST_CONTEXT_EVENT = "gomukit:hostcontext";

export function emitHostContextApplied(): void {
	document.dispatchEvent(new CustomEvent(HOST_CONTEXT_EVENT));
}

/**
 * Applies a hostContext (from ui/initialize or host-context-changed) to the
 * document. Only custom properties (keys starting with "--") are accepted
 * from styles.variables.
 */
export function applyHostContext(
  ctx: HostContext | null | undefined,
  root: HTMLElement = document.documentElement,
): void {
  if (!ctx) return;

  const vars = ctx.styles?.variables;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      if (k.startsWith("--") && typeof v === "string") {
        root.style.setProperty(k, v);
      }
    }
  }

  const fonts = ctx.styles?.css?.fonts;
  if (typeof fonts === "string" && fonts !== "") {
    let el = document.getElementById(FONT_STYLE_ID);
    if (!el) {
      el = document.createElement("style");
      el.id = FONT_STYLE_ID;
      document.head.appendChild(el);
    }
    el.textContent = fonts;
  }

  if (ctx.theme === "light" || ctx.theme === "dark") {
    root.setAttribute("data-gomu-theme", ctx.theme);
    // Pin the root color scheme to the host's. An iframe canvas is transparent
    // only while the embedded root element's used color scheme matches the
    // <iframe> element's; on a mismatch the UA (Chrome; not Firefox) paints an
    // opaque Canvas rectangle behind the document, which no author-level
    // "background: transparent" can undo. Without this, a dark host embedding a
    // widget on a light OS gets a white slab around it.
    root.style.colorScheme = ctx.theme;
  }

  if (ctx.locale !== undefined || ctx.timeZone !== undefined) {
    setLocale(ctx.locale, ctx.timeZone);
  }
}

/**
 * Watches the document body and reports content size to the host via
 * ui/notifications/size-changed (view -> host per spec). Reports once
 * immediately; returns a stop function.
 */
export function watchSize(bridge: Bridge, el?: HTMLElement): () => void {
  const target = el ?? document.body;
  let raf = 0;

  const report = (): void => {
    // Height only; width is deliberately omitted. A host that pins the iframe
    // width to a reported value couples the frame to this iframe's own inner
    // measurement — the vertical scrollbar and any reflow (e.g. a wrapping
    // toolbar) shave a few pixels off every read, so each tick reports a smaller
    // width and the frame ratchets to zero. Per MCP Apps the iframe fills the
    // available width; only height is content-driven, so report the height and
    // leave width to the host.
    //
    // Rounded up, and never below either measurement: scrollHeight is an
    // integer, so content 410.4px tall reports 410, the host sizes the frame a
    // rounding short of it, and the frame grows a scrollbar over four tenths
    // of a pixel. The rect carries the fraction; scrollHeight carries anything
    // overflowing the body box.
    bridge.sizeChanged(
      Math.max(target.scrollHeight, Math.ceil(target.getBoundingClientRect().height)),
    );
  };
  const schedule = (): void => {
    if (raf) return;
    raf = requestAnimationFrame(() => {
      raf = 0;
      report();
    });
  };

  report();
  if (typeof ResizeObserver === "undefined") {
    return () => {};
  }
  const ro = new ResizeObserver(schedule);
  ro.observe(target);
  return () => {
    ro.disconnect();
    if (raf) cancelAnimationFrame(raf);
  };
}
