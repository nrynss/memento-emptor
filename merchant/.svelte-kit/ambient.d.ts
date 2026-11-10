
// this file is generated — do not edit it


/// <reference types="@sveltejs/kit" />

/**
 * This module provides access to environment variables that are injected _statically_ into your bundle at build time and are limited to _private_ access.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Static environment variables are [loaded by Vite](https://vitejs.dev/guide/env-and-mode.html#env-files) from `.env` files and `process.env` at build time and then statically injected into your bundle at build time, enabling optimisations like dead code elimination.
 * 
 * **_Private_ access:**
 * 
 * - This module cannot be imported into client-side code
 * - This module only includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured)
 * 
 * For example, given the following build time environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { ENVIRONMENT, PUBLIC_BASE_URL } from '$env/static/private';
 * 
 * console.log(ENVIRONMENT); // => "production"
 * console.log(PUBLIC_BASE_URL); // => throws error during build
 * ```
 * 
 * The above values will be the same _even if_ different values for `ENVIRONMENT` or `PUBLIC_BASE_URL` are set at runtime, as they are statically replaced in your code with their build time values.
 */
declare module '$env/static/private' {
	export const SVELTEKIT_FORK: string;
	export const NODE_ENV: string;
	export const TERM_PROGRAM: string;
	export const OLDPWD: string;
	export const LC_NUMERIC: string;
	export const npm_config_global_ignore_file: string;
	export const GCP_LAMBO_CREDENTIALS: string;
	export const MAIL: string;
	export const KDE_APPLICATIONS_AS_SCOPE: string;
	export const npm_config_global_prefix: string;
	export const DBUS_SESSION_BUS_ADDRESS: string;
	export const OPENCODE: string;
	export const XDG_SESSION_CLASS: string;
	export const COMET_API_KEY: string;
	export const LESS: string;
	export const LC_PAPER: string;
	export const XDG_SESSION_DESKTOP: string;
	export const XKB_DEFAULT_MODEL: string;
	export const npm_command: string;
	export const XDG_SEAT: string;
	export const npm_package_version: string;
	export const LC_TELEPHONE: string;
	export const EDITOR: string;
	export const ZRO_API_KEY: string;
	export const FZF_DEFAULT_COMMAND: string;
	export const COLORTERM: string;
	export const OPENCODE_TERMINAL: string;
	export const HF_HOME: string;
	export const npm_config_globalconfig: string;
	export const GOOGLE_CLOUD_LOCATION: string;
	export const ZSH: string;
	export const MOTD_SHOWN: string;
	export const APINEX_API_KEY: string;
	export const COLOR: string;
	export const MEMORY_PRESSURE_WRITE: string;
	export const GHOSTTY_RESOURCES_DIR: string;
	export const XDG_VTNR: string;
	export const _: string;
	export const XDG_MENU_PREFIX: string;
	export const XDG_CURRENT_DESKTOP: string;
	export const LC_MEASUREMENT: string;
	export const GHOSTTY_SHELL_FEATURES: string;
	export const HISTCONTROL: string;
	export const FLYCTL_INSTALL: string;
	export const npm_config_node_gyp: string;
	export const npm_config_init_module: string;
	export const npm_node_execpath: string;
	export const PROMPT_COMMAND: string;
	export const AIMLAPI_API_KEY: string;
	export const npm_package_name: string;
	export const npm_config_local_prefix: string;
	export const XDG_SESSION_TYPE: string;
	export const npm_config_userconfig: string;
	export const LOGNAME: string;
	export const SHELL: string;
	export const MEMORY_PRESSURE_WATCH: string;
	export const DESKTOP_SESSION: string;
	export const npm_config_npm_version: string;
	export const XAUTHORITY: string;
	export const ASSEMBLYAI_API_KEY: string;
	export const DISPLAY: string;
	export const SYSTEMD_EXEC_PID: string;
	export const LSCOLORS: string;
	export const LC_NAME: string;
	export const npm_lifecycle_event: string;
	export const LS_COLORS: string;
	export const no_proxy: string;
	export const npm_package_json: string;
	export const LAMBO_POSTGRES_DSN: string;
	export const PNPM_HOME: string;
	export const XKB_DEFAULT_LAYOUT: string;
	export const TERM: string;
	export const NODE: string;
	export const npm_config_cache: string;
	export const AI_AGENT: string;
	export const GOOGLE_CLOUD_API_KEY: string;
	export const AGENT: string;
	export const SESSION_MANAGER: string;
	export const WAYLAND_DISPLAY: string;
	export const GOOGLE_CLOUD_PROJECT: string;
	export const LC_TIME: string;
	export const ICEAUTHORITY: string;
	export const HOME: string;
	export const TERM_PROGRAM_VERSION: string;
	export const KDE_SESSION_UID: string;
	export const PATH: string;
	export const GHOSTTY_BIN_DIR: string;
	export const P9K_SSH: string;
	export const LC_ADDRESS: string;
	export const XDG_CONFIG_DIRS: string;
	export const SARVAM_API_KEY: string;
	export const _JAVA_AWT_WM_NONREPARENTING: string;
	export const BUN_INSTALL: string;
	export const MANAGERPID: string;
	export const CUDA_DISABLE_PERF_BOOST: string;
	export const INIT_CWD: string;
	export const DEBUGINFOD_URLS: string;
	export const PGS_API_KEY: string;
	export const KDE_SESSION_VERSION: string;
	export const XDG_DATA_DIRS: string;
	export const npm_lifecycle_script: string;
	export const TERMINFO: string;
	export const LC_IDENTIFICATION: string;
	export const LANG: string;
	export const HISTORY_IGNORE: string;
	export const LESS_TERMCAP_me: string;
	export const VULTR_INFERENCE_API_KEY: string;
	export const LESS_TERMCAP_md: string;
	export const SHLVL: string;
	export const LC_MONETARY: string;
	export const npm_config_prefix: string;
	export const USER: string;
	export const GHOSTTY_SURFACE_ID: string;
	export const NO_PROXY: string;
	export const P9K_TTY: string;
	export const CUDA_PATH: string;
	export const OPENCODE_SESSION_ID: string;
	export const FZF_BASE: string;
	export const QT_WAYLAND_RECONNECT: string;
	export const ELEVENLABS_API_KEY: string;
	export const PAGER: string;
	export const GMI_API_KEY: string;
	export const REFLECTION_API_KEY: string;
	export const _P9K_SSH_TTY: string;
	export const XDG_SESSION_ID: string;
	export const MANAGERPIDFDID: string;
	export const npm_config_user_agent: string;
	export const DEEPSEEK_API_KEY: string;
	export const FEATHERLESS_API_KEY: string;
	export const _P9K_TTY: string;
	export const XDG_SESSION_PATH: string;
	export const VSSCRIPT_PATH: string;
	export const npm_execpath: string;
	export const PWD: string;
	export const BASETEN_API_KEY: string;
	export const YETA_API_KEY: string;
	export const KDE_FULL_SESSION: string;
	export const XDG_RUNTIME_DIR: string;
	export const XDG_SEAT_PATH: string;
	export const MKLROOT: string;
	export const INFERX_API_KEY: string;
	export const NVCC_CCBIN: string;
	export const npm_config_noproxy: string;
}

