import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountTable } from "../src/widgets/table";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

function shell({
  selection = false,
  bulk = false,
  pageSizes = [] as number[],
  loadMore = false,
} = {}): HTMLElement {
  document.body.innerHTML = "";
  const root = document.createElement("div");
  root.className = "gomu-root";
  root.setAttribute("data-gomu-widget", "table");
  root.innerHTML = `
    <div class="gomu-toolbar">
      <input type="search" data-gomu-filter="">
      ${
        bulk
          ? `<div data-gomu-bulk="" hidden><span data-gomu-bulk-count=""></span>` +
            `<button type="button" data-gomu-bulk-menu="" aria-haspopup="menu" aria-expanded="false">Actions</button>` +
            `</div>`
          : ""
      }
    </div>
    <div data-gomu-status="" hidden></div>
    <div class="gomu-table-sort"><select data-gomu-sort-select="">
      <option value="">Sort…</option>
      <option value="name|asc">Name ↑</option>
      <option value="name|desc">Name ↓</option>
    </select></div>
    <div class="gomu-table-wrap"><table class="gomu-table" role="table"><thead><tr>
      ${selection ? `<th><input type="checkbox" data-gomu-select-all=""></th>` : ""}
      <th aria-sort="none" data-gomu-sort="name"><button type="button">Name</button></th>
      <th aria-sort="none" data-gomu-sort="age"><button type="button">Age</button></th>
    </tr></thead><tbody data-gomu-rows=""></tbody></table></div>
    <div data-gomu-empty="" hidden><h3>No records yet</h3></div>
    ${
      loadMore
        ? `<div data-gomu-more="" hidden>` +
          `<button type="button" data-gomu-reveal="">Load more</button>` +
          `<span data-gomu-more-count=""></span></div>`
        : `<div data-gomu-pagination="" hidden>` +
          (pageSizes.length > 0
            ? `<div class="gomu-page-size"><span>Per page</span><select data-gomu-page-size="">` +
              pageSizes.map((n) => `<option value="${n}">${n}</option>`).join("") +
              `</select></div>`
            : "") +
          `<button type="button" data-gomu-page="prev">Prev</button>` +
          `<span data-gomu-page-info=""></span>` +
          `<button type="button" data-gomu-page="next">Next</button></div>`
    }`;
  document.body.append(root);
  return root;
}

/** Opens an action menu and returns its items, in order. */
function openMenu(root: HTMLElement, selector: string): HTMLElement[] {
  root.querySelector<HTMLElement>(selector)!.click();
  return [...root.querySelectorAll<HTMLElement>(".gomu-action-panel [data-gomu-action-index]")];
}

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    widget: "table",
    rowsKey: "rows",
    rowId: "id",
    pageSize: 0,
    filterable: true,
    columns: [
      { key: "name", label: "Name", type: "text", sortable: true },
      { key: "age", label: "Age", type: "number", sortable: true },
    ],
    ...over,
  };
}

const ROWS = [
  { id: 1, name: "Carol", age: 30 },
  { id: 2, name: "Alice", age: 25 },
  { id: 3, name: "Bob", age: 35 },
];

function cellTexts(root: HTMLElement, col: number): string[] {
  return [...root.querySelectorAll("tbody tr")].map(
    (tr) => tr.querySelectorAll("td")[col]?.textContent ?? "",
  );
}

