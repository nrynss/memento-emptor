import "./index-server.js";
// node_modules/@nrynss/chaaya/dist/core/observable.js
/**
* The framework-neutral change signal. A class that holds public state
* extends `Notifier`, keeps each field private behind a getter, and calls
* `notify()` once per logical change. A binding turns that signal into its
* framework's reactivity. `@nrynss/chaaya/svelte` is the Svelte one. This
* module imports no framework.
*/
/** Log an error a listener threw. The change it observed has already
* landed, and the other listeners still run, so the error is logged rather
* than thrown into the class that changed or into the host. */
function report(error) {
	console.error("A Chaaya listener threw. The other listeners still ran.", error);
}
/**
* The base class for observable state. Construction touches nothing outside
* the instance, so it is safe at import time and on the server.
*
* Notification is synchronous. Listeners run in subscription order. One that
* throws does not stop the others, and its error is logged with
* `console.error`. A listener added during a notification first runs on the
* next one. A listener removed during a notification does not run again,
* even in the
* notification that removed it.
*/
var Notifier = class {
	#subscriptions = /* @__PURE__ */ new Set();
	#version = 0;
	/** Increments once per logical change, before listeners run. A cheap
	* snapshot key for bindings. */
	get version() {
		return this.#version;
	}
	/** Call listener after any public state change. Returns the unsubscribe,
	* which is safe to call more than once. The function is bound to its
	* instance, so a binding may pass `x.subscribe` around unbound, the way
	* an external-store hook takes it. */
	subscribe = (listener) => {
		if (typeof listener !== "function") throw new TypeError("subscribe needs a listener function.");
		const subscription = { listener };
		this.#subscriptions.add(subscription);
		return () => {
			this.#subscriptions.delete(subscription);
		};
	};
	/** Mark one logical change. Bumps `version`, then calls every listener. */
	notify() {
		this.#version += 1;
		for (const subscription of [...this.#subscriptions]) {
			if (!this.#subscriptions.has(subscription)) continue;
			try {
				subscription.listener();
			} catch (error) {
				report(error);
			}
		}
	}
};
// node_modules/@nrynss/chaaya/dist/theme/theme.js
/** The storage key that holds the chosen mode. */
var THEME_KEY = "chaaya-theme";
/** Resolve one theme from a stored mode and the system preference. An
* explicit light or dark wins over the system. Any other value, including
* the system mode and junk, defers to the system. Pure, so a unit test
* reads it with no DOM. */
function resolveTheme(mode, systemPrefersDark) {
	if (mode === "light" || mode === "dark") return mode;
	return systemPrefersDark ? "dark" : "light";
}
/** The accessible name for the control that switches to the other theme.
* The consumer renders its own control and reads this for its label. */
function themeToggleLabel(theme) {
	return theme === "dark" ? "Switch to light theme" : "Switch to dark theme";
}
// node_modules/@nrynss/chaaya/dist/theme/store.js
/** True when the operating system asks for the dark theme. It reads the
* media query on each call, so no import touches the DOM. */
function systemPrefersDark() {
	if (typeof window === "undefined") return false;
	return window.matchMedia("(prefers-color-scheme: dark)").matches;
}
/** The stored mode, or the system mode when storage is empty or blocked.
* The whole read sits in a try block, so a missing or throwing storage is
* not an error. The browser exposes storage as a getter that throws, and
* even a typeof check on it throws, so nothing reads it outside the block. */
function storedMode() {
	try {
		const stored = localStorage.getItem(THEME_KEY);
		if (stored === "light" || stored === "dark") return stored;
	} catch {}
	return "system";
}
/** Write the mode to storage. Persistence is best effort, so a blocked
* storage only costs the next visit. */
function persist(mode) {
	try {
		localStorage.setItem(THEME_KEY, mode);
	} catch {}
}
/** Paint one mode onto the document. An explicit mode sets the attribute
* that drives the stylesheet. The system mode removes it, so the media
* query keeps control. */
function paint(mode) {
	if (mode === "system") {
		delete document.documentElement.dataset.theme;
		return;
	}
	document.documentElement.dataset.theme = mode;
}
/** The live theme state. One object serves the whole app. It reads storage
* and the media query on its first use, not at import, so a plain node
* import stays inert. It notifies once per change of `mode` or `resolved`. */
var ThemeStore = class extends Notifier {
	#ready = false;
	#mode = "system";
	#systemDark = false;
	/** Read the stored mode and the system preference once, paint, and
	* start listening to the system preference. */
	#init() {
		if (this.#ready) return;
		this.#ready = true;
		this.#mode = storedMode();
		this.#systemDark = systemPrefersDark();
		if (typeof document !== "undefined") paint(this.#mode);
		if (typeof window !== "undefined") window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", this.#onSystemChange);
	}
	/** The chosen mode. */
	get mode() {
		this.#init();
		return this.#mode;
	}
	/** The theme the document paints. It follows the system preference live
	* while the mode is system. */
	get resolved() {
		this.#init();
		return resolveTheme(this.#mode, this.#systemDark);
	}
	/** Choose a mode, persist it, and paint it. */
	set(mode) {
		this.#init();
		const changed = mode !== this.#mode;
		this.#mode = mode;
		persist(mode);
		paint(mode);
		if (changed) this.notify();
	}
	#onSystemChange = () => {
		const dark = systemPrefersDark();
		if (dark === this.#systemDark) return;
		this.#systemDark = dark;
		if (this.#mode === "system") this.notify();
	};
};
/** The one theme object the app reads. A consumer reads `mode` and
* `resolved`, and calls `set` from its own control. A Svelte view reads it
* through `reactive(theme)`. */
var theme = new ThemeStore();
globalThis.Date;
globalThis.Set;
globalThis.Map;
globalThis.URL;
globalThis.URLSearchParams;
/**
* @param {any} _
*/
function createSubscriber(_) {
	return () => {};
}
// node_modules/@nrynss/chaaya/dist/svelte/reactive.js
/** One proxy per target, so wrapping twice hands back the same object. A
* proxy maps to itself, so wrapping a proxy hands it back too. */
var proxies = /* @__PURE__ */ new WeakMap();
/**
* Make an observable's reads reactive in Svelte 5.
*
* The result is a `Proxy` over the target. Every property read inside a
* reactive context (a template, an effect, a `$derived`) subscribes that
* context to the target, so the next `notify()` re-runs it. A read outside a
* reactive context subscribes nothing. The target's subscription opens with
* the first reactive reader and closes with the last.
*
* Reads and writes run against the target itself, so getters, setters, and
* `#private` fields keep their brand checks. A method comes back bound to
* the target, and the bound copy is cached, so `proxy.method` is stable.
* Wrapping the same target again, or wrapping the proxy, returns the same
* proxy.
*/
function reactive(target) {
	if (target === null || typeof target !== "object" || typeof target.subscribe !== "function") throw new TypeError("reactive needs an observable with a subscribe method.");
	const existing = proxies.get(target);
	if (existing !== void 0) return existing;
	const track = createSubscriber((update) => target.subscribe(update));
	const bound = /* @__PURE__ */ new Map();
	const proxy = new Proxy(target, {
		get(object, key) {
			track();
			const value = Reflect.get(object, key, object);
			if (typeof value !== "function") return value;
			const own = Reflect.getOwnPropertyDescriptor(object, key);
			if (own !== void 0 && own.configurable === false && own.writable === false) return value;
			const cached = bound.get(key);
			if (cached !== void 0 && cached.source === value) return cached.bound;
			const method = value.bind(object);
			bound.set(key, {
				source: value,
				bound: method
			});
			return method;
		},
		set(object, key, value) {
			return Reflect.set(object, key, value, object);
		}
	});
	proxies.set(target, proxy);
	proxies.set(proxy, proxy);
	return proxy;
}
// src/lib/product.ts
var merchantName = "Memento Merchant";
var tagline = "Why the products come back, by reason and size. Read-only: pooled counts, never a personal row.";
export { themeToggleLabel as a, theme as i, tagline as n, reactive as r, merchantName as t };
