// The why view model: one belief with the evidence that earned it, the
// evidence deliberately not counted toward it, the correction beside it,
// and the agent traces behind each recommendation. Pure data shaping over
// the frozen ledger client: no Chaaya, no DOM, so the gate tests it
// headless against fixtures.

import type {
	BeliefKind,
	BeliefView,
	CorrectionView,
	EvidenceView,
	LedgerEvidenceRow,
	QuestionView,
	ReasonCategory
} from '$lib/api';

/** One evidence row the layer deliberately did not count toward a belief, and why. */
export interface NotCountedEvidence {
	evidence: EvidenceView;
	why_not: string;
}

/** One refund still missing its reason: an outcome counting toward nothing yet. */
export interface PendingNotCounted {
	question: QuestionView;
	why_not: string;
}

/** One agent turn behind a recommendation from this belief. */
export interface BeliefTraceLink {
	trace_id: string;
	label: string;
	href: string;
}

/** Everything the why view shows for one belief. */
export interface WhyViewData {
	belief: BeliefView;
	corrections: CorrectionView[];
	seeded_notice: string | null;
	counted: EvidenceView[];
	not_counted: NotCountedEvidence[];
	pending: PendingNotCounted[];
	traces: BeliefTraceLink[];
}

/** The shopper-facing word for what a belief kind is about. */
export function kindWord(kind: BeliefKind): string {
	return kind === 'fit' ? 'sizing' : 'seller reliability';
}

/**
 * The classified reason categories in the project's own plain words: the
 * merchant grid's reason columns (Fit: small, Fit: large, Damaged, Not
 * arrived, Changed mind) and the domain's category comments, which read
 * fit_too_small as "too small or too tight" and fit_too_large as "too big or
 * too loose". The layer's `none` is absent on purpose: it is the zero value,
 * no classified reason rather than a verdict, so a card that named a word for
 * it would assert a reason the layer never classified.
 */
const reasonWords: Record<Exclude<ReasonCategory, 'none'>, string> = {
	fit_too_small: 'too small',
	fit_too_large: 'too large',
	damaged_or_defective: 'damaged',
	not_arrived: 'not arrived',
	changed_mind: 'changed mind',
	unclear: 'unclear'
};

/**
 * The plain-language name of one classified reason category, or null for the
 * layer's `none`: that is no classified reason, so the card shows the recorded
 * text alone instead of naming a category nobody classified. The unauthorised-
 * transaction dispute is stored with `none`, which is why `none` cannot read
 * as a fit verdict.
 */
export function reasonWord(category: ReasonCategory): string | null {
	return category === 'none' ? null : reasonWords[category];
}

/** One evidence row's reason as a card shows it. */
export interface ReasonReading {
	/** The recorded text: the shopper's words, or a dispute's code. */
	text: string;
	/** Whether the text is the shopper's own words, and so reads as a quote. */
	quoted: boolean;
	/** The plain category word, or null when the layer classified no reason. */
	word: string | null;
}

/**
 * Reads one evidence row's reason for a card, or null when the outcome has no
 * known cause yet. The wire type says the reason text is the shopper's words,
 * or the dispute code for a dispute, so a dispute's text never reads as a
 * quote: the plain category word leads and the code follows as the record.
 */
export function reasonReading(evidence: EvidenceView): ReasonReading | null {
	const reason = evidence.reason;
	if (reason === null) {
		return null;
	}
	return {
		text: reason.text,
		quoted: evidence.outcome.kind !== 'dispute',
		word: reasonWord(reason.category)
	};
}

/**
 * Why one evidence row does not count toward a belief it does not support.
 * Rows that count toward a belief of some kind say which kind and that they
 * support another belief; rows that count toward this belief's own kind say
 * so explicitly. Every other row follows the frozen reason table: fit
 * reasons count toward sizing, damage and non-arrival count against the
 * seller, and reasonless rows with no counts_toward count toward nothing.
 */
