// Card widget behavior: renders a single record (the first row under
// RowsKey) as a card, fires its actions as MCP tool calls, and updates from
// tool-result notifications. A read-only-ish detail view; no filter/sort/
// pagination/selection.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear } from "../dom";
import { awaitData, seeded, skeletonCard } from "../loading";
import { confirmAction } from "../confirm-modal";
import { CallToolResult, M } from "../protocol";
import { errorText, textOf } from "../status";
import { releaseDropdowns } from "../dropdown";
import {
	ActionCfg,
	CardTemplateCfg,
	renderCard,
	resolveArgs,
	templateActions,
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

interface CardCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	card: CardTemplateCfg;
	empty?: { title?: string; body?: string; immediate?: boolean };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

const STATUS_CLEAR_MS = 4000;

export function mountCard(ctx: MountContext): void {
	const cfg = ctx.config as unknown as CardCfg;
	const { root, bridge } = ctx;

	const hostEl = root.querySelector<HTMLElement>("[data-gomu-card]");
	if (!hostEl || typeof cfg.card !== "object" || typeof cfg.card.header !== "object") return;
	const actions = templateActions(cfg.card);
	const host: HTMLElement = hostEl;
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gomu-empty]");

	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	let row: Row | null = rowsFrom(ctx.initialData, cfg.rowsKey)[0] ?? null;
	// False until the record has actually resolved. An unloaded card shows a
	// skeleton: "no record yet" and "no such record" are different answers, and
	// only the second one is the empty state's to give.
	let loaded = seeded(ctx.initialData, cfg.rowsKey) || !!cfg.empty?.immediate;
	let busy = false;
	let statusTimer: ReturnType<typeof setTimeout> | undefined;
	// The card is rebuilt on every state change, so what the reader has put
	// into its content controls is kept here and handed back to each render.
	const inputs: InputValues = {};
	const asks = hasInputs(cfg.card.content?.items ?? []);

	function showStatus(kind: "loading" | "error" | "success" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gomu-status" + (kind ? ` gomu-status--${kind}` : "");
	}

	function render(): void {
		// The dropdowns of the card being replaced own panels outside it.
		releaseDropdowns(host);
		clear(host);
		if (!loaded) {
			host.append(skeletonCard());
		} else if (row) {
			host.append(renderCard(cfg.card, row, { id: rowID(row), busy, values: inputs }));
			// Selects become dropdowns only now: a panel is placed against the
			// widget root, which the card could not reach while detached.
			if (asks) enhanceDescriptionInputs(host);
		}
		// Never while pending: "no record" is a claim the card cannot make
		// before it has been given one.
		if (emptyEl) emptyEl.hidden = !loaded || !!row;
		if (!loaded) showStatus("loading", "Loading…");
	}

	function applyResult(res: CallToolResult): void {
		busy = false;
		// Any answer from the host ends the wait, including one that carries no
		// record: that is the card's data, and there is none.
		loaded = true;
		if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
			row = rowsFrom(res.structuredContent, cfg.rowsKey)[0] ?? null;
		}
		render();
		if (res.isError) {
			showStatus("error", textOf(res) ?? "The action failed.");
		} else {
			showStatus("success", textOf(res) ?? "Done");
			clearTimeout(statusTimer);
			statusTimer = setTimeout(() => showStatus("", ""), STATUS_CLEAR_MS);
		}
	}

	async function fire(action: ActionCfg): Promise<void> {
		if (action.kind === "link") {
			const href = row?.[action.hrefKey ?? ""];
			if (typeof href === "string" && href !== "") void bridge.openLink(href);
			return;
		}
		if (!action.tool) return;
		// What the card's controls hold travels with the action, so an
		// unanswered required question stops it the way a form's would.
		if (asks && !validateInputs(host)) {
			showStatus("error", "Please fix the highlighted fields.");
			return;
		}
		const answers = asks ? collectInputs(host) : {};
		clearTimeout(statusTimer);
		busy = true;
		render();
		showStatus("loading", "Working…");
		try {
			// A prompt action hands the request to the host's chat: the model makes
			// the call, so there is no result of ours to apply — only the turn being
			// accepted.
			if (action.prompt) {
				await bridge.sendMessage(action.prompt);
				busy = false;
				render();
				showStatus("", "");
				return;
			}
			applyResult(
				await bridge.callTool(action.tool, { ...resolveArgs(action, row, []), ...answers }),
			);
		} catch (e) {
			busy = false;
			render();
			showStatus("error", errorText(e, "The action failed."));
		}
	}

	// Native confirm() is silently disabled in sandboxed MCP Apps iframes; a
	// confirmed action asks over the frame instead (see confirm-modal.ts).
	function armOrFire(btn: HTMLElement, action: ActionCfg): void {
		if (action.confirm) {
			confirmAction(
				btn,
				{ message: action.confirm, confirmLabel: action.label, variant: action.variant },
				() => void fire(action),
			);
			return;
		}
		void fire(action);
	}

	if (asks) {
		watchInputs(host, (name, value, el) => {
			inputs[name] = value;
			if (el.checkValidity()) clearInputErrors(el.closest(".gomu-desc-item") ?? host);
		});
	}

	host.addEventListener("click", (ev) => {
		const target = ev.target;
		if (!(target instanceof Element)) return;
		const btn = target.closest<HTMLElement>("[data-gomu-action]");
		if (btn && host.contains(btn)) {
			const action = actions[Number(btn.getAttribute("data-gomu-action"))];
			if (action) armOrFire(btn, action);
			return;
		}
		const link = target.closest<HTMLElement>("[data-gomu-link]");
		if (link && host.contains(link)) {
			const href = link.getAttribute("data-gomu-link") ?? "";
			if (href !== "") void bridge.openLink(href);
		}
	});

	// Host-pushed results and re-render on host context (Intl locale/timeZone).
	bridge.on(M.toolInput, () => {
		busy = true;
		render();
		showStatus("loading", "Loading…");
	});
	bridge.on(M.toolResult, (params) => applyResult((params ?? {}) as CallToolResult));
	bridge.on(M.toolCancelled, () => {
		busy = false;
		// The call this card was waiting on is not coming back.
		loaded = true;
		render();
		showStatus("", "");
	});
	document.addEventListener(HOST_CONTEXT_EVENT, render);

	render();

	// Load-time hydration: once a host is connected, fetch the fresh record
	// and replace the baked snapshot. Silent on success and on failure.
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (!ok) return;
			busy = true;
			render();
			showStatus("loading", "Loading…");
			bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {}).then(
				(res) => {
					busy = false;
					loaded = true;
					if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
						row = rowsFrom(res.structuredContent, cfg.rowsKey)[0] ?? null;
					}
					render();
					showStatus("", "");
				},
				() => {
					busy = false;
					// A load that failed still answers the question the skeleton asks.
					loaded = true;
					render();
					showStatus("", "");
				},
			);
		});
	}

	// Bounded wait for a record the host may push on its own (a tool-result
	// notification): without it a card with no snapshot and no load tool would
	// hold its skeleton forever against a host that never sends one. Also ends
	// the wait when no host answered at all, load tool or not.
	if (!loaded) {
		awaitData(ctx.ready, !!cfg.loadTool, () => {
			if (loaded) return;
			loaded = true;
			render();
			showStatus("", "");
		});
	}
}
