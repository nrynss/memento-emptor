// A failed pool read: an unknown tool or request, or a misshapen response.
export class MerchantApiError extends Error {
	readonly code: 'unknown-tool' | 'unknown-request' | 'bad-response';

	constructor(code: MerchantApiError['code'], message: string) {
		super(message);
		this.name = 'MerchantApiError';
		this.code = code;
	}
}
