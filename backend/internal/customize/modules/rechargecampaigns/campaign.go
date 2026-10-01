// Package rechargecampaigns owns recharge activity catalog/configuration. It has
// no dependency on the shared payment service or generated Ent schema.
package rechargecampaigns

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"math"
	"strings"
	"time"
)

// Percent is a gift percentage for bonus, and the payable percentage for discount (90 = 九折).
type Campaign struct {
	Revision        string    `json:"revision"`
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Enabled         bool      `json:"enabled"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	Kind            string    `json:"kind"`
	Percent         float64   `json:"percent"`
	MinAmount       float64   `json:"min_amount"`
	RewardPercent   float64   `json:"reward_percent"`
	RewardCap       float64   `json:"reward_cap"`
	FreezeHours     int       `json:"freeze_hours"`
	NewInviteesOnly bool      `json:"new_invitees_only"`
}

func Revision(a Campaign) string {
	a.ID = 0
	a.Revision = ""
	raw, _ := json.Marshal(a)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (a Campaign) Validate() error {
	if strings.TrimSpace(a.Name) == "" || len([]rune(a.Name)) > 60 || len([]rune(a.Description)) > 500 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "活动名称须为1–60字，说明不超过500字")
	}
	if a.StartsAt.IsZero() || !a.EndsAt.After(a.StartsAt) {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "请选择有效的开始和结束时间；限时活动也需设置持续时间")
	}
	for _, v := range []float64{a.Percent, a.MinAmount, a.RewardPercent, a.RewardCap} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return infraerrors.BadRequest("INVALID_CAMPAIGN", "金额和比例必须是有限非负数")
		}
	}
	if (a.Kind != "bonus" && a.Kind != "discount") || a.Percent <= 0 || a.Percent > 100 || (a.Kind == "discount" && a.Percent >= 100) || a.MinAmount > 1000000 || a.RewardPercent > 100 || a.RewardCap > 1000000 || a.FreezeHours < 0 || a.FreezeHours > 8760 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "活动比例、奖励或冻结时长超出范围")
	}
	if a.RewardPercent > 0 && a.RewardCap <= 0 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "开启邀请奖励时必须设置每单奖励上限")
	}
	return nil
}
func (a Campaign) Active(now time.Time) bool {
	return a.Enabled && !now.Before(a.StartsAt) && now.Before(a.EndsAt)
}
