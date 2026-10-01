package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const adminMergedUsageSelectColumns = `
	id,
	request_kind,
	created_at,
	sort_model,
	user_id,
	api_key_id,
	account_id,
	request_id,
	model,
	requested_model,
	upstream_model,
	group_id,
	subscription_id,
	input_tokens,
	output_tokens,
	cache_creation_tokens,
	cache_read_tokens,
	cache_creation_5m_tokens,
	cache_creation_1h_tokens,
	image_output_tokens,
	image_output_cost,
	input_cost,
	output_cost,
	cache_creation_cost,
	cache_read_cost,
	total_cost,
	actual_cost,
	rate_multiplier,
	account_rate_multiplier,
	billing_type,
	request_type,
	stream,
	openai_ws_mode,
	duration_ms,
	first_token_ms,
	user_agent,
	ip_address,
	image_count,
	image_size,
	image_input_size,
	image_output_size,
	image_size_source,
	image_size_breakdown,
	video_count,
	video_resolution,
	video_duration_seconds,
	service_tier,
	reasoning_effort,
	inbound_endpoint,
	upstream_endpoint,
	cache_ttl_overridden,
	channel_id,
	model_mapping_chain,
	billing_tier,
	billing_mode,
	account_stats_cost,
	status_code,
	error_message,
	error_phase,
	error_severity
`

func (r *usageLogRepository) ListAdminWithFilters(ctx context.Context, params pagination.PaginationParams, filters UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	conditions := make([]string, 0, 10)
	args := make([]any, 0, 10)

	if filters.UserID > 0 {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", len(args)+1))
		args = append(args, filters.UserID)
	}
	if filters.APIKeyID > 0 {
		conditions = append(conditions, fmt.Sprintf("api_key_id = $%d", len(args)+1))
		args = append(args, filters.APIKeyID)
	}
	if filters.AccountID > 0 {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", len(args)+1))
		args = append(args, filters.AccountID)
	}
	if filters.GroupID > 0 {
		conditions = append(conditions, fmt.Sprintf("group_id = $%d", len(args)+1))
		args = append(args, filters.GroupID)
	}
	if strings.TrimSpace(filters.Model) != "" {
		conditions = append(conditions, fmt.Sprintf("(model = $%d OR requested_model = $%d OR upstream_model = $%d)", len(args)+1, len(args)+1, len(args)+1))
		args = append(args, filters.Model)
	}
	conditions, args = appendRequestTypeOrStreamWhereCondition(conditions, args, filters.RequestType, filters.Stream)
	if filters.BillingType != nil {
		conditions = append(conditions, fmt.Sprintf("billing_type = $%d", len(args)+1))
		args = append(args, int16(*filters.BillingType))
	}
	conditions, args = appendUsageLogBillingModeWhereCondition(conditions, args, filters.BillingMode)
	conditions, args = appendUsageLogCanvasManagedWhereCondition(conditions, args, filters.CanvasManaged, "")
	if filters.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, *filters.EndTime)
	}

	whereClause := buildWhere(conditions)
	countQuery := "WITH merged AS (" + r.adminUsageMergedCTE() + ") SELECT COUNT(*) FROM merged " + whereClause
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, nil, err
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	listArgs := append(append([]any{}, args...), params.Limit(), params.Offset())
	query := fmt.Sprintf(
		"WITH merged AS (%s) SELECT %s FROM merged %s ORDER BY %s LIMIT $%d OFFSET $%d",
		r.adminUsageMergedCTE(),
		adminMergedUsageSelectColumns,
		whereClause,
		adminUsageOrderBy(params),
		limitPos,
		offsetPos,
	)
	logs, err := r.queryAdminMergedUsageLogs(ctx, query, listArgs...)
	if err != nil {
		return nil, nil, err
	}
	if err := r.hydrateUsageLogAssociations(ctx, logs); err != nil {
		return nil, nil, err
	}
	return logs, paginationResultFromTotal(total, params), nil
}

