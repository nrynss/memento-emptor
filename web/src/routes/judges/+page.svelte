<script lang="ts">
	// The judges' page: a static guide, never a live read. Every row and every
	// walkthrough step names the page or the committed record behind it, so a
	// claim here is one link from its evidence. The comparison's round 2 has a
	// placeholder instead of results: T10.5 publishes them, and this page does
	// not restate a number before it lands.
	//
	// The repository links are absolute because this page is served from the
	// demo deployment, not from GitHub. They name the public repository and no
	// account value, key or credential: the sandbox test logins are carried by
	// the submission form, never by this page.

	import { resolve } from '$app/paths';
	import { productName } from '$lib/product';
	import { SEEDED_MARK_LABEL, SEEDED_MARK_TOOLTIP } from '$lib/ledger/rows';

	const repository = 'https://github.com/nrynss/memento-emptor';
	const repositoryFile = (path: string): string => `${repository}/blob/main/${path}`;
	const round2Run = (name: string): string => repositoryFile(`bench/round2/runs/${name}`);
	const round2Scores = round2Run('r2-scores.json');
	const round2Summary = round2Run('r2-summary.json');
	const round2Recall = round2Run('r2-recall-compare.json');
	const round2Readme = repositoryFile('bench/round2/README.md');
	const merchantApp = 'https://memento-merchant.onrender.com';
	const astropods = 'https://astropods.com';
</script>

<svelte:head>
	<title>For judges: start here · {productName}</title>
	<meta
		name="description"
		content="The shortest path through Memento Emptor: the demo, the evidence ledger, the merchant app, the traces, both comparison rounds, and what is simulated and seeded."
	/>
</svelte:head>

<h1>For judges: start here</h1>
<p class="lede" data-claim>
	Memento Emptor is a memory layer for shopping agents. A payment outcome is not a cause: the
	layer keeps the outcome, the item and the reason apart, so a parcel that never arrived never
	changes a size recommendation. This page is the shortest path through what is built, what to
	open first, and what is simulated.
	<a href={repositoryFile('product.md')} rel="external">Read the product spec</a>.
</p>

