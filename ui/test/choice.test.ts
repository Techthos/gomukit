import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountChoice } from "../src/widgets/choice";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

// Mirrors the markup choice_render.go emits, minus the chrome the behavior
// never touches.
function choiceShell({ cancel = true, details = true, multiple = false } = {}): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "choice");
	root.innerHTML = `
    <div class="gomu-card gomu-choice gomu-choice--auto">
      <div class="gomu-choice-prompt"><h3 class="gomu-choice-question" id="gomu-choice-question">How should we ship?</h3></div>
      ${details ? `<dl class="gomu-descriptions" data-gomu-descriptions="" hidden></dl>` : ""}
      <div class="gomu-choice-body">
        <div class="gomu-choice-list" data-gomu-options="" role="${multiple ? "group" : "radiogroup"}"></div>
        <div class="gomu-choice-panel" data-gomu-panel="" hidden></div>
      </div>
      <div class="gomu-empty" data-gomu-empty="" hidden><h3>Nothing to choose from</h3></div>
      <p class="gomu-choice-hint" data-gomu-hint="" hidden></p>
      <div class="gomu-choice-actions" data-gomu-decision="">
        ${cancel ? `<button type="button" data-gomu-cancel="">Cancel</button>` : ""}
        <button type="button" data-gomu-submit="" disabled>Continue</button>
      </div>
      <p class="gomu-choice-outcome" data-gomu-outcome="" hidden></p>
      <div class="gomu-statusbar"><div class="gomu-status" data-gomu-status="" hidden></div></div>
    </div>`;
	document.body.append(root);
	return root;
}

const OPTIONS = [
	{
		value: "standard",
		label: "Standard",
		summary: "3-5 business days",
		body: "Handed over tonight.",
		bullets: ["Tracked to the depot"],
		details: [{ key: "price", label: "Price", type: "number" }],
		data: { price: 4.9 },
		default: true,
	},
	{
		value: "express",
		label: "Express",
		summary: "next business day",
		body: "Arrives before 12:00.",
		badge: "fastest",
		badgeVariant: "success",
	},
	{ value: "pickup", label: "Depot pickup", disabled: true },
];

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "choice",
		layout: "stacked",
		rowsKey: "rows",
		optionsKey: "options",
		rowId: "id",
		min: 1,
		options: OPTIONS,
		details: [{ key: "reference", label: "Order", type: "text" }],
		submit: {
			tool: "ship_order",
			valueArg: "method",
			args: { order: { row: "id" } },
			successMessage: "On its way.",
		},
		cancel: { message: "Cancelled." },
		...over,
	};
}

const DATA = { rows: [{ id: 4471, reference: "ORD-4471" }] };

const el = <T extends HTMLElement>(root: HTMLElement, sel: string): T =>
	root.querySelector<T>(sel)!;
const submit = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-submit]");
const cancel = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-cancel]");
const outcome = (root: HTMLElement) => el(root, "[data-gomu-outcome]");
const decision = (root: HTMLElement) => el(root, "[data-gomu-decision]");
const status = (root: HTMLElement) => el(root, "[data-gomu-status]");
const panel = (root: HTMLElement) => el(root, "[data-gomu-panel]");
const hint = (root: HTMLElement) => el(root, "[data-gomu-hint]");
const list = (root: HTMLElement) => el(root, "[data-gomu-options]");
const options = (root: HTMLElement) => [
	...root.querySelectorAll<HTMLElement>("[data-gomu-option]"),
];
const option = (root: HTMLElement, i: number) => options(root)[i]!;
const input = (root: HTMLElement, i: number) =>
	option(root, i).querySelector<HTMLInputElement>("input")!;
const labels = (root: HTMLElement) =>
	options(root).map((o) => o.querySelector(".gomu-choice-label")?.textContent);

