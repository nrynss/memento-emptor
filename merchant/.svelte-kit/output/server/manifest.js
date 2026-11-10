export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.B4r2JQuM.js",app:"_app/immutable/entry/app._9JebP4Q.js",imports:["_app/immutable/entry/start.B4r2JQuM.js","_app/immutable/chunks/DRihsDVf.js","_app/immutable/chunks/D2bUyeWH.js","_app/immutable/entry/app._9JebP4Q.js","_app/immutable/chunks/D2bUyeWH.js","_app/immutable/chunks/xihTtKlq.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js'))
		],
		remotes: {
			
		},
		routes: [
			
		],
		prerendered_routes: new Set(["/"]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
