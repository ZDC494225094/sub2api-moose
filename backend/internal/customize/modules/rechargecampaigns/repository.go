package rechargecampaigns

import (
	"context"
	"database/sql"
	"encoding/json"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Queryer is satisfied by sql.DB and by the host's Ent client/transaction.
// This adapter changes neither historical migration names nor table layout.
type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type SQLRepository struct{ db Queryer }

func NewSQLRepository(db Queryer) *SQLRepository { return &SQLRepository{db: db} }

func (r *SQLRepository) List(ctx context.Context) ([]Campaign, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, config FROM recharge_campaigns ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Campaign{}
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var a Campaign
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		a.ID = id
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *SQLRepository) Save(ctx context.Context, a Campaign) (*Campaign, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	query := "INSERT INTO recharge_campaigns(config) VALUES($1) RETURNING id"
	args := []any{string(raw)}
	if a.ID > 0 {
		query = "UPDATE recharge_campaigns SET config=$1,updated_at=NOW() WHERE id=$2 RETURNING id"
		args = append(args, a.ID)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, infraerrors.NotFound("CAMPAIGN_NOT_FOUND", "活动不存在")
	}
	if err := rows.Scan(&a.ID); err != nil {
		return nil, err
	}
	// Exhaust RETURNING before checking the driver's terminal row error.
	if rows.Next() {
		return nil, infraerrors.ServiceUnavailable("CAMPAIGN_WRITE_FAILED", "活动写入结果异常")
	}
	return &a, rows.Err()
}
