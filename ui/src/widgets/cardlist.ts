// CardList widget behavior: renders records as cards in a horizontally
// scrolling strip, wires filter/sort/pagination/selection, and fires per-card
// and bulk actions as MCP tool calls — the same runtime model as the table,
// laid out as a carousel that fits a narrow chat pane.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear, delegate, h } from "../dom";
import { awaitData, seeded, SKELETON_ROWS, skeletonCard } from "../loading";
import { confirmAction } from "../confirm-modal";
import { refreshDropdown, releaseDropdowns } from "../dropdown";
import { CallToolResult, M } from "../protocol";
import {
	clampPage,
	filterRows,
	pageCount,
	pageSlice,
	SortSpec,
	sortRows,
	Store,
} from "../state";
import { errorText, textOf } from "../status";
import {
	ActionCfg,
	CardTemplateCfg,
	renderCard,
	resolveArgs,
	templateActions,
	templateKeys,
} from "./card-common";
import {
	clearInputErrors,
	collectInputs,
	enhanceDescriptionInputs,
	hasInputs,
	type InputValues,
	validateInputs,
	watchInputs,
} from "./descriptions";
import { carouselState, stepFor } from "./carousel";

interface CardListCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	pageSize: number;
	filterable: boolean;
	card: CardTemplateCfg;
	// Grow the strip in place instead of paging it (CardList.LoadMore).
	loadMore?: boolean;
	sort?: { key: string; label: string }[];
	defaultSort?: SortSpec;
	selection?: { bulk: ActionCfg[] };
	empty?: { title?: string; body?: string; immediate?: boolean };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

interface CardListState {
	// False until data has actually resolved. An unloaded strip shows skeleton
	// cards: "no records yet" and "no records at all" are different answers,
	// and only the second one is the empty state's to give.
	loaded: boolean;
	rows: Row[];
	sort: SortSpec | null;
	filter: string;
	page: number;
	// Starts at the configured PageSize; the pagination bar's chooser (when the
	// widget renders one) moves it.
	pageSize: number;
	// LoadMore mode only: how many records the strip currently shows. Grows by
	// pageSize per "Load more", and resets whenever the filter or sort changes
	// the set being read.
	revealed: number;
	selected: string[];
	status: "idle" | "loading";
	statusKind?: "error" | "success";
	statusMsg?: string;
}

const STATUS_CLEAR_MS = 4000;
const FILTER_DEBOUNCE_MS = 150;
// Pointer travel before a press turns into a drag rather than a click.
const DRAG_THRESHOLD_PX = 5;

