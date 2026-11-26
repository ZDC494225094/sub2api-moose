package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// ipSharingUserSampleLimit 每个 IP 返回的账号样本上限，避免单条风控条目过大。
const ipSharingUserSampleLimit = 5

// ListSharedIPs 聚合时间窗内被多个账号共用的来源 IP。
func (r *usageLogRepository) ListSharedIPs(
	ctx context.Context,
	startTime, endTime time.Time,
	minAccounts, limit int,
) ([]usagestats.IPSharingStat, error) {
	if minAccounts < 2 {
		minAccounts = 2
	}
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT ip_address, COUNT(DISTINCT user_id) AS user_count,
		       COUNT(*) AS request_count, MAX(created_at) AS last_seen
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
		  AND ip_address IS NOT NULL AND ip_address <> ''
		GROUP BY ip_address
		HAVING COUNT(DISTINCT user_id) >= $3
		ORDER BY user_count DESC, request_count DESC
		LIMIT $4`

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, minAccounts, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	stats := make([]usagestats.IPSharingStat, 0, limit)
	for rows.Next() {
		var item usagestats.IPSharingStat
		if err := rows.Scan(&item.IPAddress, &item.UserCount, &item.RequestCount, &item.LastSeen); err != nil {
			return nil, err
		}
		stats = append(stats, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// ListIPSharingUsers 返回命中 IP 的账号样本（按请求数降序，每个 IP 最多 5 个）。
func (r *usageLogRepository) ListIPSharingUsers(
	ctx context.Context,
	ipAddresses []string,
	startTime, endTime time.Time,
) (map[string][]usagestats.IPSharingUser, error) {
	result := make(map[string][]usagestats.IPSharingUser, len(ipAddresses))
	if len(ipAddresses) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(ipAddresses)+2)
	args = append(args, startTime, endTime)
	placeholders := make([]string, 0, len(ipAddresses))
	for _, addr := range ipAddresses {
		args = append(args, addr)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}

	query := fmt.Sprintf(`
		SELECT ul.ip_address, ul.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), COUNT(*) AS requests
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND ul.ip_address IN (%s)
		GROUP BY ul.ip_address, ul.user_id, u.email, u.username
		ORDER BY ul.ip_address ASC, requests DESC`, strings.Join(placeholders, ", "))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	for rows.Next() {
		var addr string
		var item usagestats.IPSharingUser
		if err := rows.Scan(&addr, &item.UserID, &item.Email, &item.Username, &item.Requests); err != nil {
			return nil, err
		}
		if len(result[addr]) >= ipSharingUserSampleLimit {
			continue
		}
		result[addr] = append(result[addr], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
