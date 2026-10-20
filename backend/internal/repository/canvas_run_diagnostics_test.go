package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCanvasRefundAndDiagnosticCommitTogether(t *testing.T) {
	for _, diagnosticFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[diagnosticFails], func(t *testing.T) {
			r, m := canvasTestRepo(t)
			c := canvasTestCharge()
			d := service.CanvasRunDiagnostics{Phase: "upstream_poll", UpstreamTaskID: "provider-task", Error: "HTTP 400 unsupported reference"}
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WithArgs(c.ID).WillReturnRows(canvasRows(c))
			write := m.ExpectExec(`UPDATE canvas_run_charges SET phase=\$2,upstream_task_id=\$3,error_message=\$4 WHERE id=\$1`).WithArgs(c.ID, d.Phase, d.UpstreamTaskID, d.Error)
			if diagnosticFails {
				write.WillReturnError(errors.New("database unavailable"))
				m.ExpectRollback()
			} else {
				write.WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(canvasMoneySQL).WithArgs(-c.ReservedCost, c.ReservedCost, c.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`UPDATE canvas_run_charges SET status=\$2,charged_cost=\$3,updated_at=NOW\(\) WHERE id=\$1`).WithArgs(c.ID, "failed", 0.0).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			got, err := r.FinishCanvasChargeWithDiagnostics(context.Background(), c.ID, false, d)
			if diagnosticFails {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, d.Error, got.ErrorMessage)
				require.Equal(t, d.UpstreamTaskID, got.UpstreamTaskID)
				require.Equal(t, "failed", got.Status)
			}
		})
	}
}

func TestCanvasExpiredLeaseRecordsInterruption(t *testing.T) {
	r, m := canvasTestRepo(t)
	c := canvasTestCharge()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 AND status='reserved' AND lease_until <= NOW\(\) FOR UPDATE`).WithArgs(c.ID).WillReturnRows(canvasRows(c))
	m.ExpectExec(canvasMoneySQL).WithArgs(-c.ReservedCost, c.ReservedCost, c.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE canvas_run_charges SET error_message=\$2 WHERE id=\$1`).WithArgs(c.ID, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE canvas_run_charges SET status=`).WithArgs(c.ID, "failed", 0.0).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	got, err := r.FailExpiredCanvasCharge(context.Background(), c.ID)
	require.NoError(t, err)
	require.Contains(t, got.ErrorMessage, "租约已过期")
}

func TestCanvasDiagnosticUpdateCannotOverwriteTerminalRun(t *testing.T) {
	r, m := canvasTestRepo(t)
	m.ExpectExec(`UPDATE canvas_run_charges SET phase=\$2,upstream_task_id=\$3,error_message=\$4,updated_at=NOW\(\) WHERE id=\$1 AND status='reserved'`).WithArgs("charge", "upstream_poll", "provider-task", "").WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, r.UpdateCanvasRunDiagnostics(context.Background(), "charge", service.CanvasRunDiagnostics{Phase: "upstream_poll", UpstreamTaskID: "provider-task"}))
}

func TestCanvasRunAdminListReadsDurableErrors(t *testing.T) {
	r, m := canvasTestRepo(t)
	now := time.Now()
	m.ExpectQuery(`SELECT COUNT\(\*\).*FROM canvas_run_charges`).WithArgs("run", "%run%", "failed").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectQuery(`SELECT c.run_id.*ORDER BY c.created_at DESC,c.id DESC LIMIT \$4 OFFSET \$5`).WithArgs("run", "%run%", "failed", 20, 20).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "email", "mode", "model", "status", "phase", "upstream_task_id", "error", "created_at", "updated_at"}).AddRow("run", 7, "user@test.com", "video", "model", "failed", "reference_upload", "", "access denied", now, now))
	got, err := r.ListCanvasRuns(context.Background(), 2, 20, "run", "failed")
	require.NoError(t, err)
	require.EqualValues(t, 1, got.Total)
	require.Len(t, got.Items, 1)
	require.Equal(t, "access denied", got.Items[0].Error)
	require.Equal(t, "user@test.com", got.Items[0].UserEmail)
}
