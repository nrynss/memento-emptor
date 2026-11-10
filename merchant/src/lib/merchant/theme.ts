// The grid theme, painted from Chaaya's live tokens. The forced data-theme
// attribute and the system media query both flow through the same computed
// style, so the theme builder needs no listener of its own. Dark mode adds
// the dark colour-scheme part, which supplies dark defaults for everything
// the tokens do not name.

import { colorSchemeDark, themeQuartz, type Theme } from 'ag-grid-community';

/** The two themes the app paints: the forced mode, or the system preference. */
export type MerchantGridMode = 'light' | 'dark';

/**
 * Reads one Chaaya token from the live stylesheet. Falls back to the light
 * reference value when the token is absent, which is also what a headless
 * DOM without the stylesheet reports.
 */
export function merchantToken(name: string, fallback: string): string {
	if (typeof document === 'undefined' || typeof getComputedStyle !== 'function') {
		return fallback;
	}
	const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
	return value === '' ? fallback : value;
}

/** Builds the grid theme from Chaaya's surface, text, header and accent. */
export function merchantGridTheme(mode: MerchantGridMode): Theme {
	const light = themeQuartz.withParams({
		backgroundColor: merchantToken('--surface', '#ffffff'),
		foregroundColor: merchantToken('--text', '#16161a'),
		headerBackgroundColor: merchantToken('--raised', '#fafafb'),
		headerTextColor: merchantToken('--text', '#16161a'),
		accentColor: merchantToken('--accent', '#4b4b57'),
		borderColor: merchantToken('--line', '#d6d6dc'),
		fontFamily: merchantToken('--font-ui', 'system-ui, sans-serif')
	});
	return mode === 'dark' ? light.withPart(colorSchemeDark) : light;
}