<section class="judges-section" aria-labelledby="start-heading">
	<h2 id="start-heading">The first screen</h2>

	<p class="judges-seeded-notice" data-seeded-notice>
		<span class="judges-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
		Seeded rows are generated, not lived. Every seeded row in the ledger carries this mark, and
		the <a href={merchantApp} rel="external">merchant app</a> shows how many of a pool's settled
		purchases are seeded. The <a href={resolve('/ledger')}>ledger</a> and the
		<a
			href={repositoryFile('demo/evidence/run-6/01-three-returns-and-belief.json')}
			rel="external">three seeded returns</a
		>
		are marked this way.
	</p>

	<div class="judges-table-wrap">
		<table class="judges-table" data-judges-table>
			<caption class="judges-caption">
				Where to start, what each place shows, and the record behind it.
			</caption>
			<thead>
				<tr>
					<th scope="col">Start with</th>
					<th scope="col">Open</th>
					<th scope="col">What it shows</th>
					<th scope="col">Evidence</th>
				</tr>
			</thead>
			<tbody>
				<tr data-row="demo">
					<th scope="row">Hosted demo</th>
					<td>
						<a href={resolve('/')}>Open the hosted demo</a>
					</td>
					<td>
						The served memory layer and the web app it fronts. The recorded run follows the
						demo's steps against these services.
					</td>
					<td><a href={repositoryFile('demo/RUN-6.md')} rel="external">demo/RUN-6.md</a></td>
				</tr>

				<tr data-row="ledger">
					<th scope="row">Evidence ledger</th>
					<td>
						<a href={resolve('/ledger')} data-seeded-link>Open the ledger replay</a>
						<span class="judges-seeded" data-seeded-mark title={SEEDED_MARK_TOOLTIP}
							>{SEEDED_MARK_LABEL}</span
						>
					</td>
					<td>
						The demo story at a chosen step. The page labels itself a replay, so a replay is
						never shown as live data, and every seeded row carries the mark shown beside this
						link.
					</td>
					<td>
						<a href={repositoryFile('demo/evidence/run-6/06-ledger.json')} rel="external"
							>06-ledger.json</a
						>
					</td>
				</tr>

				<tr data-row="why">
					<th scope="row">Why view</th>
					<td>
						<a href={resolve('/belief')} data-seeded-link>Open the sizing belief</a>
						<span class="judges-seeded" data-seeded-mark title={SEEDED_MARK_TOOLTIP}
							>{SEEDED_MARK_LABEL}</span
						>
					</td>
					<td>
						One belief with its evidence: the three seeded returns, the correction beside it,
						and what was deliberately not counted and why.
					</td>
					<td>
						<a
							href={repositoryFile('demo/evidence/run-6/02-belief-explained.json')}
							rel="external">02-belief-explained.json</a
						>
					</td>
				</tr>

				<tr data-row="merchant">
					<th scope="row">Merchant app</th>
					<td><a href={merchantApp} rel="external">Open the merchant app</a></td>
					<td>
						The seller's read-only view: why a product line comes back, by reason and size,
						read through <code>product_fit</code> only. Pooled counts, never a shopper row,
						with the seeded split shown.
					</td>
					<td>
						<a
							href={repositoryFile('demo/evidence/run-6/07-second-agent-and-merchant.json')}
							rel="external">07-second-agent-and-merchant.json</a
						>
					</td>
				</tr>

				<tr data-row="traces">
					<th scope="row">Traces</th>
					<td><a href={astropods} rel="external">Open the Astropods console</a></td>
					<td>
						The Astropods console holds the two deployments, <code>MEMENTO-REF</code> and
						<code>MEMENTO-SECOND</code>. The committed trace records are the public evidence,
						and sit beside each demo step's evidence.
					</td>
					<td>
						<a href={repositoryFile('demo/README.md')} rel="external">demo/README.md</a>,
						<a
							href={repositoryFile('demo/evidence/run-6/03-correction-through-the-agent.json')}
							rel="external">03-correction-through-the-agent.json</a
						>
					</td>
				</tr>

				<tr data-row="round1">
					<th scope="row">Comparison, round 1</th>
					<td>
						<a href={repositoryFile('bench/README.md')} rel="external"
							>Read the round-1 results</a
						>
					</td>
					<td>
						The round-1 counts with the limits stated first. The arm called raw history is
						published as Memento evidence without beliefs.
					</td>
					<td>
						<a href={repositoryFile('bench/runs/scores.json')} rel="external"
							>bench/runs/scores.json</a
						>
					</td>
				</tr>

				<tr data-round2-row>
					<th scope="row">Comparison, round 2</th>
					<td>
						<a href="#round2-heading">Read the measured counts</a>
					</td>
					<td>
						First-turn right size, committed: arm 3 10/16, arm 4 12/16 (hand reading 12/16
						and 16/16). Correction honoured: 4/4 next turn in every arm; 4/4 next session and
						4/4 for another agent in both Memento arms. Spend $0.789309 of $5.00.
					</td>
					<td>
						<a href={round2Scores} rel="external">r2-scores.json</a>
					</td>
				</tr>

				<tr data-row="repository">
					<th scope="row">Repository</th>
					<td><a href={repository} rel="external">Open the repository</a></td>
					<td>
						The source, the comparison runs, the demo records and the phase documents, under
						the repository's open-source licence.
					</td>
					<td><a href={repositoryFile('LICENSE')} rel="external">LICENSE</a></td>
				</tr>
			</tbody>
		</table>
	</div>

	<p class="judges-credential-note" data-credential-note>
		Test logins for the PayPal sandbox are carried by the submission form, not by this page. No
		credential value is stored in the repository or served here. The run's setup is documented in
		<a href={repositoryFile('demo/README.md')} rel="external">demo/README.md</a>.
	</p>
</section>

