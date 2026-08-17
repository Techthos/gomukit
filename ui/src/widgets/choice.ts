// Choice widget behavior: pick one option or several, read the case for the
// one in hand, then submit. Go rendered the question and the buttons; the
// options are runtime state — a tool result can replace the whole list — so
// this builds them, decides where each option's description goes, enforces the
// selection rules, and drives the submit/cancel calls.
//
// The decision is terminal: once it is made the controls go away and the
// outcome stays. The single exception is a failed submit, which re-arms the
// widget so a transient error can be retried.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { checkbox, clear, delegate, h } from "../dom";
import { awaitData, SKELETON_ROWS, skeletonLines } from "../loading";
import { CallToolResult, M } from "../protocol";
import { errorText, textOf } from "../status";
import { ActionCfg, resolveArgs } from "./card-common";
import { DescriptionItemCfg, fillDescriptions } from "./descriptions";

interface OptionCfg {
	value: string;
	label: string;
	summary?: string;
	body?: string;
	bullets?: string[];
	details?: DescriptionItemCfg[];
	data?: Row;
	badge?: string;
	badgeVariant?: string;
	default?: boolean;
	disabled?: boolean;
}

interface ChoiceCfg {
	widget: string;
	layout: "auto" | "split" | "stacked";
	rowsKey: string;
	optionsKey: string;
	rowId: string;
	multiple?: boolean;
	min: number;
	max?: number;
	options?: OptionCfg[];
	details?: DescriptionItemCfg[];
	submit: {
		tool: string;
		valueArg: string;
		args?: ActionCfg["args"];
		/** Posts this plus the decision as a user turn. See ChoiceSubmit.ChatPrompt. */
		chatPrompt?: string;
		successMessage?: string;
	};
	cancel?: { tool?: string; args?: ActionCfg["args"]; message: string };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

// Badge variants that may become a class name. Options can arrive as tool
// data, so the variant is matched against this set rather than interpolated —
// the same reason data never reaches the DOM as markup.
const VARIANTS = new Set(["neutral", "info", "success", "warning", "danger"]);

// The width, in rem, at or above which the auto layout puts the description
// beside the options. Below it there is no room for two readable columns, and
// the description goes under the option it belongs to. Matches the breakpoint
// the rest of the bundle narrows at (grep "container gomu").
const SPLIT_AT_REM = 34;

export function mountChoice(ctx: MountContext): void {
	const cfg = ctx.config as unknown as ChoiceCfg;
	const { root, bridge } = ctx;

	const cardEl = root.querySelector<HTMLElement>(".gomu-choice");
	const detailsEl = root.querySelector<HTMLElement>("[data-gomu-descriptions]");
	const listEl = root.querySelector<HTMLElement>("[data-gomu-options]");
	const panelEl = root.querySelector<HTMLElement>("[data-gomu-panel]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gomu-empty]");
	const hintEl = root.querySelector<HTMLElement>("[data-gomu-hint]");
	const decisionEl = root.querySelector<HTMLElement>("[data-gomu-decision]");
	const outcomeEl = root.querySelector<HTMLElement>("[data-gomu-outcome]");
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");
	const submitEl = root.querySelector<HTMLButtonElement>("[data-gomu-submit]");
	const cancelEl = root.querySelector<HTMLButtonElement>("[data-gomu-cancel]");

	const items = Array.isArray(cfg.details) ? cfg.details : [];
	const multiple = !!cfg.multiple;
	const min = Math.max(1, cfg.min || 1);
	const max = cfg.max && cfg.max > 0 ? cfg.max : 0;

	let options: OptionCfg[] = optionsFrom(ctx.initialData, cfg.optionsKey) ?? cfg.options ?? [];
	// False until the option list has actually resolved. An unloaded widget
	// shows a skeleton: "no options yet" and "nothing to choose from" are
	// different answers, and only the second one is the empty state's to give.
	let loaded = options.length > 0;
	let row: Row | null = rowsFrom(ctx.initialData, cfg.rowsKey)[0] ?? null;
	let chosen = new Set<string>();
	// The option the description block is about. Follows the last option the
	// reader touched, so the panel answers what they are looking at.
	let active = "";
	let phase: "deciding" | "working" | "settled" = "deciding";
	let layout: "split" | "stacked" = cfg.layout === "split" ? "split" : "stacked";

	function showStatus(kind: "loading" | "error" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gomu-status" + (kind ? ` gomu-status--${kind}` : "");
	}

	function selectable(): OptionCfg[] {
		return options.filter((o) => !o.disabled);
	}

	/** Applies the options' own defaults, dropping anything stale. Called
	 * whenever the list is replaced, never after a decision is under way. */
	function resetSelection(): void {
		chosen = new Set(selectable().filter((o) => o.default).map((o) => o.value));
		if (!multiple && chosen.size > 1) chosen = new Set([[...chosen][0] as string]);
		if (max && chosen.size > max) chosen = new Set([...chosen].slice(0, max));
		active = [...chosen][0] ?? selectable()[0]?.value ?? "";
	}

	/** The effective layout: what the author asked for, or — with "auto" — what
	 * the width the host gave this widget can carry. */
	function measureLayout(): void {
		let next: "split" | "stacked";
		if (cfg.layout === "auto") {
			const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
			next = root.clientWidth >= SPLIT_AT_REM * rem ? "split" : "stacked";
		} else {
			next = cfg.layout === "split" ? "split" : "stacked";
		}
		if (next === layout) return;
		layout = next;
		renderOptions();
	}

	function syncControls(): void {
		const locked = phase !== "deciding";
		const enough = chosen.size >= min && (!max || chosen.size <= max);
		if (submitEl) submitEl.disabled = locked || !enough;
		if (cancelEl) cancelEl.disabled = locked;
		if (hintEl) {
			const text = hintText();
			hintEl.hidden = text === "" || locked;
			hintEl.textContent = text;
		}
	}

	// What is still needed, or what has been picked. Single choices say
	// nothing: the radios already say it.
	function hintText(): string {
		if (!multiple || options.length === 0) return "";
		if (chosen.size < min) {
			return min === 1 ? "Choose at least one option." : `Choose at least ${min} options.`;
		}
		return max ? `${chosen.size} of ${max} chosen.` : `${chosen.size} chosen.`;
	}

	/** The description block for an option: prose, points, and a detail list
	 * resolved against the option's own record. Null when it has nothing to
	 * say, so no empty panel is rendered. */
	function infoNode(opt: OptionCfg): HTMLElement | null {
		const bullets = (opt.bullets ?? []).filter((b) => b !== "");
		const details = opt.details ?? [];
		if (!opt.body && bullets.length === 0 && details.length === 0) return null;

		const info = h("div", { class: "gomu-choice-info" });
		info.append(h("h4", { class: "gomu-choice-info-title" }, opt.label));
		if (opt.body) info.append(h("p", { class: "gomu-choice-info-body" }, opt.body));
		if (bullets.length > 0) {
			const ul = h("ul", { class: "gomu-choice-bullets" });
			for (const b of bullets) ul.append(h("li", {}, b));
			info.append(ul);
		}
		if (details.length > 0) {
			const dl = h("dl", { class: "gomu-descriptions gomu-choice-details" });
			fillDescriptions(dl, details, opt.data ?? {});
			info.append(dl);
		}
		return info;
	}

	function optionNode(opt: OptionCfg, index: number): HTMLElement {
		const selected = chosen.has(opt.value);
		// Max reached: what is not already chosen can no longer be, and says so
		// rather than failing silently on click.
		const capped = multiple && !!max && !selected && chosen.size >= max;
		const disabled = !!opt.disabled || capped || phase !== "deciding";

		let cls = "gomu-choice-option";
		if (selected) cls += " gomu-choice-option--selected";
		if (disabled) cls += " gomu-choice-option--disabled";
		if (opt.value === active) cls += " gomu-choice-option--active";

		const label = h("label", { class: cls, "data-gomu-option": String(index) });

		let input: HTMLInputElement;
		if (multiple) {
			const box = checkbox({ "data-gomu-pick": "" }, "gomu-choice-mark");
			input = box.input;
			label.append(box.wrap);
		} else {
			input = h("input", {
				type: "radio",
				name: "gomu-choice",
				class: "gomu-choice-radio",
				"data-gomu-pick": "",
			}) as HTMLInputElement;
			label.append(h("span", { class: "gomu-choice-mark gomu-choice-mark--radio" }, input));
		}
		input.checked = selected;
		input.disabled = disabled;
		input.value = opt.value;

		const text = h("div", { class: "gomu-choice-text" });
		const head = h("div", { class: "gomu-choice-head" });
		head.append(h("span", { class: "gomu-choice-label" }, opt.label));
		if (opt.badge) {
			const variant =
				opt.badgeVariant && VARIANTS.has(opt.badgeVariant) && opt.badgeVariant !== "neutral"
					? ` gomu-badge--${opt.badgeVariant}`
					: "";
			head.append(h("span", { class: `gomu-badge${variant}` }, opt.badge));
		}
		text.append(head);
		if (opt.summary) text.append(h("span", { class: "gomu-choice-summary" }, opt.summary));

		// Stacked: the description lives with its option, and only the chosen
		// one is unfolded — an unfolded list of every case is not a list. An
		// option that cannot be chosen unfolds when it is pointed at, since
		// choosing it is not a way to read it.
		if (layout === "stacked" && (selected || (opt.value === active && !!opt.disabled))) {
			const info = infoNode(opt);
			if (info) text.append(info);
		}
		label.append(text);
		return label;
	}

	function renderOptions(): void {
		if (!listEl) return;
		// A rebuild throws away the focused input; keyboard users would land
		// back at the top of the document on every arrow press.
		const hadFocus = listEl.contains(document.activeElement);

		clear(listEl);
		if (loaded) {
			for (const [i, opt] of options.entries()) listEl.append(optionNode(opt, i));
		} else {
			for (let i = 0; i < SKELETON_ROWS; i++) {
				listEl.append(
					h("div", { class: "gomu-choice-option gomu-choice-option--skeleton", "aria-hidden": "true" },
						skeletonLines(2),
					),
				);
			}
		}
		listEl.hidden = loaded && options.length === 0;
		// Never while pending: "nothing to choose from" is a claim the widget
		// cannot make before it has been given a list.
		if (emptyEl) emptyEl.hidden = !loaded || options.length > 0;
		if (cardEl) {
			cardEl.classList.remove("gomu-choice--auto", "gomu-choice--split", "gomu-choice--stacked");
			cardEl.classList.add(`gomu-choice--${layout}`);
		}

		renderPanel();
		if (hadFocus) focusActive();
	}

	/** Puts keyboard focus on the option in hand — after a rebuild, and after a
	 * click on a row, whose default focusing behavior we suppress. */
	function focusActive(): void {
		const idx = options.findIndex((o) => o.value === active);
		if (idx < 0) return;
		listEl
			?.querySelector<HTMLInputElement>(`[data-gomu-option="${idx}"] input`)
			?.focus({ preventScroll: true });
	}

	// Split: one panel, describing the option in hand.
	function renderPanel(): void {
		if (!panelEl) return;
		clear(panelEl);
		const opt = options.find((o) => o.value === active);
		const info = layout === "split" && opt ? infoNode(opt) : null;
		panelEl.hidden = info === null;
		if (info) panelEl.append(info);
	}

	function renderDetails(): void {
		if (detailsEl) fillDescriptions(detailsEl, items, row);
	}

	function render(): void {
		renderDetails();
		renderOptions();
		syncControls();
	}

	function pick(index: number): void {
		if (phase !== "deciding") return;
		const opt = options[index];
		if (!opt) return;
		// The description follows whatever is pointed at, including an option
		// that cannot be taken: why it cannot is exactly what it has to say.
		active = opt.value;
		if (!opt.disabled) {
			if (!multiple) {
				chosen = new Set([opt.value]);
			} else if (chosen.has(opt.value)) {
				chosen.delete(opt.value);
			} else if (!max || chosen.size < max) {
				chosen.add(opt.value);
			}
		}
		renderOptions();
		syncControls();
	}

	// The end of the widget's life: the controls go, the outcome stays.
	function settle(message: string, kind: "accepted" | "declined"): void {
		phase = "settled";
		syncControls();
		renderOptions();
		if (decisionEl) decisionEl.hidden = true;
		if (hintEl) hintEl.hidden = true;
		if (outcomeEl) {
			outcomeEl.hidden = false;
			outcomeEl.textContent = message;
			outcomeEl.className = `gomu-choice-outcome gomu-choice-outcome--${kind}`;
		}
		showStatus("", "");
	}

	function fail(message: string): void {
		phase = "deciding";
		renderOptions();
		syncControls();
		showStatus("error", message);
	}

	// Runs one side of the decision. Returns the result, or null when the call
	// failed and the widget has already re-armed for a retry.
	async function call(
		tool: string,
		args: ActionCfg["args"],
		extra: Record<string, unknown>,
		fallback: string,
	): Promise<CallToolResult | null> {
		phase = "working";
		renderOptions();
		syncControls();
		showStatus("loading", "Working…");
		try {
			const action: ActionCfg = { label: "", kind: "tool", tool, args };
			const res = await bridge.callTool(tool, { ...resolveArgs(action, row, []), ...extra });
			if (res.isError) {
				fail(textOf(res) ?? fallback);
				return null;
			}
			return res;
		} catch (e) {
			fail(errorText(e, fallback));
			return null;
		}
	}

	/** The decision, in option order: one value, or the array of them. */
	function decision(): unknown {
		const picked = options.filter((o) => chosen.has(o.value)).map((o) => o.value);
		return multiple ? picked : (picked[0] ?? "");
	}

	/** What the reader picked, by label, for a message a person will read. */
	function decisionText(): string {
		return options
			.filter((o) => chosen.has(o.value))
			.map((o) => o.label || o.value)
			.join(", ");
	}

	// Hands the request to the host's chat instead of calling the tool: the
	// model makes the call, so there is no result to apply — only the turn
	// being accepted. Returns false when the host refused and the widget has
	// re-armed for a retry.
	async function chat(text: string): Promise<boolean> {
		phase = "working";
		renderOptions();
		syncControls();
		showStatus("loading", "Working…");
		try {
			await bridge.sendMessage(text);
			return true;
		} catch (e) {
			fail(errorText(e, "The request failed."));
			return false;
		}
	}

	async function submit(): Promise<void> {
		if (phase !== "deciding" || !cfg.submit?.tool) return;
		if (chosen.size < min || (max && chosen.size > max)) return;
		if (cfg.submit.chatPrompt) {
			// A choice's whole output is what the reader picked, and a chat turn
			// has no argument to carry it, so the decision goes in the text.
			if (!(await chat(`${cfg.submit.chatPrompt} — chose: ${decisionText()}`))) return;
			settle(cfg.submit.successMessage || "Sent.", "accepted");
			return;
		}
		const res = await call(
			cfg.submit.tool,
			cfg.submit.args,
			{ [cfg.submit.valueArg || "choice"]: decision() },
			"The action failed.",
		);
		if (!res) return;
		applyData(res);
		settle(cfg.submit.successMessage || textOf(res) || "Done.", "accepted");
	}

	async function cancel(): Promise<void> {
		if (phase !== "deciding" || !cfg.cancel) return;
		if (cfg.cancel.tool) {
			const res = await call(cfg.cancel.tool, cfg.cancel.args, {}, "Could not cancel.");
			if (!res) return;
		}
		settle(cfg.cancel.message || "Cancelled.", "declined");
	}

	// Refreshes whichever parts of the widget a payload carries. Never changes
	// the phase: a host may push results long after the decision. A replaced
	// option list resets the selection — the old values are not on offer any
	// more — unless the decision is already made.
	function applyData(data: CallToolResult | { structuredContent?: unknown }): void {
		const sc = (data as CallToolResult).structuredContent;
		if (!sc || typeof sc !== "object") return;
		// Any answer from the host ends the wait, including one that names no
		// options: that is the list, and it is empty.
		loaded = true;
		const content = sc as Record<string, unknown>;
		if (cfg.rowsKey in content) {
			row = rowsFrom(content, cfg.rowsKey)[0] ?? null;
		}
		const pushed = optionsFrom(content, cfg.optionsKey);
		if (pushed) {
			options = pushed;
			if (phase === "deciding") resetSelection();
		}
		render();
	}

	// A pick can come from the input (mouse, space bar) or from a click on the
	// row that misses it; both mean the same thing.
	if (listEl) {
		delegate(listEl, "click", "option", (_el, index, ev) => {
			// The label forwards a click to its input, which clicks the label
			// again; without this a pick would toggle twice and cancel itself.
			if (ev.target instanceof HTMLInputElement) return;
			ev.preventDefault();
			pick(Number(index));
			focusActive();
		});
		delegate(listEl, "change", "option", (_el, index) => pick(Number(index)));
		// Arrowing through radios moves the description with the focus.
		delegate(listEl, "focusin", "option", (_el, index) => {
			const opt = options[Number(index)];
			if (!opt || opt.value === active) return;
			active = opt.value;
			renderOptions();
		});
	}

	submitEl?.addEventListener("click", () => void submit());
	cancelEl?.addEventListener("click", () => void cancel());

	// Link values in a detail list go to the host: navigation is blocked
	// inside the sandboxed iframe.
	delegate(root, "click", "link", (_el, href) => {
		if (href !== "") void bridge.openLink(href);
	});

	bridge.on(M.toolInput, () => {
		if (phase === "deciding") showStatus("loading", "Loading…");
	});
	bridge.on(M.toolResult, (params) => {
		applyData((params ?? {}) as CallToolResult);
		if (phase === "deciding") showStatus("", "");
	});
	bridge.on(M.toolCancelled, () => {
		if (phase === "working") phase = "deciding";
		// The call this widget was waiting on is not coming back.
		loaded = true;
		renderOptions();
		syncControls();
		showStatus("", "");
	});
	document.addEventListener(HOST_CONTEXT_EVENT, () => {
		renderDetails();
		renderOptions();
	});

	resetSelection();
	measureLayout();
	render();
	if (!loaded) showStatus("loading", "Loading…");

	// The auto layout is a running measurement, not a boot-time one: the host
	// resizes the frame as the conversation pane changes.
	if (cfg.layout === "auto" && typeof ResizeObserver !== "undefined") {
		new ResizeObserver(() => measureLayout()).observe(root);
	}

	// Load-time hydration: with a host connected, offer what is on the shelf
	// now rather than what was there when the document was registered.
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (!ok) return;
			showStatus("loading", "Loading…");
			bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {}).then(
				(res) => {
					applyData(res);
					if (phase === "deciding") showStatus("", "");
				},
				() => {
					// A load that failed still answers the question the skeleton asks.
					stopWaiting();
					if (phase === "deciding") showStatus("", "");
				},
			);
		});
	}

	// Bounded wait for options the host may push on its own (a tool-result
	// notification): without it a widget with no authored options and no load
	// tool would hold its skeleton forever against a host that never sends one.
	// Also ends the wait when no host answered at all, load tool or not.
	function stopWaiting(): void {
		if (loaded) return;
		loaded = true;
		renderOptions();
		syncControls();
		if (phase === "deciding") showStatus("", "");
	}
	if (!loaded) awaitData(ctx.ready, !!cfg.loadTool, stopWaiting);
}