/**
 * This module provides access to environment variables that are injected _statically_ into your bundle at build time and are _publicly_ accessible.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Static environment variables are [loaded by Vite](https://vitejs.dev/guide/env-and-mode.html#env-files) from `.env` files and `process.env` at build time and then statically injected into your bundle at build time, enabling optimisations like dead code elimination.
 * 
 * **_Public_ access:**
 * 
 * - This module _can_ be imported into client-side code
 * - **Only** variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`) are included
 * 
 * For example, given the following build time environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { ENVIRONMENT, PUBLIC_BASE_URL } from '$env/static/public';
 * 
 * console.log(ENVIRONMENT); // => throws error during build
 * console.log(PUBLIC_BASE_URL); // => "http://site.com"
 * ```
 * 
 * The above values will be the same _even if_ different values for `ENVIRONMENT` or `PUBLIC_BASE_URL` are set at runtime, as they are statically replaced in your code with their build time values.
 */
declare module '$env/static/public' {
	
}

/**
 * This module provides access to environment variables set _dynamically_ at runtime and that are limited to _private_ access.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Dynamic environment variables are defined by the platform you're running on. For example if you're using [`adapter-node`](https://github.com/sveltejs/kit/tree/main/packages/adapter-node) (or running [`vite preview`](https://svelte.dev/docs/kit/cli)), this is equivalent to `process.env`.
 * 
 * **_Private_ access:**
 * 
 * - This module cannot be imported into client-side code
 * - This module includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured)
 * 
 * > [!NOTE] In `dev`, `$env/dynamic` includes environment variables from `.env`. In `prod`, this behavior will depend on your adapter.
 * 
 * > [!NOTE] To get correct types, environment variables referenced in your code should be declared (for example in an `.env` file), even if they don't have a value until the app is deployed:
 * >
 * > ```env
 * > MY_FEATURE_FLAG=
 * > ```
 * >
 * > You can override `.env` values from the command line like so:
 * >
 * > ```sh
 * > MY_FEATURE_FLAG="enabled" npm run dev
 * > ```
 * 
 * For example, given the following runtime environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { env } from '$env/dynamic/private';
 * 
 * console.log(env.ENVIRONMENT); // => "production"
 * console.log(env.PUBLIC_BASE_URL); // => undefined
 * ```
 */
