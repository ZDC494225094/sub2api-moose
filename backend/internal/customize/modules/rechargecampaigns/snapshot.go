package rechargecampaigns

import (
	"encoding/json"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SnapshotKey = "recharge_campaign"

// Snapshot keeps the historical persisted JSON shape and exact field order.
// It contains rules/amounts only, never payment-provider credentials.
type Snapshot struct {
	Campaign  Campaign `json:"campaign"`
	Principal float64  `json:"principal"`
	Credited  float64  `json:"credited"`
	InviterID int64    `json:"inviter_id"`
	Reward    float64  `json:"reward"`
}

// ReadSnapshot never reads activation settings or mutable campaign configuration.
func ReadSnapshot(providerSnapshot map[string]any) (*Snapshot, error) {
	v, ok := providerSnapshot[SnapshotKey]
	if !ok {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err = json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("invalid campaign snapshot: %w", err)
	}
	return &snap, nil
}

// AttachSnapshot checks eligibility at order creation, not payment completion.
// It preserves the provider fields and the native order timeout. An order placed
// near the deadline can still be paid after the campaign ends.
func AttachSnapshot(providerSnapshot map[string]any, snap *Snapshot, now time.Time) (map[string]any, error) {
	if snap == nil {
		return providerSnapshot, nil
	}
	if !snap.Campaign.Active(now) {
		return nil, infraerrors.BadRequest("CAMPAIGN_UNAVAILABLE", "活动已结束，请刷新后重试")
	}
	if providerSnapshot == nil {
		providerSnapshot = map[string]any{}
	}
	providerSnapshot[SnapshotKey] = snap
	return providerSnapshot, nil
}