<section class="judges-section" aria-labelledby="walkthrough-heading">
	<h2 id="walkthrough-heading">A short walkthrough</h2>
	<ol class="judges-steps">
		<li data-claim>
			<h3>Start with the recorded run</h3>
			<p>
				The hosted services are the layer, the ledger, the merchant app and the two agent
				deployments. The run record names each demo step, its outcome and the evidence file that
				carries it.
				<a href={repositoryFile('demo/RUN-6.md')} rel="external">demo/RUN-6.md</a>
			</p>
		</li>

		<li data-claim>
			<h3>Read the ledger as a replay</h3>
			<p>
				The ledger without a shopper token replays the demo story from fixtures and says so on
				the page. Choose a story step to see the beliefs at that moment; every seeded row carries
				the seeded mark.
				<a href={resolve('/ledger')}>Evidence ledger</a>,
				<a href={repositoryFile('demo/evidence/run-6/06-ledger.json')} rel="external"
					>06-ledger.json</a
				>.
			</p>
		</li>

		<li data-claim>
			<h3>Open the sizing belief's why view</h3>
			<p>
				The why view opens the sizing belief first. Its three seeded returns are all classified
				<em>fit: too small</em> with the shopper's words; the correction sits beside the belief,
				which stays on record as <em>Out of true</em>.
				<a href={resolve('/belief')}>Why view</a>,
				<a
					href={repositoryFile('demo/evidence/run-6/02-belief-explained.json')}
					rel="external">02-belief-explained.json</a
				>.
			</p>
		</li>

		<li data-claim>
			<h3>Watch the correction arrive through the reference agent</h3>
			<p>
				Trace <code>c94448ee</code> recalls first, then calls <code>record_correction</code>; the
				layer mints <code>corr_000004</code>, and the next turn recommends M under it while the
				sizing belief stays beside the correction.
				<a
					href={repositoryFile('demo/evidence/run-6/03-correction-through-the-agent.json')}
					rel="external">03-correction-through-the-agent.json</a
				>.
			</p>
		</li>

		<li data-claim>
			<h3>Ask the second agent</h3>
			<p>
				A fresh conversation with no correction in its prompt recalls first and recommends M
				citing <code>corr_000004</code>: trace <code>9d4893e7</code>.
				<a
					href={repositoryFile('demo/evidence/run-6/07-second-agent-and-merchant.json')}
					rel="external">07-second-agent-and-merchant.json</a
				>.
			</p>
		</li>

		<li data-claim>
			<h3>Read the merchant view</h3>
			<p>
				The merchant app reads the pool through <code>product_fit</code> only, and its why panel
				names the counts and the rule behind the status. No shopper row can reach it.
				<a href={merchantApp} rel="external">Merchant app</a>,
				<a
					href={repositoryFile('demo/evidence/run-6/07-second-agent-and-merchant.json')}
					rel="external">07-second-agent-and-merchant.json</a
				>.
			</p>
		</li>

		<li data-claim>
			<h3>Read round 1 as it is published</h3>
			<p>
				Round 1 reports counts, never percentages, with its limits first: we wrote the shoppers
				and the hidden answers, so it shows the mechanism on constructed cases.
				<a href={repositoryFile('bench/README.md')} rel="external">bench/README.md</a>.
			</p>
		</li>

		<li data-claim>
			<h3>Stop at round 2</h3>
			<p>
				Round 2 has no result on this page. Its results are T10.5's to publish, with every claim
				linked to its run files.
				<a
					href={repositoryFile('dev-diary/PHASE-10-comparison-round-2.md')}
					rel="external">PHASE-10-comparison-round-2.md</a
				>.
			</p>
		</li>
	</ol>
</section>

