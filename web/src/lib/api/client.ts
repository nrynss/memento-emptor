// A typed client for the ledger's read endpoints. The client takes a
// transport so the ledger page uses one call shape in both modes: the
// fixture transport replays the demo story offline, and a later task can
// add a transport that calls the MCP surface without touching call sites.

import {
	decodeLedgerBeliefResponse,
	decodeLedgerBeliefsResponse,
	decodeLedgerEvidenceResponse,
	decodePendingQuestionsResponse
} from './decode';
import type {
	LedgerBeliefRequest,
	LedgerBeliefResponse,
	LedgerBeliefsRequest,
	LedgerBeliefsResponse,
	LedgerEvidenceRequest,
	LedgerEvidenceResponse,
	LedgerToolName,
	PendingQuestionsRequest,
	PendingQuestionsResponse
} from './types';

/** Sends one read to the layer. The response shape is checked by the client. */
export interface LedgerTransport {
	call(tool: LedgerToolName, request: Record<string, unknown>): Promise<unknown>;
}

/** Typed reads over any transport. Responses that miss their shape fail. */
export class LedgerClient {
	private readonly transport: LedgerTransport;

	constructor(transport: LedgerTransport) {
		this.transport = transport;
	}

	/** Lists the ledger's belief rows, strongest first. */
	async listBeliefs(request: LedgerBeliefsRequest): Promise<LedgerBeliefsResponse> {
		const raw = await this.transport.call('ledger_beliefs', { ...request });
		return decodeLedgerBeliefsResponse(raw);
	}

	/** Reads one ledger row in full, with its corrections and seeded notice. */
	async getBelief(request: LedgerBeliefRequest): Promise<LedgerBeliefResponse> {
		const raw = await this.transport.call('ledger_belief', { ...request });
		return decodeLedgerBeliefResponse(raw);
	}

	/** Lists the ledger's evidence rows, newest first. */
	async listEvidence(request: LedgerEvidenceRequest): Promise<LedgerEvidenceResponse> {
		const raw = await this.transport.call('ledger_evidence', { ...request });
		return decodeLedgerEvidenceResponse(raw);
	}

	/** Lists the refunds still missing a reason, oldest first. */
	async listPendingQuestions(request: PendingQuestionsRequest): Promise<PendingQuestionsResponse> {
		const raw = await this.transport.call('pending_questions', { ...request });
		return decodePendingQuestionsResponse(raw);
	}
}
