/**
 * jsdom gap-filling for AG Grid. The grid reads layout and observation APIs
 * the headless DOM does not provide; the stubs below are enough for creation,
 * rendering, editing and teardown. Recorded in research/ag-grid.md.
 */
const win = window as unknown as Record<string, unknown>;

if (typeof globalThis.ResizeObserver === 'undefined') {
	class NoopResizeObserver {
		observe(): void {}
		unobserve(): void {}
		disconnect(): void {}
	}
	globalThis.ResizeObserver = NoopResizeObserver as unknown as typeof ResizeObserver;
}

if (typeof window.matchMedia !== 'function') {
	win['matchMedia'] = () => ({
		matches: false,
		media: '',
		addEventListener: () => {},
		removeEventListener: () => {},
		addListener: () => {},
		removeListener: () => {},
		dispatchEvent: () => false,
		onchange: null
	});
}

if (typeof globalThis.requestAnimationFrame !== 'function') {
	globalThis.requestAnimationFrame = (callback: FrameRequestCallback): number =>
		window.setTimeout(() => callback(Date.now()), 0);
	globalThis.cancelAnimationFrame = (id: number): void => window.clearTimeout(id);
}
