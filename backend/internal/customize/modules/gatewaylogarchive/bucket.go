package gatewaylogarchive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// BucketConfig is one storage destination. Uploads use the enabled bucket with
// the lowest Priority (then lowest ID) and fall over to the next on failure.
type BucketConfig struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	Bucket         string    `json:"bucket"`
	Region         string    `json:"region"`
	Endpoint       string    `json:"endpoint"`
	Prefix         string    `json:"prefix"`
	ForcePathStyle bool      `json:"force_path_style"`
	Enabled        bool      `json:"enabled"`
	Priority       int       `json:"priority"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	AccessKeyID     string `json:"-"`
	SecretAccessKey string `json:"-"`
	SessionToken    string `json:"-"`
}

const (
	maxBuckets       = 50
	defaultAWSRegion = "us-east-1"
)

// Normalize trims and validates the bucket. An "s3://bucket/prefix" value in
// the prefix field fills Bucket and Prefix.
func (b *BucketConfig) Normalize() error {
	b.Name = strings.TrimSpace(b.Name)
	if len([]rune(b.Name)) > 64 {
		return errors.New("名称最多 64 个字符")
	}
	b.Provider = strings.TrimSpace(b.Provider)
	if b.Provider != ProviderCompat {
		b.Provider = ProviderAWS
	}
	b.Bucket = strings.TrimSpace(b.Bucket)
	b.Region = strings.TrimSpace(b.Region)
	b.Endpoint = strings.TrimRight(strings.TrimSpace(b.Endpoint), "/")
	b.Prefix = strings.TrimSpace(b.Prefix)
	if rest, ok := strings.CutPrefix(b.Prefix, "s3://"); ok {
		bucket, prefix, _ := strings.Cut(rest, "/")
		if b.Bucket != "" && bucket != b.Bucket {
			return errors.New("S3 路径中的存储桶与 Bucket 字段不一致")
		}
		b.Bucket, b.Prefix = bucket, prefix
	}
	b.Prefix = strings.Trim(b.Prefix, "/")
	if strings.Contains(b.Prefix, "..") || strings.Contains(b.Prefix, "//") {
		return errors.New("S3 路径格式错误")
	}
	if b.Bucket == "" {
		return errors.New("请填写 Bucket / 存储桶")
	}
	if !bucketPattern.MatchString(b.Bucket) {
		return errors.New("Bucket 名称格式错误")
	}
	if b.Endpoint != "" {
		u, err := url.Parse(b.Endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("Endpoint 必须是 http(s):// 开头的地址")
		}
	}
	if b.Provider == ProviderAWS && b.Region == "" {
		b.Region = defaultAWSRegion
	}
	if b.Provider == ProviderCompat && b.Endpoint == "" {
		return errors.New("S3 兼容服务需要填写 Endpoint")
	}
	b.AccessKeyID = strings.TrimSpace(b.AccessKeyID)
	b.SecretAccessKey = strings.TrimSpace(b.SecretAccessKey)
	b.SessionToken = strings.TrimSpace(b.SessionToken)
	if (b.AccessKeyID == "") != (b.SecretAccessKey == "") {
		return errors.New("Access Key ID 与 Secret Access Key 需同时填写")
	}
	if b.Priority < 0 {
		b.Priority = 0
	}
	if b.Priority > 10000 {
		b.Priority = 10000
	}
	return nil
}

var errTooManyBuckets = fmt.Errorf("最多添加 %d 个存储桶", maxBuckets)

// ObjectKey is "<prefix>/<day>/<file>".
func (b BucketConfig) ObjectKey(day, file string) string {
	if b.Prefix == "" {
		return day + "/" + file
	}
	return b.Prefix + "/" + day + "/" + file
}

// Label is the display name used in errors and listings.
func (b BucketConfig) Label() string {
	if b.Name != "" {
		return b.Name
	}
	return b.Bucket
}

// enabledBuckets keeps upload order: priority, then creation order.
func enabledBuckets(all []BucketConfig) []BucketConfig {
	out := make([]BucketConfig, 0, len(all))
	for _, b := range all {
		if b.Enabled {
			out = append(out, b)
		}
	}
	return out
}

// resolveBucket finds the configuration that holds an uploaded bundle: by the
// recorded ID, else (bundles uploaded before multi-bucket) by bucket name.
func resolveBucket(all []BucketConfig, b Bundle) (BucketConfig, bool) {
	if b.BucketID != nil {
		for _, item := range all {
			if item.ID == *b.BucketID {
				return item, true
			}
		}
		return BucketConfig{}, false
	}
	for _, item := range all {
		if b.Bucket != "" && item.Bucket == b.Bucket {
			return item, true
		}
	}
	return BucketConfig{}, false
}

const bucketColumns = `id, name, provider, bucket, region, endpoint, prefix, force_path_style, access_key_id, secret_access_key,
	session_token, enabled, priority, created_at, updated_at`

func (s *Store) scanBucket(row scanner) (BucketConfig, error) {
	var b BucketConfig
	var ak, sk, token string
	if err := row.Scan(&b.ID, &b.Name, &b.Provider, &b.Bucket, &b.Region, &b.Endpoint, &b.Prefix, &b.ForcePathStyle,
		&ak, &sk, &token, &b.Enabled, &b.Priority, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return b, err
	}
	for _, item := range []struct {
		enc string
		dst *string
	}{{ak, &b.AccessKeyID}, {sk, &b.SecretAccessKey}, {token, &b.SessionToken}} {
		if item.enc == "" {
			continue
		}
		plain, err := s.decrypt(item.enc)
		if err != nil {
			return b, fmt.Errorf("gateway-log-archive: decrypt credentials of bucket %d: %w", b.ID, err)
		}
		*item.dst = plain
	}
	return b, nil
}

// Buckets lists every bucket in upload order with decrypted credentials.
func (s *Store) Buckets(ctx context.Context) ([]BucketConfig, error) {
	if err := s.ensureBuckets(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+bucketColumns+` FROM gateway_log_archive_buckets ORDER BY priority, id`)
	if err != nil {
		return nil, fmt.Errorf("gateway-log-archive: list buckets: %w", err)
	}
	defer rows.Close()
	out := []BucketConfig{}
	for rows.Next() {
		b, err := s.scanBucket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) Bucket(ctx context.Context, id int64) (BucketConfig, error) {
	if err := s.ensureBuckets(ctx); err != nil {
		return BucketConfig{}, err
	}
	return s.scanBucket(s.DB.QueryRowContext(ctx, `SELECT `+bucketColumns+` FROM gateway_log_archive_buckets WHERE id = $1`, id))
}

func (s *Store) encryptedCredentials(b BucketConfig) (ak, sk, token string, err error) {
	if ak, err = s.encrypt(b.AccessKeyID); err != nil {
		return
	}
	if sk, err = s.encrypt(b.SecretAccessKey); err != nil {
		return
	}
	token, err = s.encrypt(b.SessionToken)
	return
}

func (s *Store) CreateBucket(ctx context.Context, b BucketConfig) (BucketConfig, error) {
	if err := s.ensureBuckets(ctx); err != nil {
		return BucketConfig{}, err
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM gateway_log_archive_buckets`).Scan(&n); err != nil {
		return BucketConfig{}, err
	}
	if n >= maxBuckets {
		return BucketConfig{}, errTooManyBuckets
	}
	ak, sk, token, err := s.encryptedCredentials(b)
	if err != nil {
		return BucketConfig{}, err
	}
	return s.scanBucket(s.DB.QueryRowContext(ctx, `
		INSERT INTO gateway_log_archive_buckets (name, provider, bucket, region, endpoint, prefix, force_path_style,
			access_key_id, secret_access_key, session_token, enabled, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING `+bucketColumns,
		b.Name, b.Provider, b.Bucket, b.Region, b.Endpoint, b.Prefix, b.ForcePathStyle, ak, sk, token, b.Enabled, b.Priority))
}

