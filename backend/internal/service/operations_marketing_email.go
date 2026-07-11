package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/mail"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	marketingEmailDefaultActiveDays = 30
	marketingEmailDefaultLimit      = 50
	marketingEmailMaxLimit          = 100
	marketingEmailSampleLimit       = 10
)

type OperationsMarketingEmailService struct {
	entClient    *dbent.Client
	emailService *EmailService
	sqlDB        *sql.DB
}

func NewOperationsMarketingEmailService(entClient *dbent.Client, emailService *EmailService, sqlDB *sql.DB) *OperationsMarketingEmailService {
	return &OperationsMarketingEmailService{
		entClient:    entClient,
		emailService: emailService,
		sqlDB:        sqlDB,
	}
}

type OperationsMarketingEmailRequest struct {
	Subject           string   `json:"subject"`
	Body              string   `json:"body"`
	BodyFormat        string   `json:"body_format"`
	Audience          string   `json:"audience"`
	Status            string   `json:"status"`
	ActiveDays        int      `json:"active_days"`
	MinBalance        *float64 `json:"min_balance"`
	MaxBalance        *float64 `json:"max_balance"`
	MinTotalRecharged *float64 `json:"min_total_recharged"`
	MaxTotalRecharged *float64 `json:"max_total_recharged"`
	UserIDs           []int64  `json:"user_ids"`
	Limit             int      `json:"limit"`
	DryRun            bool     `json:"dry_run"`
	Confirm           bool     `json:"confirm"`
}

type OperationsMarketingRecipientQuery struct {
	Keyword           string
	Audience          string
	Status            string
	ActiveDays        int
	MinBalance        *float64
	MaxBalance        *float64
	MinTotalRecharged *float64
	MaxTotalRecharged *float64
	Page              int
	PageSize          int
}

