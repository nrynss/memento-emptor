// A typed client for the shopper's correction path. The client takes a
// transport so the ledger page uses one call shape in both modes: the
// fixture transport replays the demo correction offline, and a later task
// can add a transport that calls the MCP surface without touching call
// sites. This is the only place contest code touches the transport, and
// the transport type only allows the correction tool.
import { decodeRecordCorrectionResponse } from './decode';
import type {
	CorrectionTransport,
	RecordCorrectionRequest,
	RecordCorrectionResponse
} from './types';

/** The shopper's edits, sent through record_correction and nothing else. */
export class CorrectionClient {
	private readonly transport: CorrectionTransport;

	constructor(transport: CorrectionTransport) {
		this.transport = transport;
	}

	/** Records the shopper's correction against the belief it contradicts. */
	async recordCorrection(request: RecordCorrectionRequest): Promise<RecordCorrectionResponse> {
		const raw = await this.transport.call('record_correction', { ...request });
		return decodeRecordCorrectionResponse(raw);
	}
}
