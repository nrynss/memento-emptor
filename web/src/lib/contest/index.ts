// Contesting a belief: the shopper's edit through record_correction.
export { CorrectionClient } from './client';
export {
	contestCellRenderer,
	contestEditColumnDefs,
	CONTEST_ACTION_LABEL,
	CONTEST_BUTTON_ATTRIBUTE
} from './columns';
export { default as ContestEditor } from './ContestEditor.svelte';
export { decodeRecordCorrectionResponse } from './decode';
export {
	createFixtureCorrectionTransport,
	fixtureContradictedBeliefId,
	fixtureCorrectionText
} from './fixture';
export {
	createFixtureContestState,
	confirmationLineFor,
	correctionPreviewLine,
	ContestState,
	type ContestStatus
} from './state';
export type {
	CorrectionGuidance,
	CorrectionToolName,
	CorrectionTransport,
	RecordCorrectionRequest,
	RecordCorrectionResponse
} from './types';
