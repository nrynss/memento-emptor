// The shopper's correction path in TypeScript, mirroring the pivoted
// record_correction schemas in testdata/api. Field names stay snake_case
// so the frozen examples assign without mapping. A correction stands on
// its own when no belief exists yet: contradicts_belief_id is absent, and
// so is belief_id on the response.
import type { BeliefStatus, GraphStanding } from '$lib/api';

/** The shopper's explicit correction, declared against one belief when one exists. */
export interface RecordCorrectionRequest {
	shopper_id: string;
	text: string;
	contradicts_belief_id?: string;
	trace_id?: string;
}

/** The precedence step the layer recommends from, in the pivot's words. */
export type CorrectionGuidanceBasis =
	| 'correction'
	| 'own_evidence'
	| 'pool'
	| 'statement'
	| 'none';

/** What the layer will recommend next, and why. */
export interface CorrectionGuidance {
	text: string;
	basis: CorrectionGuidanceBasis;
	correction_id: string | null;
	belief_id: string | null;
}

/**
 * The shopper's correction as the pivot returns it. With a belief it is
 * declared against it at once; standing on its own it carries no against
 * declaration until a matching belief appears.
 */
export interface ContestCorrectionView {
	correction_id: string;
	text: string;
	contradicts_belief_id?: string;
	recorded_at: string;
	takes_precedence: true;
}

/**
 * The correction applies at once. With a belief, the contradicted belief
 * keeps standing; standing alone, there is no belief to report and the
 * correction carries the Claimed label with no standing.
 */
export interface RecordCorrectionResponse {
	correction: ContestCorrectionView;
	belief_id?: string;
	status: BeliefStatus;
	standing: GraphStanding;
	contradicted: boolean;
	guidance: CorrectionGuidance;
	trace_id: string | null;
}

/** The only tool the contest path may call. Any other tool is a type error. */
export type CorrectionToolName = 'record_correction';

/** Sends the shopper's correction to the layer. One tool, nothing else. */
export interface CorrectionTransport {
	call(tool: CorrectionToolName, request: Record<string, unknown>): Promise<unknown>;
}
