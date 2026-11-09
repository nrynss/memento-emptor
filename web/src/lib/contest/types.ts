// The shopper's correction path in TypeScript, mirroring the frozen
// record_correction schemas in testdata/api. Field names stay snake_case
// so the frozen examples assign without mapping.
import type { BeliefStatus, CorrectionView, GraphStanding } from '$lib/api';

/** The shopper's explicit correction, declared against one belief. */
export interface RecordCorrectionRequest {
	shopper_id: string;
	text: string;
	contradicts_belief_id: string;
	trace_id?: string;
}

/** What the layer will recommend next, and why. */
export interface CorrectionGuidance {
	text: string;
	basis: 'correction' | 'belief' | 'none';
	correction_id: string | null;
	belief_id: string | null;
}

/** The correction applies at once; the contradicted belief keeps standing. */
export interface RecordCorrectionResponse {
	correction: CorrectionView;
	belief_id: string;
	status: BeliefStatus;
	standing: GraphStanding;
	contradicted: true;
	guidance: CorrectionGuidance;
	trace_id: string | null;
}

/** The only tool the contest path may call. Any other tool is a type error. */
export type CorrectionToolName = 'record_correction';

/** Sends the shopper's correction to the layer. One tool, nothing else. */
export interface CorrectionTransport {
	call(tool: CorrectionToolName, request: Record<string, unknown>): Promise<unknown>;
}
