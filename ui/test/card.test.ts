import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountCard } from "../src/widgets/card";
import { mountCardList } from "../src/widgets/cardlist";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

const TEMPLATE = {
	header: {
		titleKey: "name",
		descriptionKey: "email",
		badge: {
			key: "status",
			label: "Status",
			type: "badge",
			badge: { active: "success", banned: "danger" },
		},
	},
	content: {
		items: [
			{ key: "balance", label: "Balance", type: "number", format: "currency:EUR" },
			{ key: "website", label: "Website", type: "link", link: { hrefKey: "website" } },
		],
	},
	footer: {
		text: "Balances update hourly.",
		actions: [
			{ label: "Edit", kind: "tool", tool: "edit_user", args: { id: { row: "id" } } },
			{ label: "Delete", kind: "tool", tool: "delete_user", confirm: "Really delete?", args: { id: { row: "id" } } },
		],
	},
};

const ROWS = [
	{ id: 1, name: "Carol", email: "carol@x.io", status: "active", balance: 30, website: "https://c.io" },
	{ id: 2, name: "Alice", email: "alice@x.io", status: "banned", balance: 25, website: "" },
	{ id: 3, name: "Bob", email: "bob@x.io", status: "active", balance: 35, website: "" },
];

// A roster whose buttons depend on the record's state: exactly one direction
// of the switch applies to any one card (Action.VisibleWhen).
const SCHEDULE_TEMPLATE = {
	header: {
		titleKey: "name",
		action: {
			label: "Open",
			kind: "tool",
			tool: "schedule_open",
			args: { id: { row: "id" } },
			visibleWhen: { key: "state", equals: "running" },
		},
	},
	footer: {
		actions: [
			{
				label: "Activate",
				kind: "tool",
				tool: "schedule_activate",
				args: { id: { row: "id" } },
				visibleWhen: { key: "state", equals: "paused" },
			},
			{
				label: "Pause",
				kind: "tool",
				tool: "schedule_pause",
				args: { id: { row: "id" } },
				visibleWhen: { key: "state", equals: "running" },
			},
			{
				label: "Edit",
				kind: "tool",
				tool: "schedule_edit",
				args: { id: { row: "id" } },
				visibleWhen: { key: "state", in: ["running", "paused"] },
			},
		],
	},
};

const SCHEDULES = [
	{ id: 1, name: "Nightly", state: "running" },
	{ id: 2, name: "Weekly", state: "paused" },
	{ id: 3, name: "Retired", state: "archived" },
];

/** The action buttons of one card, as label → data-gomu-action index. */
function actionIndices(card: Element): Record<string, string> {
	const out: Record<string, string> = {};
	for (const btn of card.querySelectorAll("[data-gomu-action]")) {
		out[btn.textContent ?? ""] = btn.getAttribute("data-gomu-action") ?? "";
	}
	return out;
}

// --- CardList ---

function listShell({
	selection = false,
	bulk = 0,
	sort = true,
	pageSizes = [] as number[],
} = {}): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "cardlist");
	root.innerHTML = `
    <div class="gomu-toolbar">
      ${selection ? `<label><input type="checkbox" data-gomu-select-all=""></label>` : ""}
      <input type="search" data-gomu-filter="">
      ${
			sort
				? `<select data-gomu-sort-select="">
             <option value="">Sort…</option>
             <option value="balance|asc">Balance ↑</option>
             <option value="balance|desc">Balance ↓</option>
           </select>`
				: ""
		}
      ${
			bulk > 0
				? `<div data-gomu-bulk="" hidden><span data-gomu-bulk-count=""></span>` +
					Array.from({ length: bulk }, (_, i) => `<button type="button" data-gomu-bulk-action="${i}">Bulk${i}</button>`).join("") +
					`</div>`
				: ""
		}
    </div>
    <div data-gomu-status="" hidden></div>
    <div class="gomu-carousel">
      <button type="button" data-gomu-scroll="prev" hidden>‹</button>
      <div class="gomu-card-strip" data-gomu-cards=""></div>
      <button type="button" data-gomu-scroll="next" hidden>›</button>
    </div>
    <div data-gomu-empty="" hidden><h3>No records yet</h3></div>
    <div data-gomu-pagination="" hidden>
      ${
				pageSizes.length > 0
					? `<div class="gomu-page-size"><span>Per page</span><select data-gomu-page-size="">` +
						pageSizes.map((n) => `<option value="${n}">${n}</option>`).join("") +
						`</select></div>`
					: ""
			}
      <button type="button" data-gomu-page="prev">Prev</button>
      <span data-gomu-page-info=""></span>
      <button type="button" data-gomu-page="next">Next</button>
    </div>`;
	document.body.append(root);
	return root;
}