func (r *usageLogRepository) adminUsageMergedCTE() string {
	return `
	SELECT
		ul.id,
		'success'::text AS request_kind,
		ul.created_at,
		COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model) AS sort_model,
		ul.user_id,
		ul.api_key_id,
		ul.account_id,
		ul.request_id,
		ul.model,
		ul.requested_model,
		ul.upstream_model,
		ul.group_id,
		ul.subscription_id,
		ul.input_tokens,
		ul.output_tokens,
		ul.cache_creation_tokens,
		ul.cache_read_tokens,
		ul.cache_creation_5m_tokens,
		ul.cache_creation_1h_tokens,
		ul.image_output_tokens,
		ul.image_output_cost,
		ul.input_cost,
		ul.output_cost,
		ul.cache_creation_cost,
		ul.cache_read_cost,
		ul.total_cost,
		ul.actual_cost,
		ul.rate_multiplier,
		ul.account_rate_multiplier,
		ul.billing_type,
		ul.request_type,
		ul.stream,
		ul.openai_ws_mode,
		ul.duration_ms,
		ul.first_token_ms,
		ul.user_agent,
		CASE
			WHEN ul.ip_address IS NULL THEN NULL::text
			ELSE ul.ip_address::text
		END AS ip_address,
		ul.image_count,
		ul.image_size,
		ul.image_input_size,
		ul.image_output_size,
		ul.image_size_source,
		ul.image_size_breakdown,
		ul.video_count,
		ul.video_resolution,
		ul.video_duration_seconds,
		ul.service_tier,
		ul.reasoning_effort,
		ul.inbound_endpoint,
		ul.upstream_endpoint,
		ul.cache_ttl_overridden,
		ul.channel_id,
		ul.model_mapping_chain,
		ul.billing_tier,
		ul.billing_mode,
		ul.account_stats_cost,
		NULL::integer AS status_code,
		NULL::text AS error_message,
		NULL::text AS error_phase,
		NULL::text AS error_severity
	FROM usage_logs ul

	UNION ALL

	SELECT
		-o.id AS id,
		'error'::text AS request_kind,
		o.created_at,
		COALESCE(NULLIF(TRIM(o.requested_model), ''), NULLIF(TRIM(o.model), ''), NULLIF(TRIM(o.upstream_model), '')) AS sort_model,
		COALESCE(o.user_id, 0) AS user_id,
		COALESCE(o.api_key_id, 0) AS api_key_id,
		COALESCE(o.account_id, 0) AS account_id,
		COALESCE(NULLIF(o.request_id, ''), NULLIF(o.client_request_id, ''), '') AS request_id,
		COALESCE(NULLIF(o.requested_model, ''), NULLIF(o.model, ''), NULLIF(o.upstream_model, ''), '') AS model,
		NULLIF(o.requested_model, '') AS requested_model,
		NULLIF(o.upstream_model, '') AS upstream_model,
		o.group_id,
		NULL::bigint AS subscription_id,
		0 AS input_tokens,
		0 AS output_tokens,
		0 AS cache_creation_tokens,
		0 AS cache_read_tokens,
		0 AS cache_creation_5m_tokens,
		0 AS cache_creation_1h_tokens,
		0 AS image_output_tokens,
		0::numeric AS image_output_cost,
		0::numeric AS input_cost,
		0::numeric AS output_cost,
		0::numeric AS cache_creation_cost,
		0::numeric AS cache_read_cost,
		0::numeric AS total_cost,
		0::numeric AS actual_cost,
		1::numeric AS rate_multiplier,
		NULL::numeric AS account_rate_multiplier,
		0::smallint AS billing_type,
		COALESCE(o.request_type, 0)::smallint AS request_type,
		COALESCE(o.stream, false) AS stream,
		false AS openai_ws_mode,
		o.duration_ms,
		COALESCE(o.time_to_first_token_ms::integer, NULL) AS first_token_ms,
		NULLIF(o.user_agent, '') AS user_agent,
		CASE
			WHEN o.client_ip IS NULL OR o.client_ip::text = '' THEN NULL::text
			ELSE o.client_ip::text
		END AS ip_address,
		0 AS image_count,
		NULL::text AS image_size,
		NULL::text AS image_input_size,
		NULL::text AS image_output_size,
		NULL::text AS image_size_source,
		NULL::jsonb AS image_size_breakdown,
		0 AS video_count,
		NULL::text AS video_resolution,
		NULL::integer AS video_duration_seconds,
		NULL::text AS service_tier,
		NULL::text AS reasoning_effort,
		NULLIF(o.inbound_endpoint, '') AS inbound_endpoint,
		NULLIF(o.upstream_endpoint, '') AS upstream_endpoint,
		false AS cache_ttl_overridden,
		NULL::bigint AS channel_id,
		NULL::text AS model_mapping_chain,
		NULL::text AS billing_tier,
		NULL::text AS billing_mode,
		NULL::numeric AS account_stats_cost,
		o.status_code,
		NULLIF(o.error_message, '') AS error_message,
		NULLIF(o.error_phase, '') AS error_phase,
		NULLIF(o.severity, '') AS error_severity
	FROM ops_error_logs o
	WHERE COALESCE(o.status_code, 0) >= 400
	`
}

