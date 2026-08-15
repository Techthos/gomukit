import { afterEach, describe, expect, it } from "vitest";
import { applyHostContext, watchSize } from "../src/host";
import { formatCell, setLocale } from "../src/format";
import { Bridge } from "../src/bridge";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

afterEach(() => {
  document.documentElement.removeAttribute("data-gomu-theme");
  document.documentElement.removeAttribute("style");
  document.getElementById("gomu-host-fonts")?.remove();
  setLocale(undefined, undefined);
});

describe("applyHostContext", () => {
  it("applies custom-property variables and rejects other keys", () => {
    applyHostContext({
      styles: {
        variables: {
          "--color-background-primary": "#111",
          "color": "red", // not a custom property — must be ignored
        },
      },
    });
    const root = document.documentElement;
    expect(root.style.getPropertyValue("--color-background-primary")).toBe("#111");
    expect(root.style.getPropertyValue("color")).toBe("");
  });

  it("sets the theme attribute for valid themes only", () => {
    applyHostContext({ theme: "dark" });
    expect(document.documentElement.getAttribute("data-gomu-theme")).toBe("dark");
    applyHostContext({ theme: "purple" as never });
    expect(document.documentElement.getAttribute("data-gomu-theme")).toBe("dark");
  });

  it("pins the root color scheme to the host theme so the canvas stays transparent", () => {
    applyHostContext({ theme: "dark" });
    expect(document.documentElement.style.colorScheme).toBe("dark");
    applyHostContext({ theme: "light" });
    expect(document.documentElement.style.colorScheme).toBe("light");
    applyHostContext({ theme: "purple" as never });
    expect(document.documentElement.style.colorScheme).toBe("light");
  });

  it("injects host fonts once and replaces on update", () => {
    applyHostContext({ styles: { css: { fonts: "@font-face{font-family:A}" } } });
    applyHostContext({ styles: { css: { fonts: "@font-face{font-family:B}" } } });
    const els = document.querySelectorAll("#gomu-host-fonts");
    expect(els).toHaveLength(1);
    expect(els[0]!.textContent).toContain("font-family:B");
  });

  it("tolerates null and empty contexts", () => {
    expect(() => applyHostContext(null)).not.toThrow();
    expect(() => applyHostContext({})).not.toThrow();
  });
});

describe("formatCell", () => {
  it("formats numbers by format spec", () => {
    setLocale("en-US", undefined);
    expect(formatCell(1234.567, "number", "int")).toBe("1,235");
    expect(formatCell(1234.5, "number", "decimal:2")).toBe("1,234.50");
    expect(formatCell(0.42, "number", "percent")).toBe("42%");
    expect(formatCell(9.5, "number", "currency:EUR")).toBe("€9.50");
  });

  it("respects the host locale", () => {
    setLocale("de-DE", undefined);
    expect(formatCell(1234.5, "number", "decimal:2")).toBe("1.234,50");
  });

  it("formats dates with the host time zone", () => {
    setLocale("en-US", "UTC");
    expect(formatCell("2026-07-22T10:30:00Z", "date", "datetime")).toContain("10:30");
    expect(formatCell("2026-07-22T10:30:00Z", "date", "date")).toContain("2026");
  });

  it("falls back to string form for malformed values", () => {
    expect(formatCell("not-a-date", "date", "date")).toBe("not-a-date");
    expect(formatCell("NaN?", "number", "int")).toBe("NaN?");
    expect(formatCell(null, "text")).toBe("");
  });
});

describe("watchSize", () => {
  it("reports a height that is never short of the content", async () => {
    const host = new FakeHost();
    const bridge = new Bridge({ timeoutMs: 500 });
    const el = document.createElement("div");
    document.body.append(el);
    // jsdom has no layout: this is a body 410.40625px tall, which is what
    // scrollHeight rounds down to 410 — one rounding short of the content, and
    // the frame the host sizes from it grows a scrollbar.
    Object.defineProperty(el, "scrollHeight", { configurable: true, get: () => 410 });
    el.getBoundingClientRect = () => ({ height: 410.40625 }) as DOMRect;

    const stop = watchSize(bridge, el);
    await flush();
    const sizes = host.received(M.sizeChanged);
    expect(sizes[0]!.params).toEqual({ height: 411 });

    stop();
    bridge.dispose();
    host.dispose();
    el.remove();
  });
});
