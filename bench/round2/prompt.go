package round2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nrynss/memento-emptor/seed"
)

// The frozen prompt template. Every arm gets the same base and differs
// only in its memory block, so a reading of the results cannot be an
// artefact of how the question was asked.

// BaseSection builds the part of the prompt every arm shares: who the
// shopper is, the candidate, and the task. It carries no memory at all, so
// it is byte-identical for the four arms of one shopper.
func BaseSection(shopper seed.Round2Shopper) string {
	var out strings.Builder
	out.WriteString("Shopper: " + shopper.ShopperID + "\n")
	out.WriteString("Candidate: a new " + shopper.Candidate.LineName + " from " + shopper.Candidate.BrandName)
	out.WriteString(" (new SKU, size unchosen). The shopper wants another one.\n")
	out.WriteString("Task: recommend one size (a single token such as M or L)")
	out.WriteString(" and one sentence of justification citing only what you were given.")
	out.WriteString(" Do not invent purchases, reasons, or corrections.")
	return out.String()
}

// MemoryFetch is one arm's memory, as the driver obtained it: the toolkit
// extraction for arm 2 and the raw recall_for_purchase response for arms 3
// and 4. Arm 1 fetches nothing.
type MemoryFetch struct {
	// Tool is the call that fetched it: none, paypal_history, or
	// recall_for_purchase.
	Tool string
	// Response is the fetch's payload verbatim.
	Response []byte
}

// MemorySection builds the part of the prompt that differs by arm, from
// the memory the arm's fetch returned. Arms 3 and 4 are the same recall
// response put through ProjectRecall, so the only difference between them
// is the pool.
func MemorySection(arm Arm, fetch MemoryFetch) (string, error) {
	switch arm {
	case ArmNoMemory:
		return "Memory: none. No past purchases were given for this recommendation. Recommend from the candidate alone.", nil
	case ArmPayPalHistory:
		if fetch.Tool != arm.memoryTool() || len(fetch.Response) == 0 {
			return "", fmt.Errorf("round2: arm %s needs a %s fetch", arm, arm.memoryTool())
		}
		return "Memory: PayPal history. Exactly what the shopper's PayPal account returns — transactions, " +
			"orders with line items, refunds and disputes — and nothing the memory layer produced:\n" +
			string(fetch.Response), nil
	case ArmMementoShopperOnly, ArmMementoFull:
		if fetch.Tool != arm.memoryTool() || len(fetch.Response) == 0 {
			return "", fmt.Errorf("round2: arm %s needs a %s fetch", arm, arm.memoryTool())
		}
		memory, err := ProjectRecall(fetch.Response, arm)
		if err != nil {
			return "", err
		}
		if arm == ArmMementoShopperOnly {
			return "Memory: Memento layer, shopper only. The shopper's own beliefs, evidence, labels and " +
				"corrections, with no pooled product fit in this condition:\n" + string(memory), nil
		}
		return "Memory: Memento layer. The shopper's own beliefs, evidence, labels and corrections, and the " +
			"pooled product fit, each with the precedence step it carries:\n" + string(memory), nil
	default:
		return "", fmt.Errorf("%w: %d", ErrUnknownArm, int(arm))
	}
}

// PromptFor builds the full prompt for one shopper through one arm: the
// shared base plus the arm's memory block. The memory is fetched by
// FetchMemory.
func PromptFor(shopper seed.Round2Shopper, arm Arm, population seed.Population) (string, MemoryFetch, error) {
	fetch, err := FetchMemory(shopper, arm, population)
	if err != nil {
		return "", MemoryFetch{}, err
	}
	memory, err := MemorySection(arm, fetch)
	if err != nil {
		return "", MemoryFetch{}, err
	}
	return BaseSection(shopper) + "\n" + memory, fetch, nil
}

// FetchMemory obtains one arm's memory from the frozen inputs: nothing for
// arm 1, the toolkit extraction for arm 2, and the expected recall
// response for arms 3 and 4. The live driver replaces the last one with
// the layer's own answer; the prompt rendering is the same either way.
func FetchMemory(shopper seed.Round2Shopper, arm Arm, population seed.Population) (MemoryFetch, error) {
	switch arm {
	case ArmNoMemory:
		return MemoryFetch{Tool: arm.memoryTool()}, nil
	case ArmPayPalHistory:
		input, err := ArmTwoInput(shopper)
		if err != nil {
			return MemoryFetch{}, err
		}
		return MemoryFetch{Tool: arm.memoryTool(), Response: input}, nil
	case ArmMementoShopperOnly, ArmMementoFull:
		recall, err := ExpectedRecall(shopper, population)
		if err != nil {
			return MemoryFetch{}, err
		}
		return MemoryFetch{Tool: arm.memoryTool(), Response: recall}, nil
	default:
		return MemoryFetch{}, fmt.Errorf("%w: %d", ErrUnknownArm, int(arm))
	}
}

// ProjectRecall turns one recall_for_purchase response into a Memento
// arm's memory block. Both Memento arms get the same projection, and the
// only difference between them is the pool:
//
//   - the guidance block is dropped for both: it is the layer's own
//     recommendation in prose, and leaving it in would hand both arms the
//     layer's answer instead of the memory it rests on;
//   - arm 3's pool is set to null, which is that arm's condition: the
//     shopper's own evidence, labels and corrections with no pool;
//   - everything else — beliefs, their evidence, their precedence, any
//     overriding correction and the pending count — is carried unchanged.
func ProjectRecall(raw []byte, arm Arm) ([]byte, error) {
	if arm != ArmMementoShopperOnly && arm != ArmMementoFull {
		return nil, fmt.Errorf("%w: %s carries no recall", ErrUnknownArm, arm)
	}
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("round2: decode recall response: %w", err)
	}
	delete(response, "guidance")
	if arm == ArmMementoShopperOnly {
		response["pool"] = nil
	}
	encoded, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("round2: encode recall memory: %w", err)
	}
	return encoded, nil
}
