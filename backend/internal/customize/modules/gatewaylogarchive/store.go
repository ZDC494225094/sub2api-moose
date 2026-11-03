package gatewaylogarchive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Encryptor protects storage credentials at rest (host SecretEncryptor).
type Encryptor interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

const (
	StatusOpen      = "open"
	StatusSealed    = "sealed"
	StatusUploading = "uploading"
	StatusUploaded  = "uploaded"
	StatusFailed    = "failed"
)

type Bundle struct {
	ID           int64      `json:"id"`
	NodeID       string     `json:"node_id"`
	Day          string     `json:"day"`
	ModelKey     string     `json:"model_key"`
	Part         int        `json:"part"`
	FileName     string     `json:"file_name"`
	LocalPath    string     `json:"-"`
	Status       string     `json:"status"`
	SizeBytes    int64      `json:"size_bytes"`
	EntryCount   int        `json:"entry_count"`
	Bucket       string     `json:"bucket"`
	ObjectKey    string     `json:"object_key"`
	Attempts     int        `json:"attempts"`
	LastError    string     `json:"last_error"`
	LocalDeleted bool       `json:"local_deleted"`
	CreatedAt    time.Time  `json:"created_at"`
	SealedAt     *time.Time `json:"sealed_at"`
	UploadedAt   *time.Time `json:"uploaded_at"`
}

type Entry struct {
	ID            int64     `json:"id"`
	BundleID      int64     `json:"bundle_id"`
	Seq           int       `json:"seq"`
	Name          string    `json:"entry_name"`
	SessionID     string    `json:"session_id"`
	Model         string    `json:"model"`
	Platform      string    `json:"platform"`
	UserID        int64     `json:"user_id"`
	APIKeyID      int64     `json:"api_key_id"`
	ExchangeCount int       `json:"exchange_count"`
	Offset        int64     `json:"-"`
	CSize         int64     `json:"compressed_size"`
	USize         int64     `json:"uncompressed_size"`
	CRC           uint32    `json:"-"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined bundle fields for listings.
	Day          string `json:"day"`
	FileName     string `json:"file_name"`
	BundleStatus string `json:"bundle_status"`
}

// archiveStore is what the pipeline needs; Store implements it with SQL.
type archiveStore interface {
	CreateBundle(ctx context.Context, node, day, modelKey string, pathFor func(file string) string) (Bundle, error)
	CommitEntry(ctx context.Context, bundleSize int64, e Entry) error
	OpenBundles(ctx context.Context, node string) ([]Bundle, error)
	BundleEntries(ctx context.Context, bundleID int64) ([]zipEntry, error)
	MarkSealed(ctx context.Context, id, size int64) error
	MarkBroken(ctx context.Context, id int64, reason string) error
	ResetUploading(ctx context.Context, node string) error
	ClaimUpload(ctx context.Context, node string, now time.Time) (*Bundle, error)
	ObjectKeyTaken(ctx context.Context, bucket, key string, exceptID int64) (bool, error)
	MarkUploaded(ctx context.Context, id int64, bucket, key string) error
	MarkUploadFailed(ctx context.Context, id int64, reason string, next time.Time) error
	MarkLocalDeleted(ctx context.Context, id int64) error
	LocalExpired(ctx context.Context, node string, before time.Time) ([]Bundle, error)
	Prune(ctx context.Context, before time.Time) error
}

type Store struct {
	DB     *sql.DB
	Cipher Encryptor
}

var errStoreUnavailable = errors.New("gateway-log-archive: store unavailable")

func (s *Store) ready() error {
	if s == nil || s.DB == nil {
		return errStoreUnavailable
	}
	return nil
}

// Config loads the configuration with decrypted credentials.
func (s *Store) Config(ctx context.Context) (Config, *time.Time, error) {
	cfg := DefaultConfig()
	if err := s.ready(); err != nil {
		return cfg, nil, err
	}
	var raw, ak, sk, token string
	var updated sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT config, access_key_id, secret_access_key, session_token, updated_at FROM gateway_log_archive_config WHERE id = 1`).
		Scan(&raw, &ak, &sk, &token, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, fmt.Errorf("gateway-log-archive: load config: %w", err)
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return DefaultConfig(), nil, fmt.Errorf("gateway-log-archive: decode config: %w", err)
		}
	}
	for _, item := range []struct {
		enc string
		dst *string
	}{{ak, &cfg.AccessKeyID}, {sk, &cfg.SecretAccessKey}, {token, &cfg.SessionToken}} {
		if item.enc == "" {
			continue
		}
		plain, err := s.decrypt(item.enc)
		if err != nil {
			return cfg, nil, fmt.Errorf("gateway-log-archive: decrypt credentials: %w", err)
		}
		*item.dst = plain
	}
	_ = cfg.Normalize()
	var at *time.Time
	if updated.Valid {
		at = &updated.Time
	}
	return cfg, at, nil
}

