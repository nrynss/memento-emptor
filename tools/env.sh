# Source this file before a live run:
#
#   source tools/env.sh
#
# It exports every key the code reads from the system keyring, under the
# variable name the code uses. A variable already set in the environment is
# left alone. A key the keyring does not hold is reported by its variable name.
# This file holds no value and never prints one. It reads no path, so it works
# from any checkout or worktree.
#
# The keyring entries are `service <name> account memento-emptor`, with a
# `kind` attribute where one service holds more than one value.

_memento_env_load() {
	local name service kind value missing=""

	while IFS='|' read -r name service kind; do
		[ -n "$name" ] || continue
		eval "[ -n \"\${$name+x}\" ]" && continue

		if [ -n "$kind" ]; then
			value=$(secret-tool lookup service "$service" account memento-emptor kind "$kind" 2>/dev/null) || value=
		else
			value=$(secret-tool lookup service "$service" account memento-emptor 2>/dev/null) || value=
		fi

		if [ -n "$value" ]; then
			export "$name=$value"
		else
			missing="$missing
  $name"
		fi
		unset value
	done <<'KEYS'
PAYPAL_CLIENT_ID|paypal-sandbox|client-id
PAYPAL_CLIENT_SECRET|paypal-sandbox|client-secret
CHANNEL3_API_KEY|channel3|
OPENROUTER_API_KEY|openrouter|
LAMBO_ES_API_KEY|elastic|api-key
ELASTIC_URL|elastic|url
RENDER_API_KEY|render|
MEMENTO_IDENTITY_SALT|identity|salt
KEYS

	if ! command -v secret-tool >/dev/null 2>&1; then
		printf 'env.sh: secret-tool is not on PATH, so no key was read.\n' >&2
	fi

	if [ -n "$missing" ]; then
		printf 'env.sh: missing from the keyring:%s\n' "$missing" >&2
	fi
}

_memento_env_load
unset -f _memento_env_load