func adminUsageOrderBy(params pagination.PaginationParams) string {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := strings.ToUpper(params.NormalizedSortOrder(pagination.SortOrderDesc))

	var column string
	switch sortBy {
	case "model":
		column = "sort_model"
	case "created_at":
		column = "created_at"
	default:
		column = "created_at"
	}
	return fmt.Sprintf("%s %s, request_kind %s, request_id %s", column, sortOrder, sortOrder, sortOrder)
}

func (r *usageLogRepository) queryAdminMergedUsageLogs(ctx context.Context, query string, args ...any) (logs []service.UsageLog, err error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			logs = nil
		}
	}()

	logs = make([]service.UsageLog, 0)
	for rows.Next() {
		log, scanErr := scanAdminMergedUsageLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs = append(logs, *log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func scanAdminMergedUsageLog(scanner interface{ Scan(...any) error }) (*service.UsageLog, error) {
	var (
		id                    int64
		requestKind           string
		createdAt             time.Time
		sortModel             sql.NullString
		userID                int64
		apiKeyID              int64
		accountID             int64
		requestID             sql.NullString
		model                 string
		requestedModel        sql.NullString
		upstreamModel         sql.NullString
		groupID               sql.NullInt64
		subscriptionID        sql.NullInt64
		inputTokens           int
		outputTokens          int
		cacheCreationTokens   int
		cacheReadTokens       int
		cacheCreation5m       int
		cacheCreation1h       int
		imageOutputTokens     int
		imageOutputCost       float64
		inputCost             float64
		outputCost            float64
		cacheCreationCost     float64
		cacheReadCost         float64
		totalCost             float64
		actualCost            float64
		rateMultiplier        float64
		accountRateMultiplier sql.NullFloat64
		billingType           int16
		requestTypeRaw        int16
		stream                bool
		openaiWSMode          bool
		durationMs            sql.NullInt64
		firstTokenMs          sql.NullInt64
		userAgent             sql.NullString
		ipAddress             sql.NullString
		imageCount            int
		imageSize             sql.NullString
		imageInputSize        sql.NullString
		imageOutputSize       sql.NullString
		imageSizeSource       sql.NullString
		imageSizeBreakdown    sql.NullString
		videoCount            int
		videoResolution       sql.NullString
		videoDurationSeconds  sql.NullInt64
		serviceTier           sql.NullString
		reasoningEffort       sql.NullString
		inboundEndpoint       sql.NullString
		upstreamEndpoint      sql.NullString
		cacheTTLOverridden    bool
		channelID             sql.NullInt64
		modelMappingChain     sql.NullString
		billingTier           sql.NullString
		billingMode           sql.NullString
		accountStatsCost      sql.NullFloat64
		statusCode            sql.NullInt64
		errorMessage          sql.NullString
		errorPhase            sql.NullString
		errorSeverity         sql.NullString
	)

	if err := scanner.Scan(
		&id,
		&requestKind,
		&createdAt,
		&sortModel,
		&userID,
		&apiKeyID,
		&accountID,
		&requestID,
		&model,
		&requestedModel,
		&upstreamModel,
		&groupID,
		&subscriptionID,
		&inputTokens,
		&outputTokens,
		&cacheCreationTokens,
		&cacheReadTokens,
		&cacheCreation5m,
		&cacheCreation1h,
		&imageOutputTokens,
		&imageOutputCost,
		&inputCost,
		&outputCost,
		&cacheCreationCost,
		&cacheReadCost,
		&totalCost,
		&actualCost,
		&rateMultiplier,
		&accountRateMultiplier,
		&billingType,
		&requestTypeRaw,
		&stream,
		&openaiWSMode,
		&durationMs,
		&firstTokenMs,
		&userAgent,
		&ipAddress,
		&imageCount,
		&imageSize,
		&imageInputSize,
		&imageOutputSize,
		&imageSizeSource,
		&imageSizeBreakdown,
		&videoCount,
		&videoResolution,
		&videoDurationSeconds,
		&serviceTier,
		&reasoningEffort,
		&inboundEndpoint,
		&upstreamEndpoint,
		&cacheTTLOverridden,
		&channelID,
		&modelMappingChain,
		&billingTier,
		&billingMode,
		&accountStatsCost,
		&statusCode,
		&errorMessage,
		&errorPhase,
		&errorSeverity,
	); err != nil {
		return nil, err
	}

	log := &service.UsageLog{
		ID:                    id,
		RequestKind:           requestKind,
		UserID:                userID,
		APIKeyID:              apiKeyID,
		AccountID:             accountID,
		Model:                 model,
		RequestedModel:        coalesceTrimmedString(requestedModel, coalesceTrimmedString(sortModel, model)),
		InputTokens:           inputTokens,
		OutputTokens:          outputTokens,
		CacheCreationTokens:   cacheCreationTokens,
		CacheReadTokens:       cacheReadTokens,
		CacheCreation5mTokens: cacheCreation5m,
		CacheCreation1hTokens: cacheCreation1h,
		ImageOutputTokens:     imageOutputTokens,
		ImageOutputCost:       imageOutputCost,
		InputCost:             inputCost,
		OutputCost:            outputCost,
		CacheCreationCost:     cacheCreationCost,
		CacheReadCost:         cacheReadCost,
		TotalCost:             totalCost,
		ActualCost:            actualCost,
		RateMultiplier:        rateMultiplier,
		AccountRateMultiplier: nullFloat64Ptr(accountRateMultiplier),
		BillingType:           int8(billingType),
		RequestType:           service.RequestTypeFromInt16(requestTypeRaw),
		ImageCount:            imageCount,
		VideoCount:            videoCount,
		CacheTTLOverridden:    cacheTTLOverridden,
		CreatedAt:             createdAt,
		StatusCode:            nullIntPtr(statusCode),
		ErrorMessage:          nullStringPtr(errorMessage),
		ErrorPhase:            nullStringPtr(errorPhase),
		ErrorSeverity:         nullStringPtr(errorSeverity),
	}
	log.Stream = stream
	log.OpenAIWSMode = openaiWSMode
	log.RequestType = log.EffectiveRequestType()
	log.Stream, log.OpenAIWSMode = service.ApplyLegacyRequestFields(log.RequestType, stream, openaiWSMode)

	if requestID.Valid {
		log.RequestID = requestID.String
	}
	if upstreamModel.Valid {
		log.UpstreamModel = strPtr(strings.TrimSpace(upstreamModel.String))
	}
	if groupID.Valid {
		value := groupID.Int64
		log.GroupID = &value
	}
	if subscriptionID.Valid {
		value := subscriptionID.Int64
		log.SubscriptionID = &value
	}
	if durationMs.Valid {
		value := int(durationMs.Int64)
		log.DurationMs = &value
	}
	if firstTokenMs.Valid {
		value := int(firstTokenMs.Int64)
		log.FirstTokenMs = &value
	}
	if userAgent.Valid {
		log.UserAgent = strPtr(userAgent.String)
	}
	if ipAddress.Valid {
		log.IPAddress = strPtr(ipAddress.String)
	}
	if imageSize.Valid {
		log.ImageSize = strPtr(imageSize.String)
	}
	if imageInputSize.Valid {
		log.ImageInputSize = strPtr(imageInputSize.String)
	}
	if imageOutputSize.Valid {
		log.ImageOutputSize = strPtr(imageOutputSize.String)
	}
	if imageSizeSource.Valid {
		log.ImageSizeSource = strPtr(imageSizeSource.String)
	}
	if imageSizeBreakdown.Valid && imageSizeBreakdown.String != "" {
		var breakdown map[string]int
		if err := json.Unmarshal([]byte(imageSizeBreakdown.String), &breakdown); err == nil {
			log.ImageSizeBreakdown = breakdown
		}
	}
	if videoResolution.Valid {
		log.VideoResolution = strPtr(videoResolution.String)
	}
	if videoDurationSeconds.Valid {
		value := int(videoDurationSeconds.Int64)
		log.VideoDurationSeconds = &value
	}
	if serviceTier.Valid {
		log.ServiceTier = strPtr(serviceTier.String)
	}
	if reasoningEffort.Valid {
		log.ReasoningEffort = strPtr(reasoningEffort.String)
	}
	if inboundEndpoint.Valid {
		log.InboundEndpoint = strPtr(inboundEndpoint.String)
	}
	if upstreamEndpoint.Valid {
		log.UpstreamEndpoint = strPtr(upstreamEndpoint.String)
	}
	if channelID.Valid {
		value := channelID.Int64
		log.ChannelID = &value
	}
	if modelMappingChain.Valid {
		log.ModelMappingChain = strPtr(modelMappingChain.String)
	}
	if billingTier.Valid {
		log.BillingTier = strPtr(billingTier.String)
	}
	if billingMode.Valid {
		log.BillingMode = strPtr(billingMode.String)
	}
	if accountStatsCost.Valid {
		value := accountStatsCost.Float64
		log.AccountStatsCost = &value
	}

	return log, nil
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return strPtr(v.String)
}

func strPtr(v string) *string {
	value := strings.TrimSpace(v)
	return &value
}
