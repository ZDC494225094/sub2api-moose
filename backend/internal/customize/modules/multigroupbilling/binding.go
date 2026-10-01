package multigroupbilling

// Binding is a detached view of the persisted binding, not an auth-time selection.
type Binding struct {
	Primary  *int64
	GroupIDs []int64
	Platform string
}

// BindingPatch preserves transport field presence. An absent list is not a clear.
type BindingPatch struct {
	Primary     *int64
	GroupIDs    []int64
	GroupIDsSet bool
	Platform    *string
}

// BindingPlan contains proposed data only. The host must validate all candidate
// groups and permissions before applying it, and persist only the declared fields.
type BindingPlan struct {
	Binding
	Changed     bool
	WriteGroups bool
}

func PlanCreate(primary *int64, ids []int64, platform string) Binding {
	ids = NormalizeGroupIDs(primary, ids)
	if primary == nil && len(ids) > 0 {
		id := ids[0]
		primary = &id
	}
	return Binding{Primary: cloneID(primary), GroupIDs: ids, Platform: platform}
}

func PlanUpdate(current Binding, patch BindingPatch) BindingPlan {
	changed := patch.Primary != nil || patch.GroupIDsSet || patch.Platform != nil
	plan := BindingPlan{Changed: changed}
	if !changed {
		return plan
	}
	plan.Binding = Binding{Primary: cloneID(current.Primary), GroupIDs: NormalizeGroupIDs(current.Primary, current.GroupIDs), Platform: current.Platform}
	plan.WriteGroups = patch.Primary != nil || patch.GroupIDsSet
	if plan.WriteGroups {
		plan.GroupIDs = NormalizeGroupIDs(patch.Primary, patch.GroupIDs)
		if patch.Platform == nil {
			plan.Platform = ""
		}
		if patch.Primary != nil {
			plan.Primary = cloneID(patch.Primary)
		} else if len(plan.GroupIDs) > 0 {
			id := plan.GroupIDs[0]
			plan.Primary = &id
		} else {
			plan.Primary = nil
		}
	}
	if patch.Platform != nil {
		plan.Platform = *patch.Platform
	}
	return plan
}

func cloneID(id *int64) *int64 {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}