export function notCountedReason(evidence: EvidenceView, belief: BeliefView): string {
	if (evidence.counts_toward === belief.kind) {
		return `Counts toward ${kindWord(belief.kind)}, but supports another belief, not this one.`;
	}
	switch (evidence.reason?.category ?? null) {
		case 'fit_too_small':
		case 'fit_too_large':
			return 'Fit evidence counts toward sizing, never against the seller.';
		case 'damaged_or_defective':
		case 'not_arrived':
			return 'Counts against the seller, never toward sizing.';
		default:
			if (evidence.counts_toward !== null) {
				return `Counts toward ${kindWord(evidence.counts_toward)}, but supports another belief, not this one.`;
			}
			return 'Counts toward nothing.';
	}
}

/** Why a refund still missing its reason counts toward nothing. */
export const PENDING_WHY_NOT =
	'A refund with no reason counts toward nothing until the shopper answers.';

/**
 * Assembles the why view for one belief. Counted evidence comes from the
 * belief itself; everything else in the ledger is listed as deliberately
 * not counted, each with its reason.
 */
export function buildWhyView(
	belief: BeliefView,
	corrections: CorrectionView[],
	seeded_notice: string | null,
	allEvidence: LedgerEvidenceRow[],
	pending: QuestionView[],
	traces: BeliefTraceLink[]
): WhyViewData {
	const countedIds = new Set(belief.evidence.map((row) => row.evidence_id));
	return {
		belief,
		corrections,
		seeded_notice,
		counted: belief.evidence,
		not_counted: allEvidence
			.filter((row) => !countedIds.has(row.evidence.evidence_id))
			.map((row) => ({ evidence: row.evidence, why_not: notCountedReason(row.evidence, belief) })),
		pending: pending.map((question) => ({ question, why_not: PENDING_WHY_NOT })),
		traces
	};
}

/** Every searchable word of one evidence row, lowercased. */
export function evidenceSearchText(evidence: EvidenceView): string {
	return [
		evidence.evidence_id,
		evidence.outcome.kind,
		evidence.outcome.status,
		evidence.item.brand_name,
		evidence.item.line_name,
		evidence.item.sku,
		evidence.item.size,
		evidence.item.category,
		evidence.reason?.category ?? '',
		evidence.reason?.text ?? '',
		evidence.link.order_id ?? '',
		evidence.link.capture_id ?? '',
		evidence.link.refund_id ?? '',
		evidence.link.dispute_id ?? ''
	]
		.join(' ')
		.toLowerCase();
}

/** Every searchable word of one pending question, lowercased. */
export function pendingSearchText(question: QuestionView): string {
	return [
		question.question_id,
		question.outcome.kind,
		question.outcome.status,
		question.item.brand_name,
		question.item.line_name,
		question.item.sku,
		question.item.size,
		question.item.category
	]
		.join(' ')
		.toLowerCase();
}

/** Whether one evidence row matches a search query. Blank matches everything. */
export function matchesEvidenceQuery(evidence: EvidenceView, query: string): boolean {
	const q = query.trim().toLowerCase();
	return q === '' || evidenceSearchText(evidence).includes(q);
}

/** Whether one pending question matches a search query. Blank matches everything. */
export function matchesPendingQuery(question: QuestionView, query: string): boolean {
	const q = query.trim().toLowerCase();
	return q === '' || pendingSearchText(question).includes(q);
}

/** The filtered lists the why view renders for one search query. */
export interface FilteredWhyLists {
	counted: EvidenceView[];
	not_counted: NotCountedEvidence[];
	pending: PendingNotCounted[];
}

/** Filters every evidence list of a why view by one search query. */
export function filterWhyView(view: WhyViewData, query: string): FilteredWhyLists {
	return {
		counted: view.counted.filter((row) => matchesEvidenceQuery(row, query)),
		not_counted: view.not_counted.filter((row) => matchesEvidenceQuery(row.evidence, query)),
		pending: view.pending.filter((row) => matchesPendingQuery(row.question, query))
	};
}
