// Deep links into the why view. The ledger's per-row link names the belief
// to explain; the why view resolves that name against the rows the chosen
// step carries, so a link to a belief the step does not have falls back to
// the page's preferred belief instead of a blank view.

/** The belief id a why-view link names, or null when it names none. */
export function linkedBeliefId(url: URL | string | null | undefined): string | null {
	if (url === null || url === undefined) return null;
	const parsed = typeof url === 'string' ? new URL(url, 'http://ledger.invalid') : url;
	const linked = parsed.searchParams.get('belief');
	if (linked === null || linked.trim() === '') return null;
	return linked;
}

/**
 * Chooses the belief the why view explains: the linked belief when the step
 * carries it, else the page's preferred belief, else the first available
 * row. Null only when the step carries no belief at all.
 */
export function pickBeliefId(
	linkedId: string | null,
	available: string[],
	preferredId: string
): string | null {
	if (linkedId !== null && available.includes(linkedId)) return linkedId;
	if (available.includes(preferredId)) return preferredId;
	return available[0] ?? null;
}
