package adminefficiency

import "context"

// GroupAccountPorts delegates native membership safety and atomic persistence to
// the host while the extension owns replace-list orchestration and ID ordering.
type GroupAccountPorts[T any] struct {
	Validate func(context.Context, int64, []int64) error
	Replace  func(context.Context, int64, []int64) error
	List     func(context.Context, int64) ([]T, error)
}

func ReplaceGroupAccounts[T any](ctx context.Context, states StateReader, groupID int64, ids []int64, ports GroupAccountPorts[T]) ([]T, error) {
	if err := CheckWrite(ctx, states); err != nil {
		return nil, err
	}
	ids = NormalizeGroupAccountIDs(ids)
	if err := ports.Validate(ctx, groupID, ids); err != nil {
		return nil, err
	}
	if err := ports.Replace(ctx, groupID, ids); err != nil {
		return nil, err
	}
	return ports.List(ctx, groupID)
}
