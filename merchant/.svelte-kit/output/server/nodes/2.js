

export const index = 2;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_page.svelte.js')).default;
export const imports = ["_app/immutable/nodes/2.2mmsFy1Z.js","_app/immutable/chunks/xihTtKlq.js","_app/immutable/chunks/D2bUyeWH.js","_app/immutable/chunks/DKazQFzE.js"];
export const stylesheets = ["_app/immutable/assets/2.Dk5OwPAg.css"];
export const fonts = [];
