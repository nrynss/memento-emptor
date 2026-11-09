package fakes

import (
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// assembleFinal builds the replay's final ledger state: the four belief
// rows, the five evidence rows, and the one pending question left open.
// The content matches the frozen contract snapshots, so P4 renders the
// replay with the same code that renders the contract examples.
func assembleFinal() FinalState {
	fit := domain.BeliefKindFit
	seller := domain.BeliefKindSeller

	fitReturn := func(id, sku, order, capture, refund, custom, event, at, text string) contract.EvidenceView {
		return contract.EvidenceView{
			EvidenceID: id,
			Outcome:    contract.OutcomeView{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
			Item: contract.ItemView{
				BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
				SKU: sku, Size: "M", Category: "shirts",
			},
			Reason:       &contract.ReasonView{Category: domain.CategoryFitTooSmall, Text: text, Confidence: 1.0},
			CountsToward: &fit,
			WriteOp:      domain.WriteRecordAction,
			Link: contract.TransactionLinkView{
				OrderID: order, CaptureID: capture, RefundID: refund,
				CustomID: custom, EventID: event,
			},
			Seeded:     true,
			OccurredAt: at,
		}
	}
	r1 := fitReturn("ev_6d1a09", "tbnrl46", "ORD-7XK-01", "CAP-7XK-01", "RFD-7XK-01", "memento-shopper01-01", "WH-2EV-01", "2026-10-14T10:00:00Z", "too tight in the shoulders")
	r2 := fitReturn("ev_6d1a10", "tbnrl46", "ORD-7XK-02", "CAP-7XK-02", "RFD-7XK-02", "memento-shopper01-02", "WH-2EV-02", "2026-10-20T10:00:00Z", "too tight across the chest")
	r3 := fitReturn("ev_6d1a11", "tbnrl46", "ORD-7XK-03", "CAP-7XK-03", "RFD-7XK-03", "memento-shopper01-03", "WH-2EV-03", "2026-10-26T10:00:00Z", "sleeves too short in M")

	correction := contract.CorrectionView{
		CorrectionID: "corr_3b7e52", Text: "I've lost weight, use M",
		ContradictsBeliefID: "blf_8f2c41", RecordedAt: "2026-11-05T14:02:11Z",
		TakesPrecedence: true,
	}
	sizing := contract.BeliefView{
		BeliefID: "blf_8f2c41", Kind: domain.BeliefKindFit,
		Subject:        domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
		Value:          "runs small, take size L",
		Status:         domain.StatusOutOfTrue,
		Standing:       domain.GraphVenerable,
		Contradicted:   true,
		ContradictedBy: &correction,
		Evidence:       []contract.EvidenceView{r1, r2, r3},
		Seeded:         true,
		UpdatedAt:      "2026-11-05T14:02:11Z",
	}

	feedbackEvidence := contract.EvidenceView{
		EvidenceID: "ev_51c2d8",
		Outcome:    contract.OutcomeView{Kind: domain.OutcomeCapture, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
		Item: contract.ItemView{
			BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
			SKU: "PSW8GeK", Size: "M", Category: "shirts",
		},
		Reason:       &contract.ReasonView{Category: domain.CategoryNone, Text: "the M fits well across the shoulders", Confidence: 1.0},
		CountsToward: &fit,
		WriteOp:      domain.WriteRecordAction,
		Link: contract.TransactionLinkView{
			OrderID: "ORD-7XK-06", CaptureID: "CAP-7XK-06",
			CustomID: "memento-shopper01-06", EventID: "WH-2EV-06",
		},
		Seeded:     false,
		OccurredAt: "2026-11-08T20:10:00Z",
	}
	rival := contract.BeliefView{
		BeliefID: "blf_2b9d60", Kind: domain.BeliefKindFit,
		Subject:        domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
		Value:          "size M fits this shopper now",
		Status:         domain.StatusClaimed,
		Standing:       domain.GraphNone,
		Contradicted:   false,
		ContradictedBy: nil,
		Evidence:       []contract.EvidenceView{feedbackEvidence},
		Seeded:         false,
		UpdatedAt:      "2026-11-08T20:10:00Z",
	}

	disputeEvidence := contract.EvidenceView{
		EvidenceID: "ev_9d4c22",
		Outcome:    contract.OutcomeView{Kind: domain.OutcomeDispute, Status: "OPEN", AmountMinor: 8900, Currency: "USD"},
		Item: contract.ItemView{
			BrandID: "brand-x-001", BrandName: "Brand X", LineName: "chino",
			SKU: "chino-khaki-32", Size: "32", Category: "pants",
		},
		Reason:       &contract.ReasonView{Category: domain.CategoryNotArrived, Text: "MERCHANDISE_OR_SERVICE_NOT_RECEIVED", Confidence: 1.0},
		CountsToward: &seller,
		WriteOp:      domain.WriteRecordAction,
		Link: contract.TransactionLinkView{
			OrderID: "ORD-7XK-04", CaptureID: "CAP-7XK-04", DisputeID: "PP-D-51713",
			CustomID: "memento-shopper01-04", EventID: "WH-2EV-04",
		},
		Seeded:     false,
		OccurredAt: "2026-11-06T11:30:00Z",
	}
	sellerBelief := contract.BeliefView{
		BeliefID: "blf_1a5e09", Kind: domain.BeliefKindSeller,
		Subject:        domain.Subject{Kind: domain.SubjectSeller, ID: "seller-acme-01", Name: "Acme"},
		Value:          "ships reliably for this shopper",
		Status:         domain.StatusClaimed,
		Standing:       domain.GraphNone,
		Contradicted:   false,
		ContradictedBy: nil,
		Evidence:       []contract.EvidenceView{disputeEvidence},
		Seeded:         false,
		UpdatedAt:      "2026-11-06T11:30:00Z",
	}

	statement := contract.BeliefView{
		BeliefID: "blf_77c310", Kind: domain.BeliefKindFit,
		Subject:        domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
		Value:          "shopper says they are usually size M",
		Status:         domain.StatusClaimed,
		Standing:       domain.GraphNone,
		Contradicted:   false,
		ContradictedBy: nil,
		Evidence:       []contract.EvidenceView{},
		Seeded:         true,
		UpdatedAt:      "2026-10-14T10:00:00Z",
	}

	lastAsked := "2026-11-08T18:40:00Z"
	pending := contract.PendingQuestionsResponse{
		ShopperID: "shopper_01",
		Questions: []contract.QuestionView{
			{
				QuestionID: "q_9f04ab",
				Outcome:    contract.OutcomeView{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
				Item: contract.ItemView{
					BrandID: "brand-x-001", BrandName: "Brand X", LineName: "oxford shirt",
					SKU: "tbnrl46", Size: "M", Category: "shirts",
				},
				Link: contract.TransactionLinkView{
					OrderID: "ORD-7XK-05", CaptureID: "CAP-7XK-05", RefundID: "RFD-7XK-05",
					CustomID: "memento-shopper01-05", EventID: "WH-2EV-05",
				},
				Seeded:        false,
				AskedCount:    1,
				LastAskedAt:   &lastAsked,
				AwaitingSince: "2026-11-07T09:15:00Z",
			},
		},
	}

	return FinalState{
		Beliefs: []contract.LedgerBeliefRow{
			{Belief: sizing, EvidenceCount: 3, LastChangeAt: "2026-11-05T14:02:11Z"},
			{Belief: rival, EvidenceCount: 1, LastChangeAt: "2026-11-08T20:10:00Z"},
			{Belief: sellerBelief, EvidenceCount: 1, LastChangeAt: "2026-11-06T11:30:00Z"},
			{Belief: statement, EvidenceCount: 0, LastChangeAt: "2026-10-14T10:00:00Z"},
		},
		Evidence: []contract.LedgerEvidenceRow{
			{Evidence: feedbackEvidence, BeliefIDs: []string{"blf_2b9d60"}},
			{Evidence: disputeEvidence, BeliefIDs: []string{"blf_1a5e09"}},
			{Evidence: r3, BeliefIDs: []string{"blf_8f2c41"}},
			{Evidence: r2, BeliefIDs: []string{"blf_8f2c41"}},
			{Evidence: r1, BeliefIDs: []string{"blf_8f2c41"}},
		},
		Pending: pending,
	}
}
