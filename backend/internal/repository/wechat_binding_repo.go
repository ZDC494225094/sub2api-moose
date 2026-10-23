package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/wechat"
)

type wechatBindingRepository struct{ db *sql.DB }

// NewWechatBindingRepository builds the openid binding store.
func NewWechatBindingRepository(db *sql.DB) wechat.BindingRepository {
	return &wechatBindingRepository{db: db}
}

func (r *wechatBindingRepository) GetByUserID(ctx context.Context, userID int64) (*wechat.Binding, error) {
	return r.getBy(ctx, "user_id = $1", userID)
}

func (r *wechatBindingRepository) GetByOpenID(ctx context.Context, openid string) (*wechat.Binding, error) {
	return r.getBy(ctx, "openid = $1", openid)
}

func (r *wechatBindingRepository) getBy(ctx context.Context, predicate string, arg any) (*wechat.Binding, error) {
	b := &wechat.Binding{}
	err := r.db.QueryRowContext(ctx, `
SELECT id, user_id, openid, created_at, updated_at
FROM wechat_oa_bindings
WHERE `+predicate+` LIMIT 1`, arg).Scan(&b.ID, &b.UserID, &b.OpenID, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Upsert binds the openid to the user, replacing any previous binding on
// either side so the relation stays one-to-one.
func (r *wechatBindingRepository) Upsert(ctx context.Context, userID int64, openid string) (*wechat.Binding, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM wechat_oa_bindings WHERE user_id = $1 OR openid = $2`, userID, openid); err != nil {
		return nil, err
	}
	b := &wechat.Binding{UserID: userID, OpenID: openid}
	if err := tx.QueryRowContext(ctx, `
INSERT INTO wechat_oa_bindings (user_id, openid) VALUES ($1, $2)
RETURNING id, created_at, updated_at`, userID, openid).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}
