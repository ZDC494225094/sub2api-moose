package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ClearGroupIDByGroupID 将指定分组的所有 API Key 的 group_id 设为 nil
func (r *apiKeyRepository) ClearGroupIDByGroupID(ctx context.Context, groupID int64) (int64, error) {
	if r.sql != nil && dbent.TxFromContext(ctx) == nil {
		res, err := r.sql.ExecContext(ctx, multigroupbilling.ClearGroupBindingsSQL, groupID, fmt.Sprintf("[%d]", groupID))
		if err != nil {
			return 0, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		return affected, nil
	}

	return r.clearGroupIDByGroupIDWithEnt(ctx, groupID)
}

// UpdateGroupIDByUserAndGroup 将用户下绑定 oldGroupID 的所有 Key 迁移到 newGroupID
func (r *apiKeyRepository) UpdateGroupIDByUserAndGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	if dbent.TxFromContext(ctx) != nil || r.sql == nil {
		return r.updateGroupIDByUserAndGroupWithEnt(ctx, userID, oldGroupID, newGroupID)
	}
	if r.sql != nil {
		res, err := r.sql.ExecContext(ctx, multigroupbilling.ReplaceGroupBindingsSQL, userID, oldGroupID, newGroupID, fmt.Sprintf("[%d]", oldGroupID), service.PlatformAnthropic)
		if err != nil {
			return 0, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		return affected, nil
	}

	return r.updateGroupIDByUserAndGroupWithEnt(ctx, userID, oldGroupID, newGroupID)
}

func (r *apiKeyRepository) clearGroupIDByGroupIDWithEnt(ctx context.Context, groupID int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	keys, err := client.APIKey.Query().
		Where(apikey.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	var affected int64
	for _, key := range keys {
		if !multigroupbilling.ContainsGroupID(key.GroupID, key.GroupIds, groupID) {
			continue
		}
		remaining := multigroupbilling.RemoveGroupID(key.GroupID, key.GroupIds, groupID)
		updater := client.APIKey.Update().
			Where(apikey.IDEQ(key.ID), apikey.DeletedAtIsNil()).
			SetGroupIds(remaining).
			SetUpdatedAt(now)
		if key.GroupID != nil && *key.GroupID == groupID {
			updater.ClearGroupID()
		}
		n, err := updater.Save(ctx)
		if err != nil {
			return affected, err
		}
		affected += int64(n)
	}
	return affected, nil
}

func (r *apiKeyRepository) updateGroupIDByUserAndGroupWithEnt(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	newGroup, err := client.Group.Query().
		Where(group.IDEQ(newGroupID), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, service.ErrGroupNotFound
		}
		return 0, err
	}
	platform := service.DefaultAPIKeyPlatform(newGroup.Platform)
	keys, err := client.APIKey.Query().
		Where(apikey.UserIDEQ(userID), apikey.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	var affected int64
	for _, key := range keys {
		if !multigroupbilling.ContainsGroupID(key.GroupID, key.GroupIds, oldGroupID) {
			continue
		}
		groupID := key.GroupID
		if groupID != nil && *groupID == oldGroupID {
			gid := newGroupID
			groupID = &gid
		}
		groupIDs := multigroupbilling.ReplaceGroupID(key.GroupID, key.GroupIds, oldGroupID, newGroupID)
		updater := client.APIKey.Update().
			Where(apikey.IDEQ(key.ID), apikey.DeletedAtIsNil()).
			SetPlatform(platform).
			SetGroupIds(groupIDs).
			SetUpdatedAt(now)
		if groupID != nil {
			updater.SetGroupID(*groupID)
		} else {
			updater.ClearGroupID()
		}
		n, err := updater.Save(ctx)
		if err != nil {
			return affected, err
		}
		affected += int64(n)
	}
	return affected, nil
}