type OperationsMarketingEmailRecipient struct {
	UserID         int64      `json:"user_id"`
	Email          string     `json:"email"`
	UserName       string     `json:"user_name,omitempty"`
	Status         string     `json:"status"`
	Balance        float64    `json:"balance"`
	TotalRecharged float64    `json:"total_recharged"`
	LastActiveAt   *time.Time `json:"last_active_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type OperationsMarketingEmailResult struct {
	DryRun         bool                                `json:"dry_run"`
	TotalMatched   int                                 `json:"total_matched"`
	Targeted       int                                 `json:"targeted"`
	Sent           int                                 `json:"sent"`
	Failed         int                                 `json:"failed"`
	SkippedInvalid int                                 `json:"skipped_invalid"`
	Limit          int                                 `json:"limit"`
	Sample         []OperationsMarketingEmailRecipient `json:"sample,omitempty"`
	Errors         []string                            `json:"errors,omitempty"`
}

type OperationsMarketingEmailRecord struct {
	ID                int64                               `json:"id"`
	Subject           string                              `json:"subject"`
	BodyFormat        string                              `json:"body_format"`
	BodyPreview       string                              `json:"body_preview"`
	Audience          string                              `json:"audience"`
	Status            string                              `json:"status"`
	ActiveDays        int                                 `json:"active_days"`
	MinBalance        *float64                            `json:"min_balance,omitempty"`
	MaxBalance        *float64                            `json:"max_balance,omitempty"`
	MinTotalRecharged *float64                            `json:"min_total_recharged,omitempty"`
	MaxTotalRecharged *float64                            `json:"max_total_recharged,omitempty"`
	SelectedUserCount int                                 `json:"selected_user_count"`
	TotalMatched      int                                 `json:"total_matched"`
	Targeted          int                                 `json:"targeted"`
	Sent              int                                 `json:"sent"`
	Failed            int                                 `json:"failed"`
	SkippedInvalid    int                                 `json:"skipped_invalid"`
	Errors            []string                            `json:"errors,omitempty"`
	Sample            []OperationsMarketingEmailRecipient `json:"sample,omitempty"`
	CreatedAt         time.Time                           `json:"created_at"`
}

func (s *OperationsMarketingEmailService) Send(ctx context.Context, req OperationsMarketingEmailRequest) (*OperationsMarketingEmailResult, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("OPERATIONS_EMAIL_NOT_READY", "operations marketing email service is not configured")
	}
	normalized, err := normalizeOperationsMarketingEmailRequest(req)
	if err != nil {
		return nil, err
	}

	query, err := s.buildMarketingRecipientQuery(normalized)
	if err != nil {
		return nil, err
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count marketing email recipients: %w", err)
	}
	users, err := query.
		Order(dbent.Asc(dbuser.FieldID)).
		Limit(normalized.Limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list marketing email recipients: %w", err)
	}

	result := &OperationsMarketingEmailResult{
		DryRun:       normalized.DryRun,
		TotalMatched: total,
		Limit:        normalized.Limit,
		Sample:       make([]OperationsMarketingEmailRecipient, 0, marketingEmailSampleLimit),
	}
	recipients := make([]OperationsMarketingEmailRecipient, 0, len(users))
	for _, user := range users {
		recipient, ok := marketingRecipientFromUser(user)
		if !ok {
			result.SkippedInvalid++
			continue
		}
		recipients = append(recipients, recipient)
		if len(result.Sample) < marketingEmailSampleLimit {
			result.Sample = append(result.Sample, recipient)
		}
	}
	result.Targeted = len(recipients)
	if normalized.DryRun {
		return result, nil
	}
	if !normalized.Confirm {
		return nil, infraerrors.BadRequest("MARKETING_EMAIL_CONFIRM_REQUIRED", "confirm is required before sending marketing emails")
	}
	if s.emailService == nil {
		return nil, ErrEmailNotConfigured
	}
	smtpConfig, err := s.emailService.GetSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}

	body := buildMarketingEmailBody(normalized.Body, normalized.BodyFormat)
	for _, recipient := range recipients {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if err := s.emailService.SendEmailWithConfig(smtpConfig, recipient.Email, normalized.Subject, body); err != nil {
			result.Failed++
			if len(result.Errors) < marketingEmailSampleLimit {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", recipient.Email, err.Error()))
			}
			continue
		}
		result.Sent++
	}
	if err := s.insertMarketingEmailRecord(ctx, normalized, result); err != nil {
		if len(result.Errors) < marketingEmailSampleLimit {
			result.Errors = append(result.Errors, fmt.Sprintf("record: %s", err.Error()))
		}
	}
	return result, nil
}

func (s *OperationsMarketingEmailService) ListRecipients(ctx context.Context, query OperationsMarketingRecipientQuery) ([]OperationsMarketingEmailRecipient, int64, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, infraerrors.ServiceUnavailable("OPERATIONS_EMAIL_NOT_READY", "operations marketing email service is not configured")
	}
	query = normalizeOperationsMarketingRecipientQuery(query)
	req := OperationsMarketingEmailRequest{
		Audience:          query.Audience,
		Status:            query.Status,
		ActiveDays:        query.ActiveDays,
		MinBalance:        query.MinBalance,
		MaxBalance:        query.MaxBalance,
		MinTotalRecharged: query.MinTotalRecharged,
		MaxTotalRecharged: query.MaxTotalRecharged,
		Limit:             query.PageSize,
		DryRun:            true,
	}
	normalized, err := normalizeOperationsMarketingRecipientRequest(req)
	if err != nil {
		return nil, 0, err
	}
	q, err := s.buildMarketingRecipientQuery(normalized)
	if err != nil {
		return nil, 0, err
	}
	if query.Keyword != "" {
		q = q.Where(dbuser.Or(
			dbuser.EmailContainsFold(query.Keyword),
			dbuser.UsernameContainsFold(query.Keyword),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count marketing recipients: %w", err)
	}
	users, err := q.
		Order(dbent.Desc(dbuser.FieldTotalRecharged), dbent.Desc(dbuser.FieldID)).
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list marketing recipients: %w", err)
	}
	recipients := make([]OperationsMarketingEmailRecipient, 0, len(users))
	for _, user := range users {
		recipient, ok := marketingRecipientFromUser(user)
		if ok {
			recipients = append(recipients, recipient)
		}
	}
	return recipients, int64(total), nil
}

func (s *OperationsMarketingEmailService) ListRecords(ctx context.Context, page, pageSize int) ([]OperationsMarketingEmailRecord, int64, error) {
	if s == nil || s.sqlDB == nil {
		return nil, 0, infraerrors.ServiceUnavailable("OPERATIONS_EMAIL_RECORDS_NOT_READY", "operations marketing email records are not configured")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	if err := s.sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM operations_marketing_email_records`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count marketing email records: %w", err)
	}
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT
			id,
			subject,
			body_format,
			body_preview,
			audience,
			status_filter,
			active_days,
			min_balance,
			max_balance,
			min_total_recharged,
			max_total_recharged,
			selected_user_count,
			total_matched,
			targeted,
			sent,
			failed,
			skipped_invalid,
			errors,
			sample,
			created_at
		FROM operations_marketing_email_records
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list marketing email records: %w", err)
	}
	defer rows.Close()

	records := make([]OperationsMarketingEmailRecord, 0, pageSize)
	for rows.Next() {
		var record OperationsMarketingEmailRecord
		var minBalance, maxBalance, minTotal, maxTotal sql.NullFloat64
		var errorsJSON, sampleJSON []byte
		if err := rows.Scan(
			&record.ID,
			&record.Subject,
			&record.BodyFormat,
			&record.BodyPreview,
			&record.Audience,
			&record.Status,
			&record.ActiveDays,
			&minBalance,
			&maxBalance,
			&minTotal,
			&maxTotal,
			&record.SelectedUserCount,
			&record.TotalMatched,
			&record.Targeted,
			&record.Sent,
			&record.Failed,
			&record.SkippedInvalid,
			&errorsJSON,
			&sampleJSON,
			&record.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		record.MinBalance = nullableFloat64Ptr(minBalance)
		record.MaxBalance = nullableFloat64Ptr(maxBalance)
		record.MinTotalRecharged = nullableFloat64Ptr(minTotal)
		record.MaxTotalRecharged = nullableFloat64Ptr(maxTotal)
		_ = json.Unmarshal(errorsJSON, &record.Errors)
		_ = json.Unmarshal(sampleJSON, &record.Sample)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func normalizeOperationsMarketingEmailRequest(req OperationsMarketingEmailRequest) (OperationsMarketingEmailRequest, error) {
	req.Subject = strings.TrimSpace(req.Subject)
	req.Body = strings.TrimSpace(req.Body)
	req, err := normalizeOperationsMarketingRecipientRequest(req)
	if err != nil {
		return req, err
	}
	if req.Subject == "" || len([]rune(req.Subject)) > 120 {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_SUBJECT_INVALID", "subject is required and must be 120 characters or fewer")
	}
	if req.Body == "" || len([]rune(req.Body)) > 10000 {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_BODY_INVALID", "body is required and must be 10000 characters or fewer")
	}
	return req, nil
}

func normalizeOperationsMarketingRecipientRequest(req OperationsMarketingEmailRequest) (OperationsMarketingEmailRequest, error) {
	req.BodyFormat = strings.ToLower(strings.TrimSpace(req.BodyFormat))
	req.Audience = strings.ToLower(strings.TrimSpace(req.Audience))
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.BodyFormat == "" {
		req.BodyFormat = "plain"
	}
	if req.Audience == "" {
		req.Audience = "all"
	}
	if req.Status == "" {
		req.Status = StatusActive
	}
	if req.ActiveDays <= 0 {
		req.ActiveDays = marketingEmailDefaultActiveDays
	}
	req.UserIDs = normalizeMarketingUserIDs(req.UserIDs)
	if req.Limit <= 0 {
		if len(req.UserIDs) > 0 {
			req.Limit = len(req.UserIDs)
		} else {
			req.Limit = marketingEmailDefaultLimit
		}
	}
	if req.Limit > marketingEmailMaxLimit {
		req.Limit = marketingEmailMaxLimit
	}
	if len(req.UserIDs) > marketingEmailMaxLimit {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_USER_LIMIT_EXCEEDED", "selected users must be 100 or fewer")
	}

	if req.BodyFormat != "plain" && req.BodyFormat != "html" {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_BODY_FORMAT_INVALID", "body_format must be plain or html")
	}
	switch req.Audience {
	case "all", "active", "inactive":
	default:
		return req, infraerrors.BadRequest("MARKETING_EMAIL_AUDIENCE_INVALID", "audience must be all, active, or inactive")
	}
	switch req.Status {
	case "all", StatusActive, StatusDisabled:
	default:
		return req, infraerrors.BadRequest("MARKETING_EMAIL_STATUS_INVALID", "status must be all, active, or disabled")
	}
	if req.MinBalance != nil && req.MaxBalance != nil && *req.MinBalance > *req.MaxBalance {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_BALANCE_RANGE_INVALID", "min_balance must be less than or equal to max_balance")
	}
	if req.MinTotalRecharged != nil && req.MaxTotalRecharged != nil && *req.MinTotalRecharged > *req.MaxTotalRecharged {
		return req, infraerrors.BadRequest("MARKETING_EMAIL_RECHARGE_RANGE_INVALID", "min_total_recharged must be less than or equal to max_total_recharged")
	}
	return req, nil
}

func (s *OperationsMarketingEmailService) buildMarketingRecipientQuery(req OperationsMarketingEmailRequest) (*dbent.UserQuery, error) {
	q := s.entClient.User.Query().
		Where(
			dbuser.EmailNEQ(""),
			dbuser.DeletedAtIsNil(),
			dbuser.Not(dbuser.EmailHasSuffix(LinuxDoConnectSyntheticEmailDomain)),
			dbuser.Not(dbuser.EmailHasSuffix(OIDCConnectSyntheticEmailDomain)),
			dbuser.Not(dbuser.EmailHasSuffix(WeChatConnectSyntheticEmailDomain)),
			dbuser.Not(dbuser.EmailHasSuffix(DingTalkConnectSyntheticEmailDomain)),
		)
	if req.Status != "all" {
		q = q.Where(dbuser.StatusEQ(req.Status))
	}
	if req.MinBalance != nil {
		q = q.Where(dbuser.BalanceGTE(*req.MinBalance))
	}
	if req.MaxBalance != nil {
		q = q.Where(dbuser.BalanceLTE(*req.MaxBalance))
	}
	if req.MinTotalRecharged != nil {
		q = q.Where(dbuser.TotalRechargedGTE(*req.MinTotalRecharged))
	}
	if req.MaxTotalRecharged != nil {
		q = q.Where(dbuser.TotalRechargedLTE(*req.MaxTotalRecharged))
	}
	if len(req.UserIDs) > 0 {
		q = q.Where(dbuser.IDIn(req.UserIDs...))
	}
	cutoff := time.Now().AddDate(0, 0, -req.ActiveDays)
	switch req.Audience {
	case "active":
		q = q.Where(dbuser.LastActiveAtGTE(cutoff))
	case "inactive":
		q = q.Where(dbuser.Or(
			dbuser.LastActiveAtIsNil(),
			dbuser.LastActiveAtLT(cutoff),
		))
	}
	return q, nil
}

func marketingRecipientFromUser(user *dbent.User) (OperationsMarketingEmailRecipient, bool) {
	if user == nil || !isDeliverableMarketingEmail(user.Email) {
		return OperationsMarketingEmailRecipient{}, false
	}
	return OperationsMarketingEmailRecipient{
		UserID:         user.ID,
		Email:          strings.TrimSpace(user.Email),
		UserName:       strings.TrimSpace(user.Username),
		Status:         strings.TrimSpace(user.Status),
		Balance:        user.Balance,
		TotalRecharged: user.TotalRecharged,
		LastActiveAt:   user.LastActiveAt,
		CreatedAt:      user.CreatedAt,
	}, true
}

func normalizeOperationsMarketingRecipientQuery(query OperationsMarketingRecipientQuery) OperationsMarketingRecipientQuery {
	query.Keyword = strings.TrimSpace(query.Keyword)
	query.Audience = strings.ToLower(strings.TrimSpace(query.Audience))
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Audience == "" {
		query.Audience = "all"
	}
	if query.Status == "" {
		query.Status = StatusActive
	}
	if query.ActiveDays <= 0 {
		query.ActiveDays = marketingEmailDefaultActiveDays
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > marketingEmailMaxLimit {
		query.PageSize = marketingEmailMaxLimit
	}
	return query
}

func normalizeMarketingUserIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *OperationsMarketingEmailService) insertMarketingEmailRecord(ctx context.Context, req OperationsMarketingEmailRequest, result *OperationsMarketingEmailResult) error {
	if s == nil || s.sqlDB == nil || result == nil || result.DryRun {
		return nil
	}
	errorsJSON, err := json.Marshal(result.Errors)
	if err != nil {
		return err
	}
	sampleJSON, err := json.Marshal(result.Sample)
	if err != nil {
		return err
	}
	_, err = s.sqlDB.ExecContext(ctx, `
		INSERT INTO operations_marketing_email_records (
			subject,
			body_format,
			body_preview,
			body_hash,
			audience,
			status_filter,
			active_days,
			min_balance,
			max_balance,
			min_total_recharged,
			max_total_recharged,
			selected_user_count,
			total_matched,
			targeted,
			sent,
			failed,
			skipped_invalid,
			errors,
			sample
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18::jsonb, $19::jsonb
		)
	`,
		req.Subject,
		req.BodyFormat,
		marketingBodyPreview(req.Body),
		marketingBodyHash(req.Body),
		req.Audience,
		req.Status,
		req.ActiveDays,
		nullableFloat64Value(req.MinBalance),
		nullableFloat64Value(req.MaxBalance),
		nullableFloat64Value(req.MinTotalRecharged),
		nullableFloat64Value(req.MaxTotalRecharged),
		len(req.UserIDs),
		result.TotalMatched,
		result.Targeted,
		result.Sent,
		result.Failed,
		result.SkippedInvalid,
		string(errorsJSON),
		string(sampleJSON),
	)
	if err != nil {
		return fmt.Errorf("insert marketing email record: %w", err)
	}
	return nil
}

func marketingBodyPreview(body string) string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	body = strings.Join(strings.Fields(body), " ")
	runes := []rune(body)
	if len(runes) > 240 {
		return string(runes[:240])
	}
	return body
}

func marketingBodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func nullableFloat64Value(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	out := value.Float64
	return &out
}

func isDeliverableMarketingEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || isReservedMarketingEmail(email) {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(strings.TrimSpace(addr.Address), email)
}

func isReservedMarketingEmail(email string) bool {
	return strings.HasSuffix(email, LinuxDoConnectSyntheticEmailDomain) ||
		strings.HasSuffix(email, OIDCConnectSyntheticEmailDomain) ||
		strings.HasSuffix(email, WeChatConnectSyntheticEmailDomain) ||
		strings.HasSuffix(email, DingTalkConnectSyntheticEmailDomain)
}

func buildMarketingEmailBody(body, format string) string {
	if strings.EqualFold(strings.TrimSpace(format), "html") {
		return body
	}
	escaped := html.EscapeString(body)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	escaped = strings.ReplaceAll(escaped, "\n", "<br>\n")
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.6;color:#111827;">` +
		escaped +
		`</body></html>`
}
