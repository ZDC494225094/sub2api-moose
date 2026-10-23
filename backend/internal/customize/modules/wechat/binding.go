package wechat

import (
	"context"
	"time"
)

// Binding links a sub2api user to a WeChat Official Account openid.
type Binding struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	OpenID    string    `json:"openid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BindingRepository persists openid bindings.
type BindingRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*Binding, error)
	GetByOpenID(ctx context.Context, openid string) (*Binding, error)
	// Upsert binds the openid to the user, replacing any previous binding of
	// either side (one user <-> one openid).
	Upsert(ctx context.Context, userID int64, openid string) (*Binding, error)
}
