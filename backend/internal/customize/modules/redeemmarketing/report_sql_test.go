package redeemmarketing

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Execute the cohort SQL on isolated fixtures. Only PostgreSQL JSON/currency
// syntax is adapted; this is not a substitute for PostgreSQL acceptance.
func TestBatchCohortSQL(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
CREATE TABLE redeem_codes(id INTEGER, type TEXT, batch_id TEXT, created_at TEXT, value REAL);
CREATE TABLE redeem_code_batch_usages(batch_id TEXT, user_id INTEGER, redeem_code_id INTEGER, used_at TEXT);
CREATE TABLE custom_redeem_marketing_batches(batch_id TEXT, registered_after TEXT);
CREATE TABLE payment_orders(user_id INTEGER, created_at TEXT, paid_at TEXT, status TEXT, pay_amount REAL, refund_amount REAL, provider_snapshot TEXT);
INSERT INTO redeem_codes VALUES (1,'marketing','A','2026-10-01',10),(2,'marketing','A','2026-10-01',20),(3,'marketing','A','2026-10-01',10),(4,'marketing','B','2026-10-03',7),(5,'balance',NULL,'2026-10-03',9);
INSERT INTO redeem_code_batch_usages VALUES ('A',1,1,'2026-10-02'),('A',2,2,'2026-10-02'),('B',1,4,'2026-10-05');
INSERT INTO payment_orders VALUES
 (1,'2026-10-03','2026-10-03','COMPLETED',10,0,'{"currency":" cny "}'),
 (1,'2026-10-04','2026-10-04','PAID',20,0,NULL),
 (2,'2026-10-03','2026-10-03','RECHARGING',5,0,'{"currency":"USD"}'),
 (1,'2026-10-01','2026-10-01','COMPLETED',100,0,NULL),
 (1,'2026-10-01','2026-10-03','COMPLETED',100,0,NULL),
 (1,'2026-10-03','2026-10-03','PENDING',100,0,NULL),
 (1,'2026-10-03','2026-10-03','FAILED',100,0,NULL),
 (1,'2026-10-03','2026-10-03','REFUNDED',100,100,NULL),
 (1,'2026-10-03','2026-10-03','PARTIALLY_REFUNDED',100,1,NULL),
 (1,'2026-10-03','2026-10-03','COMPLETED',100,1,NULL),
 (1,'2026-10-03',NULL,'COMPLETED',100,0,NULL),
 (1,'2026-10-03','2026-10-03','COMPLETED',0,0,NULL),
 (9,'2026-10-03','2026-10-03','COMPLETED',100,0,NULL);
`)
	require.NoError(t, err)
	query := strings.ReplaceAll(batchQuery, "upper(btrim(o.provider_snapshot->>'currency')) ~ '^[A-Z]{3}$'", "length(trim(o.provider_snapshot->>'currency')) = 3 AND upper(trim(o.provider_snapshot->>'currency')) NOT GLOB '*[^A-Z]*'")
	query = strings.ReplaceAll(query, "btrim(", "trim(")
	query = strings.ReplaceAll(query, "jsonb_object_agg(", "json_group_object(")
	query = strings.ReplaceAll(query, "'{}'::jsonb", "'{}'")
	for _, tc := range []struct {
		batch string
		page  int
		count int
	}{{"", 1, 2}, {"A", 1, 1}, {"missing", 1, 0}, {"", 3, 0}} {
		rows, err := db.QueryContext(context.Background(), query, tc.batch, 2, (tc.page-1)*2)
		require.NoError(t, err)
		count := 0
		for rows.Next() {
			var id, created string
			var cutoff any
			var codes, users, paying, orders int64
			var value float64
			var raw string
			require.NoError(t, rows.Scan(&id, &created, &cutoff, &codes, &users, &value, &paying, &orders, &raw))
			var amounts map[string]float64
			require.NoError(t, json.Unmarshal([]byte(raw), &amounts))
			if id == "A" {
				require.Equal(t, int64(3), codes)
				require.Equal(t, int64(2), users)
				require.Equal(t, float64(30), value)
				require.Equal(t, int64(2), paying)
				require.Equal(t, int64(3), orders)
				require.Equal(t, map[string]float64{"CNY": 30, "USD": 5}, amounts)
			} else {
				require.Equal(t, "B", id)
				require.Zero(t, paying)
				require.Zero(t, orders)
				require.Empty(t, amounts)
			}
			count++
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Equal(t, tc.count, count)
	}
}
