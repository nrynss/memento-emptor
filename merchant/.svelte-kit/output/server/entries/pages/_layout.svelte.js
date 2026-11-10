import { l as attr, r as derived, u as escape_html } from "../../chunks/index-server.js";
import { t as resolve } from "../../chunks/paths.js";
import { i as theme, n as tagline, r as reactive, t as merchantName } from "../../chunks/product.js";
// src/routes/+layout.svelte
function _layout($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		let { children } = $$props;
		reactive(theme);
		const label = derived(() => "Switch theme");
		$$renderer.push(`<div class="shell"><header class="shell-header"><a class="shell-brand"${attr("href", resolve("/"))}>${escape_html(merchantName)}</a> <button type="button">${escape_html(label())}</button></header> <main class="shell-main">`);
		children($$renderer);
		$$renderer.push(`<!----></main> <footer class="shell-footer">${escape_html(tagline)}</footer></div>`);
	});
}
export { _layout as default };
