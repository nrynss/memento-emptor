// A failed read: an unknown tool or request, a misshapen response, or a
// live call the edge refused or could not answer. A view branches on the
// code, never on the message.
export class LedgerApiError extends Error {
	readonly code: 'unknown-tool' | 'unknown-request' | 'bad-response' | 'refused' | 'unreachable';

	/** The HTTP status the edge answered with, when one arrived. */
	readonly status: number | null;

	constructor(code: LedgerApiError['code'], message: string, status: number | null = null) {
		super(message);
		this.name = 'LedgerApiError';
		this.code = code;
		this.status = status;
	}
}