<section class="judges-section" aria-labelledby="simulated-heading">
	<h2 id="simulated-heading">What is simulated and seeded</h2>
	<ul class="judges-facts">
		<li data-claim data-simulated-mark>
			<span class="judges-tag">Simulated</span>
			<p>
				<strong>Purchases are simulated.</strong> They run in PayPal's sandbox through our own
				merchant account. Channel3 supplies real product data and buy links, not the right to take
				payment for another retailer, so real-retailer fulfilment is simulated too.
				<a href={`${repositoryFile('product.md')}#cautions`} rel="external">product.md, Cautions</a>.
			</p>
		</li>

		<li data-claim data-seeded-mark>
			<span class="judges-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
			<p>
				<strong>The payment history is seeded.</strong> Sandbox history has to be generated.
				Every seeded row carries the mark the ledger uses, and the ledger shows it.
				<a href={resolve('/ledger')}>Evidence ledger</a>,
				<a
					href={repositoryFile('demo/evidence/run-6/01-three-returns-and-belief.json')}
					rel="external">01-three-returns-and-belief.json</a
				>.
			</p>
		</li>

		<li data-claim data-seeded-mark>
			<span class="judges-seeded" title={SEEDED_MARK_TOOLTIP}>{SEEDED_MARK_LABEL}</span>
			<p>
				<strong>The pooled population is seeded too.</strong> The shoppers behind a pooled belief
				are generated and marked like every seeded row, and the pool shows how many of its
				settled purchases are seeded. The comparison shows the mechanism on a constructed
				population, not real-world return rates.
				<a
					href={repositoryFile('demo/evidence/run-6/07-second-agent-and-merchant.json')}
					rel="external">07-second-agent-and-merchant.json</a
				>,
				<a href={repositoryFile('bench/README.md')} rel="external">bench/README.md</a>.
			</p>
		</li>

		<li data-claim>
			<span class="judges-tag">Inferred</span>
			<p>
				<strong>"Kept" is inferred.</strong> A kept item without explicit feedback is a weak
				signal, never confirmed satisfaction. The ledger marks it as a weak signal.
				<a href={resolve('/ledger')}>Evidence ledger</a>,
				<a href={`${repositoryFile('product.md')}#cautions`} rel="external">product.md, Cautions</a>.
			</p>
		</li>

		<li data-claim>
			<span class="judges-tag">Constructed</span>
			<p>
				<strong>The comparison is constructed.</strong> We wrote the shoppers and the hidden
				answers, so the comparison shows the mechanism on constructed cases, not real-world
				accuracy.
				<a href={repositoryFile('bench/README.md')} rel="external">bench/README.md, Limits</a>.
			</p>
		</li>
	</ul>
</section>

