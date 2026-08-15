// Table widget behavior: renders rows from state, wires sort/filter/
// pagination/selection, and fires row/bulk actions as MCP tool calls.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { actionMenuTrigger, createActionMenu } from "../actionmenu";
import { Row, rowsFrom } from "../data";
import { checkbox, clear, delegate, h } from "../dom";
import { refreshDropdown } from "../dropdown";
import { formatCell } from "../format";
import { anyVisible, type RowPredicateCfg, visibleActions } from "../predicate";
import { CallToolResult, M } from "../protocol";
import { errorText, textOf } from "../status";
import {
	clampPage,
	filterRows,
	pageCount,
	pageSlice,
	SortSpec,
	sortRows,
	Store,
} from "../state";

interface ArgSourceCfg {
	static?: unknown;
	row?: string;
	selection?: string;
}

interface ActionCfg {
	label: string;
	kind: "tool" | "link";
	tool?: string;
	args?: Record<string, ArgSourceCfg>;
	/** Posts this as a user turn instead of calling tool. See Action.Prompt. */
	prompt?: string;
	hrefKey?: string;
	confirm?: string;
	variant?: string;
	/** Shows the action only on the rows it matches. See Action.VisibleWhen. */
	visibleWhen?: RowPredicateCfg;
}

interface ColumnCfg {
	key: string;
	label: string;
	type: string;
	sortable: boolean;
	align?: string;
	format?: string;
	badge?: Record<string, string>;
	link?: { hrefKey: string; textKey?: string; text?: string };
	actions?: ActionCfg[];
}

interface TableCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	pageSize: number;
	filterable: boolean;
	columns: ColumnCfg[];
	// Grow the list in place instead of paging it (Table.LoadMore).
	loadMore?: boolean;
	defaultSort?: SortSpec;
	selection?: { bulk: ActionCfg[] };
	empty?: { title?: string; body?: string };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

interface TableState {
	rows: Row[];
	sort: SortSpec | null;
	filter: string;
	page: number;
	// Starts at the configured PageSize; the pagination bar's chooser (when the
	// widget renders one) moves it.
	pageSize: number;
	// LoadMore mode only: how many rows the table currently shows. Grows by
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

export function mountTable(ctx: MountContext): void {
	const cfg = ctx.config as unknown as TableCfg;
	const { root, bridge } = ctx;

	const tbodyEl = root.querySelector<HTMLElement>("[data-gomu-rows]");
	if (!tbodyEl || !Array.isArray(cfg.columns)) return;
	const tbody: HTMLElement = tbodyEl;
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gomu-empty]");
	const emptyTitleEl = emptyEl?.querySelector("h3") ?? null;
	const emptyTitleDefault = emptyTitleEl?.textContent ?? "";
	const paginationEl = root.querySelector<HTMLElement>("[data-gomu-pagination]");
	const pageInfoEl = root.querySelector<HTMLElement>("[data-gomu-page-info]");
	const moreEl = root.querySelector<HTMLElement>("[data-gomu-more]");
	const moreBtnEl = root.querySelector<HTMLButtonElement>("[data-gomu-reveal]");
	const moreCountEl = root.querySelector<HTMLElement>("[data-gomu-more-count]");
	const bulkEl = root.querySelector<HTMLElement>("[data-gomu-bulk]");
	const bulkCountEl = root.querySelector<HTMLElement>("[data-gomu-bulk-count]");
	const bulkMenuEl = root.querySelector<HTMLButtonElement>("[data-gomu-bulk-menu]");
	const selectAllEl = root.querySelector<HTMLInputElement>("[data-gomu-select-all]");
	const pageSizeEl = root.querySelector<HTMLSelectElement>("[data-gomu-page-size]");
	// Visible only in the compact tier, where the header row (and its sort
	// buttons) is hidden. Both drive the same sort state, so either stays right
	// when the widget crosses the breakpoint.
	const sortSelectEl = root.querySelector<HTMLSelectElement>("[data-gomu-sort-select]");

	const wrapEl = root.querySelector<HTMLElement>(".gomu-table-wrap");
	const tableEl = root.querySelector<HTMLTableElement>(".gomu-table");

	// --- stacked / grid layout ---
	//
	// Whether a table fits is a question about its columns, not about the pane:
	// five columns of email addresses overflow a 600px pane while three short
	// ones are comfortable at 320px. No CSS breakpoint can know which it is
	// looking at, so the runtime measures and CSS renders the verdict
	// (.gomu-root[data-gomu-stacked] in table.css).
	//
	// The measurement is always taken in grid layout, because stacked rows have
	// no column widths to measure: the attribute comes off, the table's
	// min-content width is read, and it goes back on if that overflows. Both
	// steps run in one synchronous block, so no intermediate state is painted.
	// The verdict therefore depends only on content and available width — never
	// on the layout currently showing — which is what keeps it from oscillating
	// around the crossover point.
	function updateStacking(): void {
		if (!wrapEl || !tableEl) return;
		root.removeAttribute("data-gomu-stacked");
		if (tableEl.scrollWidth > wrapEl.clientWidth) {
			root.setAttribute("data-gomu-stacked", "");
		}
		lastWidth = wrapEl.clientWidth;
	}

