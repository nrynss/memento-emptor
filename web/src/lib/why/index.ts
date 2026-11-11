// The why view: one belief with its counted evidence, the evidence
// deliberately not counted, the correction beside it, and trace links.
export { traceHref, tracesForBelief } from './traces';
export {
	buildWhyView,
	evidenceSearchText,
	filterWhyView,
	kindWord,
	matchesEvidenceQuery,
	matchesPendingQuery,
	notCountedReason,
	PENDING_WHY_NOT,
	pendingSearchText,
	reasonWord,
	type BeliefTraceLink,
	type FilteredWhyLists,
	type NotCountedEvidence,
	type PendingNotCounted,
	type WhyViewData
} from './why';
export { default as WhyView } from './WhyView.svelte';