function listConfig(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "cardlist",
		rowsKey: "rows",
		rowId: "id",
		pageSize: 0,
		filterable: true,
		card: TEMPLATE,
		sort: [{ key: "balance", label: "Balance" }],
		...over,
	};
}

function titles(root: HTMLElement): string[] {
	return [...root.querySelectorAll(".gomu-card-title")].map((e) => e.textContent ?? "");
}

describe("cardlist behavior", () => {
	let host: FakeHost;
	let bridge: Bridge;

	beforeEach(async () => {
		host = new FakeHost();
		bridge = new Bridge({ timeoutMs: 500 });
		await bridge.initialize();
		host.requests.length = 0;
	});

	afterEach(() => {
		bridge.dispose();
		host.dispose();
		document.body.innerHTML = "";
	});

	it("grows the strip from a Load more tile instead of paging it", async () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ pageSize: 2, loadMore: true }),
			initialData: { rows: ROWS },
			bridge,
		});
		const more = (): HTMLButtonElement | null =>
			root.querySelector<HTMLButtonElement>("[data-gomu-reveal]");

		expect(titles(root)).toEqual(["Carol", "Alice"]);
		expect(more()?.textContent).toContain("2 of 3");
		// LoadMore replaces the pager rather than sitting alongside it.
		expect(root.querySelector<HTMLElement>("[data-gomu-pagination]")?.hidden).toBe(true);

		more()?.click();
		await flush();
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
		// Nothing left to reveal, so the tail goes away.
		expect(more()).toBeNull();
	});

	it("restarts the revealed run when the filter narrows the set", async () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ pageSize: 2, loadMore: true }),
			initialData: { rows: ROWS },
			bridge,
		});
		root.querySelector<HTMLButtonElement>("[data-gomu-reveal]")?.click();
		await flush();
		expect(titles(root)).toHaveLength(3);

		const filter = root.querySelector<HTMLInputElement>("[data-gomu-filter]") as HTMLInputElement;
		filter.value = "";
		filter.dispatchEvent(new Event("input", { bubbles: true }));
		await new Promise((r) => setTimeout(r, 200));
		expect(titles(root)).toEqual(["Carol", "Alice"]);
	});

	it("keeps the reader's place in the strip when a card is selected", async () => {
		const root = listShell({ selection: true });
		const strip = root.querySelector<HTMLElement>("[data-gomu-cards]") as HTMLElement;
		// jsdom does not scroll; the behavior only reads and writes scrollLeft.
		let scrollLeft = 0;
		Object.defineProperty(strip, "scrollWidth", { configurable: true, get: () => 1200 });
		Object.defineProperty(strip, "clientWidth", { configurable: true, get: () => 400 });
		Object.defineProperty(strip, "scrollLeft", {
			configurable: true,
			get: () => scrollLeft,
			set: (v: number) => {
				scrollLeft = v;
			},
		});
		mountCardList({
			root,
			config: listConfig({ selection: { bulk: [] } }),
			initialData: { rows: ROWS },
			bridge,
		});

		scrollLeft = 640;
		const box = root.querySelectorAll<HTMLInputElement>("[data-gomu-select-card]")[2] as HTMLInputElement;
		box.click();
		await flush();

		expect(scrollLeft).toBe(640);
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
	});

	it("renders cards from the data island", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(3);
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
		expect(root.querySelector(".gomu-card-description")?.textContent).toBe("carol@x.io");
	});

	it("renders the three sections and leaves out the ones with nothing to show", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ card: { header: { titleKey: "name" } } }),
			initialData: { rows: [ROWS[0]] },
			bridge,
		});
		const card = root.querySelector(".gomu-card-item")!;
		expect(card.querySelector(".gomu-card-item-header")).not.toBeNull();
		expect(card.querySelector(".gomu-card-content")).toBeNull();
		expect(card.querySelector(".gomu-card-item-footer")).toBeNull();
	});

	it("renders content prose and a footer note from the template", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({
				card: {
					header: { titleKey: "name" },
					content: { textKey: "bio" },
					footer: { text: "Updated hourly" },
				},
			}),
			initialData: { rows: [{ ...ROWS[0], bio: "Runs the billing team." }] },
			bridge,
		});
		expect(root.querySelector(".gomu-card-text")?.textContent).toBe("Runs the billing team.");
		expect(root.querySelector(".gomu-card-note")?.textContent).toBe("Updated hourly");
	});

	it("indexes the header action ahead of the footer actions", async () => {
		const root = listShell();
		host.onToolCall = () => ({ structuredContent: {} });
		mountCardList({
			root,
			config: listConfig({
				card: {
					header: { titleKey: "name", action: { label: "Open", kind: "tool", tool: "open_user", args: { id: { row: "id" } } } },
					footer: { actions: [{ label: "Edit", kind: "tool", tool: "edit_user", args: { id: { row: "id" } } }] },
				},
			}),
			initialData: { rows: [ROWS[0]] },
			bridge,
		});
		const card = root.querySelector(".gomu-card-item")!;
		expect(card.querySelector('.gomu-card-action [data-gomu-action="0"]')?.textContent).toBe("Open");

		card.querySelector<HTMLElement>('[data-gomu-action="1"]')!.click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "edit_user",
			arguments: { id: 1 },
		});
	});

	// --- per-record visibility (Action.VisibleWhen) ---

	it("draws each card only the actions that apply to it, keeping their indices", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ card: SCHEDULE_TEMPLATE }),
			initialData: { rows: SCHEDULES },
			bridge,
		});
		const cards = [...root.querySelectorAll(".gomu-card-item")];
		expect(cards).toHaveLength(3);
		// Indices count the header slot and then the whole footer list, so a
		// hidden button leaves a gap rather than renumbering the rest.
		expect(actionIndices(cards[0]!)).toEqual({ Open: "0", Pause: "2", Edit: "3" });
		expect(actionIndices(cards[1]!)).toEqual({ Activate: "1", Edit: "3" });
		expect(actionIndices(cards[2]!)).toEqual({});
	});

	it("fires the tool a button declares on a card whose earlier actions are hidden", async () => {
		const root = listShell();
		host.onToolCall = () => ({ structuredContent: {} });
		mountCardList({
			root,
			config: listConfig({ card: SCHEDULE_TEMPLATE }),
			initialData: { rows: SCHEDULES },
			bridge,
		});
		// The paused card: its header action and the first footer action are both
		// gone, so the leading button is "Activate" at index 1.
		const paused = root.querySelectorAll(".gomu-card-item")[1]!;
		const buttons = [...paused.querySelectorAll<HTMLElement>("[data-gomu-action]")];
		expect(buttons[0]!.textContent).toBe("Activate");
		buttons[0]!.click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "schedule_activate",
			arguments: { id: 2 },
		});

		// The strip is rebuilt after a call, so the second button is looked up
		// again — and is still its own action, not its hidden neighbour's.
		const rebuilt = root.querySelectorAll(".gomu-card-item")[1]!;
		rebuilt.querySelectorAll<HTMLElement>("[data-gomu-action]")[1]!.click();
		await flush();
		expect(host.received(M.toolsCall)[1]!.params).toMatchObject({
			name: "schedule_edit",
			arguments: { id: 2 },
		});
	});

	it("renders no action bar on a record every action excludes", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ card: SCHEDULE_TEMPLATE }),
			initialData: { rows: SCHEDULES },
			bridge,
		});
		const archived = root.querySelectorAll(".gomu-card-item")[2]!;
		expect(archived.querySelector(".gomu-card-item-actions")).toBeNull();
		// The footer carries nothing else here, so it is left out entirely.
		expect(archived.querySelector(".gomu-card-item-footer")).toBeNull();
		expect(archived.querySelector(".gomu-card-action")).toBeNull();
	});

	// jsdom has no layout, so the strip's geometry is stubbed; the behavior
	// only reads scrollLeft/scrollWidth/clientWidth.
	it("offers the carousel controls only when the strip overflows", () => {
		const root = listShell();
		const strip = root.querySelector<HTMLElement>("[data-gomu-cards]") as HTMLElement;
		let scrollLeft = 0;
		Object.defineProperty(strip, "scrollWidth", { configurable: true, get: () => 1200 });
		Object.defineProperty(strip, "clientWidth", { configurable: true, get: () => 400 });
		Object.defineProperty(strip, "scrollLeft", {
			configurable: true,
			get: () => scrollLeft,
			set: (v: number) => {
				scrollLeft = v;
			},
		});
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });

		const prev = root.querySelector<HTMLButtonElement>('[data-gomu-scroll="prev"]');
		const next = root.querySelector<HTMLButtonElement>('[data-gomu-scroll="next"]');
		expect(prev?.hidden).toBe(false);
		expect(prev?.disabled).toBe(true); // resting at the start
		expect(next?.disabled).toBe(false);
	});

	it("hides the carousel controls when everything fits", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		for (const btn of root.querySelectorAll<HTMLButtonElement>("[data-gomu-scroll]")) {
			expect(btn.hidden).toBe(true);
		}
	});

	it("shows the empty state when there are no rows", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: null, bridge });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(false);
	});

	it("shows skeleton cards, not the empty state, while data has not resolved", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig(),
			initialData: null,
			bridge,
			// A host answered, so records may still be pushed: the list waits.
			ready: Promise.resolve(true),
		});
		expect(root.querySelectorAll(".gomu-card-item--skeleton").length).toBeGreaterThan(0);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(true);
		expect(root.querySelector<HTMLElement>("[data-gomu-status]")?.className).toContain(
			"gomu-status--loading",
		);
	});

	it("replaces the skeleton with the empty state once a result carries no rows", async () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig(),
			initialData: null,
			bridge,
			ready: Promise.resolve(true),
		});
		host.pushToolResult({ structuredContent: { rows: [] } });
		await flush();
		expect(root.querySelectorAll(".gomu-card-item--skeleton")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(false);
	});

	it("renders badge and link fields", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: [ROWS[0]] }, bridge });
		const badge = root.querySelector(".gomu-card-action .gomu-badge")!;
		expect(badge.textContent).toBe("active");
		expect(badge.className).toContain("gomu-badge--success");
		const link = root.querySelector<HTMLElement>("[data-gomu-link]")!;
		expect(link.getAttribute("data-gomu-link")).toBe("https://c.io");
	});

	it("sorts via the sort select", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		const sel = root.querySelector<HTMLSelectElement>("[data-gomu-sort-select]")!;
		sel.value = "balance|asc";
		sel.dispatchEvent(new Event("change", { bubbles: true }));
		expect(titles(root)).toEqual(["Alice", "Carol", "Bob"]);
		sel.value = "balance|desc";
		sel.dispatchEvent(new Event("change", { bubbles: true }));
		expect(titles(root)).toEqual(["Bob", "Carol", "Alice"]);
	});

	it("resizes the page from the page-size chooser", () => {
		const root = listShell({ pageSizes: [2, 10] });
		mountCardList({
			root,
			config: listConfig({ pageSize: 2 }),
			initialData: { rows: ROWS },
			bridge,
		});
		const pagination = root.querySelector<HTMLElement>("[data-gomu-pagination]")!;
		const sizeEl = root.querySelector<HTMLSelectElement>("[data-gomu-page-size]")!;
		expect(sizeEl.value).toBe("2");
		expect(titles(root)).toHaveLength(2);

		sizeEl.value = "10";
		sizeEl.dispatchEvent(new Event("change", { bubbles: true }));
		expect(titles(root)).toHaveLength(3);
		expect(root.querySelector("[data-gomu-page-info]")?.textContent).toBe("1–3 of 3");
		// One page now, but the bar stays: it is the way back to a smaller page.
		expect(pagination.hidden).toBe(false);
	});

	it("applies a default sort from config", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ defaultSort: { key: "balance", desc: true } }),
			initialData: { rows: ROWS },
			bridge,
		});
		expect(titles(root)).toEqual(["Bob", "Carol", "Alice"]);
		expect(root.querySelector<HTMLSelectElement>("[data-gomu-sort-select]")?.value).toBe("balance|desc");
	});

	it("filters across title, description, and content items after the debounce", async () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		const input = root.querySelector<HTMLInputElement>("[data-gomu-filter]")!;
		input.value = "ali";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		await new Promise((r) => setTimeout(r, 250));
		expect(titles(root)).toEqual(["Alice"]);
	});

	it("paginates and updates the page info", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(2);
		const info = root.querySelector("[data-gomu-page-info]")!;
		expect(info.textContent).toBe("1–2 of 3");
		root.querySelector<HTMLElement>('[data-gomu-page="next"]')!.click();
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(1);
		expect(info.textContent).toBe("3–3 of 3");
	});

	it("fires per-card actions with FromRow args and applies returned rows", async () => {
		const root = listShell();
		host.onToolCall = (_name, args) => ({
			content: [{ type: "text", text: "Deleted." }],
			structuredContent: { rows: ROWS.filter((r) => r.id !== args.id) },
		});
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });

		// First card (Carol, id 1), action index 1 = Delete (has confirm).
		const delBtn = root
			.querySelector(".gomu-card-item")!
			.querySelector<HTMLElement>('[data-gomu-action="1"]')!;
		delBtn.click(); // opens the confirmation over the frame
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
		const ask = root.querySelector<HTMLElement>(".gomu-ask-panel")!;
		expect(ask.querySelector(".gomu-ask-message")!.textContent).toBe("Really delete?");
		ask.querySelector<HTMLButtonElement>(".gomu-ask-confirm")!.click(); // confirms → fires
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "delete_user", arguments: { id: 1 } });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(2);
		expect(root.querySelector<HTMLElement>("[data-gomu-status]")?.textContent).toBe("Deleted.");
	});

	it("selects cards, shows bulk actions, and resolves FromSelection args", async () => {
		const root = listShell({ selection: true, bulk: 1 });
		const cfg = listConfig({
			selection: { bulk: [{ label: "Archive", kind: "tool", tool: "archive_users", args: { ids: { selection: "id" } } }] },
		});
		host.onToolCall = () => ({ structuredContent: { rows: [] } });
		mountCardList({ root, config: cfg, initialData: { rows: ROWS }, bridge });

		const bulkBar = root.querySelector<HTMLElement>("[data-gomu-bulk]")!;
		expect(bulkBar.hidden).toBe(true);

		const selectAll = root.querySelector<HTMLInputElement>("[data-gomu-select-all]")!;
		selectAll.checked = true;
		selectAll.dispatchEvent(new Event("change", { bubbles: true }));
		expect(bulkBar.hidden).toBe(false);
		expect(root.querySelector("[data-gomu-bulk-count]")?.textContent).toBe("3 selected");

		root.querySelector<HTMLElement>('[data-gomu-bulk-action="0"]')!.click();
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "archive_users", arguments: { ids: [1, 2, 3] } });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(0);
		expect(bulkBar.hidden).toBe(true);
	});

	it("keeps each card's answers to itself and sends them with that card's action", async () => {
		const root = listShell({ selection: true });
		const asking = {
			...TEMPLATE,
			content: {
				items: [
					{
						key: "",
						label: "Note",
						type: "text",
						input: { name: "note", type: "text" },
					},
				],
			},
			footer: {
				actions: [{ label: "Flag", kind: "tool", tool: "flag_user", args: { id: { row: "id" } } }],
			},
		};
		host.onToolCall = () => ({ content: [{ type: "text", text: "Flagged." }] });
		mountCardList({
			root,
			config: listConfig({ card: asking, selection: { bulk: [] } }),
			initialData: { rows: ROWS },
			bridge,
		});

		const notes = [...root.querySelectorAll<HTMLInputElement>('[data-gomu-input="note"]')];
		expect(notes).toHaveLength(3);
		notes[1]!.value = "check this one";
		notes[1]!.dispatchEvent(new Event("input", { bubbles: true }));

		// A re-render rebuilds the whole strip; the answer belongs to its record.
		const selectCard = root.querySelectorAll<HTMLInputElement>("[data-gomu-select-card]")[0]!;
		selectCard.checked = true;
		selectCard.dispatchEvent(new Event("change", { bubbles: true }));
		const after = [...root.querySelectorAll<HTMLInputElement>('[data-gomu-input="note"]')];
		expect(after.map((el) => el.value)).toEqual(["", "check this one", ""]);

		root.querySelectorAll<HTMLElement>('[data-gomu-action="0"]')[1]!.click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "flag_user",
			arguments: { id: 2, note: "check this one" },
		});
	});

	it("updates from tool-result notifications", async () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		host.pushToolResult({ structuredContent: { rows: [{ id: 9, name: "Zoe", email: "z@x.io", status: "active", balance: 1, website: "" }] } });
		await flush();
		expect(titles(root)).toEqual(["Zoe"]);
	});

	it("hydrates from loadTool on mount, replacing the baked snapshot", async () => {
		const root = listShell();
		host.onToolCall = (name) =>
			name === "list_users"
				? { structuredContent: { rows: [{ id: 9, name: "Zed", email: "z@x.io", status: "active", balance: 40, website: "" }] } }
				: { structuredContent: {} };
		mountCardList({
			root,
			config: listConfig({ loadTool: "list_users", loadArgs: { scope: "all" } }),
			initialData: { rows: ROWS },
			bridge,
			ready: Promise.resolve(true),
		});
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "list_users", arguments: { scope: "all" } });
		expect(titles(root)).toEqual(["Zed"]);
	});
});

