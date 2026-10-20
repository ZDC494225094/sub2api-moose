package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageBillingRepository) UpdateCanvasRunDiagnostics(ctx context.Context, id string, d service.CanvasRunDiagnostics) error {
	// Late producers must not overwrite a terminal refund or its recovery reason.
	_, err := r.db.ExecContext(ctx, `UPDATE canvas_run_charges SET phase=$2,upstream_task_id=$3,error_message=$4,updated_at=NOW() WHERE id=$1 AND status='reserved'`, id, d.Phase, d.UpstreamTaskID, d.Error)
	return err
}

func (r *usageBillingRepository) ListCanvasRuns(ctx context.Context, page, size int, search, status string) (*service.CanvasRunPage, error) {
	result := &service.CanvasRunPage{Items: []service.CanvasRunRecord{}}
	filter := ` FROM canvas_run_charges c LEFT JOIN users u ON u.id=c.user_id
        WHERE ($1='' OR u.email ILIKE $2 OR c.run_id ILIKE $2 OR c.model ILIKE $2 OR c.upstream_task_id ILIKE $2 OR c.user_id::text=$1)
        AND ($3='' OR c.status=$3)`
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+filter, search, "%"+search+"%", status).Scan(&result.Total); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT c.run_id,c.user_id,COALESCE(u.email,''),c.mode,c.model,c.status,c.phase,c.upstream_task_id,c.error_message,c.created_at,c.updated_at`+filter+` ORDER BY c.created_at DESC,c.id DESC LIMIT $4 OFFSET $5`, search, "%"+search+"%", status, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.CanvasRunRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Mode, &item.Model, &item.Status, &item.Phase, &item.UpstreamTaskID, &item.Error, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
