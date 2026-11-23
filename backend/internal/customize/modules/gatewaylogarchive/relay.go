package gatewaylogarchive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type delivery struct {
	BundleID  int64  `json:"bundle_id"`
	BucketID  int64  `json:"bucket_id"`
	Key       string `json:"object_key"`
	Status    string `json:"status"`
	LastError string `json:"last_error"`
	Attempts  int    `json:"attempts"`
	Reserved  bool   `json:"-"`
	Owner     string `json:"-"`
}

func (p *pipeline) wakeRelay() {
	select {
	case p.relayWake <- struct{}{}:
	default:
	}
}

func (p *pipeline) relayLoop() {
	defer p.workers.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		p.relayPending()
		select {
		case <-p.ctx.Done():
			return
		case <-p.relayWake:
		case <-ticker.C:
		}
	}
}

func (p *pipeline) deliveryTargets(row *Bundle, primary BucketConfig) []delivery {
	var out []delivery
	for _, bucket := range p.enabledBuckets() {
		if bucket.StorageRole == "rustfs" || bucket.ID == primary.ID {
			continue
		}
		out = append(out, delivery{BundleID: row.ID, BucketID: bucket.ID})
	}
	return out
}

func (s *Store) deliveries(ctx context.Context, bundleID int64) ([]delivery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT bundle_id,bucket_id,object_key,status,last_error,attempts FROM gateway_log_archive_deliveries WHERE bundle_id=$1 ORDER BY bucket_id`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []delivery{}
	for rows.Next() {
		var job delivery
		if err := rows.Scan(&job.BundleID, &job.BucketID, &job.Key, &job.Status, &job.LastError, &job.Attempts); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (p *pipeline) relayPending() {
	store, ok := p.store.(*Store)
	if !ok || p.ctx.Err() != nil {
		return
	}
	rows, err := store.DB.QueryContext(p.ctx, `SELECT d.bundle_id,d.bucket_id,d.object_key,d.attempts,d.key_reserved,d.last_error FROM gateway_log_archive_deliveries d JOIN gateway_log_archive_buckets t ON t.id=d.bucket_id WHERE t.enabled AND t.storage_role='destination' AND d.status <> 'uploaded' AND (d.next_attempt_at IS NULL OR d.next_attempt_at <= $1) AND (d.lease_until IS NULL OR d.lease_until < $1) ORDER BY d.bundle_id,d.bucket_id LIMIT 32`, time.Now().UTC())
	if err != nil {
		p.report(err)
		return
	}
	var jobs []delivery
	for rows.Next() {
		var job delivery
		if err = rows.Scan(&job.BundleID, &job.BucketID, &job.Key, &job.Attempts, &job.Reserved, &job.LastError); err != nil {
			break
		}
		jobs = append(jobs, job)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		p.report(err)
		return
	}
	for _, job := range jobs {
		if p.ctx.Err() != nil {
			return
		}
		job.Owner = uuid.NewString()
		now := time.Now().UTC()
		res, err := store.DB.ExecContext(p.ctx, `UPDATE gateway_log_archive_deliveries SET lease_owner=$1,lease_until=$2 WHERE bundle_id=$3 AND bucket_id=$4 AND status<>'uploaded' AND (next_attempt_at IS NULL OR next_attempt_at<=$5) AND (lease_until IS NULL OR lease_until<$5)`, job.Owner, now.Add(2*time.Minute), job.BundleID, job.BucketID, now)
		if err != nil {
			p.report(err)
			return
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			continue
		}
		if err := p.leasedRelay(store, job); err != nil && p.ctx.Err() == nil {
			p.report(fmt.Errorf("归档 #%d 转存到桶 #%d 失败：%w", job.BundleID, job.BucketID, err))
			if err := store.exec(p.ctx, `UPDATE gateway_log_archive_deliveries SET status='failed',last_error=$1,attempts=attempts+1,next_attempt_at=$2,lease_owner='',lease_until=NULL WHERE bundle_id=$3 AND bucket_id=$4 AND status<>'uploaded' AND lease_owner=$5`, limit(storageErrorText(err), 2000), time.Now().Add(backoff(job.Attempts+1)).UTC(), job.BundleID, job.BucketID, job.Owner); err != nil {
				p.report(err)
			}
		}
	}
}

func (p *pipeline) leasedRelay(store *Store, job delivery) error {
	ctx, cancel := context.WithCancel(p.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, stop := context.WithTimeout(ctx, 15*time.Second)
				res, err := store.DB.ExecContext(check, `UPDATE gateway_log_archive_deliveries SET lease_until=$1 WHERE bundle_id=$2 AND bucket_id=$3 AND lease_owner=$4`, time.Now().UTC().Add(2*time.Minute), job.BundleID, job.BucketID, job.Owner)
				stop()
				if err != nil {
					cancel()
					return
				}
				if n, _ := res.RowsAffected(); n != 1 {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	// Use the persisted failure even if another node (or a newer binary) owns
	// this attempt. Keep using the compatible transport for this delivery.
	if retryRelayHTTP1(job.LastError) {
		if err := p.relayOne(withRelayProtocol(ctx, true), store, job); err != nil {
			return fmt.Errorf("%s：%w", relayHTTP1Retry, err)
		}
		return nil
	}
	return p.relayOne(ctx, store, job)
}

func (p *pipeline) relayOne(ctx context.Context, store *Store, job delivery) error {
	row, err := store.Bundle(ctx, job.BundleID)
	if err != nil {
		return err
	}
	if row.BucketID == nil || row.Status != StatusUploaded {
		return errors.New("主归档尚未上传")
	}
	source, err := store.Bucket(ctx, *row.BucketID)
	if err != nil {
		return err
	}
	target, err := store.Bucket(ctx, job.BucketID)
	if err != nil {
		return err
	}
	if !target.Enabled || target.StorageRole != "destination" {
		return nil
	}
	redact := target.redact(p.defaultPrivacy())
	if row.StageRedacted && !redact {
		return errors.New("主归档已脱敏，此副本无法还原原文；请为该目标选择脱敏")
	}
	from, err := p.bucketClient(source)
	if err != nil {
		return err
	}
	to, err := p.bucketClient(target)
	if err != nil {
		return err
	}
	if !job.Reserved {
		key, err := p.chooseKey(ctx, target, to, &row)
		if err != nil {
			return err
		}
		if err = store.DB.QueryRowContext(ctx, `UPDATE gateway_log_archive_deliveries SET object_key=CASE WHEN key_reserved THEN object_key ELSE $1 END,key_reserved=true WHERE bundle_id=$2 AND bucket_id=$3 AND lease_owner=$4 RETURNING object_key`, key, row.ID, target.ID, job.Owner).Scan(&job.Key); err != nil {
			return err
		}
	}
	if source.Bucket == target.Bucket && source.Endpoint == target.Endpoint && row.ObjectKey == job.Key {
		return errors.New("转存位置与主归档相同，请使用独立存储桶或前缀")
	}
	entries, err := store.BundleEntries(ctx, row.ID)
	if err != nil {
		return err
	}
	if len(entries) != row.EntryCount || len(entries) == 0 {
		return errors.New("主归档索引不完整")
	}
	size, err := transferRelay(ctx, from, to, row, job.Key, entries, redact && !row.StageRedacted, p.detector())
	if err != nil {
		return err
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE gateway_log_archive_deliveries SET status='uploaded',size_bytes=$1,last_error='',next_attempt_at=NULL,attempts=attempts+1,lease_owner='',lease_until=NULL WHERE bundle_id=$2 AND bucket_id=$3 AND lease_owner=$4 AND status<>'uploaded'`, size, row.ID, job.BucketID, job.Owner)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("转存任务已由其他节点接管")
	}
	if err = recordUploadStats(ctx, tx, job.BucketID, row.ModelKey, row.EntryCount, size); err != nil {
		return err
	}
	return tx.Commit()
}