func (s *Store) decrypt(v string) (string, error) {
	if s.Cipher == nil {
		return "", errors.New("no cipher")
	}
	return s.Cipher.Decrypt(v)
}

func (s *Store) encrypt(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if s.Cipher == nil {
		return "", errors.New("no cipher")
	}
	return s.Cipher.Encrypt(v)
}

// SaveConfig persists cfg; its credential fields must already be resolved
// (kept or replaced) by the caller.
func (s *Store) SaveConfig(ctx context.Context, cfg Config) error {
	if err := s.ready(); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	ak, err := s.encrypt(cfg.AccessKeyID)
	if err != nil {
		return err
	}
	sk, err := s.encrypt(cfg.SecretAccessKey)
	if err != nil {
		return err
	}
	token, err := s.encrypt(cfg.SessionToken)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
		INSERT INTO gateway_log_archive_config (id, config, access_key_id, secret_access_key, session_token, updated_at)
		VALUES (1, $1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET config = EXCLUDED.config, access_key_id = EXCLUDED.access_key_id,
			secret_access_key = EXCLUDED.secret_access_key, session_token = EXCLUDED.session_token, updated_at = CURRENT_TIMESTAMP`,
		string(raw), ak, sk, token)
	if err != nil {
		return fmt.Errorf("gateway-log-archive: save config: %w", err)
	}
	return nil
}

const bundleColumns = `id, node_id, day, model_key, part, file_name, local_path, status, size_bytes, entry_count,
	bucket, object_key, attempts, last_error, local_deleted, created_at, sealed_at, uploaded_at`

type scanner interface{ Scan(...any) error }

func scanBundle(row scanner) (Bundle, error) {
	var b Bundle
	var sealed, uploaded sql.NullTime
	err := row.Scan(&b.ID, &b.NodeID, &b.Day, &b.ModelKey, &b.Part, &b.FileName, &b.LocalPath, &b.Status, &b.SizeBytes,
		&b.EntryCount, &b.Bucket, &b.ObjectKey, &b.Attempts, &b.LastError, &b.LocalDeleted, &b.CreatedAt, &sealed, &uploaded)
	if sealed.Valid {
		b.SealedAt = &sealed.Time
	}
	if uploaded.Valid {
		b.UploadedAt = &uploaded.Time
	}
	return b, err
}

func (s *Store) bundles(ctx context.Context, query string, args ...any) ([]Bundle, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bundle
	for rows.Next() {
		b, err := scanBundle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CreateBundle allocates the next part number of (day, model); the unique
// constraint arbitrates races between server instances sharing the database.
func (s *Store) CreateBundle(ctx context.Context, node, day, modelKey string, pathFor func(string) string) (Bundle, error) {
	if err := s.ready(); err != nil {
		return Bundle{}, err
	}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		var part int
		if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(part), 0) + 1 FROM gateway_log_archive_bundles WHERE day = $1 AND model_key = $2`, day, modelKey).Scan(&part); err != nil {
			return Bundle{}, err
		}
		file := BundleFileName(modelKey, day, part)
		path := pathFor(file)
		row := s.DB.QueryRowContext(ctx, `
			INSERT INTO gateway_log_archive_bundles (node_id, day, model_key, part, file_name, local_path, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'open') RETURNING `+bundleColumns, node, day, modelKey, part, file, path)
		b, err := scanBundle(row)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return Bundle{}, fmt.Errorf("gateway-log-archive: create bundle: %w", lastErr)
}

func (s *Store) CommitEntry(ctx context.Context, bundleSize int64, e Entry) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO gateway_log_archive_entries (bundle_id, seq, entry_name, session_id, model, platform, user_id, api_key_id,
			exchange_count, header_offset, compressed_size, uncompressed_size, crc32, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		e.BundleID, e.Seq, e.Name, e.SessionID, limit(e.Model, 255), limit(e.Platform, 32), e.UserID, e.APIKeyID,
		e.ExchangeCount, e.Offset, e.CSize, e.USize, int64(e.CRC), e.StartedAt.UTC(), e.EndedAt.UTC()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gateway_log_archive_bundles SET size_bytes = $1, entry_count = entry_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, bundleSize, e.BundleID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) OpenBundles(ctx context.Context, node string) ([]Bundle, error) {
	return s.bundles(ctx, `SELECT `+bundleColumns+` FROM gateway_log_archive_bundles WHERE status = 'open' AND node_id = $1 ORDER BY id`, node)
}

