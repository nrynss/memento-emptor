<script lang="ts">
	// The shopper's correction form. Everything a first-time judge needs is
	// on the form itself: what the ledger says now, what will happen, the
	// box, the button, and the result in plain words. With a belief the edit
	// goes through the contest state's declared path; with none it stands
	// on its own through the standing path. The form never touches a belief
	// itself. Svelte 5 runes only: props in, local $state out.

	import type { ContestState } from './state';
	import type { RecordCorrectionResponse } from './types';
	import { confirmationLineFor, correctionPreviewLine } from './state';

	interface Props {
		contest: ContestState;
		beliefId?: string | null;
		currentValue?: string;
		onRecorded?: (response: RecordCorrectionResponse) => void;
	}

	let { contest, beliefId = null, currentValue, onRecorded }: Props = $props();

	let text = $state('');
	let sending = $state(false);
	let formError = $state<string | null>(null);
	let confirmed = $state<string | null>(null);

	const standing = $derived(beliefId === null);

	async function submit(event: Event): Promise<void> {
		event.preventDefault();
		if (text.trim() === '') {
			formError = 'Write the correction in your own words.';
			return;
		}
		sending = true;
		formError = null;
		confirmed = null;
		try {
			const response =
				beliefId === null
					? await contest.submitStandingCorrection(text.trim())
					: await contest.submitCorrection(beliefId, text.trim());
			confirmed = response.correction.text;
			text = '';
			onRecorded?.(response);
		} catch (error) {
			formError = error instanceof Error ? error.message : 'The correction failed.';
		} finally {
			sending = false;
		}
	}
</script>

<form class="contest-editor" onsubmit={submit}>
	<label class="contest-label" for="contest-text">
		{standing ? 'Record a standing correction' : 'Contest this belief'}
	</label>
	{#if currentValue !== undefined}
		<p class="contest-current">The ledger currently says: “{currentValue}”.</p>
	{/if}
	{#if standing}
		<p class="contest-preview" id="contest-preview">
			No belief yet: your correction stands on its own and decides at once.
		</p>
	{:else}
		<p class="contest-preview" id="contest-preview">
			{correctionPreviewLine}
		</p>
	{/if}
	<textarea
		id="contest-text"
		class="contest-input"
		bind:value={text}
		rows="2"
		aria-describedby="contest-preview"
		placeholder="I've lost weight, use M"
		disabled={sending}
	></textarea>
	{#if formError !== null}
		<p role="alert" class="contest-error">{formError}</p>
	{/if}
	{#if confirmed !== null}
		<p role="status" class="contest-done">
			{confirmationLineFor(confirmed)}
		</p>
	{/if}
	<button type="submit" class="contest-submit" disabled={sending}>
		{sending ? 'Sending…' : 'Send correction'}
	</button>
</form>

<style>
	.contest-editor {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin: 0;
		width: 100%;
		max-width: 100%;
	}

	.contest-label {
		font-weight: 600;
	}

	.contest-current,
	.contest-preview {
		margin: 0;
		color: var(--dim);
	}

	.contest-input {
		font: inherit;
		color: inherit;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.7rem;
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
		resize: vertical;
	}

	.contest-error {
		color: var(--stop);
		margin: 0;
	}

	.contest-done {
		color: var(--ok);
		margin: 0;
	}

	.contest-submit {
		align-self: start;
		font: inherit;
		color: var(--text);
		background: var(--raised);
		border: 1px solid var(--line);
		border-radius: var(--radius-control);
		padding: 0.35rem 0.9rem;
		cursor: pointer;
	}

	.contest-submit:disabled {
		cursor: wait;
		opacity: 0.7;
	}
</style>