describe("table behavior", () => {
  let host: FakeHost;
  let bridge: Bridge;

  beforeEach(async () => {
    host = new FakeHost();
    bridge = new Bridge({ timeoutMs: 500 });
    // Mirrors boot(): initialize runs before user clicks.
    await bridge.initialize();
    host.requests.length = 0;
  });

  afterEach(() => {
    bridge.dispose();
    host.dispose();
    document.body.innerHTML = "";
  });

  // jsdom has no layout, so the two widths the decision rests on are stubbed.
  it("stacks rows only when the columns do not fit the wrap", async () => {
    const root = shell();
    const wrap = root.querySelector<HTMLElement>(".gomu-table-wrap") as HTMLElement;
    const table = root.querySelector<HTMLElement>(".gomu-table") as HTMLElement;
    let needed = 300;
    Object.defineProperty(wrap, "clientWidth", { configurable: true, get: () => 500 });
    Object.defineProperty(table, "scrollWidth", { configurable: true, get: () => needed });

    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    expect(root.hasAttribute("data-gomu-stacked")).toBe(false);

    // Same pane, wider columns: the verdict follows the content.
    needed = 800;
    root.querySelector<HTMLInputElement>("[data-gomu-filter]")!.dispatchEvent(
      new Event("input", { bubbles: true }),
    );
    await new Promise((r) => setTimeout(r, 200));
    expect(root.hasAttribute("data-gomu-stacked")).toBe(true);

    // …and back, so crossing the threshold is not one-way.
    needed = 300;
    root.querySelector<HTMLInputElement>("[data-gomu-filter]")!.dispatchEvent(
      new Event("input", { bubbles: true }),
    );
    await new Promise((r) => setTimeout(r, 200));
    expect(root.hasAttribute("data-gomu-stacked")).toBe(false);
  });

  it("labels every cell so stacked rows can reprint the header", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const cells = [...root.querySelectorAll("tbody tr:first-child td")];
    expect(cells.map((c) => c.getAttribute("data-gomu-label"))).toEqual(["Name", "Age"]);
    expect(cells.every((c) => c.getAttribute("role") === "cell")).toBe(true);
  });

  it("sorts from the compact control and keeps the header in step", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const sel = root.querySelector<HTMLSelectElement>("[data-gomu-sort-select]") as HTMLSelectElement;
    sel.value = "name|desc";
    sel.dispatchEvent(new Event("change", { bubbles: true }));
    await flush();

    const names = [...root.querySelectorAll('td[data-gomu-label="Name"]')].map((c) => c.textContent);
    expect(names).toEqual([...names].sort().reverse());
    expect(
      root.querySelector('th[data-gomu-sort="name"]')?.getAttribute("aria-sort"),
    ).toBe("descending");
  });

  it("renders initial rows from the data island", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(true);
  });

  it("shows the empty state when there are no rows", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: null, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(0);
    expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(false);
  });

  it("sorts on header click, toggling direction", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const sortBtn = root.querySelector<HTMLElement>('[data-gomu-sort="name"] button')!;
    sortBtn.click();
    expect(cellTexts(root, 0)).toEqual(["Alice", "Bob", "Carol"]);
    expect(root.querySelector('[data-gomu-sort="name"]')?.getAttribute("aria-sort")).toBe("ascending");
    sortBtn.click();
    expect(cellTexts(root, 0)).toEqual(["Carol", "Bob", "Alice"]);
    expect(root.querySelector('[data-gomu-sort="name"]')?.getAttribute("aria-sort")).toBe("descending");
  });

  it("applies a default sort from config", () => {
    const root = shell();
    mountTable({
      root,
      config: config({ defaultSort: { key: "age", desc: true } }),
      initialData: { rows: ROWS },
      bridge,
    });
    expect(cellTexts(root, 0)).toEqual(["Bob", "Carol", "Alice"]);
  });

  it("filters rows after the debounce", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const input = root.querySelector<HTMLInputElement>("[data-gomu-filter]")!;
    input.value = "ali";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((r) => setTimeout(r, 250));
    expect(cellTexts(root, 0)).toEqual(["Alice"]);
    expect(root.querySelector<HTMLElement>("[data-gomu-empty]")?.hidden).toBe(true);
  });

  it("shows 'no matching rows' when the filter eliminates everything", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const input = root.querySelector<HTMLInputElement>("[data-gomu-filter]")!;
    input.value = "zzz";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((r) => setTimeout(r, 250));
    const empty = root.querySelector<HTMLElement>("[data-gomu-empty]")!;
    expect(empty.hidden).toBe(false);
    expect(empty.querySelector("h3")?.textContent).toBe("No matching rows");
  });

  it("paginates and updates the page info", () => {
    const root = shell();
    mountTable({ root, config: config({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    const info = root.querySelector("[data-gomu-page-info]")!;
    expect(info.textContent).toBe("1–2 of 3");
    root.querySelector<HTMLElement>('[data-gomu-page="next"]')!.click();
    expect(root.querySelectorAll("tbody tr")).toHaveLength(1);
    expect(info.textContent).toBe("3–3 of 3");
  });

  it("resizes the page from the page-size chooser", () => {
    const root = shell({ pageSizes: [2, 10] });
    mountTable({ root, config: config({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
    const pagination = root.querySelector<HTMLElement>("[data-gomu-pagination]")!;
    const sizeEl = root.querySelector<HTMLSelectElement>("[data-gomu-page-size]")!;
    expect(sizeEl.value).toBe("2");
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);

    sizeEl.value = "10";
    sizeEl.dispatchEvent(new Event("change", { bubbles: true }));
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(root.querySelector("[data-gomu-page-info]")?.textContent).toBe("1–3 of 3");
    // One page now, but the bar stays: it is the way back to a smaller page.
    expect(pagination.hidden).toBe(false);
  });

  it("grows the list under load more and retires the bar at the end", () => {
    const root = shell({ loadMore: true });
    mountTable({
      root,
      config: config({ pageSize: 2, loadMore: true }),
      initialData: { rows: ROWS },
      bridge,
    });
    const more = root.querySelector<HTMLElement>("[data-gomu-more]")!;
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    expect(more.hidden).toBe(false);
    expect(root.querySelector("[data-gomu-more-count]")?.textContent).toBe("2 of 3");

    root.querySelector<HTMLElement>("[data-gomu-reveal]")!.click();
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(more.hidden).toBe(true);
  });

  it("load more starts the run over when the filter changes the set", async () => {
    const rows = [...ROWS, { id: 4, name: "Cathy", age: 41 }];
    const root = shell({ loadMore: true });
    mountTable({
      root,
      config: config({ pageSize: 1, loadMore: true }),
      initialData: { rows },
      bridge,
    });
    root.querySelector<HTMLElement>("[data-gomu-reveal]")!.click();
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);

    const input = root.querySelector<HTMLInputElement>("[data-gomu-filter]")!;
    input.value = "ca";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((r) => setTimeout(r, 250));
    // Carol and Cathy match, but the run is back to its first batch of one.
    expect(root.querySelectorAll("tbody tr")).toHaveLength(1);
    expect(root.querySelector("[data-gomu-more-count]")?.textContent).toBe("1 of 2");
  });

  it("reveals the next batch when the scroll-capped wrap runs out of scroll", () => {
    const root = shell({ loadMore: true });
    const wrap = root.querySelector<HTMLElement>(".gomu-table-wrap")!;
    wrap.classList.add("gomu-table-wrap--scroll");
    // jsdom has no layout: pin the geometry at "resting on the bottom".
    Object.defineProperty(wrap, "scrollTop", { configurable: true, get: () => 100 });
    Object.defineProperty(wrap, "clientHeight", { configurable: true, get: () => 200 });
    Object.defineProperty(wrap, "scrollHeight", { configurable: true, get: () => 300 });
    mountTable({
      root,
      config: config({ pageSize: 2, loadMore: true }),
      initialData: { rows: ROWS },
      bridge,
    });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);

    wrap.dispatchEvent(new Event("scroll"));
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    // Nothing left: another scroll must not grow past the set.
    wrap.dispatchEvent(new Event("scroll"));
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
  });

  it("posts a row action's prompt as a chat message instead of calling the tool", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            {
              label: "Edit",
              kind: "tool",
              tool: "edit_user",
              prompt: "Open the edit form for this user",
            },
          ],
        },
      ],
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    openMenu(root, 'tbody [data-gomu-action-menu="1"]')[0]!.click();
    await flush();

    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(host.received(M.message)[0]!.params).toMatchObject({
      role: "user",
      content: [{ type: "text", text: "Open the edit form for this user" }],
    });
    // Nothing came back to apply, so the rows are untouched and the working
    // status is cleared.
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(root.querySelector<HTMLElement>("[data-gomu-status]")!.hidden).toBe(true);
  });

  it("fires row actions with FromRow args and applies returned rows", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            {
              label: "Delete",
              kind: "tool",
              tool: "delete_user",
              args: { id: { row: "id" }, hard: { static: true } },
            },
          ],
        },
      ],
    });
    host.onToolCall = (name, args) => ({
      content: [{ type: "text", text: "Deleted." }],
      structuredContent: { rows: ROWS.filter((r) => r.id !== args.id) },
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const items = openMenu(root, 'tbody [data-gomu-action-menu="1"]');
    expect(items.map((el) => el.textContent)).toEqual(["Delete"]);
    items[0]!.click();
    await flush();

    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({
      name: "delete_user",
      arguments: { id: 1, hard: true },
    });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    const status = root.querySelector<HTMLElement>("[data-gomu-status]")!;
    expect(status.hidden).toBe(false);
    expect(status.textContent).toBe("Deleted.");
    expect(status.className).toContain("gomu-status--success");
  });

  it("never shows a serialized structuredContent payload as status", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [{ label: "Delete", kind: "tool", tool: "delete_user", args: { id: { row: "id" } } }],
        },
      ],
    });
    // What an SDK sends when the handler returns only structured output: the
    // JSON is mirrored into a text block.
    const rows = ROWS.filter((r) => r.id !== 1);
    host.onToolCall = () => ({
      content: [{ type: "text", text: JSON.stringify({ rows }) }],
      structuredContent: { rows },
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    openMenu(root, 'tbody [data-gomu-action-menu="1"]')[0]!.click();
    await flush();

    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    const status = root.querySelector<HTMLElement>("[data-gomu-status]")!;
    expect(status.textContent).toBe("Done");
  });

  it("keeps a pushed tool result's JSON text out of the status bar", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    host.pushToolResult({
      content: [{ type: "text", text: JSON.stringify({ rows: [] }) }],
      structuredContent: { rows: [] },
    });
    await flush();
    expect(root.querySelector<HTMLElement>("[data-gomu-status]")!.textContent).toBe("Done");
  });

  it("asks for confirmation over the frame before a confirmed action fires", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            { label: "Delete", kind: "tool", tool: "delete_user", confirm: "Really delete?" },
          ],
        },
      ],
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const item = openMenu(root, 'tbody [data-gomu-action-menu="1"]')[0]!;
    item.click();
    await flush();
    // No call yet: the menu has closed and a confirmation stands over the frame.
    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(root.querySelector<HTMLElement>(".gomu-action-panel")!.parentElement!.hidden).toBe(true);
    const ask = root.querySelector<HTMLElement>(".gomu-ask-panel")!;
    expect(ask.querySelector(".gomu-ask-message")!.textContent).toBe("Really delete?");

    ask.querySelector<HTMLButtonElement>(".gomu-ask-confirm")!.click();
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(1);
    expect(root.querySelector(".gomu-ask-panel")).toBeNull();
  });

  // --- per-row visibility (Action.VisibleWhen) ---

  const SCHEDULES = [
    { id: 1, name: "Nightly", state: "running" },
    { id: 2, name: "Weekly", state: "paused" },
    { id: 3, name: "Retired", state: "archived" },
  ];

  function schedulesConfig(): Record<string, unknown> {
    return config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
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
      ],
    });
  }

  it("shows a row only the actions that apply to it", () => {
    const root = shell();
    mountTable({ root, config: schedulesConfig(), initialData: { rows: SCHEDULES }, bridge });

    expect(openMenu(root, 'tbody tr:nth-child(1) [data-gomu-action-menu]').map((el) => el.textContent))
      .toEqual(["Pause", "Edit"]);
    root.querySelector<HTMLElement>('tbody tr:nth-child(1) [data-gomu-action-menu]')!.click();
    expect(openMenu(root, 'tbody tr:nth-child(2) [data-gomu-action-menu]').map((el) => el.textContent))
      .toEqual(["Activate", "Edit"]);
  });

  // The regression this guards: actions are addressed by position, so a row
  // whose first action is hidden must still fire what its buttons say.
  it("fires the action the reader chose when earlier ones are hidden", async () => {
    const root = shell();
    mountTable({ root, config: schedulesConfig(), initialData: { rows: SCHEDULES }, bridge });

    // Row 1 is running: "Activate" is gone, so the first item is "Pause".
    const items = openMenu(root, 'tbody tr:nth-child(1) [data-gomu-action-menu]');
    expect(items[0]!.textContent).toBe("Pause");
    items[0]!.click();
    await flush();
    expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
      name: "schedule_pause",
      arguments: { id: 1 },
    });

    // …and the item after it is still its own action, not its neighbour's.
    const rest = openMenu(root, 'tbody tr:nth-child(1) [data-gomu-action-menu]');
    rest[1]!.click();
    await flush();
    expect(host.received(M.toolsCall)[1]!.params).toMatchObject({
      name: "schedule_edit",
      arguments: { id: 1 },
    });
  });

  it("renders no trigger on a row every action excludes", () => {
    const root = shell();
    mountTable({ root, config: schedulesConfig(), initialData: { rows: SCHEDULES }, bridge });

    const triggers = [...root.querySelectorAll("tbody tr")].map(
      (tr) => tr.querySelector("[data-gomu-action-menu]") !== null,
    );
    expect(triggers).toEqual([true, true, false]);
    // The cell itself stays, so the column keeps its shape.
    expect(root.querySelectorAll("tbody tr:nth-child(3) td")).toHaveLength(2);
  });

  it("selects rows, shows bulk actions, and resolves FromSelection args", async () => {
    const root = shell({ selection: true, bulk: true });
    const cfg = config({
      selection: {
        bulk: [
          {
            label: "Archive",
            kind: "tool",
            tool: "archive_users",
            args: { ids: { selection: "id" } },
          },
        ],
      },
    });
    host.onToolCall = () => ({ structuredContent: { rows: [] } });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const bulkBar = root.querySelector<HTMLElement>("[data-gomu-bulk]")!;
    expect(bulkBar.hidden).toBe(true);

    const selectAll = root.querySelector<HTMLInputElement>("[data-gomu-select-all]")!;
    selectAll.checked = true;
    selectAll.dispatchEvent(new Event("change", { bubbles: true }));
    expect(bulkBar.hidden).toBe(false);
    expect(root.querySelector("[data-gomu-bulk-count]")?.textContent).toBe("3 selected");

    openMenu(root, "[data-gomu-bulk-menu]")[0]!.click();
    await flush();
    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({
      name: "archive_users",
      arguments: { ids: [1, 2, 3] }, // raw field values, not stringified selection keys
    });
    // rows replaced by result; selection cleared
    expect(root.querySelectorAll("tbody tr")).toHaveLength(0);
    expect(bulkBar.hidden).toBe(true);
  });

  it("renders badge and link cells", () => {
    const root = shell();
    const cfg = config({
      columns: [
        {
          key: "status",
          label: "Status",
          type: "badge",
          sortable: false,
          badge: { active: "success", banned: "danger" },
        },
        {
          key: "url",
          label: "Site",
          type: "link",
          sortable: false,
          link: { hrefKey: "url", text: "Visit" },
        },
      ],
    });
    mountTable({
      root,
      config: cfg,
      initialData: { rows: [{ id: 1, status: "active", url: "https://example.com" }] },
      bridge,
    });
    const badge = root.querySelector("tbody .gomu-badge")!;
    expect(badge.textContent).toBe("active");
    expect(badge.className).toContain("gomu-badge--success");
    const link = root.querySelector<HTMLElement>("tbody [data-gomu-link]")!;
    expect(link.textContent).toBe("Visit");

    link.click();
    // openLink request goes to the host
    return flush().then(() => {
      expect(host.received(M.openLink)).toHaveLength(1);
      expect(host.received(M.openLink)[0]!.params).toEqual({ url: "https://example.com" });
    });
  });

  it("updates rows from tool-result notifications and shows loading on tool-input", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });

    host.notify(M.toolInput, { arguments: {} });
    await flush();
    const status = root.querySelector<HTMLElement>("[data-gomu-status]")!;
    expect(status.className).toContain("gomu-status--loading");

    host.pushToolResult({ structuredContent: { rows: [{ id: 9, name: "Zoe", age: 1 }] } });
    await flush();
    expect(cellTexts(root, 0)).toEqual(["Zoe"]);
    expect(status.className).not.toContain("gomu-status--loading");
  });

  it("hydrates from loadTool on mount, replacing the baked snapshot", async () => {
    const root = shell();
    host.onToolCall = (name) =>
      name === "list_users"
        ? { structuredContent: { rows: [{ id: 9, name: "Zed", age: 40 }] } }
        : { structuredContent: {} };
    mountTable({
      root,
      config: config({ loadTool: "list_users", loadArgs: { scope: "all" } }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(true),
    });

    // Baked snapshot paints first...
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    await flush();

    // ...then the load tool fires (with its static args) and replaces it.
    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({ name: "list_users", arguments: { scope: "all" } });
    expect(cellTexts(root, 0)).toEqual(["Zed"]);
    const status = root.querySelector<HTMLElement>("[data-gomu-status]")!;
    expect(status.className).not.toContain("gomu-status--loading");
  });

  it("does not hydrate when no host answered the handshake (ready=false)", async () => {
    const root = shell();
    mountTable({
      root,
      config: config({ loadTool: "list_users" }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(false),
    });
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
  });

  it("keeps the baked snapshot when loadTool fails", async () => {
    const root = shell();
    host.onToolCall = () => {
      throw new Error("boom");
    };
    mountTable({
      root,
      config: config({ loadTool: "list_users" }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(true),
    });
    await flush();
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    const status = root.querySelector<HTMLElement>("[data-gomu-status]")!;
    expect(status.className).not.toContain("gomu-status--loading");
  });
});
