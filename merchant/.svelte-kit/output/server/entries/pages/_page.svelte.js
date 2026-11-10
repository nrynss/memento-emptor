import { a as head, i as ensure_array_like, l as attr, r as derived, u as escape_html } from "../../chunks/index-server.js";
import { i as theme, r as reactive, t as merchantName } from "../../chunks/product.js";
import { AllCommunityModule, ModuleRegistry } from "ag-grid-community";
// src/lib/pool/errors.ts
var MerchantApiError = class extends Error {
	code;
	constructor(code, message) {
		super(message);
		this.name = "MerchantApiError";
		this.code = code;
	}
};
// src/lib/pool/decode.ts
var poolStatuses = [
	"Claimed",
	"Building",
	"Earned",
	"Out of true"
];
var fitDirections = ["small", "large"];
/** Whether a wire value is one of the two pool directions. */
function isFitDirection(value) {
	return fitDirections.includes(value);
}
/** The only fields a product_fit request may carry. Anything else fails. */
var productFitRequestFields = [
	"brand_id",
	"category",
	"fit_handle",
	"size"
];
function isRecord(value) {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}
function fail(what) {
	throw new MerchantApiError("bad-response", `product_fit response has a bad ${what}`);
}
function stringField(record, name, what) {
	const value = record[name];
	if (typeof value !== "string") fail(`${what} ${name}`);
	return value;
}
function countField(record, name, what) {
	const value = record[name];
	if (typeof value !== "number" || !Number.isInteger(value) || value < 0) fail(`${what} ${name}`);
	return value;
}
function enumField(record, name, allowed, what) {
	const value = record[name];
	if (typeof value !== "string" || !allowed.includes(value)) fail(`${what} ${name}`);
	return value;
}
function decodeLine(value) {
	if (!isRecord(value)) fail("line");
	return {
		brand_id: stringField(value, "brand_id", "line"),
		category: stringField(value, "category", "line"),
		fit_handle: stringField(value, "fit_handle", "line")
	};
}
function decodeDirectionCounts(value) {
	if (!isRecord(value)) fail("returns");
	return {
		too_small: countField(value, "too_small", "returns"),
		too_large: countField(value, "too_large", "returns")
	};
}
function decodeVerdictCounts(value) {
	if (!isRecord(value)) fail("feedback");
	return {
		fits_well: countField(value, "fits_well", "feedback"),
		too_small: countField(value, "too_small", "feedback"),
		too_large: countField(value, "too_large", "feedback")
	};
}
function decodeReasonCounts(value) {
	if (!isRecord(value)) fail("reasons");
	return {
		fit_too_small: countField(value, "fit_too_small", "reasons"),
		fit_too_large: countField(value, "fit_too_large", "reasons"),
		damaged_or_defective: countField(value, "damaged_or_defective", "reasons"),
		not_arrived: countField(value, "not_arrived", "reasons"),
		changed_mind: countField(value, "changed_mind", "reasons")
	};
}
function decodeShopperCounts(value) {
	if (!isRecord(value)) fail("shoppers");
	return {
		fit: countField(value, "fit", "shoppers"),
		too_small: countField(value, "too_small", "shoppers"),
		too_large: countField(value, "too_large", "shoppers")
	};
}
function decodeCounts(value, what) {
	if (!isRecord(value)) fail(what);
	return {
		settled: countField(value, "settled", what),
		kept: countField(value, "kept", what),
		returns: decodeDirectionCounts(value["returns"]),
		feedback: decodeVerdictCounts(value["feedback"]),
		reasons: decodeReasonCounts(value["reasons"]),
		shoppers: decodeShopperCounts(value["shoppers"])
	};
}
/** Decodes one pooled product-fit belief: counts, never identities. */
function decodePool(value) {
	if (!isRecord(value)) fail("pool");
	const direction = value["direction"];
	if (direction !== null && (typeof direction !== "string" || !isFitDirection(direction))) fail("pool direction");
	return {
		line: decodeLine(value["line"]),
		size: stringField(value, "size", "pool"),
		status: enumField(value, "status", poolStatuses, "pool"),
		direction,
		counts: decodeCounts(value["counts"], "counts"),
		seeded: decodeCounts(value["seeded"], "seeded"),
		live: decodeCounts(value["live"], "live"),
		recent: decodeCounts(value["recent"], "recent")
	};
}
/** Decodes a product_fit response: the pooled belief and its counts. */
function decodeProductFitResponse(value) {
	if (!isRecord(value)) fail("response");
	return { pool: decodePool(value["pool"]) };
}
// src/lib/pool/client.ts
/** Refuses any request that is not exactly one product_fit read. */
function checkProductFitRequest(request) {
	const fields = Object.keys(request).sort();
	const wanted = [...productFitRequestFields].sort();
	if (fields.length !== wanted.length || fields.some((field, index) => field !== wanted[index])) throw new MerchantApiError("unknown-request", "product_fit reads one line and size, nothing else");
	for (const field of wanted) if (typeof request[field] !== "string" || request[field] === "") throw new MerchantApiError("unknown-request", `product_fit needs ${field}`);
	return {
		brand_id: request["brand_id"],
		category: request["category"],
		fit_handle: request["fit_handle"],
		size: request["size"]
	};
}
/** Typed reads over any transport. Responses that miss their shape fail. */
var MerchantClient = class {
	transport;
	constructor(transport) {
		this.transport = transport;
	}
	/** Reads the pooled fit for one product line and size. */
	async productFit(request) {
		return decodeProductFitResponse(await this.transport.call("product_fit", { ...request }));
	}
};
var product_fit_response_example_default = { pool: {
	"line": {
		"brand_id": "brand-x-001",
		"category": "shirts",
		"fit_handle": "regular"
	},
	"size": "M",
	"status": "Earned",
	"direction": "small",
	"counts": {
		"settled": 60,
		"kept": 6,
		"returns": {
			"too_small": 24,
			"too_large": 4
		},
		"feedback": {
			"fits_well": 12,
			"too_small": 5,
			"too_large": 2
		},
		"reasons": {
			"fit_too_small": 24,
			"fit_too_large": 4,
			"damaged_or_defective": 2,
			"not_arrived": 1,
			"changed_mind": 4
		},
		"shoppers": {
			"fit": 39,
			"too_small": 25,
			"too_large": 6
		}
	},
	"seeded": {
		"settled": 48,
		"kept": 5,
		"returns": {
			"too_small": 20,
			"too_large": 3
		},
		"feedback": {
			"fits_well": 9,
			"too_small": 4,
			"too_large": 2
		},
		"reasons": {
			"fit_too_small": 20,
			"fit_too_large": 3,
			"damaged_or_defective": 1,
			"not_arrived": 1,
			"changed_mind": 3
		},
		"shoppers": {
			"fit": 30,
			"too_small": 19,
			"too_large": 5
		}
	},
	"live": {
		"settled": 12,
		"kept": 1,
		"returns": {
			"too_small": 4,
			"too_large": 1
		},
		"feedback": {
			"fits_well": 3,
			"too_small": 1,
			"too_large": 0
		},
		"reasons": {
			"fit_too_small": 4,
			"fit_too_large": 1,
			"damaged_or_defective": 1,
			"not_arrived": 0,
			"changed_mind": 1
		},
		"shoppers": {
			"fit": 9,
			"too_small": 6,
			"too_large": 1
		}
	},
	"recent": {
		"settled": 44,
		"kept": 5,
		"returns": {
			"too_small": 20,
			"too_large": 3
		},
		"feedback": {
			"fits_well": 8,
			"too_small": 4,
			"too_large": 1
		},
		"reasons": {
			"fit_too_small": 20,
			"fit_too_large": 3,
			"damaged_or_defective": 1,
			"not_arrived": 0,
			"changed_mind": 2
		},
		"shoppers": {
			"fit": 22,
			"too_small": 18,
			"too_large": 3
		}
	}
} };
// src/lib/pool/lines.ts
/** The fixture key for one line and size. */
function lineKey(line) {
	return `${line.brand_id}/${line.category}/${line.fit_handle}/${line.size}`;
}
// src/lib/pool/fixture.ts
/** The Earned demo oxford, exactly as the frozen snapshot serves it. */
var fixtureEarnedOxford = decodeProductFitResponse(product_fit_response_example_default);
function zeroCounts() {
	return {
		settled: 0,
		kept: 0,
		returns: {
			too_small: 0,
			too_large: 0
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 0,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 0,
			too_small: 0,
			too_large: 0
		}
	};
}
/** One hand-built fixture row: the served status matches the rule's decide. */
function makeRow(pool) {
	return decodeProductFitResponse({ pool });
}
/** The L size of the demo oxford: Building, rating large. */
var fixtureOxfordLarge = makeRow({
	line: {
		brand_id: "brand-x-001",
		category: "shirts",
		fit_handle: "regular"
	},
	size: "L",
	status: "Building",
	direction: "large",
	counts: {
		settled: 5,
		kept: 1,
		returns: {
			too_small: 0,
			too_large: 4
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 0,
			fit_too_large: 4,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 4,
			too_small: 0,
			too_large: 4
		}
	},
	seeded: {
		settled: 5,
		kept: 1,
		returns: {
			too_small: 0,
			too_large: 4
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 0,
			fit_too_large: 4,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 4,
			too_small: 0,
			too_large: 4
		}
	},
	live: zeroCounts(),
	recent: {
		settled: 5,
		kept: 1,
		returns: {
			too_small: 0,
			too_large: 4
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 0,
			fit_too_large: 4,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 4,
			too_small: 0,
			too_large: 4
		}
	}
});
/** The slim line: Building, rating small. */
var fixtureSlim = makeRow({
	line: {
		brand_id: "brand-y-002",
		category: "shirts",
		fit_handle: "slim"
	},
	size: "M",
	status: "Building",
	direction: "small",
	counts: {
		settled: 8,
		kept: 0,
		returns: {
			too_small: 6,
			too_large: 1
		},
		feedback: {
			fits_well: 1,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 6,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 7,
			too_small: 6,
			too_large: 1
		}
	},
	seeded: {
		settled: 6,
		kept: 0,
		returns: {
			too_small: 5,
			too_large: 1
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 5,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 6,
			too_small: 5,
			too_large: 1
		}
	},
	live: {
		settled: 2,
		kept: 0,
		returns: {
			too_small: 1,
			too_large: 0
		},
		feedback: {
			fits_well: 1,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 1,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 1,
			too_small: 1,
			too_large: 0
		}
	},
	recent: {
		settled: 8,
		kept: 0,
		returns: {
			too_small: 6,
			too_large: 1
		},
		feedback: {
			fits_well: 1,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 6,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 7,
			too_small: 6,
			too_large: 1
		}
	}
});
/** The second oxford line: Building, diluted by fits-well feedback. */
var fixtureSecondOxford = makeRow({
	line: {
		brand_id: "brand-z-003",
		category: "shirts",
		fit_handle: "regular"
	},
	size: "M",
	status: "Building",
	direction: "small",
	counts: {
		settled: 17,
		kept: 1,
		returns: {
			too_small: 8,
			too_large: 0
		},
		feedback: {
			fits_well: 8,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 8,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 12,
			too_small: 8,
			too_large: 0
		}
	},
	seeded: {
		settled: 15,
		kept: 1,
		returns: {
			too_small: 7,
			too_large: 0
		},
		feedback: {
			fits_well: 7,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 7,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 11,
			too_small: 7,
			too_large: 0
		}
	},
	live: {
		settled: 2,
		kept: 0,
		returns: {
			too_small: 1,
			too_large: 0
		},
		feedback: {
			fits_well: 1,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 1,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 1,
			too_small: 1,
			too_large: 0
		}
	},
	recent: {
		settled: 17,
		kept: 1,
		returns: {
			too_small: 8,
			too_large: 0
		},
		feedback: {
			fits_well: 8,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 8,
			fit_too_large: 0,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 12,
			too_small: 8,
			too_large: 0
		}
	}
});
/** The relaxed line: Claimed, below the three-shopper floor. */
var fixtureRelaxed = makeRow({
	line: {
		brand_id: "brand-v-004",
		category: "shirts",
		fit_handle: "relaxed"
	},
	size: "M",
	status: "Claimed",
	direction: null,
	counts: {
		settled: 2,
		kept: 0,
		returns: {
			too_small: 1,
			too_large: 1
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 1,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 2,
			too_small: 1,
			too_large: 1
		}
	},
	seeded: {
		settled: 2,
		kept: 0,
		returns: {
			too_small: 1,
			too_large: 1
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 1,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 2,
			too_small: 1,
			too_large: 1
		}
	},
	live: zeroCounts(),
	recent: {
		settled: 2,
		kept: 0,
		returns: {
			too_small: 1,
			too_large: 1
		},
		feedback: {
			fits_well: 0,
			too_small: 0,
			too_large: 0
		},
		reasons: {
			fit_too_small: 1,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 2,
			too_small: 1,
			too_large: 1
		}
	}
});
/** The reversed line: Out of true, Earned overall but quiet lately. */
var fixtureReversed = makeRow({
	line: {
		brand_id: "brand-w-005",
		category: "shirts",
		fit_handle: "regular"
	},
	size: "M",
	status: "Out of true",
	direction: "small",
	counts: {
		settled: 30,
		kept: 2,
		returns: {
			too_small: 20,
			too_large: 4
		},
		feedback: {
			fits_well: 4,
			too_small: 2,
			too_large: 0
		},
		reasons: {
			fit_too_small: 20,
			fit_too_large: 4,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 14,
			too_small: 10,
			too_large: 3
		}
	},
	seeded: {
		settled: 24,
		kept: 2,
		returns: {
			too_small: 17,
			too_large: 3
		},
		feedback: {
			fits_well: 3,
			too_small: 1,
			too_large: 0
		},
		reasons: {
			fit_too_small: 17,
			fit_too_large: 3,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 12,
			too_small: 9,
			too_large: 2
		}
	},
	live: {
		settled: 6,
		kept: 0,
		returns: {
			too_small: 3,
			too_large: 1
		},
		feedback: {
			fits_well: 1,
			too_small: 1,
			too_large: 0
		},
		reasons: {
			fit_too_small: 3,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 2,
			too_small: 1,
			too_large: 1
		}
	},
	recent: {
		settled: 8,
		kept: 1,
		returns: {
			too_small: 4,
			too_large: 1
		},
		feedback: {
			fits_well: 1,
			too_small: 1,
			too_large: 0
		},
		reasons: {
			fit_too_small: 4,
			fit_too_large: 1,
			damaged_or_defective: 0,
			not_arrived: 0,
			changed_mind: 0
		},
		shoppers: {
			fit: 6,
			too_small: 5,
			too_large: 1
		}
	}
});
var rowsByKey = new Map([
	fixtureEarnedOxford,
	fixtureOxfordLarge,
	fixtureSlim,
	fixtureSecondOxford,
	fixtureRelaxed,
	fixtureReversed
].map((row) => [lineKey({
	brand_id: row.pool.line.brand_id,
	category: row.pool.line.category,
	fit_handle: row.pool.line.fit_handle,
	size: row.pool.size
}), row]));
/** A transport that replays the seller's lines offline. */
function createFixtureTransport() {
	return { async call(tool, request) {
		if (tool !== "product_fit") throw new MerchantApiError("unknown-tool", "the merchant app reads product_fit only");
		const read = checkProductFitRequest(request);
		const row = rowsByKey.get(lineKey(read));
		if (!row) throw new MerchantApiError("unknown-request", "fixture has no pool for this line");
		return row;
	} };
}
/** The rule the grid's why panel shows for a Claimed pool. */
var CLAIMED_RULE = "Claimed: fewer than 3 distinct shoppers with a fit-classified outcome, so the pool says nothing yet.";
/** The rule the grid's why panel shows for a Building pool. */
var BUILDING_RULE = "Building: at least 3 distinct shoppers with a fit-classified outcome, and not yet Earned.";
/** The rule the grid's why panel shows for an Earned pool. */
var EARNED_RULE = "Earned: at least 8 distinct shoppers in one direction, at least 60% of fit-classified outcomes, and at least twice the opposite direction.";
/** The rule the grid's why panel shows for an Out-of-true pool. */
var OUT_OF_TRUE_RULE = "Out of true: Earned overall, but the last 90 days alone fail the Earned rule for that direction or meet it for the opposite one.";
/** The rule judged for a pool at one status. */
function ruleForStatus(status) {
	switch (status) {
		case "Claimed": return CLAIMED_RULE;
		case "Building": return BUILDING_RULE;
		case "Earned": return EARNED_RULE;
		case "Out of true": return OUT_OF_TRUE_RULE;
	}
}
/** The settled outcomes pointing one way on a counts view. */
function tally(counts, direction) {
	if (direction === "small") return {
		outcomes: counts.returns.too_small + counts.feedback.too_small,
		shoppers: counts.shoppers.too_small
	};
	return {
		outcomes: counts.returns.too_large + counts.feedback.too_large,
		shoppers: counts.shoppers.too_large
	};
}
/** Every settled outcome with a fit classification, whichever way it points. */
function fitOutcomes(counts) {
	return counts.returns.too_small + counts.returns.too_large + counts.feedback.too_small + counts.feedback.too_large + counts.feedback.fits_well;
}
/** Whether one direction meets the Earned rule on these counts. */
function earns(counts, direction) {
	const one = tally(counts, direction);
	const other = tally(counts, direction === "small" ? "large" : "small");
	const fit = fitOutcomes(counts);
	if (fit === 0) return false;
	return one.shoppers >= 8 && one.outcomes * 100 >= fit * 60 && one.outcomes >= 2 * other.outcomes;
}
/** Which direction leads on outcomes, or null when the two are level. */
function leading(counts) {
	const small = tally(counts, "small").outcomes;
	const large = tally(counts, "large").outcomes;
	if (small > large) return "small";
	if (large > small) return "large";
	return null;
}
/**
* Decides the pooled label from the all-time and recent counts, the same
* rule the layer reads: below the floor the pool says nothing, at the
* floor it is Building, an Earned direction holds unless the recent window
* on its own fails it.
*/
function decidePool(all, recent) {
	if (all.shoppers.fit < 3) return {
		status: "Claimed",
		direction: null
	};
	let earned = null;
	if (earns(all, "small")) earned = "small";
	else if (earns(all, "large")) earned = "large";
	if (earned === null) return {
		status: "Building",
		direction: leading(all)
	};
	if (recent.shoppers.fit >= 3) {
		if (!earns(recent, earned)) return {
			status: "Out of true",
			direction: earned
		};
	}
	return {
		status: "Earned",
		direction: earned
	};
}
/** The word the why panel shows for a direction, or an em dash for none. */
function directionLabel(direction) {
	if (direction === "small") return "runs small";
	if (direction === "large") return "runs large";
	return "—";
}
/**
* The why behind one pooled row: the counts behind its status and the rule
* it was judged by. A row opens this in the panel beside the grid.
*/
function explainWhy(pool) {
	const decided = decidePool(pool.counts, pool.recent);
	const agreement = decided.status === pool.status && decided.direction === pool.direction ? "The counts agree with the served status." : "The counts disagree with the served status: ask the layer, not the grid.";
	return [
		`${pool.line.brand_id} · ${pool.line.category} · ${pool.line.fit_handle}, size ${pool.size}: ${pool.status}${pool.direction === null ? "" : `, ${directionLabel(pool.direction)}`}.`,
		`Settled purchases: ${pool.counts.settled} (${pool.seeded.settled} seeded, ${pool.counts.settled - pool.seeded.settled} live).`,
		`Distinct shoppers with a fit-classified outcome: ${pool.counts.shoppers.fit} (${pool.counts.shoppers.too_small} small, ${pool.counts.shoppers.too_large} large).`,
		`Returns: ${pool.counts.returns.too_small} too small, ${pool.counts.returns.too_large} too large.`,
		`Reasons: fit too small ${pool.counts.reasons.fit_too_small}, fit too large ${pool.counts.reasons.fit_too_large}, damaged or defective ${pool.counts.reasons.damaged_or_defective}, not arrived ${pool.counts.reasons.not_arrived}, changed mind ${pool.counts.reasons.changed_mind}.`,
		`Feedback: fits well ${pool.counts.feedback.fits_well}, too small ${pool.counts.feedback.too_small}, too large ${pool.counts.feedback.too_large}.`,
		`Kept past the window with no feedback: ${pool.counts.kept}.`,
		ruleForStatus(pool.status),
		agreement
	];
}
// src/lib/merchant/rows.ts
/** The line words the grid shows: brand, category, cut and size. */
function lineLabelFor(pool) {
	return `${pool.line.brand_id} · ${pool.line.category} · ${pool.line.fit_handle}`;
}
/** The stable row id for one line and size. */
function rowIdFor(pool) {
	return `${pool.line.brand_id}/${pool.line.category}/${pool.line.fit_handle}/${pool.size}`;
}
/** Flattens one pooled belief for the grid, keeping every display field. */
function toMerchantGridRow(pool) {
	return {
		id: rowIdFor(pool),
		lineLabel: lineLabelFor(pool),
		size: pool.size,
		settled: pool.counts.settled,
		returnsSmall: pool.counts.returns.too_small,
		returnsLarge: pool.counts.returns.too_large,
		fitsWell: pool.counts.feedback.fits_well,
		reasonsFitSmall: pool.counts.reasons.fit_too_small,
		reasonsFitLarge: pool.counts.reasons.fit_too_large,
		reasonsDamaged: pool.counts.reasons.damaged_or_defective,
		reasonsNotArrived: pool.counts.reasons.not_arrived,
		reasonsChangedMind: pool.counts.reasons.changed_mind,
		status: pool.status,
		direction: directionLabel(pool.direction),
		seededSettled: pool.seeded.settled
	};
}
/** Flattens every pooled row for the grid, keeping the line order. */
function toMerchantGridRows(pools) {
	return pools.map(toMerchantGridRow);
}
// src/lib/merchant/grid.ts
ModuleRegistry.registerModules([AllCommunityModule]);
// src/lib/merchant/MerchantGrid.svelte
function MerchantGrid($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		let { rows, selectedId = null, onSelect } = $$props;
		reactive(theme);
		$$renderer.push(`<!---->// The merchant grid as a Svelte 5 component. The grid is created once in
// an effect and destroyed in its teardown; row updates, theme switches and
// the selected row arrive through grid options, so in-flight state survives
// a theme toggle. A row click reports its id to the page, which opens the
// why panel beside the grid. <div class="merchant-grid-host svelte-gbm54w" role="region" aria-label="Merchant pool"${attr("data-selected", selectedId ?? "")}></div>`);
	});
}
// src/routes/+page.svelte
function _page($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		new MerchantClient(createFixtureTransport());
		let pools = [];
		let selectedId = null;
		const rows = derived(() => toMerchantGridRows(pools));
		const selected = derived(() => {
			if (selectedId === null) return null;
			return pools.find((pool) => `${pool.line.brand_id}/${pool.line.category}/${pool.line.fit_handle}/${pool.size}` === selectedId) ?? null;
		});
		const why = derived(() => selected() === null ? [] : explainWhy(selected()));
		function onSelect(rowId) {
			selectedId = rowId;
		}
		head("1uha8ag", $$renderer, ($$renderer) => {
			$$renderer.title(($$renderer) => {
				$$renderer.push(`<title>${escape_html(merchantName)}</title>`);
			});
			$$renderer.push(`<meta name="description" content="The seller's read-only view of pooled fit: why products come back, by reason and size."/>`);
		});
		$$renderer.push(`<!---->// The merchant page: one read-only table over the seller's lines, and the
// why for the selected row. Every row comes from product_fit: counts and
// a status, never a personal row. Selecting a row opens the counts behind
// its status and the rule it was judged by.  <h1>Why it comes back.</h1> <p class="lede">One row per product line and size: settled purchases, returns by reason category and
	direction, fits-well feedback, the pool status, and the seeded count. Select a row to see
	the counts behind its status and the rule it was judged by.</p> `);
		$$renderer.push("<!--[-1-->");
		MerchantGrid($$renderer, {
			rows: rows(),
			selectedId,
			onSelect
		});
		$$renderer.push(`<!----> `);
		if (selected() !== null) {
			$$renderer.push(`<!--[0--><section class="why-panel" aria-label="Why this row"><h2>${escape_html(selected().line.brand_id)} · ${escape_html(selected().line.category)} · ${escape_html(selected().line.fit_handle)}, size ${escape_html(selected().size)}</h2> <ul><!--[-->`);
			const each_array = ensure_array_like(why());
			for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
				let line = each_array[$$index];
				$$renderer.push(`<li>${escape_html(line)}</li>`);
			}
			$$renderer.push(`<!--]--></ul></section>`);
		} else $$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]-->`);
		$$renderer.push(`<!--]-->`);
	});
}
export { _page as default };