func (s *Store) BundleEntries(ctx context.Context, bundleID int64) ([]zipEntry, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT entry_name, header_offset, crc32, compressed_size, uncompressed_size, ended_at
		FROM gateway_log_archive_entries WHERE bundle_id = $1 ORDER BY seq`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []zipEntry
	for rows.Next() {
		var e zipEntry
		var crc int64
		if err := rows.Scan(&e.Name, &e.Offset, &crc, &e.CSize, &e.USize, &e.Mod); err != nil {
			return nil, err
		}
		e.CRC = uint32(crc)
		e.Mod = e.Mod.UTC().Truncate(time.Second)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) exec(ctx context.Context, query string, args ...any) error {
	_, err := s.DB.ExecContext(ctx, query, args...)
	return err
}

func (s *Store) MarkSealed(ctx context.Context, id, size int64) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'sealed', size_bytes = $1, sealed_at = CURRENT_TIMESTAMP,
		next_attempt_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, size, id)
}

func (s *Store) MarkBroken(ctx context.Context, id int64, reason string) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'failed', last_error = $1, next_attempt_at = NULL,
		updated_at = CURRENT_TIMESTAMP WHERE id = $2`, limit(reason, 2000), id)
}

func (s *Store) ResetUploading(ctx context.Context, node string) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'sealed', updated_at = CURRENT_TIMESTAMP WHERE status = 'uploading' AND node_id = $1`, node)
}

func (s *Store) ClaimUpload(ctx context.Context, node string, now time.Time) (*Bundle, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var id int64
		err := s.DB.QueryRowContext(ctx, `SELECT id FROM gateway_log_archive_bundles
			WHERE node_id = $1 AND local_deleted = false AND (status = 'sealed' OR (status = 'failed' AND next_attempt_at IS NOT NULL AND next_attempt_at <= $2))
			ORDER BY id LIMIT 1`, node, now.UTC()).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		res, err := s.DB.ExecContext(ctx, `UPDATE gateway_log_archive_bundles SET status = 'uploading', attempts = attempts + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1 AND status IN ('sealed', 'failed')`, id)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			b, err := s.Bundle(ctx, id)
			return &b, err
		}
	}
	return nil, nil
}

func (s *Store) ObjectKeyTaken(ctx context.Context, bucket, key string, exceptID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM gateway_log_archive_bundles WHERE bucket = $1 AND object_key = $2 AND id <> $3`, bucket, key, exceptID).Scan(&n)
	return n > 0, err
}

func (s *Store) MarkUploaded(ctx context.Context, id int64, bucket, key string) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'uploaded', bucket = $1, object_key = $2, last_error = '',
		next_attempt_at = NULL, uploaded_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $3`, bucket, key, id)
}

func (s *Store) MarkUploadFailed(ctx context.Context, id int64, reason string, next time.Time) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'failed', last_error = $1, next_attempt_at = $2,
		updated_at = CURRENT_TIMESTAMP WHERE id = $3`, limit(reason, 2000), next.UTC(), id)
}

func (s *Store) MarkLocalDeleted(ctx context.Context, id int64) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET local_deleted = true, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
}

func (s *Store) LocalExpired(ctx context.Context, node string, before time.Time) ([]Bundle, error) {
	return s.bundles(ctx, `SELECT `+bundleColumns+` FROM gateway_log_archive_bundles
		WHERE node_id = $1 AND status = 'uploaded' AND local_deleted = false AND uploaded_at < $2 ORDER BY id LIMIT 100`, node, before.UTC())
}

// Prune drops index rows of uploaded bundles whose local copy is gone; the S3
// objects are never touched.
func (s *Store) Prune(ctx context.Context, before time.Time) error {
	if err := s.exec(ctx, `DELETE FROM gateway_log_archive_entries WHERE bundle_id IN (
		SELECT id FROM gateway_log_archive_bundles WHERE status = 'uploaded' AND local_deleted = true AND uploaded_at < $1)`, before.UTC()); err != nil {
		return err
	}
	return s.exec(ctx, `DELETE FROM gateway_log_archive_bundles WHERE status = 'uploaded' AND local_deleted = true AND uploaded_at < $1`, before.UTC())
}

