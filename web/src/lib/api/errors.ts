// A failed read: an unknown tool or request, or a misshapen response.
export class LedgerApiError extends Error {
	readonly code: 'unknown-tool' | 'unknown-request' | 'bad-response';

	constructor(code: LedgerApiError['code'], message: string) {
		super(message);
		this.name = 'LedgerApiError';
		this.code = code;
	}
}
