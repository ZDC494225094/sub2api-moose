package gatewaylogarchive

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (p *pipeline) defaultPrivacy() bool {
	cfg := p.cfg.Load()
	return cfg == nil || cfg.RedactPII
}

type stagingProbeResult struct {
	At    time.Time `json:"at"`
	Error string    `json:"error,omitempty"`
}

// A slow bundle must not prevent this node from testing its primary or
// accepting new staged fragments after the primary is reachable again.
func (p *pipeline) stagingProbeLoop() {
	defer p.workers.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stageWake:
		case <-ticker.C:
		}
		p.resumeStaging()
	}
}

func (p *pipeline) resumeStaging() {
	due := p.stageBackoff.Load()
	if due == 0 || time.Now().Unix() < due || p.ctx.Err() != nil {
		return
	}
	primary, enabled := primaryBucket(p.enabledBuckets())
	if enabled {
		ctx, cancel := context.WithTimeout(p.ctx, time.Minute)
		defer cancel()
		objects, err := p.bucketClient(primary)
		if err == nil {
			steps := objects.Test(ctx)
			if len(steps) == 0 {
				err = errors.New("RustFS 未返回连接测试结果")
			}
			for _, step := range steps {
				if !step.OK && step.Name != "访问存储桶" {
					err = fmt.Errorf("%s：%s", step.Name, step.Error)
					break
				}
			}
		}
		if p.ctx.Err() != nil {
			return
		}
		probe := &stagingProbeResult{At: time.Now()}
		if err != nil {
			probe.Error = err.Error()
		}
		p.stageProbe.Store(probe)
		if err != nil {
			p.stageBackoff.CompareAndSwap(due, probe.At.Add(30*time.Second).Unix())
			return
		}
	}
	if p.stageBackoff.CompareAndSwap(due, 0) {
		p.sealWhere(func(item *bundle) bool { return item.hasFallback.Load() })
	}
}

func (p *pipeline) bucketByID(id int64) (BucketConfig, error) {
	if list := p.buckets.Load(); list != nil {
		for _, bucket := range *list {
			if bucket.ID == id {
				return bucket, nil
			}
		}
	}
	return BucketConfig{}, fmt.Errorf("找不到存储桶配置 #%d", id)
}

func fallbackPath(row Bundle, key string) string {
	name := fmt.Sprintf("%x", sha256.Sum256([]byte(path.Base(key))))
	return filepath.Join(row.LocalPath, name[:2], name+".jsonl")
}

func writeIndexedLine(dst io.Writer, write func(io.Writer) error) (zipEntry, error) {
	checksum := crc32.NewIEEE()
	count := &countWriter{w: io.MultiWriter(dst, checksum)}
	buffer := bufio.NewWriterSize(count, 64<<10)
	if err := write(buffer); err != nil {
		return zipEntry{}, err
	}
	if err := buffer.Flush(); err != nil {
		return zipEntry{}, err
	}
	if _, err := io.WriteString(dst, "\n"); err != nil {
		return zipEntry{}, err
	}
	return zipEntry{USize: count.n, CSize: count.n, CRC: checksum.Sum32()}, nil
}

