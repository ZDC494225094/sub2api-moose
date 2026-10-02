package adminefficiency

import "errors"

type BatchUserActionSkipped struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason"`
}
type BatchUserActionResult struct {
	Affected int                      `json:"affected"`
	Skipped  []BatchUserActionSkipped `json:"skipped"`
}

func NormalizeBatchUserIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, errors.New("user_ids is required")
	}
	if len(ids) > 500 {
		return nil, errors.New("user_ids cannot exceed 500")
	}
	result := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("user_ids must contain positive integers")
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result, nil
}

// The host callback retains per-user authorization, admin protection and cleanup.
func RunBatchUserAction(ids []int64, action func(int64) error) BatchUserActionResult {
	result := BatchUserActionResult{Skipped: make([]BatchUserActionSkipped, 0)}
	for _, id := range ids {
		if err := action(id); err != nil {
			result.Skipped = append(result.Skipped, BatchUserActionSkipped{UserID: id, Reason: err.Error()})
		} else {
			result.Affected++
		}
	}
	return result
}

// NormalizeGroupAccountIDs keeps request order (call priority), ignoring invalid IDs.
func NormalizeGroupAccountIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
