import * as universal from '../entries/pages/_layout.ts.js';

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export { universal };
export const universal_id = "src/routes/+layout.ts";
export const imports = ["_app/immutable/nodes/0.ChtyZPrc.js","_app/immutable/chunks/xihTtKlq.js","_app/immutable/chunks/D2bUyeWH.js","_app/immutable/chunks/DRihsDVf.js","_app/immutable/chunks/DKazQFzE.js"];
export const stylesheets = ["_app/immutable/assets/0.Bjhlx3or.css"];
export const fonts = [];
