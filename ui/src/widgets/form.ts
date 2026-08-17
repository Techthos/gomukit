// Form widget behavior: native-validation gating, submit as an MCP tool
// call, server-side field errors mapped inline, prefill from tool results.
import type { MountContext } from "../index";
import { enhanceDateFields, refreshDateFields, type CalendarCfg } from "../calendar";
import { refreshDropdowns } from "../dropdown";
import { awaitData, seeded } from "../loading";
import { CallToolResult, M } from "../protocol";
import { errorText, textOf } from "../status";

interface FieldCfg {
	name: string;
	type: string;
	message?: string;
	// Date fields only: the grid the field opens, and — for a range — the
	// argument its end travels in.
	calendar?: CalendarCfg;
	endName?: string;
	required?: boolean;
}

interface FormCfg {
	widget: string;
	prefillKey: string;
	errorsKey: string;
	submit: {
		tool: string;
		staticArgs?: Record<string, unknown>;
		successMessage?: string;
	};
	fields: FieldCfg[];
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

type FormControl = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export function mountForm(ctx: MountContext): void {
	const cfg = ctx.config as unknown as FormCfg;
	const { root, bridge } = ctx;

	const formMaybe = root.querySelector<HTMLFormElement>("form[data-gomu-form]");
	if (!formMaybe || !Array.isArray(cfg.fields)) return;
	const form: HTMLFormElement = formMaybe;
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");

	// Every date field becomes a gomukit calendar before anything reads the form.
	// The native inputs survive the upgrade and stay the value holders, so
	// everything below goes on driving them (see ui/src/calendar.ts).
	const byName = new Map(cfg.fields.map((f) => [f.name, f]));
	enhanceDateFields(form, (name) => byName.get(name) ?? null);

	function controlFor(name: string): FormControl | null {
		const el = form.elements.namedItem(name);
		return el instanceof HTMLInputElement ||
			el instanceof HTMLTextAreaElement ||
			el instanceof HTMLSelectElement
			? el
			: null;
	}

	function showStatus(kind: "loading" | "error" | "success" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gomu-status" + (kind ? ` gomu-status--${kind}` : "");
	}

	function setBusy(busy: boolean): void {
		for (const el of form.querySelectorAll<HTMLElement>("input, textarea, select, button")) {
			(el as FormControl | HTMLButtonElement).disabled = busy;
		}
	}

	function clearErrors(): void {
		for (const slot of root.querySelectorAll<HTMLElement>("[data-gomu-error-for]")) {
			slot.hidden = true;
			slot.textContent = "";
		}
		for (const f of cfg.fields) {
			controlFor(f.name)?.removeAttribute("aria-invalid");
		}
	}

	function showFieldError(name: string, message: string): void {
		for (const slot of root.querySelectorAll<HTMLElement>("[data-gomu-error-for]")) {
			if (slot.getAttribute("data-gomu-error-for") === name) {
				slot.hidden = false;
				slot.textContent = message;
			}
		}
		controlFor(name)?.setAttribute("aria-invalid", "true");
	}

	function showErrors(errors: Record<string, unknown>): number {
		let n = 0;
		for (const [name, msg] of Object.entries(errors)) {
			if (typeof msg === "string" && msg !== "") {
				showFieldError(name, msg);
				n++;
			}
		}
		return n;
	}

	function applyValues(values: Record<string, unknown>): void {
		for (const f of cfg.fields) {
			// A range prefill may name either end, so the field is in play as
			// soon as one of its two arguments is.
			const end = f.type === "daterange" ? (f.endName ?? "") : "";
			if (!(f.name in values) && !(end !== "" && end in values)) continue;
			const control = controlFor(f.name);
			if (!control) continue;
			const v = values[f.name];
			if (f.type === "checkbox" && control instanceof HTMLInputElement) {
				control.checked = v === true;
			} else if (f.type === "multiselect" && control instanceof HTMLSelectElement) {
				const wanted = Array.isArray(v) ? v.map(String) : [];
				for (const opt of control.options) {
					opt.selected = wanted.includes(opt.value);
				}
			} else {
				control.value = text(v);
				if (end !== "") {
					const endControl = controlFor(end);
					if (endControl) endControl.value = text(values[end]);
				}
			}
		}
		// A programmatic write fires no event, so the controls built over the
		// native ones — dropdowns over selects, calendars over date inputs —
		// cannot see the new prefill on their own.
		refreshDropdowns(form);
		refreshDateFields(form);
	}

	function text(v: unknown): string {
		return v === null || v === undefined ? "" : String(v);
	}

	function collectValues(): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const f of cfg.fields) {
			const control = controlFor(f.name);
			if (!control) continue;
			if (f.type === "checkbox" && control instanceof HTMLInputElement) {
				out[f.name] = control.checked;
			} else if (f.type === "multiselect" && control instanceof HTMLSelectElement) {
				out[f.name] = [...control.selectedOptions].map((o) => o.value);
			} else if (f.type === "number") {
				if (control.value !== "") out[f.name] = Number(control.value);
			} else if (f.type === "daterange") {
				// Two flat arguments rather than one composite: a tool schema can
				// declare two date strings, and a server can read them without
				// unpacking anything.
				out[f.name] = control.value;
				const end = f.endName ?? "";
				const endControl = end === "" ? null : controlFor(end);
				if (endControl) out[end] = endControl.value;
			} else {
				out[f.name] = control.value;
			}
		}
		return out;
	}

	function validate(): boolean {
		let ok = form.checkValidity();
		for (const f of cfg.fields) {
			const control = controlFor(f.name);
			if (control && !control.validity.valid) {
				showFieldError(f.name, f.message ?? control.validationMessage);
			}
			if (f.type !== "daterange") continue;
			const endControl = f.endName ? controlFor(f.endName) : null;
			if (endControl && !endControl.validity.valid) {
				showFieldError(f.name, f.message ?? endControl.validationMessage);
			}
			// An order no native constraint can express. The calendar cannot
			// produce a backwards range, but the date inputs it stands in front
			// of can — that is the fallback control, and it is still live.
			if (control && endControl && control.value !== "" && endControl.value !== "") {
				if (endControl.value < control.value) {
					showFieldError(f.name, f.message ?? "The end date is before the start date.");
					ok = false;
				}
			}
		}
		if (ok) return true;
		showStatus("error", "Please fix the highlighted fields.");
		return false;
	}

	function applyResult(res: CallToolResult, viaSubmit: boolean): void {
		const sc = res.structuredContent ?? {};
		const values = sc[cfg.prefillKey];
		if (values && typeof values === "object" && !Array.isArray(values)) {
			applyValues(values as Record<string, unknown>);
		}
		const errors = sc[cfg.errorsKey];
		const errCount =
			errors && typeof errors === "object" && !Array.isArray(errors)
				? showErrors(errors as Record<string, unknown>)
				: 0;

		if (errCount > 0) {
			showStatus("error", "Please fix the highlighted fields.");
		} else if (res.isError) {
			showStatus("error", textOf(res) ?? "The request failed.");
		} else if (viaSubmit) {
			showStatus("success", cfg.submit.successMessage ?? textOf(res) ?? "Saved.");
		} else {
			showStatus("", "");
		}
	}

	function doSubmit(): void {
		clearErrors();
		if (!validate()) return;
		const args = { ...(cfg.submit.staticArgs ?? {}), ...collectValues() };
		showStatus("loading", "Submitting…");
		setBusy(true);
		bridge.callTool(cfg.submit.tool, args).then(
			(res) => {
				setBusy(false);
				applyResult(res, true);
			},
			(e: unknown) => {
				setBusy(false);
				showStatus("error", errorText(e, "The request failed."));
			},
		);
	}

	// Hosts sandbox the widget iframe without allow-forms, so a native submit
	// is blocked before its event ever fires: the button click is the real
	// entry point. The submit listener stays for hosts that do allow forms,
	// and Enter in a single-line field is wired by hand for the same reason.
	root.querySelector<HTMLElement>("[data-gomu-submit]")?.addEventListener("click", doSubmit);

	form.addEventListener("submit", (ev) => {
		ev.preventDefault();
		doSubmit();
	});

	form.addEventListener("keydown", (ev) => {
		if (ev.key !== "Enter" || ev.isComposing) return;
		const target = ev.target;
		if (!(target instanceof HTMLInputElement) || target.type === "checkbox") return;
		ev.preventDefault();
		doSubmit();
	});

	root.querySelector<HTMLElement>("[data-gomu-cancel]")?.addEventListener("click", () => {
		form.reset();
		refreshDropdowns(form);
		refreshDateFields(form);
		clearErrors();
		showStatus("", "");
	});

	// Host-pushed results (e.g. the model invoked the edit tool: prefill).
	bridge.on(M.toolInput, () => showStatus("loading", "Loading…"));
	bridge.on(M.toolResult, (params) => applyResult((params ?? {}) as CallToolResult, false));
	bridge.on(M.toolCancelled, () => showStatus("", ""));

	// Baked snapshot: prefill and errors, if present.
	if (ctx.initialData) {
		applyResult({ structuredContent: ctx.initialData }, false);
	}

	// A form's structure is not its data, so it gets no skeleton: the fields
	// are real and correct from the first paint. What it can be missing is
	// prefill, and only a form that names a load tool is waiting for any — a
	// create form must stay typeable, not sit disabled waiting for values that
	// were never coming. While the wait is on, the controls are locked, so
	// nothing the reader types is overwritten when the values land.
	let loaded = !cfg.loadTool || seeded(ctx.initialData, cfg.prefillKey);
	function stopWaiting(): void {
		if (loaded) return;
		loaded = true;
		setBusy(false);
		showStatus("", "");
	}
	if (!loaded) {
		setBusy(true);
		showStatus("loading", "Loading…");
	}

	// Load-time hydration: once a host is connected, fetch fresh prefill and
	// replace the baked snapshot, so a reloaded form shows current values
	// instead of the state frozen at render time.
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (!ok) return;
			showStatus("loading", "Loading…");
			setBusy(true);
			bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {}).then(
				(res) => {
					loaded = true;
					setBusy(false);
					applyResult(res, false);
				},
				() => {
					// A load that failed still ends the wait: a blank form the
					// reader can use beats a locked one they cannot.
					stopWaiting();
				},
			);
		});
	}

	// Ends the wait when no host answered the handshake, so a form previewed
	// without a host is not left disabled forever.
	if (!loaded) awaitData(ctx.ready, !!cfg.loadTool, stopWaiting);
}