func localFragment(path string, produce func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	err = produce(file)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

func (p *pipeline) flushStaged(job flushJob, h sessionHeader, records []spoolRecord, b *bundle, primary BucketConfig) error {
	store := p.store.(*Store)
	event := fmt.Sprintf("%x", sha256.Sum256([]byte(p.node+"/"+filepath.Base(job.path))))
	var committed bool
	var entryCount, committedSize int64
	if err := store.DB.QueryRowContext(p.ctx, `SELECT entry_count,size_bytes,EXISTS(SELECT 1 FROM gateway_log_archive_entries WHERE stage_event=$1) FROM gateway_log_archive_bundles WHERE id=$2 AND status='open'`, event, b.row.ID).Scan(&entryCount, &committedSize, &committed); err != nil {
		return err
	}
	b.entries.Store(entryCount)
	b.size.Store(committedSize)
	if committed {
		return os.Remove(job.path)
	}
	seq := int(b.entries.Load()) + 1
	key := b.row.StagePrefix + "/" + event + ".jsonl"
	var indexed zipEntry
	produce := func(ctx context.Context, redact bool, dst io.Writer) error {
		doc, err := openDocument(job.path, h, records)
		if err != nil {
			return err
		}
		defer doc.Close()
		indexed, err = writeIndexedLine(dst, func(out io.Writer) error {
			if redact {
				return redactJSON(ctx, out, doc, true, p.detector())
			}
			return compactJSON(out, doc)
		})
		return err
	}
	var hint int64
	for _, record := range records {
		hint += int64(record.length)
	}
	remote := false
	if p.stageBackoff.Load() == 0 {
		objects, err := p.bucketClient(primary)
		if err == nil {
			// Each S3 request has its own timeout. A slow, advancing session
			// can span many parts; time spent on earlier parts is not a failure.
			err = objects.UploadStream(p.ctx, key, hint, func(out io.Writer) error { return produce(p.ctx, b.row.StageRedacted, out) }, nil)
		}
		if err == nil {
			remote = true
		} else {
			if p.ctx.Err() != nil {
				return p.ctx.Err()
			}
			if errors.Is(err, errDetector) {
				// RustFS is fine; keep the session spool and retry the flush.
				return err
			}
			p.stageBackoff.Store(time.Now().Add(30 * time.Second).Unix())
			p.report(fmt.Errorf("RustFS 写入失败，新会话暂存本地：%w", err))
		}
	}
	if !remote {
		if err := localFragment(fallbackPath(b.row, key), func(out io.Writer) error { return produce(p.ctx, false, out) }); err != nil {
			return fmt.Errorf("写入本地回退文件失败，保留会话缓存：%w", err)
		}
	}
	end := records[len(records)-1].at
	size := b.size.Load() + indexed.USize + 1
	err := store.CommitEntry(p.ctx, size, Entry{BundleID: b.row.ID, Seq: seq, Name: EntryName(seq), SessionID: h.SessionID, Model: h.Model, Platform: h.Platform, UserID: h.UserID, APIKeyID: h.APIKeyID, ExchangeCount: len(records), StartedAt: h.StartedAt, EndedAt: end, Offset: b.size.Load(), USize: indexed.USize, CSize: indexed.CSize, CRC: indexed.CRC, StagedRemote: remote, StageKey: key, StageEvent: event})
	if err != nil {
		return err
	}
	b.size.Store(size)
	b.entries.Add(1)
	if err := os.Remove(job.path); err != nil && !os.IsNotExist(err) {
		p.report(err)
	}
	if !remote {
		b.hasFallback.Store(true)
	}
	// The independent probe may have recovered while this local write was
	// in flight, before it could observe hasFallback in its scan.
	if size >= p.maxFile.Load() || (!remote && p.stageBackoff.Load() == 0) {
		p.sealLocked(b)
	}
	return nil
}

func copyVerified(ctx context.Context, dst io.Writer, src io.Reader, entry zipEntry, redact bool, det textDetector) error {
	limited := &io.LimitedReader{R: src, N: entry.USize}
	checksum := crc32.NewIEEE()
	reader := io.TeeReader(limited, checksum)
	var err error
	if redact {
		err = redactJSON(ctx, dst, reader, true, det)
	} else {
		_, err = io.Copy(dst, reader)
	}
	if err != nil {
		return err
	}
	if limited.N != 0 || checksum.Sum32() != entry.CRC {
		return fmt.Errorf("条目 %s 的长度或校验值不符", entry.Name)
	}
	return ctx.Err()
}

func (p *pipeline) sealStaged(row *Bundle) error {
	store := p.store.(*Store)
	primary, err := p.bucketByID(*row.StageBucketID)
	if err != nil {
		return err
	}
	objects, err := p.bucketClient(primary)
	if err != nil {
		return err
	}
	entries, err := store.BundleEntries(p.ctx, row.ID)
	if err != nil {
		return err
	}
	if len(entries) != row.EntryCount || len(entries) == 0 {
		return errors.New("RustFS 暂存索引不完整")
	}
	key, err := p.primaryKey(row, primary, objects)
	if err != nil {
		return err
	}
	redact := row.StageRedacted || primary.redact(p.defaultPrivacy())
	det := p.detector()
	layout := &uploadLayout{FileName: row.FileName, Entries: make([]zipEntry, 0, len(entries))}
	progress := &uploadProgress{BundleID: row.ID, FileName: row.FileName}
	progress.setPhase("读取暂存片段")
	p.uploading.Store(progress)
	defer p.uploading.Store(nil)
	uploadCtx := context.WithValue(p.ctx, uploadProgressKey{}, progress)
	err = objects.UploadStream(uploadCtx, key, row.SizeBytes, func(out io.Writer) error {
		for _, entry := range entries {
			if err := p.ctx.Err(); err != nil {
				return err
			}
			var src io.ReadCloser
			var err error
			if entry.StagedRemote {
				src, err = objects.OpenRange(p.ctx, entry.StageKey, 0, entry.USize)
			} else {
				src, err = os.Open(fallbackPath(*row, entry.StageKey))
			}
			if err != nil {
				return err
			}
			next, err := writeIndexedLine(out, func(dst io.Writer) error {
				return copyVerified(p.ctx, dst, src, entry, redact && !(entry.StagedRemote && row.StageRedacted), det)
			})
			_ = src.Close()
			if err != nil {
				return err
			}
			next.Offset = layout.Size
			layout.Size += next.USize + 1
			layout.Entries = append(layout.Entries, next)
		}
		return nil
	}, func(n int64) { progress.sent.Add(n) })
	if err != nil {
		if p.ctx.Err() == nil {
			p.stageBackoff.CompareAndSwap(0, time.Now().Add(30*time.Second).Unix())
		}
		return err
	}
	progress.setPhase("记录上传结果")
	if err := store.commitUploaded(p.ctx, row.ID, primary.ID, primary.Bucket, key, layout, p.deliveryTargets(row, primary), redact); err != nil {
		return err
	}
	p.wakeRelay()
	return nil
}

func (p *pipeline) primaryKey(row *Bundle, bucket BucketConfig, objects objectStore) (string, error) {
	if row.ObjectKey != "" {
		return row.ObjectKey, nil
	}
	key, err := p.chooseKey(p.ctx, bucket, objects, row)
	if err != nil {
		return "", err
	}
	err = p.store.(*Store).DB.QueryRowContext(p.ctx, `UPDATE gateway_log_archive_bundles SET object_key=CASE WHEN object_key='' THEN $1 ELSE object_key END WHERE id=$2 AND status='uploading' RETURNING object_key`, key, row.ID).Scan(&key)
	return key, err
}

func (p *pipeline) uploadLocalToPrimary(row *Bundle, bucket BucketConfig) error {
	prepared, err := p.prepareUpload(row, bucket.redact(p.defaultPrivacy()))
	if err != nil {
		return err
	}
	objects, err := p.bucketClient(bucket)
	if err != nil {
		return err
	}
	return p.uploadOne(bucket, objects, row, prepared)
}

func (p *pipeline) cleanupStaged(ctx context.Context, row Bundle) error {
	store := p.store.(*Store)
	expired := row.UploadedAt != nil && time.Since(*row.UploadedAt) >= time.Duration(p.keepLocal.Load())
	var objects objectStore
	for {
		rows, err := store.DB.QueryContext(ctx, `SELECT seq, stage_key FROM gateway_log_archive_entries WHERE bundle_id=$1 AND NOT stage_cleaned AND (staged_remote OR $2) ORDER BY seq LIMIT 100`, row.ID, expired)
		if err != nil {
			return err
		}
		type fragment struct {
			seq int
			key string
		}
		var batch []fragment
		for rows.Next() {
			var part fragment
			if err := rows.Scan(&part.seq, &part.key); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, part)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, part := range batch {
			if err := os.Remove(fallbackPath(row, part.key)); err != nil && !os.IsNotExist(err) {
				return err
			}
			if objects == nil {
				cfg, err := p.bucketByID(*row.StageBucketID)
				if err != nil {
					return err
				}
				objects, err = p.bucketClient(cfg)
				if err != nil {
					return err
				}
			}
			if err := objects.Delete(ctx, part.key); err != nil {
				return err
			}
			if err := store.exec(ctx, `UPDATE gateway_log_archive_entries SET stage_cleaned=true WHERE bundle_id=$1 AND seq=$2`, row.ID, part.seq); err != nil {
				return err
			}
		}
	}
	return store.exec(ctx, `UPDATE gateway_log_archive_bundles SET local_deleted=true WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM gateway_log_archive_entries WHERE bundle_id=$1 AND NOT stage_cleaned)`, row.ID)
}

func (s *Store) closeStagedBundle(ctx context.Context, id int64) error {
	if err := s.exec(ctx, `DELETE FROM gateway_log_archive_bundles WHERE id=$1 AND stage_bucket_id IS NOT NULL AND status='open' AND entry_count=0`, id); err != nil {
		return err
	}
	return s.exec(ctx, `UPDATE gateway_log_archive_bundles SET status='sealed', sealed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND stage_bucket_id IS NOT NULL AND status='open' AND entry_count>0`, id)
}
