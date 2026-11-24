package gatewaylogarchive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// An admin delete of a stuck bundle (pending, uploading or failed) is a
// tombstone in the shared index. The node that owns the local file cancels its
// upload (any open multipart upload is aborted), removes the file or staged
// fragments, then the row. Other nodes never touch that file.
var (
	errBundleDeleted      = errors.New("归档文件已被管理员删除")
	errBundleNotDeletable = errors.New("只能删除待上传、上传中或上传失败的归档文件")
)

const deleteWatchInterval = 10 * time.Second

// RequestDelete records the request. Repeating it for the same bundle is fine.
func (s *Store) RequestDelete(ctx context.Context, id int64) (Bundle, error) {
	b, err := scanBundle(s.DB.QueryRowContext(ctx, `UPDATE gateway_log_archive_bundles SET delete_requested_at = CURRENT_TIMESTAMP,
		next_attempt_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status IN ('sealed', 'uploading', 'failed') AND delete_requested_at IS NULL RETURNING `+bundleColumns, id))
	if !errors.Is(err, sql.ErrNoRows) {
		return b, err
	}
	current, err := s.Bundle(ctx, id)
	if err != nil {
		return Bundle{}, err // sql.ErrNoRows: not found
	}
	if current.DeleteRequestedAt != nil {
		return current, nil
	}
	return Bundle{}, errBundleNotDeletable
}

// deleteRequested reports whether an upload of id must stop. A missing row
// counts; a read error does not, so a database hiccup never aborts uploads.
func (s *Store) deleteRequested(ctx context.Context, id int64) bool {
	var requested bool
	err := s.DB.QueryRowContext(ctx, `SELECT delete_requested_at IS NOT NULL FROM gateway_log_archive_bundles WHERE id = $1`, id).Scan(&requested)
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return err == nil && requested
}

// Rows still marked uploading belong to an upload that has not stopped yet.
func (s *Store) deleteRequestedBundles(ctx context.Context, node string) ([]Bundle, error) {
	return s.bundles(ctx, `SELECT `+bundleColumns+` FROM gateway_log_archive_bundles
		WHERE node_id = $1 AND delete_requested_at IS NOT NULL AND status IN ('sealed', 'failed') ORDER BY id LIMIT 50`, node)
}