	// The verdict can only change when the room does, and taking it costs a
	// synchronous reflow — so the width is remembered and anything that leaves
	// it alone is ignored. This is not an optimisation: toggling the layout
	// changes the widget's height, the host follows that with a frame resize,
	// and a re-measure on every resize would answer its own writes forever.
	let lastWidth = -1;
	function updateStackingIfResized(): void {
		const width = wrapEl?.clientWidth ?? -1;
		if (width === lastWidth) return;
		lastWidth = width;
		updateStacking();
	}

	const filterKeys = cfg.columns.map((c) => c.key).filter((k) => k !== "");
	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	const store = new Store<TableState>({
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

	function visible(s: TableState): { pageRows: Row[]; total: number } {
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

	function resolveArgs(action: ActionCfg, row: Row | null): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const [name, src] of Object.entries(action.args ?? {})) {
			if ("static" in src) {
				out[name] = src.static;
			} else if (src.row !== undefined) {
				out[name] = row?.[src.row];
			} else if (src.selection !== undefined) {
				const field = src.selection;
				out[name] = selectedRows().map((r) => r[field]);
			}
		}
		return out;
	}

	function applyResult(res: CallToolResult): void {
		const patch: Partial<TableState> = { status: "idle" };
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

	async function fire(action: ActionCfg, row: Row | null): Promise<void> {
		if (action.kind === "link") {
			const href = row?.[action.hrefKey ?? ""];
			if (typeof href === "string" && href !== "") void bridge.openLink(href);
			return;
		}
		if (!action.tool) return;
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Working…" });
		try {
			// A prompt action hands the request to the host's chat: the model makes
			// the call, so there is no result of ours to apply — only the turn being
			// accepted.
			if (action.prompt) {
				await bridge.sendMessage(action.prompt);
				store.set({ status: "idle", statusKind: undefined, statusMsg: undefined });
				return;
			}
			applyResult(await bridge.callTool(action.tool, resolveArgs(action, row)));
		} catch (e) {
			store.set({
				status: "idle",
				statusKind: "error",
				statusMsg: errorText(e, "The action failed."),
			});
		}
	}

	// --- action menus ---
	//
	// Row and bulk actions are both a "⋯" trigger over one shared popup (see
	// actionmenu.ts), which is also where a confirmed action's two-phase
	// question is asked — native confirm() is silently disabled in sandboxed
	// MCP Apps iframes.
	const menu = createActionMenu(root);

	menu.bind(root, "action-menu", (el, value) => {
		const id = el.closest("tr")?.getAttribute("data-gomu-row-id");
		const row = store.get().rows.find((r) => rowID(r) === id) ?? null;
		// The menu is filled per row, so a choice resolves against the actions
		// this row actually offers rather than against a position in the column's
		// full list.
		const items = visibleActions(cfg.columns[Number(value)]?.actions ?? [], row);
		if (items.length === 0) return null;
		return {
			items: items.map((i) => i.action),
			onSelect: (i) => {
				const chosen = items[i]?.action;
				if (chosen) void fire(chosen, row);
			},
		};
	});

	menu.bind(root, "bulk-menu", () => {
		const actions = cfg.selection?.bulk ?? [];
		if (actions.length === 0) return null;
		return {
			items: actions,
			onSelect: (i) => void fire(actions[i] as ActionCfg, null),
		};
	});

	// --- rendering ---

	// Cell attributes shared by every column type. The label is what the
	// compact tier prints in front of the value once the header row is gone
	// (CSS reads it as attr(data-gomu-label)); the role keeps the cell a
	// cell there, where `display: block` has stripped the implicit one.
	function cellAttrs(col: ColumnCfg, extraClass?: string): Record<string, string | null> {
		const alignCls = col.align ? `gomu-align-${col.align}` : "";
		const cls = [extraClass, alignCls].filter(Boolean).join(" ");
		return {
			role: "cell",
			class: cls || null,
			"data-gomu-label": col.label || null,
		};
	}

	function cellFor(col: ColumnCfg, colIdx: number, row: Row, busy: boolean): HTMLElement {
		switch (col.type) {
			case "badge": {
				const value = String(row[col.key] ?? "");
				const variant = col.badge?.[value];
				const cls = "gomu-badge" + (variant && variant !== "neutral" ? ` gomu-badge--${variant}` : "");
				return h("td", cellAttrs(col), value === "" ? "" : h("span", { class: cls }, value));
			}
			case "link": {
				const href = row[col.link?.hrefKey ?? col.key];
				if (typeof href !== "string" || href === "") return h("td", cellAttrs(col));
				const textKey = col.link?.textKey;
				const text =
					(textKey !== undefined ? String(row[textKey] ?? "") : "") ||
					col.link?.text ||
					href;
				return h("td", cellAttrs(col),
					h("button", { type: "button", class: "gomu-link", "data-gomu-link": href }, text),
				);
			}
			case "actions": {
				const td = h("td", cellAttrs(col, "gomu-td-actions"));
				// A row every action has excluded gets no trigger: an empty menu is
				// a promise the row cannot keep.
				if (anyVisible(col.actions ?? [], row)) {
					td.append(
						actionMenuTrigger({ "data-gomu-action-menu": String(colIdx), disabled: busy }),
					);
				}
				return td;
			}
			default:
				return h("td", cellAttrs(col), formatCell(row[col.key], col.type, col.format));
		}
	}

	function render(s: TableState): void {
		const { pageRows, total } = visible(s);
		const busy = s.status === "loading";
		const selected = new Set(s.selected);

		// An open menu belongs to a trigger that is about to be replaced, and
		// stands over rows that are about to change under it.
		menu.close();

		// rows
		clear(tbody);
		for (const row of pageRows) {
			const tr = h("tr", { role: "row", "data-gomu-row-id": rowID(row) });
			if (cfg.selection) {
				const cb = checkbox({
					"data-gomu-select-row": "",
					"aria-label": "Select row",
				});
				cb.input.checked = selected.has(rowID(row));
				tr.append(h("td", { role: "cell", class: "gomu-td-select" }, cb.wrap));
			}
			cfg.columns.forEach((col, i) => tr.append(cellFor(col, i, row, busy)));
			tbody.append(tr);
		}

		// sort indicators
		for (const th of root.querySelectorAll<HTMLElement>("th[data-gomu-sort]")) {
			const key = th.getAttribute("data-gomu-sort");
			th.setAttribute(
				"aria-sort",
				s.sort && s.sort.key === key ? (s.sort.desc ? "descending" : "ascending") : "none",
			);
		}
		if (sortSelectEl) {
			sortSelectEl.value = s.sort ? `${s.sort.key}|${s.sort.desc ? "desc" : "asc"}` : "";
			refreshDropdown(sortSelectEl);
		}

		// empty state
		if (emptyEl) {
			emptyEl.hidden = total > 0;
			if (emptyTitleEl) {
				emptyTitleEl.textContent =
					total === 0 && s.rows.length > 0 ? "No matching rows" : emptyTitleDefault;
			}
		}

		// pagination
		const pages = pageCount(total, s.pageSize);
		if (paginationEl) {
			// A single page normally means no bar — but with a page-size chooser
			// the bar is also the way back to a smaller page, so it stays.
			paginationEl.hidden = s.pageSize <= 0 || (pages <= 1 && !pageSizeEl);
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

		// load more
		if (moreEl) {
			moreEl.hidden = pageRows.length >= total;
			if (moreCountEl) moreCountEl.textContent = `${pageRows.length} of ${total}`;
			if (moreBtnEl) moreBtnEl.disabled = busy;
		}

		// selection
		if (selectAllEl) {
			const onPage = pageRows.filter((r) => selected.has(rowID(r))).length;
			selectAllEl.checked = pageRows.length > 0 && onPage === pageRows.length;
			// Part of the page selected reads as a dash rather than an empty
			// box — the toggle's next click selects the rest, not nothing.
			selectAllEl.indeterminate = onPage > 0 && onPage < pageRows.length;
		}
		if (bulkEl) {
			bulkEl.hidden = selected.size === 0;
			if (bulkCountEl) bulkCountEl.textContent = `${selected.size} selected`;
			if (bulkMenuEl) bulkMenuEl.disabled = busy;
		}

		// status
		if (statusEl) {
			const msg = s.statusMsg ?? "";
			statusEl.hidden = msg === "";
			statusEl.textContent = msg;
			statusEl.className = "gomu-status";
			if (busy) statusEl.className += " gomu-status--loading";
			else if (s.statusKind) statusEl.className += ` gomu-status--${s.statusKind}`;
		}

		// Last: the rows just written are what the columns have to fit.
		updateStacking();
	}

	// Rows settle the content side of the measurement; these two settle the
	// space side. The observer catches the widget being given a different
	// amount of room; the frame's own resize event catches the same thing in a
	// host that has throttled the iframe, where a rendering update — and with
	// it the observer's callback — never arrives.
	if (wrapEl && typeof ResizeObserver !== "undefined") {
		new ResizeObserver(updateStackingIfResized).observe(wrapEl);
	}
	window.addEventListener("resize", updateStackingIfResized);

	// --- events ---

	delegate(root, "click", "sort", (_el, key) => {
		const s = store.get().sort;
		store.set({
			sort: { key, desc: s?.key === key ? !s.desc : false },
			page: 0,
			revealed: cfg.pageSize,
		});
	});

	// The compact sort control carries direction in its value, so one change
	// sets both halves of the sort; the empty option clears it.
	if (sortSelectEl) {
		sortSelectEl.addEventListener("change", () => {
			const v = sortSelectEl.value;
			if (v === "") {
				store.set({ sort: null, page: 0, revealed: cfg.pageSize });
				return;
			}
			const sep = v.lastIndexOf("|");
			store.set({
				sort: { key: v.slice(0, sep), desc: v.slice(sep + 1) === "desc" },
				page: 0,
				revealed: cfg.pageSize,
			});
		});
	}

	let filterTimer: ReturnType<typeof setTimeout> | undefined;
	delegate(root, "input", "filter", (el) => {
		clearTimeout(filterTimer);
		filterTimer = setTimeout(() => {
			// A different set of rows is a different run: LoadMore starts it
			// from the first batch again.
			store.set({
				filter: (el as HTMLInputElement).value,
				page: 0,
				revealed: cfg.pageSize,
			});
		}, FILTER_DEBOUNCE_MS);
	});

	// Reveals the next batch. The wrap keeps its scroll position across the
	// re-render, so the new rows extend the run from where the bar stood.
	delegate(root, "click", "reveal", () => {
		const s = store.get();
		store.set({ revealed: s.revealed + (s.pageSize > 0 ? s.pageSize : s.rows.length) });
	});

	// Load on scroll: with a scroll-capped wrap (Table.MaxHeight), running out
	// of scroll reveals the next batch by itself; the bar's button stays as
	// the fallback and the path without a pointer.
	if (cfg.loadMore && wrapEl?.classList.contains("gomu-table-wrap--scroll")) {
		const wrap = wrapEl;
		wrap.addEventListener("scroll", () => {
			if (wrap.scrollTop + wrap.clientHeight < wrap.scrollHeight - 48) return;
			const s = store.get();
			if (s.status === "loading") return;
			const { pageRows, total } = visible(s);
			if (pageRows.length >= total) return;
			store.set({ revealed: s.revealed + s.pageSize });
		});
	}

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

	delegate(root, "change", "select-row", (el) => {
		const id = el.closest("tr")?.getAttribute("data-gomu-row-id");
		if (id === null || id === undefined) return;
		const selected = new Set(store.get().selected);
		if ((el as HTMLInputElement).checked) selected.add(id);
		else selected.delete(id);
		store.set({ selected: [...selected] });
	});

	delegate(root, "change", "select-all", (el) => {
		const s = store.get();
		const selected = new Set(s.selected);
		const { pageRows } = visible(s);
		if ((el as HTMLInputElement).checked) {
			for (const r of pageRows) selected.add(rowID(r));
		} else {
			for (const r of pageRows) selected.delete(rowID(r));
		}
		store.set({ selected: [...selected] });
	});

	delegate(root, "click", "link", (_el, href) => {
		if (href !== "") void bridge.openLink(href);
	});

	// --- host notifications ---

	bridge.on(M.toolInput, () => {
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
	});
	bridge.on(M.toolResult, (params) => {
		applyResult((params ?? {}) as CallToolResult);
	});
	bridge.on(M.toolCancelled, () => {
		store.set({ status: "idle", statusKind: undefined, statusMsg: undefined });
	});

	// Re-render when a host context lands: Intl formatting depends on the
	// host's locale/timeZone, which may arrive after the first paint.
	document.addEventListener(HOST_CONTEXT_EVENT, () => render(store.get()));

	store.subscribe(render);
	render(store.get());

	// Load-time hydration: once a host is connected, fetch fresh rows and
	// replace the baked snapshot, so a reloaded iframe shows current data
	// instead of the state frozen at render time. Silent on success (no
	// status toast) and on failure (the baked snapshot stays).
	async function hydrate(): Promise<void> {
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
		try {
			const res = await bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {});
			const patch: Partial<TableState> = {
				status: "idle",
				statusKind: undefined,
				statusMsg: undefined,
			};
			if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
				patch.rows = rowsFrom(res.structuredContent, cfg.rowsKey);
				patch.selected = [];
			}
			store.set(patch);
		} catch {
			store.set({ status: "idle", statusKind: undefined, statusMsg: undefined });
		}
	}
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (ok) void hydrate();
		});
	}
}