/**
 * Reads the option array from a structuredContent-shaped object, or null when
 * the payload does not mention it at all. Options delivered by a tool describe
 * themselves in plain values: a detail entry is {label, value}, formatted by
 * the server, where an authored one is a typed field config.
 */
function optionsFrom(
	data: Record<string, unknown> | null | undefined,
	key: string,
): OptionCfg[] | null {
	const v = data?.[key];
	if (!Array.isArray(v)) return null;
	return v
		.filter((o): o is Record<string, unknown> => o !== null && typeof o === "object")
		.map((o) => ({
			value: String(o.value ?? ""),
			label: String(o.label ?? o.value ?? ""),
			summary: o.summary === undefined ? undefined : String(o.summary),
			body: o.body === undefined ? undefined : String(o.body),
			bullets: Array.isArray(o.bullets) ? o.bullets.map((b) => String(b)) : undefined,
			details: detailsFrom(o.details),
			badge: o.badge === undefined ? undefined : String(o.badge),
			badgeVariant: o.badgeVariant === undefined ? undefined : String(o.badgeVariant),
			default: o.default === true,
			disabled: o.disabled === true,
		}))
		.filter((o) => o.value !== "");
}

/** Tool-supplied detail pairs, as fixed-text description items. */
function detailsFrom(v: unknown): DescriptionItemCfg[] | undefined {
	if (!Array.isArray(v)) return undefined;
	const items = v
		.filter((d): d is Record<string, unknown> => d !== null && typeof d === "object")
		.map((d) => ({
			key: "",
			type: "text",
			label: String(d.label ?? ""),
			text: String(d.value ?? d.text ?? ""),
		}))
		.filter((d) => d.label !== "");
	return items.length > 0 ? items : undefined;
}
