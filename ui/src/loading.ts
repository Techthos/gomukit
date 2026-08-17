// The third data state.
//
// A widget that has not been given data yet is neither full nor empty, but the
// runtime used to know only those two: an empty rows array painted the "No
// data" block on first paint, before any host had spoken. That is a wrong
// answer, not a missing one. So a widget starts unloaded, shows a skeleton,
// and reaches its empty state only once data has actually resolved.
import { h } from "./dom";

/** Placeholder rows a skeleton stands in for. Enough to read as a list, few
 * enough that the real data rarely shrinks the widget when it lands. */
export const SKELETON_ROWS = 3;

/** How long an unloaded widget waits for a host to push data before it settles
 * for what it knows. A host that renders a widget for a tool call delivers the
 * result right after the handshake, but nothing in the spec obliges it to, and
 * a skeleton with no end is worse than an honest empty state. */
export const LOAD_GRACE_MS = 1500;

/** Whether the document was rendered with data under key. A baked snapshot
 * means the widget is loaded from its first paint. */
export function seeded(
	data: Record<string, unknown> | null | undefined,
	key: string,
): boolean {
	return !!data && key in data;
}

/**
 * Calls settle once the widget has waited as long as it usefully can.
 * Guarding against a second settle is the caller's, which is one state write
 * either way.
 */
export function awaitData(
	ready: Promise<boolean> | undefined,
	hydrates: boolean,
	settle: () => void,
): void {
	// No handshake in play at all: nothing is ever going to push data.
	if (!ready) {
		settle();
		return;
	}
	// A load tool answers for the widget, and its own failure path ends the
	// wait. The only thing left to watch is a handshake nobody answered.
	if (hydrates) {
		void ready.then((ok) => {
			if (!ok) settle();
		});
		return;
	}
	// Otherwise the widget is waiting on a result the host pushes of its own
	// accord, which lands right after the handshake if it is coming at all.
	// Timed from mount rather than from the handshake: a document opened
	// without a host would otherwise hold its skeleton for the whole of the
	// handshake's own much longer timeout.
	setTimeout(settle, LOAD_GRACE_MS);
}

/** One placeholder bar. Bars vary their width by position in CSS, so a run of
 * them reads as text rather than as a stack of identical blocks. */
export function skeletonBar(): HTMLElement {
	return h("span", { class: "gomu-skeleton" });
}

/** A run of placeholder bars, as a block. */
export function skeletonLines(n: number): HTMLElement {
	const box = h("div", { class: "gomu-skeleton-lines" });
	for (let i = 0; i < n; i++) box.append(skeletonBar());
	return box;
}

/** A card-shaped placeholder: the tile renderCard produces, holding lines
 * instead of sections, so the strip keeps its pitch until the records land. */
export function skeletonCard(): HTMLElement {
	return h(
		"article",
		{ class: "gomu-card-item gomu-card-item--skeleton", "aria-hidden": "true" },
		h("div", { class: "gomu-card-content" }, skeletonLines(3)),
	);
}