// --- Card (single) ---

function cardShell(): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "card");
	root.innerHTML = `
    <div data-gomu-status="" hidden></div>
    <div class="gomu-card-host" data-gomu-card=""></div>
    <div data-gomu-empty="" hidden><h3>Nothing</h3></div>`;
	document.body.append(root);
	return root;
}

function cardConfig(over: Record<string, unknown> = {}): Record<string, unknown> {
	return { widget: "card", rowsKey: "rows", rowId: "id", card: TEMPLATE, ...over };
}

describe("card behavior", () => {
	let host: FakeHost;
	let bridge: Bridge;

	beforeEach(async () => {
		host = new FakeHost();
		bridge = new Bridge({ timeoutMs: 500 });
		await bridge.initialize();
		host.requests.length = 0;
	});

	afterEach(() => {
		bridge.dispose();
		host.dispose();
		document.body.innerHTML = "";
	});

	it("renders the first row as a card", () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(1);
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe("Carol");
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(true);
	});

	it("shows the empty state with no record", () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: null, bridge });
		expect(root.querySelectorAll(".gomu-card-item")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(false);
	});

	it("shows a skeleton, not the empty state, while the record has not resolved", () => {
		const root = cardShell();
		mountCard({
			root,
			config: cardConfig(),
			initialData: null,
			bridge,
			// A host answered, so a record may still be pushed: the card waits.
			ready: Promise.resolve(true),
		});
		expect(root.querySelectorAll(".gomu-card-item--skeleton")).toHaveLength(1);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(true);
		expect(root.querySelector<HTMLElement>("[data-gomu-status]")?.className).toContain(
			"gomu-status--loading",
		);
	});

	it("replaces the skeleton with the empty state once a result carries no record", async () => {
		const root = cardShell();
		mountCard({
			root,
			config: cardConfig(),
			initialData: null,
			bridge,
			ready: Promise.resolve(true),
		});
		host.pushToolResult({ structuredContent: { rows: [] } });
		await flush();
		expect(root.querySelectorAll(".gomu-card-item--skeleton")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(false);
	});

	it("posts a card action's prompt as a chat message instead of calling the tool", async () => {
		const root = cardShell();
		const withPrompt = {
			...TEMPLATE,
			footer: {
				...TEMPLATE.footer,
				actions: [
					{
						label: "Edit",
						kind: "tool",
						tool: "edit_user",
						prompt: "Open the edit form for this user",
					},
				],
			},
		};
		mountCard({
			root,
			config: cardConfig({ card: withPrompt }),
			initialData: { rows: ROWS },
			bridge,
		});

		root.querySelector<HTMLElement>('[data-gomu-action="0"]')!.click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(host.received(M.message)[0]!.params).toMatchObject({
			role: "user",
			content: [{ type: "text", text: "Open the edit form for this user" }],
		});
		// Nothing came back to apply, so the record and status are left alone.
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe(ROWS[0]!.name);
		expect(root.querySelector<HTMLElement>("[data-gomu-status]")?.hidden).toBe(true);
	});

	it("fires a card action with FromRow args and applies the result", async () => {
		const root = cardShell();
		host.onToolCall = () => ({
			content: [{ type: "text", text: "Saved." }],
			structuredContent: { rows: [{ ...ROWS[0], name: "Caroline" }] },
		});
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });

		root.querySelector<HTMLElement>('[data-gomu-action="0"]')!.click(); // Edit (no confirm)
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "edit_user", arguments: { id: 1 } });
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe("Caroline");
		expect(root.querySelector<HTMLElement>("[data-gomu-status]")?.textContent).toBe("Saved.");
	});

	it("hides the actions the record does not match and still fires the rest", async () => {
		const root = cardShell();
		host.onToolCall = () => ({ structuredContent: {} });
		// A paused schedule: the header's "Open" and the footer's "Pause" do not
		// apply to it.
		mountCard({
			root,
			config: cardConfig({ card: SCHEDULE_TEMPLATE }),
			initialData: { rows: [SCHEDULES[1]] },
			bridge,
		});
		const card = root.querySelector(".gomu-card-item")!;
		expect(actionIndices(card)).toEqual({ Activate: "1", Edit: "3" });

		card.querySelector<HTMLElement>('[data-gomu-action="1"]')!.click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "schedule_activate",
			arguments: { id: 2 },
		});
	});

	it("sends what the card's own controls collected with its action", async () => {
		const root = cardShell();
		const asking = {
			...TEMPLATE,
			content: {
				items: [
					...TEMPLATE.content.items,
					{
						key: "balance",
						label: "Top up by",
						type: "text",
						input: { name: "amount", type: "number", required: true, message: "How much?" },
					},
				],
			},
			footer: {
				actions: [{ label: "Top up", kind: "tool", tool: "top_up", args: { id: { row: "id" } } }],
			},
		};
		mountCard({ root, config: cardConfig({ card: asking }), initialData: { rows: ROWS }, bridge });

		// The item's key prefills the control from the record.
		const amount = root.querySelector<HTMLInputElement>('[data-gomu-input="amount"]')!;
		expect(amount.value).toBe("30");
		amount.value = "";
		amount.dispatchEvent(new Event("input", { bubbles: true }));

		// Required and empty: the action does not fire.
		root.querySelector<HTMLElement>('[data-gomu-action="0"]')!.click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(root.querySelector("[data-gomu-input-error]")?.textContent).toBe("How much?");

		root.querySelector<HTMLInputElement>('[data-gomu-input="amount"]')!.value = "50";
		root
			.querySelector<HTMLInputElement>('[data-gomu-input="amount"]')!
			.dispatchEvent(new Event("input", { bubbles: true }));
		root.querySelector<HTMLElement>('[data-gomu-action="0"]')!.click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "top_up",
			arguments: { id: 1, amount: 50 },
		});
	});

	it("updates from tool-result notifications", async () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });
		host.pushToolResult({ structuredContent: { rows: [{ id: 5, name: "Dave", email: "d@x.io", status: "active", balance: 9, website: "" }] } });
		await flush();
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe("Dave");
	});

	it("hydrates from loadTool on mount", async () => {
		const root = cardShell();
		host.onToolCall = () => ({ structuredContent: { rows: [{ id: 7, name: "Fetched", email: "f@x.io", status: "active", balance: 1, website: "" }] } });
		mountCard({
			root,
			config: cardConfig({ loadTool: "get_user", loadArgs: { id: 7 } }),
			initialData: { rows: ROWS },
			bridge,
			ready: Promise.resolve(true),
		});
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe("Carol");
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(1);
		expect(root.querySelector(".gomu-card-title")?.textContent).toBe("Fetched");
	});
});
