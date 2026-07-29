package service

import "testing"

func TestBuildOperationsCreditSummaryIncludesPurchaseAndRemainingBreakdowns(t *testing.T) {
	stats := &OperationsFunnelStats{
		TotalRechargeAmount:        175.25,
		BalanceRechargeAmount:      120,
		SubscriptionRechargeAmount: 55.25,
		RemainingBalance:           34.5,
		BalanceRechargeRemaining:   25,
		SubscriptionRemaining:      18.75,
		GiftedAmount:               20,
		GiftedRemaining:            9.5,
	}

	got := buildOperationsCreditSummary(stats)

	if got.TotalRechargeAmount != 175.25 || got.BalanceRechargeAmount != 120 || got.SubscriptionRechargeAmount != 55.25 {
		t.Fatalf("unexpected purchase summary: %#v", got)
	}
	if got.TotalRemainingAmount != 53.25 {
		t.Fatalf("expected total remaining 53.25, got %.2f", got.TotalRemainingAmount)
	}
	if got.BalanceRechargeRemaining != 25 || got.SubscriptionRemaining != 18.75 || got.GiftedRemaining != 9.5 {
		t.Fatalf("unexpected remaining breakdown: %#v", got)
	}
	if got.RemainingBalance != 34.5 || got.GiftedAmount != 20 {
		t.Fatalf("expected legacy credit fields to remain available: %#v", got)
	}
}
