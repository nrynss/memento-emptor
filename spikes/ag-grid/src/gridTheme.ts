import { colorSchemeDark, themeQuartz, type Theme } from 'ag-grid-community';

/** The two themes Chaaya paints: the forced mode, or the system preference. */
export type GridMode = 'light' | 'dark';

/**
 * Read one Chaaya token from the live stylesheet. The forced `data-theme`
 * attribute and the system media query both flow through the same computed
 * style, so the grid follows either without its own listener. Falls back to
 * the light reference value when the token is absent, which is also what a
 * headless DOM without the stylesheet reports.
 */
export function chaayaToken(name: string, fallback: string): string {
	if (typeof document === 'undefined' || typeof getComputedStyle !== 'function') {
		return fallback;
	}
	const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
	return value === '' ? fallback : value;
}

/**
 * Build the grid theme from Chaaya's tokens. Light mode repaints Quartz with
 * the app's own surface, text, header and accent. Dark mode adds the dark
 * colour scheme part, which supplies dark defaults for everything the tokens
 * do not name, including the browser colour scheme for native widgets.
 */
export function chaayaGridTheme(mode: GridMode): Theme {
	const light = themeQuartz.withParams({
		backgroundColor: chaayaToken('--surface', '#ffffff'),
		foregroundColor: chaayaToken('--text', '#16161a'),
		headerBackgroundColor: chaayaToken('--raised', '#fafafb'),
		headerTextColor: chaayaToken('--text', '#16161a'),
		accentColor: chaayaToken('--accent', '#4b4b57'),
		borderColor: chaayaToken('--line', '#d6d6dc'),
		fontFamily: chaayaToken('--font-ui', 'system-ui, sans-serif')
	});
	return mode === 'dark' ? light.withPart(colorSchemeDark) : light;
}