declare module '$env/dynamic/private' {
	export const env: {
		SVELTEKIT_FORK: string;
		NODE_ENV: string;
		TERM_PROGRAM: string;
		OLDPWD: string;
		LC_NUMERIC: string;
		npm_config_global_ignore_file: string;
		GCP_LAMBO_CREDENTIALS: string;
		MAIL: string;
		KDE_APPLICATIONS_AS_SCOPE: string;
		npm_config_global_prefix: string;
		DBUS_SESSION_BUS_ADDRESS: string;
		OPENCODE: string;
		XDG_SESSION_CLASS: string;
		COMET_API_KEY: string;
		LESS: string;
		LC_PAPER: string;
		XDG_SESSION_DESKTOP: string;
		XKB_DEFAULT_MODEL: string;
		npm_command: string;
		XDG_SEAT: string;
		npm_package_version: string;
		LC_TELEPHONE: string;
		EDITOR: string;
		ZRO_API_KEY: string;
		FZF_DEFAULT_COMMAND: string;
		COLORTERM: string;
		OPENCODE_TERMINAL: string;
		HF_HOME: string;
		npm_config_globalconfig: string;
		GOOGLE_CLOUD_LOCATION: string;
		ZSH: string;
		MOTD_SHOWN: string;
		APINEX_API_KEY: string;
		COLOR: string;
		MEMORY_PRESSURE_WRITE: string;
		GHOSTTY_RESOURCES_DIR: string;
		XDG_VTNR: string;
		_: string;
		XDG_MENU_PREFIX: string;
		XDG_CURRENT_DESKTOP: string;
		LC_MEASUREMENT: string;
		GHOSTTY_SHELL_FEATURES: string;
		HISTCONTROL: string;
		FLYCTL_INSTALL: string;
		npm_config_node_gyp: string;
		npm_config_init_module: string;
		npm_node_execpath: string;
		PROMPT_COMMAND: string;
		AIMLAPI_API_KEY: string;
		npm_package_name: string;
		npm_config_local_prefix: string;
		XDG_SESSION_TYPE: string;
		npm_config_userconfig: string;
		LOGNAME: string;
		SHELL: string;
		MEMORY_PRESSURE_WATCH: string;
		DESKTOP_SESSION: string;
		npm_config_npm_version: string;
		XAUTHORITY: string;
		ASSEMBLYAI_API_KEY: string;
		DISPLAY: string;
		SYSTEMD_EXEC_PID: string;
		LSCOLORS: string;
		LC_NAME: string;
		npm_lifecycle_event: string;
		LS_COLORS: string;
		no_proxy: string;
		npm_package_json: string;
		LAMBO_POSTGRES_DSN: string;
		PNPM_HOME: string;
		XKB_DEFAULT_LAYOUT: string;
		TERM: string;
		NODE: string;
		npm_config_cache: string;
		AI_AGENT: string;
		GOOGLE_CLOUD_API_KEY: string;
		AGENT: string;
		SESSION_MANAGER: string;
		WAYLAND_DISPLAY: string;
		GOOGLE_CLOUD_PROJECT: string;
		LC_TIME: string;
		ICEAUTHORITY: string;
		HOME: string;
		TERM_PROGRAM_VERSION: string;
		KDE_SESSION_UID: string;
		PATH: string;
		GHOSTTY_BIN_DIR: string;
		P9K_SSH: string;
		LC_ADDRESS: string;
		XDG_CONFIG_DIRS: string;
		SARVAM_API_KEY: string;
		_JAVA_AWT_WM_NONREPARENTING: string;
		BUN_INSTALL: string;
		MANAGERPID: string;
		CUDA_DISABLE_PERF_BOOST: string;
		INIT_CWD: string;
		DEBUGINFOD_URLS: string;
		PGS_API_KEY: string;
		KDE_SESSION_VERSION: string;
		XDG_DATA_DIRS: string;
		npm_lifecycle_script: string;
		TERMINFO: string;
		LC_IDENTIFICATION: string;
		LANG: string;
		HISTORY_IGNORE: string;
		LESS_TERMCAP_me: string;
		VULTR_INFERENCE_API_KEY: string;
		LESS_TERMCAP_md: string;
		SHLVL: string;
		LC_MONETARY: string;
		npm_config_prefix: string;
		USER: string;
		GHOSTTY_SURFACE_ID: string;
		NO_PROXY: string;
		P9K_TTY: string;
		CUDA_PATH: string;
		OPENCODE_SESSION_ID: string;
		FZF_BASE: string;
		QT_WAYLAND_RECONNECT: string;
		ELEVENLABS_API_KEY: string;
		PAGER: string;
		GMI_API_KEY: string;
		REFLECTION_API_KEY: string;
		_P9K_SSH_TTY: string;
		XDG_SESSION_ID: string;
		MANAGERPIDFDID: string;
		npm_config_user_agent: string;
		DEEPSEEK_API_KEY: string;
		FEATHERLESS_API_KEY: string;
		_P9K_TTY: string;
		XDG_SESSION_PATH: string;
		VSSCRIPT_PATH: string;
		npm_execpath: string;
		PWD: string;
		BASETEN_API_KEY: string;
		YETA_API_KEY: string;
		KDE_FULL_SESSION: string;
		XDG_RUNTIME_DIR: string;
		XDG_SEAT_PATH: string;
		MKLROOT: string;
		INFERX_API_KEY: string;
		NVCC_CCBIN: string;
		npm_config_noproxy: string;
		[key: `PUBLIC_${string}`]: undefined;
		[key: `${string}`]: string | undefined;
	}
}

