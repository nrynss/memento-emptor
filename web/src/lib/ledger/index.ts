// The ledger table: flat grid rows, the plain-API grid, and its theme.
// Beside it, the pivot's reads: which precedence step decided, the pooled
// counts, and the shopper's pooling choice.
export { createLedgerGrid, explainCellRenderer, explainColumnDefs, ledgerColumnDefs } from './grid';
export { linkedBeliefId, pickBeliefId } from './deeplink';
export {
	decisionLineFor,
	decodePrecedence,
	decodeRecallResponse,
	guidanceBases,
	overriddenLineFor,
	stepLabelFor,
	type GuidanceBasis,
	type OverriddenStep,
	type PoolLookup,
	type PrecedenceView,
	type RecallBelief,
	type RecallCorrectionView,
	type RecallGuidance,
	type RecallResponse
} from './precedence';
export type { OverriddenStep as DecisionOverriddenStep } from './precedence';
export { default as DecisionLine } from './DecisionLine.svelte';
export {
	createFixtureProductFitTransport,
	createFixtureRecallTransport,
	fixtureDemoPool,
	fixtureRecallFor,
	recallCaseLabels,
	recallCaseShopper,
	recallCases,
	type ProductFitToolName,
	type ProductFitTransport,
	type RecallCase,
	type RecallToolName,
	type RecallTransport
} from './recall';
export {
	decodePoolCounts,
	decodePoolView,
	decodeProductFitResponse,
	directionWordFor,
	poolSeededLine,
	poolSummaryLine,
	type PoolCountsView,
	type PoolDirection,
	type PoolDirectionCounts,
	type PoolLineView,
	type PoolPrecedenceView,
	type PoolReasonCounts,
	type PoolShopperCounts,
	type PoolVerdictCounts,
	type PoolView
} from './pool';
export { default as PoolCounts } from './PoolCounts.svelte';
export { default as StandingCorrection } from './StandingCorrection.svelte';
export {
	createFixturePoolingState,
	createFixturePoolingTransport,
	choiceLineFor,
	decodeSetPoolingResponse,
	PoolingClient,
	PoolingState,
	resultLineFor,
	type PoolingStatus,
	type PoolingToolName,
	type PoolingTransport,
	type SetPoolingRequest,
	type SetPoolingResponse
} from './pooling';
export { default as PoolingToggle } from './PoolingToggle.svelte';
export {
	escapeLedgerHtml,
	formatLedgerDate,
	hasWeakSignal,
	kindLabelFor,
	OUT_OF_TRUE_TOOLTIP,
	SEEDED_MARK_LABEL,
	SEEDED_MARK_TOOLTIP,
	toLedgerGridRow,
	toLedgerGridRows,
	WEAK_SIGNAL_LABEL,
	WEAK_SIGNAL_TOOLTIP,
	type LedgerGridRow
} from './rows';
export { ledgerGridTheme, ledgerToken, type LedgerGridMode } from './theme';