func (s *Store) releaseDeleted(ctx context.Context, id int64) error {
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status = 'failed', next_attempt_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'uploading' AND delete_requested_at IS NOT NULL`, id)
}

func (s *Store) purgeBundle(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gateway_log_archive_bundles WHERE id = $1 AND delete_requested_at IS NOT NULL AND status IN ('sealed', 'failed'))`, id).Scan(&exists); err != nil || !exists {
		return err
	}
	for _, query := range []string{
		`DELETE FROM gateway_log_archive_deliveries WHERE bundle_id = $1`,
		`DELETE FROM gateway_log_archive_entries WHERE bundle_id = $1`,
		`DELETE FROM gateway_log_archive_bundles WHERE id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, query, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// beginUpload publishes progress and returns the context of one upload. The
// context is cancelled when an admin deletes the bundle, on any node.
func (p *pipeline) beginUpload(row *Bundle, progress *uploadProgress, bucket string) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(p.ctx)
	progress.BundleID = row.ID
	progress.startedAt = time.Now()
	progress.attempt = row.Attempts
	progress.bucket = bucket
	progress.cancel = cancel
	p.uploading.Store(progress)
	done := make(chan struct{})
	if store, ok := p.store.(*Store); ok {
		go func() {
			ticker := time.NewTicker(deleteWatchInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-done:
					return
				case <-ticker.C:
					if store.deleteRequested(ctx, row.ID) {
						cancel(errBundleDeleted)
						return
					}
				}
			}
		}()
	}
	return context.WithValue(ctx, uploadProgressKey{}, progress), func() {
		close(done)
		p.uploading.CompareAndSwap(progress, nil)
		cancel(nil)
	}
}

// uploadError reports a deletion instead of the cancellation it caused.
func uploadError(ctx context.Context, err error) error {
	if err != nil && errors.Is(context.Cause(ctx), errBundleDeleted) {
		return errBundleDeleted
	}
	return err
}

// cancelUpload stops this node's upload of id at once and wakes the cleaner.
func (p *pipeline) cancelUpload(id int64) {
	if up := p.uploading.Load(); up != nil && up.BundleID == id && up.cancel != nil {
		up.cancel(errBundleDeleted)
	}
	select {
	case p.deleteWake <- struct{}{}:
	default:
	}
}

// deleteLoop runs apart from the upload queue: a long upload of another file
// must not delay freeing the disk space of a deleted one.
func (p *pipeline) deleteLoop() {
	defer p.workers.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		p.purgeDeleted()
		select {
		case <-p.ctx.Done():
			return
		case <-p.deleteWake:
		case <-ticker.C:
		}
	}
}

func (p *pipeline) purgeDeleted() {
	store, ok := p.store.(*Store)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Minute)
	defer cancel()
	rows, err := store.deleteRequestedBundles(ctx, p.node)
	if err != nil {
		if p.ctx.Err() == nil {
			p.report(fmt.Errorf("读取待删除的归档文件失败：%w", err))
		}
		return
	}
	for _, row := range rows {
		if up := p.uploading.Load(); up != nil && up.BundleID == row.ID {
			continue // its upload is stopping; the next pass removes it
		}
		if err := p.removeBundleData(ctx, store, row); err != nil {
			if p.ctx.Err() == nil {
				p.report(fmt.Errorf("删除归档 %s 的数据失败，稍后重试：%w", row.FileName, err))
			}
			continue
		}
		if err := store.purgeBundle(ctx, row.ID); err != nil && p.ctx.Err() == nil {
			p.report(fmt.Errorf("删除归档 %s 的索引失败：%w", row.FileName, err))
		}
	}
}

func (p *pipeline) removeBundleData(ctx context.Context, store *Store, row Bundle) error {
	if row.StageBucketID == nil {
		if row.LocalPath == "" {
			return nil
		}
		// Only ever remove a file inside this node's own bundle directory.
		root := filepath.Clean(p.bundleDir()) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(row.LocalPath), root) {
			return fmt.Errorf("本地路径 %s 不在归档目录内", row.LocalPath)
		}
		removeLegacyUploadTemp(row.LocalPath)
		if err := os.Remove(row.LocalPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	// RustFS staging: local fallback fragments and fragments on the primary.
	var objects objectStore
	for {
		rows, err := store.DB.QueryContext(ctx, `SELECT seq, stage_key, staged_remote FROM gateway_log_archive_entries
			WHERE bundle_id = $1 AND NOT stage_cleaned ORDER BY seq LIMIT 200`, row.ID)
		if err != nil {
			return err
		}
		type fragment struct {
			seq    int
			key    string
			remote bool
		}
		var batch []fragment
		for rows.Next() {
			var f fragment
			if err := rows.Scan(&f.seq, &f.key, &f.remote); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, f)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, f := range batch {
			if f.key != "" {
				if err := os.Remove(fallbackPath(row, f.key)); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			if f.remote && f.key != "" {
				if objects == nil {
					cfg, err := p.bucketByID(*row.StageBucketID)
					if err != nil {
						return fmt.Errorf("无法删除 RustFS 暂存片段：%w", err)
					}
					if objects, err = p.bucketClient(cfg); err != nil {
						return err
					}
				}
				if err := objects.Delete(ctx, f.key); err != nil && !isNotFound(err) {
					return err
				}
			}
			if err := store.exec(ctx, `UPDATE gateway_log_archive_entries SET stage_cleaned = true WHERE bundle_id = $1 AND seq = $2`, row.ID, f.seq); err != nil {
				return err
			}
		}
	}
}

// discardUploaded removes an object that finished uploading just as the bundle
// was deleted, so the deletion leaves nothing behind.
func (p *pipeline) discardUploaded(objects objectStore, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := objects.Delete(ctx, key); err != nil && !isNotFound(err) {
		p.report(fmt.Errorf("归档已删除，但未能删除存储桶中刚上传的 %s：%w", key, err))
	}
}