<section class="judges-section judges-round2" data-round2 aria-labelledby="round2-heading">
	<h2 id="round2-heading">Comparison, round 2: measured counts</h2>
	<p class="lede" data-claim>
		T10.5's results. Eight constructed shoppers, four cases, four arms, two repeats. We wrote the
		shoppers, the hidden answers and the population, so this shows the mechanism on constructed
		cases, not real-world accuracy. Counts, never percentages; every count links to its run file or
		the round-2 record, read from
		<a href={round2Scores} rel="external">r2-scores.json</a> and
		<a href={round2Readme} rel="external">bench/round2/README.md</a>.
	</p>

	<p data-claim>
		<strong>Extraction artifact, before the counts.</strong> Six Memento replies open with the
		recommendation but later name another size — the size that came back or the size the pool
		reports — so the frozen size rule reads that later size. The committed scores give arm 3 10/16
		first turns and arm 4 12/16. A hand reading of the same 64 replies, kept beside the scores and not
		substituted for them, gives arm 3 12/16 and arm 4 16/16. No score is overridden. The six runs are
		named in
		<a href={`${round2Readme}#the-extraction-artifact-before-any-count`} rel="external"
			>the round-2 record</a
		>, and the committed scores are in
		<a href={round2Scores} rel="external">r2-scores.json</a>.
	</p>

	<h3>Right size first time</h3>
	<div class="judges-table-wrap">
		<table class="judges-table judges-round2-table" data-round2-first>
			<caption class="judges-caption">
				Committed scores: the extracted size equals the hidden answer. Each cell is 4 turns.
				<a href={round2Scores} rel="external">r2-scores.json</a>
			</caption>
			<thead>
				<tr>
					<th scope="col">Case</th>
					<th scope="col">Arm 1 no memory</th>
					<th scope="col">Arm 2 PayPal history</th>
					<th scope="col">Arm 3 Memento shopper only</th>
					<th scope="col">Arm 4 Memento full</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<th scope="row">New to the brand</th>
					<td>0/4</td>
					<td>0/4</td>
					<td>0/4</td>
					<td>3/4</td>
				</tr>
				<tr>
					<th scope="row">Unlike the crowd</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>2/4</td>
				</tr>
				<tr>
					<th scope="row">Noisy refunds</th>
					<td>0/4</td>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Correction, first turn</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>3/4</td>
				</tr>
				<tr>
					<th scope="row">All four cases</th>
					<td>0/16</td>
					<td>4/16</td>
					<td>10/16</td>
					<td>12/16</td>
				</tr>
			</tbody>
		</table>
	</div>

	<div class="judges-table-wrap">
		<table class="judges-table judges-round2-table" data-round2-hand>
			<caption class="judges-caption">
				Hand reading of the same 64 replies, not the measurement; no score is changed.
			</caption>
			<thead>
				<tr>
					<th scope="col">Case</th>
					<th scope="col">Arm 1</th>
					<th scope="col">Arm 2</th>
					<th scope="col">Arm 3</th>
					<th scope="col">Arm 4</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<th scope="row">New to the brand</th>
					<td>0/4</td>
					<td>0/4</td>
					<td>0/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Unlike the crowd</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Noisy refunds</th>
					<td>0/4</td>
					<td>0/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Correction, first turn</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">All four cases</th>
					<td>0/16</td>
					<td>4/16</td>
					<td>12/16</td>
					<td>16/16</td>
				</tr>
			</tbody>
		</table>
	</div>

	<p data-claim>
		The reading moves four cells: arm 3's noisy refunds 2/4 becomes 4/4, and arm 4's new-to-brand
		3/4, unlike-the-crowd 2/4 and correction 3/4 each become 4/4. Arm 3's new-to-brand stays 0/4
		under either reading.
		<a href={`${round2Readme}#right-size-first-time`} rel="external">The round-2 record</a> names
		the six runs.
	</p>

	<h3>Corrections</h3>
	<p data-claim>
		Case 4 only: two shoppers, one correction each, recorded through
		<code>record_correction</code> after the first turn and before every later turn. Counts from
		<a href={round2Scores} rel="external">r2-scores.json</a>.
	</p>
	<div class="judges-table-wrap">
		<table class="judges-table judges-round2-table" data-round2-corrections>
			<caption class="judges-caption">
				Correction honoured, by turn and arm. Each cell is 4 turns.
				<a href={round2Scores} rel="external">r2-scores.json</a>
			</caption>
			<thead>
				<tr>
					<th scope="col">Turn</th>
					<th scope="col">Arm 1 no memory</th>
					<th scope="col">Arm 2 PayPal history</th>
					<th scope="col">Arm 3 shopper only</th>
					<th scope="col">Arm 4 full</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<th scope="row">Next turn, same conversation</th>
					<td>4/4</td>
					<td>4/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Next session, no conversation</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">Another agent, no conversation</th>
					<td>0/4</td>
					<td>2/4</td>
					<td>4/4</td>
					<td>4/4</td>
				</tr>
				<tr>
					<th scope="row">All three later turns</th>
					<td>4/12</td>
					<td>8/12</td>
					<td>12/12</td>
					<td>12/12</td>
				</tr>
			</tbody>
		</table>
	</div>
	<p data-claim>
		The correction turn is 4/4 in every arm: its prompt carries the correction in the
		conversation, so it measures the conversation, not memory. The belief stays on record beside
		the correction in both Memento arms.
		<a href={round2Run('r2-corrections.json')} rel="external">r2-corrections.json</a>.
	</p>

	<p data-claim>
		<strong>Spend.</strong> $0.789309 of the $5.00 cap: arm 1 $0.031134, arm 2 $0.460746, arm 3
		$0.137073, arm 4 $0.160356.
		<a href={round2Summary} rel="external">r2-summary.json</a>.
	</p>

	<p data-claim>
		<strong>Recall difference:</strong> The live recall differed from the frozen
		<code>recall_for_purchase</code> response in 28 of 56 Memento turns — in belief status and
		standing labels, contradiction marks, and which of the rival's returns the belief carries —
		while the answers and the deciding precedence step agreed.
		<a href={round2Recall} rel="external">r2-recall-compare.json</a>.
	</p>

	<h3>Headline candidates (the user picks one — none is chosen here)</h3>
	<ol class="judges-candidates">
		<li data-round2-candidate data-claim>
			<strong>The correction crosses sessions and agents for both Memento arms.</strong> 4/4 in
			the next session and 4/4 for another agent, against 2/4 and 2/4 for PayPal history and 0/4
			and 0/4 for no memory.
			<a href={round2Scores} rel="external">r2-scores.json</a>.
		</li>
		<li data-round2-candidate data-claim>
			<strong>The pool answers the new-to-brand case that no shopper's history can.</strong> Arm 4
			3/4 by the committed scores and 4/4 by hand, against 0/4 for Memento shopper-only, 0/4 for
			PayPal history and 0/4 for no memory.
			<a href={round2Run('r2-a4-new-plain-r1-first.json')} rel="external"
				>r2-a4-new-plain-r1-first.json</a
			>,
			<a href={round2Scores} rel="external">r2-scores.json</a>.
		</li>
		<li data-round2-candidate data-claim>
			<strong>Own evidence outranks the pool when the pool points the wrong way.</strong> In both
			Memento arms, 8/8 by hand and 6/8 by the committed scores; arm 2 is 2/4 and arm 1 is 0/4.
			<a href={round2Run('r2-a4-crowd-rival-r1-first.json')} rel="external"
				>r2-a4-crowd-rival-r1-first.json</a
			>,
			<a href={round2Scores} rel="external">r2-scores.json</a>.
		</li>
		<li data-round2-candidate data-claim>
			<strong>PayPal history costs more and carries the larger prompt.</strong> Arm 2 spends
			$0.460746 and carries 28197 words; arm 4 spends $0.160356 and carries 11263 words. First-turn
			right size is 4/16 against 12/16.
			<a href={round2Summary} rel="external">r2-summary.json</a>,
			<a href={round2Scores} rel="external">r2-scores.json</a>.
		</li>
	</ol>