export function mountCardList(ctx: MountContext): void {
	const cfg = ctx.config as unknown as CardListCfg;
	const { root, bridge } = ctx;

	const stripEl = root.querySelector<HTMLElement>("[data-gomu-cards]");
	if (!stripEl || typeof cfg.card !== "object" || typeof cfg.card.header !== "object") return;
	const strip: HTMLElement = stripEl;
	const navEls = [...root.querySelectorAll<HTMLButtonElement>("[data-gomu-scroll]")];
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gomu-empty]");
	const emptyTitleEl = emptyEl?.querySelector("h3") ?? null;
	const emptyTitleDefault = emptyTitleEl?.textContent ?? "";
	const paginationEl = root.querySelector<HTMLElement>("[data-gomu-pagination]");
	const pageInfoEl = root.querySelector<HTMLElement>("[data-gomu-page-info]");
	const bulkEl = root.querySelector<HTMLElement>("[data-gomu-bulk]");
	const bulkCountEl = root.querySelector<HTMLElement>("[data-gomu-bulk-count]");
	const selectAllEl = root.querySelector<HTMLInputElement>("[data-gomu-select-all]");
	const sortSelectEl = root.querySelector<HTMLSelectElement>("[data-gomu-sort-select]");
	const pageSizeEl = root.querySelector<HTMLSelectElement>("[data-gomu-page-size]");

	const filterKeys = templateKeys(cfg.card);
	const cardActions = templateActions(cfg.card);
	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	const store = new Store<CardListState>({
		loaded: seeded(ctx.initialData, cfg.rowsKey) || !!cfg.empty?.immediate,
		rows: rowsFrom(ctx.initialData, cfg.rowsKey),
		sort: cfg.defaultSort ?? null,
		filter: "",
		page: 0,
		pageSize: cfg.pageSize,
		revealed: cfg.pageSize,
		selected: [],
		status: "idle",
	});

	let statusTimer: ReturnType<typeof setTimeout> | undefined;
	let lastViewKey = "";
	// What the reader has put into each card's content controls, by record id.
	// The strip is rebuilt wholesale on every state change, so the answers live
	// here and are handed back to each card as it is re-rendered.
	const asks = hasInputs(cfg.card.content?.items ?? []);
	const inputs = new Map<string, InputValues>();

	function inputsFor(id: string): InputValues {
		let values = inputs.get(id);
		if (!values) {
			values = {};
			inputs.set(id, values);
		}
		return values;
	}

	/** The card element an event came from, which is the scope of its inputs. */
	function cardOf(el: Element): HTMLElement | null {
		return el.closest<HTMLElement>("[data-gomu-card-id]");
	}

	function visible(s: CardListState): { pageRows: Row[]; total: number } {
		const filtered = sortRows(filterRows(s.rows, s.filter, filterKeys), s.sort);
		return {
			// LoadMore shows one growing run from the top of the set; paging
			// shows one window of it.
			pageRows: cfg.loadMore
				? filtered.slice(0, s.revealed)
				: pageSlice(filtered, s.page, s.pageSize),
			total: filtered.length,
		};
	}

	function selectedRows(): Row[] {
		const ids = new Set(store.get().selected);
		return store.get().rows.filter((r) => ids.has(rowID(r)));
	}

	// --- actions ---

	function applyResult(res: CallToolResult): void {
		// Any answer from the host ends the wait, including one that carries no
		// records: that is the list's data, and it is empty.
		const patch: Partial<CardListState> = { status: "idle", loaded: true };
		if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
			patch.rows = rowsFrom(res.structuredContent, cfg.rowsKey);
			patch.selected = [];
		}
		if (res.isError) {
			patch.statusKind = "error";
			patch.statusMsg = textOf(res) ?? "The action failed.";
		} else {
			patch.statusKind = "success";
			patch.statusMsg = textOf(res) ?? "Done";
			clearTimeout(statusTimer);
			statusTimer = setTimeout(() => {
				store.set({ statusKind: undefined, statusMsg: undefined });
			}, STATUS_CLEAR_MS);
		}
		store.set(patch);
	}

	async function fire(action: ActionCfg, row: Row | null, answers: InputValues = {}): Promise<void> {
		if (action.kind === "link") {
			const href = row?.[action.hrefKey ?? ""];
			if (typeof href === "string" && href !== "") void bridge.openLink(href);
			return;
		}
		if (!action.tool) return;
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Working…" });
		try {
			applyResult(
				await bridge.callTool(action.tool, {
					...resolveArgs(action, row, selectedRows()),
					...answers,
				}),
			);
		} catch (e) {
			store.set({
				status: "idle",
				statusKind: "error",
				statusMsg: errorText(e, "The action failed."),
			});
		}
	}

	// Native confirm() is silently disabled in sandboxed MCP Apps iframes, so
	// confirmation is a two-phase button: first click arms it and shows the
	// confirm text, a second click within the window fires.
	function armOrFire(
		btn: HTMLElement,
		action: ActionCfg,
		row: Row | null,
		answers: InputValues = {},
	): void {
		if (action.confirm) {
			confirmAction(
				btn,
				{ message: action.confirm, confirmLabel: action.label, variant: action.variant },
				() => void fire(action, row, answers),
			);
			return;
		}
		void fire(action, row, answers);
	}

	// --- rendering ---

	// Identifies the focused control by what it is rather than by node, so it
	// can be found again on the card that replaces the one holding it.
	function focusedControl(): string | null {
		const el = document.activeElement as HTMLElement | null;
		if (!el || !strip.contains(el)) return null;
		const card = el.closest<HTMLElement>("[data-gomu-card-id]");
		if (!card) return el.hasAttribute("data-gomu-reveal") ? "reveal" : null;
		const id = card.getAttribute("data-gomu-card-id") ?? "";
		const action = el.getAttribute("data-gomu-action");
		if (action !== null) return `action:${id}:${action}`;
		const input = el.getAttribute("data-gomu-input");
		return input !== null ? `input:${id}:${input}` : `select:${id}`;
	}

	function restoreFocus(key: string | null): void {
		if (key === null) return;
		let sel: string;
		if (key === "reveal") {
			sel = "[data-gomu-reveal]";
		} else {
			const [kind, id, index] = key.split(":");
			const card = `[data-gomu-card-id="${CSS.escape(id ?? "")}"]`;
			sel =
				kind === "action"
					? `${card} [data-gomu-action="${CSS.escape(index ?? "")}"]`
					: kind === "input"
						? `${card} [data-gomu-input="${CSS.escape(index ?? "")}"]`
						: `${card} [data-gomu-select-card]`;
		}
		// preventScroll: refocusing must not undo the scroll just restored.
		strip.querySelector<HTMLElement>(sel)?.focus({ preventScroll: true });
	}

	function restoreScroll(left: number): void {
		if (strip.scrollLeft === left) return;
		// The strip scrolls smoothly by CSS; restoring a position is not a
		// journey the reader took, so it must not be animated.
		if (typeof strip.scrollTo === "function") {
			strip.scrollTo({ left, behavior: "instant" as ScrollBehavior });
		} else {
			strip.scrollLeft = left;
		}
	}

	// The tail of a LoadMore strip: reveals the next batch, and meanwhile says
	// how much of the set is already in hand.
	function moreTile(shown: number, total: number, busy: boolean): HTMLElement {
		return h(
			"button",
			{
				type: "button",
				class: "gomu-card-more",
				"data-gomu-reveal": "",
				disabled: busy,
				"aria-label": `Load more records (${shown} of ${total} shown)`,
			},
			h("span", { class: "gomu-card-more-label" }, "Load more"),
			h("span", { class: "gomu-card-more-count" }, `${shown} of ${total}`),
		);
	}

	function render(s: CardListState): void {
		const { pageRows, total } = visible(s);
		const busy = s.status === "loading";
		const pending = !s.loaded;
		const selected = new Set(s.selected);

		// The strip is rebuilt wholesale on every state change — including one
		// as small as ticking a card's checkbox — and emptying it drops both the
		// reader's place in the run and keyboard focus. A slice of different
		// records is the one case where starting from the head is right; for
		// everything else the position is carried across the swap, so the card
		// under the reader's eye stays under it.
		const viewKey = `${s.filter} ${s.sort?.key ?? ""} ${s.sort?.desc ?? ""} ${s.page}`;
		const newView = viewKey !== lastViewKey;
		lastViewKey = viewKey;
		const keepScroll = newView ? 0 : strip.scrollLeft;
		const keepFocus = newView ? null : focusedControl();

		// The dropdowns of the cards being replaced own panels outside the strip.
		releaseDropdowns(strip);
		clear(strip);
		if (pending) {
			const n = cfg.pageSize > 0 ? Math.min(cfg.pageSize, SKELETON_ROWS) : SKELETON_ROWS;
			for (let i = 0; i < n; i++) strip.append(skeletonCard());
		}
		// Nothing to iterate while pending: records only ever arrive together
		// with the flag that ends the wait.
		for (const row of pageRows) {
			strip.append(
				renderCard(cfg.card, row, {
					id: rowID(row),
					selectable: !!cfg.selection,
					selected: selected.has(rowID(row)),
					busy,
					values: inputs.get(rowID(row)),
				}),
			);
		}
		// Selects become dropdowns only now: a panel is placed against the widget
		// root, which a card could not reach while it was still detached.
		if (asks) enhanceDescriptionInputs(strip);
		if (cfg.loadMore && pageRows.length < total) {
			strip.append(moreTile(pageRows.length, total, busy));
		}
		restoreScroll(keepScroll);
		restoreFocus(keepFocus);

		// sort control
		if (sortSelectEl) {
			sortSelectEl.value = s.sort ? `${s.sort.key}|${s.sort.desc ? "desc" : "asc"}` : "";
			refreshDropdown(sortSelectEl);
		}

		// empty state — never while pending: "no records" is a claim the list
		// cannot make before it has been given any.
		if (emptyEl) {
			emptyEl.hidden = pending || total > 0;
			if (emptyTitleEl) {
				emptyTitleEl.textContent =
					total === 0 && s.rows.length > 0 ? "No matching cards" : emptyTitleDefault;
			}
		}

		// pagination
		const pages = pageCount(total, s.pageSize);
		if (paginationEl) {
			// A single page normally means no bar — but with a page-size chooser
			// the bar is also the way back to a smaller page, so it stays.
			paginationEl.hidden =
				pending || !!cfg.loadMore || s.pageSize <= 0 || (pages <= 1 && !pageSizeEl);
			if (pageInfoEl) {
				const from = total === 0 ? 0 : s.page * s.pageSize + 1;
				const to = Math.min((s.page + 1) * s.pageSize, total);
				pageInfoEl.textContent = `${from}–${to} of ${total}`;
			}
			for (const btn of paginationEl.querySelectorAll<HTMLButtonElement>("[data-gomu-page]")) {
				const dir = btn.getAttribute("data-gomu-page");
				btn.disabled = busy || (dir === "prev" ? s.page <= 0 : s.page >= pages - 1);
			}
			if (pageSizeEl) {
				pageSizeEl.value = String(s.pageSize);
				pageSizeEl.disabled = busy;
				refreshDropdown(pageSizeEl);
			}
		}

		// selection
		if (selectAllEl) {
			selectAllEl.checked = pageRows.length > 0 && pageRows.every((r) => selected.has(rowID(r)));
		}
		if (bulkEl) {
			bulkEl.hidden = selected.size === 0;
			if (bulkCountEl) bulkCountEl.textContent = `${selected.size} selected`;
			for (const btn of bulkEl.querySelectorAll<HTMLButtonElement>("[data-gomu-bulk-action]")) {
				btn.disabled = busy;
			}
		}

		// status — the skeleton says something is coming, the bar says what.
		if (statusEl) {
			const msg = s.statusMsg ?? (pending ? "Loading…" : "");
			statusEl.hidden = msg === "";
			statusEl.textContent = msg;
			statusEl.className = "gomu-status";
			if (busy || pending) statusEl.className += " gomu-status--loading";
			else if (s.statusKind) statusEl.className += ` gomu-status--${s.statusKind}`;
		}

		syncCarousel();
	}

	// --- carousel ---

	// Nav controls mirror the strip's scroll geometry: hidden when everything
	// fits, disabled at whichever end the strip is resting on. Runs on every
	// scroll event, so it writes only when the state actually changed —
	// otherwise each frame of a drag would dirty layout for nothing.
	let lastCarousel = "";
	function syncCarousel(): void {
		const state = carouselState(strip);
		const key = `${state.overflowing}${state.atStart}${state.atEnd}`;
		if (key === lastCarousel) return;
		lastCarousel = key;
		strip.classList.toggle("gomu-card-strip--scrollable", state.overflowing);
		for (const btn of navEls) {
			btn.hidden = !state.overflowing;
			btn.disabled =
				btn.getAttribute("data-gomu-scroll") === "prev" ? state.atStart : state.atEnd;
		}
	}

	function isRTL(): boolean {
		return typeof getComputedStyle === "function" && getComputedStyle(strip).direction === "rtl";
	}

	delegate(root, "click", "scroll", (_el, dir) => {
		const left = stepFor(strip, dir === "prev" ? "prev" : "next", isRTL());
		// scrollBy is absent in some non-browser DOM implementations.
		if (typeof strip.scrollBy === "function") strip.scrollBy({ left });
		else strip.scrollLeft += left;
	});

	strip.addEventListener("scroll", syncCarousel);

	// The strip's overflow depends on the host's width, which changes without
	// any state change of ours.
	if (typeof ResizeObserver !== "undefined") {
		new ResizeObserver(() => syncCarousel()).observe(strip);
	}

	// Drag-to-swipe. Touch pointers are left alone: the browser's own inertial
	// panning is better than anything reimplemented here. Mouse and pen get a
	// drag that mirrors it, and a drag that ends on a card must not fire that
	// card's action, so the click it synthesizes is swallowed.
	let drag: { id: number; startX: number; startScroll: number; moved: boolean } | null = null;
	let swallowClick = false;

	strip.addEventListener("pointerdown", (ev) => {
		if (ev.pointerType === "touch" || ev.button !== 0) return;
		if (ev.target instanceof Element && ev.target.closest("button, a, input, select, textarea, label")) {
			return;
		}
		drag = { id: ev.pointerId, startX: ev.clientX, startScroll: strip.scrollLeft, moved: false };
	});

	strip.addEventListener("pointermove", (ev) => {
		if (!drag || ev.pointerId !== drag.id) return;
		const dx = ev.clientX - drag.startX;
		if (!drag.moved) {
			if (Math.abs(dx) < DRAG_THRESHOLD_PX) return;
			drag.moved = true;
			strip.classList.add("gomu-card-strip--dragging");
			if (typeof strip.setPointerCapture === "function") strip.setPointerCapture(drag.id);
		}
		strip.scrollLeft = drag.startScroll - dx;
		ev.preventDefault();
	});

	function endDrag(): void {
		if (!drag) return;
		if (drag.moved) {
			swallowClick = true;
			strip.classList.remove("gomu-card-strip--dragging");
			// Do not wait for the trailing scroll event to settle the controls.
			syncCarousel();
		}
		drag = null;
	}
	strip.addEventListener("pointerup", endDrag);
	strip.addEventListener("pointercancel", endDrag);

	strip.addEventListener(
		"click",
		(ev) => {
			if (!swallowClick) return;
			swallowClick = false;
			ev.preventDefault();
			ev.stopPropagation();
		},
		true,
	);

	// --- events ---

	if (sortSelectEl) {
		sortSelectEl.addEventListener("change", () => {
			const v = sortSelectEl.value;
			if (v === "") {
				store.set({ sort: null, page: 0, revealed: cfg.pageSize });
				return;
			}
			const sep = v.lastIndexOf("|");
			const key = v.slice(0, sep);
			const desc = v.slice(sep + 1) === "desc";
			store.set({ sort: { key, desc }, page: 0, revealed: cfg.pageSize });
		});
	}

	let filterTimer: ReturnType<typeof setTimeout> | undefined;
	delegate(root, "input", "filter", (el) => {
		clearTimeout(filterTimer);
		filterTimer = setTimeout(() => {
			// A different set of records is a different run: LoadMore starts
			// it from the first batch again.
			store.set({
				filter: (el as HTMLInputElement).value,
				page: 0,
				revealed: cfg.pageSize,
			});
		}, FILTER_DEBOUNCE_MS);
	});

	// Reveals the next batch. The strip keeps its scroll position across the
	// re-render, so the new cards extend the run from where the tile stood
	// rather than moving the reader.
	delegate(root, "click", "reveal", () => {
		const s = store.get();
		store.set({ revealed: s.revealed + (s.pageSize > 0 ? s.pageSize : s.rows.length) });
	});

	delegate(root, "click", "page", (_el, dir) => {
		const s = store.get();
		const total = filterRows(s.rows, s.filter, filterKeys).length;
		const next = dir === "prev" ? s.page - 1 : s.page + 1;
		store.set({ page: clampPage(next, total, s.pageSize) });
	});

	// Resizing the page invalidates the current page number, so go back to the
	// first one rather than guess where the reader was.
	delegate(root, "change", "page-size", (el) => {
		const size = Number((el as HTMLSelectElement).value);
		if (Number.isFinite(size) && size > 0) store.set({ pageSize: size, page: 0 });
	});

	delegate(root, "change", "select-card", (el) => {
		const id = el.closest("[data-gomu-card-id]")?.getAttribute("data-gomu-card-id");
		if (id === null || id === undefined) return;
		const sel = new Set(store.get().selected);
		if ((el as HTMLInputElement).checked) sel.add(id);
		else sel.delete(id);
		store.set({ selected: [...sel] });
	});

	delegate(root, "change", "select-all", (el) => {
		const s = store.get();
		const sel = new Set(s.selected);
		const { pageRows } = visible(s);
		if ((el as HTMLInputElement).checked) {
			for (const r of pageRows) sel.add(rowID(r));
		} else {
			for (const r of pageRows) sel.delete(rowID(r));
		}
		store.set({ selected: [...sel] });
	});

	delegate(root, "click", "action", (el, value) => {
		const action = cardActions[Number(value)];
		if (!action) return;
		const card = cardOf(el);
		const id = card?.getAttribute("data-gomu-card-id");
		const row = store.get().rows.find((r) => rowID(r) === id) ?? null;
		// A card's controls answer for that card only, so they travel with its
		// own buttons and with no bulk action.
		if (asks && card) {
			if (!validateInputs(card)) {
				store.set({ statusKind: "error", statusMsg: "Please fix the highlighted fields." });
				return;
			}
			armOrFire(el, action, row, collectInputs(card));
			return;
		}
		armOrFire(el, action, row);
	});

	delegate(root, "click", "bulk-action", (el, value) => {
		const action = cfg.selection?.bulk[Number(value)];
		if (action) armOrFire(el, action, null);
	});

	delegate(root, "click", "link", (_el, href) => {
		if (href !== "") void bridge.openLink(href);
	});

	if (asks) {
		watchInputs(strip, (name, value, el) => {
			const card = cardOf(el);
			if (!card) return;
			inputsFor(card.getAttribute("data-gomu-card-id") ?? "")[name] = value;
			if (el.checkValidity()) clearInputErrors(el.closest(".gomu-desc-item") ?? card);
		});
	}

	// --- host notifications ---

	bridge.on(M.toolInput, () => {
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
	});
	bridge.on(M.toolResult, (params) => {
		applyResult((params ?? {}) as CallToolResult);
	});
	bridge.on(M.toolCancelled, () => {
		// The call this list was waiting on is not coming back.
		store.set({ status: "idle", loaded: true, statusKind: undefined, statusMsg: undefined });
	});

	// Re-render when a host context lands: Intl formatting depends on the
	// host's locale/timeZone, which may arrive after the first paint.
	document.addEventListener(HOST_CONTEXT_EVENT, () => render(store.get()));

	store.subscribe(render);
	render(store.get());

	// Load-time hydration: once a host is connected, fetch fresh records and
	// replace the baked snapshot. Silent on success and on failure.
	async function hydrate(): Promise<void> {
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
		try {
			const res = await bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {});
			const patch: Partial<CardListState> = {
				status: "idle",
				loaded: true,
				statusKind: undefined,
				statusMsg: undefined,
			};
			if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
				patch.rows = rowsFrom(res.structuredContent, cfg.rowsKey);
				patch.selected = [];
			}
			store.set(patch);
		} catch {
			// A load that failed still answers the question the skeleton asks.
			store.set({ status: "idle", loaded: true, statusKind: undefined, statusMsg: undefined });
		}
	}
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (ok) void hydrate();
		});
	}

	// Bounded wait for data the host may push on its own (a tool-result
	// notification): without it a list with no snapshot and no load tool would
	// hold its skeleton forever against a host that never sends one. Also ends
	// the wait when no host answered at all, load tool or not.
	if (!store.get().loaded) {
		awaitData(ctx.ready, !!cfg.loadTool, () => store.set({ loaded: true }));
	}
}