/**
 * This module provides access to environment variables set _dynamically_ at runtime and that are _publicly_ accessible.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Dynamic environment variables are defined by the platform you're running on. For example if you're using [`adapter-node`](https://github.com/sveltejs/kit/tree/main/packages/adapter-node) (or running [`vite preview`](https://svelte.dev/docs/kit/cli)), this is equivalent to `process.env`.
 * 
 * **_Public_ access:**
 * 
 * - This module _can_ be imported into client-side code
 * - **Only** variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`) are included
 * 
 * > [!NOTE] In `dev`, `$env/dynamic` includes environment variables from `.env`. In `prod`, this behavior will depend on your adapter.
 * 
 * > [!NOTE] To get correct types, environment variables referenced in your code should be declared (for example in an `.env` file), even if they don't have a value until the app is deployed:
 * >
 * > ```env
 * > MY_FEATURE_FLAG=
 * > ```
 * >
 * > You can override `.env` values from the command line like so:
 * >
 * > ```sh
 * > MY_FEATURE_FLAG="enabled" npm run dev
 * > ```
 * 
 * For example, given the following runtime environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://example.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { env } from '$env/dynamic/public';
 * console.log(env.ENVIRONMENT); // => undefined, not public
 * console.log(env.PUBLIC_BASE_URL); // => "http://example.com"
 * ```
 * 
 * ```
 * 
 * ```
 */
declare module '$env/dynamic/public' {
	export const env: {
		[key: `PUBLIC_${string}`]: string | undefined;
	}
}