func (s *Store) UpdateBucket(ctx context.Context, b BucketConfig) (BucketConfig, error) {
	ak, sk, token, err := s.encryptedCredentials(b)
	if err != nil {
		return BucketConfig{}, err
	}
	return s.scanBucket(s.DB.QueryRowContext(ctx, `
		UPDATE gateway_log_archive_buckets SET name = $1, provider = $2, bucket = $3, region = $4, endpoint = $5, prefix = $6,
			force_path_style = $7, access_key_id = $8, secret_access_key = $9, session_token = $10, enabled = $11, priority = $12,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $13 RETURNING `+bucketColumns,
		b.Name, b.Provider, b.Bucket, b.Region, b.Endpoint, b.Prefix, b.ForcePathStyle, ak, sk, token, b.Enabled, b.Priority, b.ID))
}

// DeleteBucket removes only the configuration; uploaded objects and their
// statistics stay. Bundles stored there can no longer be read back remotely.
func (s *Store) DeleteBucket(ctx context.Context, id int64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM gateway_log_archive_buckets WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// ensureBuckets adopts the single bucket of earlier versions exactly once: the
// config row is compare-and-swapped to "migrated" in the same transaction that
// inserts the bucket, so concurrent instances cannot adopt it twice.
func (s *Store) ensureBuckets(ctx context.Context) error {
	if err := s.ready(); err != nil {
		return err
	}
	if s.bucketsAdopted.Load() {
		return nil
	}
	var raw, ak, sk, token string
	err := s.DB.QueryRowContext(ctx, `SELECT config, access_key_id, secret_access_key, session_token FROM gateway_log_archive_config WHERE id = 1`).
		Scan(&raw, &ak, &sk, &token)
	if errors.Is(err, sql.ErrNoRows) {
		s.bucketsAdopted.Store(true)
		return nil
	}
	if err != nil {
		return fmt.Errorf("gateway-log-archive: load config: %w", err)
	}
	cfg := DefaultConfig()
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Errorf("gateway-log-archive: decode config: %w", err)
		}
	}
	if cfg.BucketsMigrated {
		s.bucketsAdopted.Store(true)
		return nil
	}
	legacy := BucketConfig{
		Name: "默认存储桶", Provider: cfg.Provider, Bucket: strings.TrimSpace(cfg.Bucket), Region: cfg.Region, Endpoint: cfg.Endpoint,
		Prefix: strings.Trim(cfg.Prefix, "/"), ForcePathStyle: cfg.ForcePathStyle, Enabled: true, Priority: 100,
	}
	if legacy.Provider != ProviderCompat {
		legacy.Provider = ProviderAWS
		if legacy.Region == "" {
			legacy.Region = defaultAWSRegion
		}
	}
	cfg.clearLegacyStorage()
	cfg.BucketsMigrated = true
	next, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE gateway_log_archive_config SET config = $1, access_key_id = '', secret_access_key = '',
		session_token = '', updated_at = CURRENT_TIMESTAMP WHERE id = 1 AND config = $2`, string(next), raw)
	if err != nil {
		return fmt.Errorf("gateway-log-archive: adopt bucket: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return tx.Rollback() // another instance adopted first; re-read next time
	}
	if legacy.Bucket != "" {
		var id int64
		// Credentials are copied still encrypted; the cipher is unchanged.
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO gateway_log_archive_buckets (name, provider, bucket, region, endpoint, prefix, force_path_style,
				access_key_id, secret_access_key, session_token, enabled, priority)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, 100) RETURNING id`,
			legacy.Name, legacy.Provider, legacy.Bucket, legacy.Region, legacy.Endpoint, legacy.Prefix, legacy.ForcePathStyle,
			ak, sk, token).Scan(&id); err != nil {
			return fmt.Errorf("gateway-log-archive: adopt bucket: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE gateway_log_archive_bundles SET bucket_id = $1 WHERE bucket_id IS NULL AND bucket = $2`, id, legacy.Bucket); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE gateway_log_archive_upload_stats SET bucket_id = $1 WHERE bucket_id = 0`, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.bucketsAdopted.Store(true)
	return nil
}

// ModelStat is the cumulative uploaded total of one model (archive name).
type ModelStat struct {
	ModelKey       string     `json:"model_key"`
	BundleCount    int64      `json:"bundle_count"`
	EntryCount     int64      `json:"entry_count"`
	SizeBytes      int64      `json:"size_bytes"`
	LastUploadedAt *time.Time `json:"last_uploaded_at"`
}

// BucketUsage is the cumulative uploaded total of one bucket.
type BucketUsage struct {
	BundleCount int64 `json:"bundle_count"`
	SizeBytes   int64 `json:"size_bytes"`
}

// ModelStats returns per-model totals, largest first; bucketID 0 means all.
func (s *Store) ModelStats(ctx context.Context, bucketID int64) ([]ModelStat, error) {
	if err := s.ensureBuckets(ctx); err != nil {
		return nil, err
	}
	var w where
	if bucketID > 0 {
		w.add("bucket_id = ?", bucketID)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT model_key, COALESCE(SUM(bundle_count), 0), COALESCE(SUM(entry_count), 0),
		COALESCE(SUM(size_bytes), 0), MAX(last_uploaded_at) FROM gateway_log_archive_upload_stats`+w.sql()+`
		GROUP BY model_key ORDER BY COALESCE(SUM(size_bytes), 0) DESC, model_key`, w.args...)
	if err != nil {
		return nil, fmt.Errorf("gateway-log-archive: model stats: %w", err)
	}
	defer rows.Close()
	out := []ModelStat{}
	for rows.Next() {
		var m ModelStat
		var last scanTime
		if err := rows.Scan(&m.ModelKey, &m.BundleCount, &m.EntryCount, &m.SizeBytes, &last); err != nil {
			return nil, err
		}
		m.LastUploadedAt = last.t
		out = append(out, m)
	}
	return out, rows.Err()
}

// scanTime accepts PostgreSQL time values and the text SQLite returns for
// aggregates (MAX loses the column type there).
type scanTime struct{ t *time.Time }

func (s *scanTime) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		s.t = nil
		return nil
	case time.Time:
		t := x
		s.t = &t
		return nil
	case string:
		return s.parse(x)
	case []byte:
		return s.parse(string(x))
	}
	return fmt.Errorf("gateway-log-archive: unsupported time value %T", v)
}

func (s *scanTime) parse(v string) error {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			s.t = &t
			return nil
		}
	}
	return fmt.Errorf("gateway-log-archive: invalid time %q", v)
}

// BucketUsage returns totals keyed by bucket ID (0: not attributed to a bucket).
func (s *Store) BucketUsage(ctx context.Context) (map[int64]BucketUsage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT bucket_id, COALESCE(SUM(bundle_count), 0), COALESCE(SUM(size_bytes), 0)
		FROM gateway_log_archive_upload_stats GROUP BY bucket_id`)
	if err != nil {
		return nil, fmt.Errorf("gateway-log-archive: bucket usage: %w", err)
	}
	defer rows.Close()
	out := map[int64]BucketUsage{}
	for rows.Next() {
		var id int64
		var u BucketUsage
		if err := rows.Scan(&id, &u.BundleCount, &u.SizeBytes); err != nil {
			return nil, err
		}
		out[id] = u
	}
	return out, rows.Err()
}
