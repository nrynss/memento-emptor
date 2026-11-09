// Fixture trace links for the why view: the Astropods turns behind each
// recommendation from a belief. Fixture mode has no trace viewer, so every
// href is an on-page placeholder anchor, and the view says so next to the
// list; live mode will point these at the trace viewer instead.
//
// Provenance: the T3.1 turn recommending from the correction while citing
// the contradicted L belief, the six T3.4 turns (pending poll, reason,
// correction, honoured recommendation, approval, fits-well feedback), and
// the probe turn looking up the 7XK return in testdata/astropods. The T3.1
// id below is the 8-hex prefix its handoff records; the T3.4 ids are the
// short ids theirs records.

import type { BeliefTraceLink } from './why';

/** Placeholder href for a fixture trace: an on-page anchor, never a guessed host. */
export function traceHref(traceId: string): string {
	return `#trace-${traceId}`;
}

function link(trace_id: string, label: string): BeliefTraceLink {
	return { trace_id, label, href: traceHref(trace_id) };
}

/** Turns behind recommendations from the contradicted L belief, oldest first. */
const sizingTraces: BeliefTraceLink[] = [
	link(
		'828c0f1b850595e214b3cdceb33e52d8',
		'Probe turn looking up the 7XK return behind this belief'
	),
	link('879c8752', 'Turn asking about the refund still missing its reason'),
	link('b51715f4', 'Turn recommending from the correction, citing this belief'),
	link('fad137db', 'Turn honouring the correction on the next recommendation')
];

/** The turn recording the fits-well feedback behind the emerging M belief. */
const emergingTraces: BeliefTraceLink[] = [
	link('d6e19c5a', 'Turn recording the fits-well feedback behind this belief')
];

/**
 * The fixture trace links for one belief, oldest turn first. Empty when no
 * recorded turn touches the belief: the seller belief and the general
 * statement have none in fixture mode.
 */
export function tracesForBelief(beliefId: string): BeliefTraceLink[] {
	switch (beliefId) {
		case 'blf_8f2c41':
			return sizingTraces;
		case 'blf_2b9d60':
			return emergingTraces;
		default:
			return [];
	}
}