</section>

<style>
	.judges-section {
		margin: 0 0 2.5rem;
	}

	.judges-section h2 {
		margin: 0 0 1rem;
		font-size: 1.35rem;
	}

	.judges-section h3 {
		margin: 0 0 0.35rem;
		font-size: 1.05rem;
	}

	.judges-table-wrap {
		overflow-x: auto;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
	}

	.judges-table {
		width: 100%;
		min-width: 52rem;
		border-collapse: collapse;
	}

	.judges-caption {
		padding: 0.9rem 1rem 0;
		color: var(--dim);
		font-size: 0.875rem;
		text-align: left;
	}

	.judges-table th,
	.judges-table td {
		padding: 0.8rem 1rem;
		border-bottom: 1px solid var(--line);
		text-align: left;
		vertical-align: top;
	}

	.judges-table thead th {
		color: var(--dim);
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}

	.judges-table tbody th {
		width: 12rem;
		font-weight: 600;
	}

	.judges-table tbody tr:last-child th,
	.judges-table tbody tr:last-child td {
		border-bottom: 0;
	}

	.judges-table code,
	.judges-steps code {
		font-family: var(--font-numeric);
		font-size: 0.9em;
	}

	.judges-table a,
	.judges-steps a,
	.judges-seeded-notice a,
	.judges-facts a,
	.judges-credential-note a,
	.judges-round2 a {
		color: var(--accent);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.judges-seeded-notice {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin: 0 0 1rem;
		padding: 0.6rem 0.9rem;
		border: 1px dashed var(--dim);
		border-radius: var(--radius-panel);
		color: var(--dim);
	}

	.judges-seeded {
		display: inline-block;
		padding: 0.05rem 0.5rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px dashed var(--dim);
		color: var(--dim);
		white-space: nowrap;
	}

	.judges-tag {
		display: inline-block;
		padding: 0.05rem 0.5rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px solid var(--line);
		background: var(--raised);
		color: var(--dim);
		white-space: nowrap;
	}

	.judges-credential-note {
		max-width: 46rem;
		margin: 1rem 0 0;
		color: var(--dim);
		font-size: 0.9375rem;
	}

	.judges-steps {
		max-width: 46rem;
		margin: 0;
		padding-left: 1.25rem;
	}

	.judges-steps li {
		margin: 0 0 1.1rem;
	}

	.judges-steps p {
		margin: 0;
	}

	.judges-facts {
		display: grid;
		gap: 0.9rem;
		max-width: 52rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.judges-facts li {
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
	}

	.judges-facts p {
		margin: 0.35rem 0 0;
	}

	.judges-round2 {
		padding: 1.25rem;
		border: 1px solid var(--line);
		border-radius: var(--radius-panel);
		background: var(--surface);
	}

	.judges-round2 h2 {
		color: var(--text);
	}

	.judges-round2 h3 {
		margin: 1.5rem 0 0.75rem;
	}

	.judges-round2 .judges-table-wrap {
		margin: 0 0 1rem;
	}

	.judges-candidates {
		display: grid;
		gap: 0.85rem;
		margin: 0;
		padding-left: 1.25rem;
	}

	@media (max-width: 720px) {
		.judges-table {
			min-width: 44rem;
		}
	}
</style>