// RetryBundle puts a failed bundle that still has its local file back in the
// upload queue.
func (s *Store) RetryBundle(ctx context.Context, id int64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE gateway_log_archive_bundles SET status = 'sealed', next_attempt_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'failed' AND local_deleted = false AND sealed_at IS NOT NULL`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) Bundle(ctx context.Context, id int64) (Bundle, error) {
	return scanBundle(s.DB.QueryRowContext(ctx, `SELECT `+bundleColumns+` FROM gateway_log_archive_bundles WHERE id = $1`, id))
}

type BundleFilter struct {
	Day    string
	Model  string
	Status string
}

type where struct {
	clauses []string
	args    []any
}

func (w *where) add(clause string, arg any) {
	w.args = append(w.args, arg)
	w.clauses = append(w.clauses, strings.ReplaceAll(clause, "?", "$"+strconv.Itoa(len(w.args))))
}

func (w *where) sql() string {
	if len(w.clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.clauses, " AND ")
}

func (w *where) page(page, size int) string {
	w.args = append(w.args, size, (page-1)*size)
	n := len(w.args)
	return " LIMIT $" + strconv.Itoa(n-1) + " OFFSET $" + strconv.Itoa(n)
}

func (s *Store) ListBundles(ctx context.Context, f BundleFilter, page, size int) ([]Bundle, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	var w where
	if f.Day != "" {
		w.add("day = ?", f.Day)
	}
	if f.Model != "" {
		w.add("LOWER(model_key) LIKE ?", "%"+strings.ToLower(f.Model)+"%")
	}
	if f.Status != "" {
		w.add("status = ?", f.Status)
	}
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM gateway_log_archive_bundles`+w.sql(), w.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	cond := w.sql()
	items, err := s.bundles(ctx, `SELECT `+bundleColumns+` FROM gateway_log_archive_bundles`+cond+` ORDER BY day DESC, model_key, part DESC`+w.page(page, size), w.args...)
	return items, total, err
}

type EntryFilter struct {
	BundleID  int64
	SessionID string
	Day       string
	Model     string
	UserID    int64
}

const entryColumns = `e.id, e.bundle_id, e.seq, e.entry_name, e.session_id, e.model, e.platform, e.user_id, e.api_key_id,
	e.exchange_count, e.header_offset, e.compressed_size, e.uncompressed_size, e.crc32, e.started_at, e.ended_at, e.created_at,
	b.day, b.file_name, b.status`

func scanEntry(row scanner) (Entry, error) {
	var e Entry
	var crc int64
	err := row.Scan(&e.ID, &e.BundleID, &e.Seq, &e.Name, &e.SessionID, &e.Model, &e.Platform, &e.UserID, &e.APIKeyID,
		&e.ExchangeCount, &e.Offset, &e.CSize, &e.USize, &crc, &e.StartedAt, &e.EndedAt, &e.CreatedAt, &e.Day, &e.FileName, &e.BundleStatus)
	e.CRC = uint32(crc)
	return e, err
}

func (s *Store) ListEntries(ctx context.Context, f EntryFilter, page, size int) ([]Entry, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	var w where
	if f.BundleID > 0 {
		w.add("e.bundle_id = ?", f.BundleID)
	}
	if f.SessionID != "" {
		w.add("e.session_id = ?", f.SessionID)
	}
	if f.Day != "" {
		w.add("b.day = ?", f.Day)
	}
	if f.Model != "" {
		w.add("(LOWER(e.model) LIKE ? OR LOWER(b.model_key) LIKE ?)", "%"+strings.ToLower(f.Model)+"%")
	}
	if f.UserID > 0 {
		w.add("e.user_id = ?", f.UserID)
	}
	from := ` FROM gateway_log_archive_entries e JOIN gateway_log_archive_bundles b ON b.id = e.bundle_id`
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+from+w.sql(), w.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	cond := w.sql()
	order := ` ORDER BY e.id DESC`
	if f.BundleID > 0 {
		order = ` ORDER BY e.seq`
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+entryColumns+from+cond+order+w.page(page, size), w.args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}

func (s *Store) Entry(ctx context.Context, id int64) (Entry, Bundle, error) {
	e, err := scanEntry(s.DB.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM gateway_log_archive_entries e
		JOIN gateway_log_archive_bundles b ON b.id = e.bundle_id WHERE e.id = $1`, id))
	if err != nil {
		return e, Bundle{}, err
	}
	b, err := s.Bundle(ctx, e.BundleID)
	return e, b, err
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
	Bytes  int64  `json:"bytes"`
}

func (s *Store) Summary(ctx context.Context) ([]StatusCount, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT status, COUNT(*), COALESCE(SUM(size_bytes), 0) FROM gateway_log_archive_bundles GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatusCount
	for rows.Next() {
		var c StatusCount
		if err := rows.Scan(&c.Status, &c.Count, &c.Bytes); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