describe("choice behavior", () => {
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

	it("renders the options and the record's details", () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		expect(labels(root)).toEqual(["Standard", "Express", "Depot pickup"]);
		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("ORD-4471");
		expect(option(root, 1).querySelector(".gomu-badge")?.className).toContain(
			"gomu-badge--success",
		);
		expect(input(root, 0).type).toBe("radio");
		expect(input(root, 2).disabled).toBe(true);
	});

	it("applies the option's own default and enables submit", () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		expect(input(root, 0).checked).toBe(true);
		expect(option(root, 0).className).toContain("gomu-choice-option--selected");
		expect(submit(root).disabled).toBe(false);
	});

	it("starts with nothing chosen when no option is the default", () => {
		const root = choiceShell();
		const plain = OPTIONS.map((o) => ({ ...o, default: false }));
		mountChoice({ root, config: config({ options: plain }), initialData: DATA, bridge });

		expect(submit(root).disabled).toBe(true);
		option(root, 1).click();
		expect(submit(root).disabled).toBe(false);
	});

	it("single choice: picking one replaces the other", () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		option(root, 1).click();
		expect(input(root, 0).checked).toBe(false);
		expect(input(root, 1).checked).toBe(true);
	});

	it("does not choose a disabled option, but does describe it", () => {
		const root = choiceShell();
		const cfg = config({
			options: OPTIONS.map((o) =>
				o.value === "pickup" ? { ...o, body: "No depot near this address." } : o,
			),
		});
		mountChoice({ root, config: cfg, initialData: DATA, bridge });

		option(root, 2).click();
		expect(input(root, 2).checked).toBe(false);
		expect(input(root, 0).checked).toBe(true);
		// Stacked: what it cannot be chosen for is where the reader looks.
		expect(option(root, 2).querySelector(".gomu-choice-info-body")?.textContent).toBe(
			"No depot near this address.",
		);
	});

	it("stacked: the description unfolds inside the chosen option only", () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		expect(option(root, 0).querySelector(".gomu-choice-info")).not.toBeNull();
		expect(option(root, 1).querySelector(".gomu-choice-info")).toBeNull();
		expect(panel(root).hidden).toBe(true);

		option(root, 1).click();
		expect(option(root, 0).querySelector(".gomu-choice-info")).toBeNull();
		expect(option(root, 1).querySelector(".gomu-choice-info-body")?.textContent).toBe(
			"Arrives before 12:00.",
		);
	});

	it("split: one panel, following the option in hand", () => {
		const root = choiceShell();
		mountChoice({ root, config: config({ layout: "split" }), initialData: DATA, bridge });

		expect(el(root, ".gomu-choice").className).toContain("gomu-choice--split");
		expect(panel(root).hidden).toBe(false);
		expect(panel(root).querySelector(".gomu-choice-info-title")?.textContent).toBe("Standard");
		expect(panel(root).querySelector(".gomu-choice-bullets li")?.textContent).toBe(
			"Tracked to the depot",
		);
		// The option's typed details are formatted against its own record.
		expect(panel(root).querySelector(".gomu-choice-details dd")?.textContent).toContain("4.9");
		expect(option(root, 0).querySelector(".gomu-choice-info")).toBeNull();

		option(root, 1).click();
		expect(panel(root).querySelector(".gomu-choice-info-title")?.textContent).toBe("Express");
	});

	it("split: the panel follows keyboard focus without changing the choice", () => {
		const root = choiceShell();
		mountChoice({ root, config: config({ layout: "split" }), initialData: DATA, bridge });

		input(root, 1).dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
		expect(panel(root).querySelector(".gomu-choice-info-title")?.textContent).toBe("Express");
		expect(input(root, 0).checked).toBe(true);
	});

	it("submits the chosen value under the configured argument name", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		option(root, 1).click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "ship_order",
			arguments: { order: 4471, method: "express" },
		});
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("On its way.");
		expect(outcome(root).className).toContain("gomu-choice-outcome--accepted");
	});

	it("submits through the chat with the decision appended to the prompt", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({
				submit: { tool: "ship_order", valueArg: "method", chatPrompt: "Ship order ORD-4471" },
			}),
			initialData: DATA,
			bridge,
		});

		option(root, 1).click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		// A chat turn has no argument to carry the decision, so it goes in the
		// text — by label, since a person reads it.
		expect(host.received(M.message)[0]!.params).toMatchObject({
			role: "user",
			content: [{ type: "text", text: "Ship order ORD-4471 — chose: Express" }],
		});
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Sent.");
	});

	it("lists every pick in the chat turn of a multiple choice", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({
				multiple: true,
				submit: { tool: "ship_order", valueArg: "method", chatPrompt: "Use these methods" },
			}),
			initialData: DATA,
			bridge,
		});

		option(root, 1).click();
		submit(root).click();
		await flush();

		// "Standard" is the option's own default, so both are picked, in option
		// order rather than click order.
		expect(host.received(M.message)[0]!.params).toMatchObject({
			content: [{ type: "text", text: "Use these methods — chose: Standard, Express" }],
		});
	});

	it("falls back to the result's text when no success message is configured", async () => {
		const root = choiceShell();
		host.onToolCall = () => ({ content: [{ type: "text", text: "Shipped." }] });
		mountChoice({
			root,
			config: config({ submit: { tool: "ship_order", valueArg: "method" } }),
			initialData: DATA,
			bridge,
		});

		submit(root).click();
		await flush();
		expect(outcome(root).textContent).toBe("Shipped.");
	});

	it("cannot be submitted twice", async () => {
		const root = choiceShell();
		host.onToolCall = () => ({ structuredContent: {} });
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		submit(root).click();
		await flush();
		submit(root).dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(1);
		expect(submit(root).disabled).toBe(true);
	});

	it("stays inert while the submit call is in flight", async () => {
		const root = choiceShell();
		let release: (() => void) | undefined;
		host.onToolCall = () =>
			new Promise((resolve) => {
				release = () => resolve({ structuredContent: {} });
			});
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		submit(root).click();
		await flush();
		expect(submit(root).disabled).toBe(true);
		expect(cancel(root).disabled).toBe(true);
		expect(input(root, 0).disabled).toBe(true);
		expect(status(root).textContent).toBe("Working…");

		release!();
		await flush();
		expect(decision(root).hidden).toBe(true);
	});

	it("re-arms after a failed submit so it can be retried", async () => {
		const root = choiceShell();
		host.onToolCall = () => ({ isError: true, content: [{ type: "text", text: "No couriers." }] });
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		submit(root).click();
		await flush();
		expect(status(root).textContent).toBe("No couriers.");
		expect(status(root).className).toContain("gomu-status--error");
		expect(decision(root).hidden).toBe(false);
		expect(submit(root).disabled).toBe(false);
		expect(input(root, 0).disabled).toBe(false);

		host.onToolCall = () => ({ structuredContent: {} });
		submit(root).click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(2);
		expect(decision(root).hidden).toBe(true);
	});

	it("cancels without a tool: no call, terminal message", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		cancel(root).click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(outcome(root).textContent).toBe("Cancelled.");
		expect(outcome(root).className).toContain("gomu-choice-outcome--declined");
		expect(decision(root).hidden).toBe(true);
	});

	it("cancels with a tool: calls it before settling", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({
				cancel: { tool: "postpone", args: { order: { row: "id" } }, message: "Postponed." },
			}),
			initialData: DATA,
			bridge,
		});

		cancel(root).click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "postpone",
			arguments: { order: 4471 },
		});
		expect(outcome(root).textContent).toBe("Postponed.");
	});

	describe("multiple choice", () => {
		const multi = (over: Record<string, unknown> = {}) =>
			config({
				multiple: true,
				options: OPTIONS.map((o) => ({ ...o, default: false })),
				...over,
			});

		it("ticks and unticks independently", () => {
			const root = choiceShell({ multiple: true });
			mountChoice({ root, config: multi(), initialData: DATA, bridge });

			expect(input(root, 0).type).toBe("checkbox");
			option(root, 0).click();
			option(root, 1).click();
			expect(input(root, 0).checked).toBe(true);
			expect(input(root, 1).checked).toBe(true);

			option(root, 0).click();
			expect(input(root, 0).checked).toBe(false);
			expect(input(root, 1).checked).toBe(true);
		});

		it("holds submit until the minimum is met and says what is missing", () => {
			const root = choiceShell({ multiple: true });
			mountChoice({ root, config: multi({ min: 2 }), initialData: DATA, bridge });

			expect(submit(root).disabled).toBe(true);
			expect(hint(root).hidden).toBe(false);
			expect(hint(root).textContent).toBe("Choose at least 2 options.");

			option(root, 0).click();
			expect(submit(root).disabled).toBe(true);
			option(root, 1).click();
			expect(submit(root).disabled).toBe(false);
			expect(hint(root).textContent).toBe("2 chosen.");
		});

		it("disables what is left once the maximum is reached", () => {
			const root = choiceShell({ multiple: true });
			mountChoice({ root, config: multi({ max: 1 }), initialData: DATA, bridge });

			option(root, 0).click();
			expect(hint(root).textContent).toBe("1 of 1 chosen.");
			expect(input(root, 1).disabled).toBe(true);
			option(root, 1).click();
			expect(input(root, 1).checked).toBe(false);

			// Freeing the slot puts the rest back on offer.
			option(root, 0).click();
			expect(input(root, 1).disabled).toBe(false);
		});

		it("submits the chosen values as an array, in option order", async () => {
			const root = choiceShell({ multiple: true });
			mountChoice({ root, config: multi(), initialData: DATA, bridge });

			option(root, 1).click();
			option(root, 0).click();
			submit(root).click();
			await flush();

			expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
				name: "ship_order",
				arguments: { method: ["standard", "express"] },
			});
		});
	});

	it("replaces the options from a pushed tool result and resets the selection", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		host.pushToolResult({
			structuredContent: {
				rows: [{ id: 12, reference: "ORD-12" }],
				options: [
					{
						value: "drone",
						label: "Drone",
						summary: "20 minutes",
						details: [{ label: "Price", value: "EUR 39.00" }],
						default: true,
					},
					{ value: "bike", label: "Bike courier" },
				],
			},
		});
		await flush();

		expect(labels(root)).toEqual(["Drone", "Bike courier"]);
		expect(input(root, 0).checked).toBe(true);
		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("ORD-12");
		expect(option(root, 0).querySelector(".gomu-choice-details dd")?.textContent).toBe(
			"EUR 39.00",
		);
	});

	it("shows a skeleton, not the empty state, while the options have not resolved", () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({ options: [] }),
			initialData: null,
			bridge,
			// A host answered, so options may still be pushed: the widget waits.
			ready: Promise.resolve(true),
		});
		expect(root.querySelectorAll(".gomu-choice-option--skeleton").length).toBeGreaterThan(0);
		expect(el(root, "[data-gomu-empty]").hidden).toBe(true);
		expect(el(root, "[data-gomu-status]").className).toContain("gomu-status--loading");
	});

	it("replaces the skeleton with the empty state once a result names no options", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({ options: [] }),
			initialData: null,
			bridge,
			ready: Promise.resolve(true),
		});
		host.pushToolResult({ structuredContent: { options: [] } });
		await flush();
		expect(root.querySelectorAll(".gomu-choice-option--skeleton")).toHaveLength(0);
		expect(el(root, "[data-gomu-empty]").hidden).toBe(false);
	});

	it("shows the empty state when a result leaves nothing to choose from", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		host.pushToolResult({ structuredContent: { options: [] } });
		await flush();

		expect(list(root).hidden).toBe(true);
		expect(el(root, "[data-gomu-empty]").hidden).toBe(false);
		expect(submit(root).disabled).toBe(true);
	});

	it("keeps a decided widget decided when the host pushes a later result", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		cancel(root).click();
		await flush();
		host.pushToolResult({ structuredContent: { options: [{ value: "drone", label: "Drone" }] } });
		await flush();

		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Cancelled.");
		expect(labels(root)).toEqual(["Drone"]);
		expect(input(root, 0).disabled).toBe(true);
	});

	it("ignores a badge variant that is not a known one", async () => {
		const root = choiceShell();
		mountChoice({ root, config: config(), initialData: DATA, bridge });

		host.pushToolResult({
			structuredContent: {
				options: [{ value: "x", label: "Odd", badge: "new", badgeVariant: "success; injected" }],
			},
		});
		await flush();
		expect(option(root, 0).querySelector(".gomu-badge")?.className).toBe("gomu-badge");
	});

	it("hydrates from LoadTool once a host is connected", async () => {
		const root = choiceShell();
		host.onToolCall = () => ({
			structuredContent: { options: [{ value: "drone", label: "Drone", default: true }] },
		});
		mountChoice({
			root,
			config: config({ loadTool: "get_options", loadArgs: { order: 4471 } }),
			initialData: DATA,
			bridge,
			ready: Promise.resolve(true),
		});
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "get_options",
			arguments: { order: 4471 },
		});
		expect(labels(root)).toEqual(["Drone"]);
		expect(submit(root).disabled).toBe(false);
		expect(status(root).hidden).toBe(true);
	});

	it("does not hydrate without a host", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({ loadTool: "get_options" }),
			initialData: DATA,
			bridge,
			ready: Promise.resolve(false),
		});
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
	});

	it("opens a detail link through the host", async () => {
		const root = choiceShell();
		mountChoice({
			root,
			config: config({
				details: [{ key: "", label: "Terms", type: "link", link: { hrefKey: "terms" } }],
			}),
			initialData: { rows: [{ id: 1, terms: "https://example.com/terms" }] },
			bridge,
		});

		el<HTMLElement>(root, "[data-gomu-link]").click();
		await flush();
		expect(host.received(M.openLink)[0]!.params).toMatchObject({
			url: "https://example.com/terms",
		});
	});
});
