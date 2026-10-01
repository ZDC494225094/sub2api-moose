package multigroupbilling

func ContainsGroupID(primary *int64, groupIDs []int64, groupID int64) bool {
	for _, id := range NormalizeGroupIDs(primary, groupIDs) {
		if id == groupID {
			return true
		}
	}
	return false
}

func RemoveGroupID(primary *int64, groupIDs []int64, groupID int64) []int64 {
	normalized := NormalizeGroupIDs(primary, groupIDs)
	out := normalized[:0]
	for _, id := range normalized {
		if id != groupID {
			out = append(out, id)
		}
	}
	return append([]int64(nil), out...)
}

func ReplaceGroupID(primary *int64, groupIDs []int64, oldGroupID, newGroupID int64) []int64 {
	normalized := NormalizeGroupIDs(primary, groupIDs)
	seen := make(map[int64]struct{}, len(normalized))
	out := make([]int64, 0, len(normalized))
	for _, id := range normalized {
		if id == oldGroupID {
			id = newGroupID
		}
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
