package gatewaylogarchive

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	maxSessionFragments         = 10000
	maxSessionExportBytes int64 = 2 << 30
	maxSessionExchanges         = 100000
)

type sessionFragment struct {
	Entry  Entry
	Bundle Bundle
}

// SessionFragments uses shared SQL indexes, not the receiving node's local
// session map. User and API key scope prevent merging unrelated tenants that
// happen to use the same client session string; models/days are not barriers.
func (s *Store) SessionFragments(ctx context.Context, anchor Entry) ([]sessionFragment, error) {
	if strings.TrimSpace(anchor.SessionID) == "" {
		return nil, errors.New("会话 ID 为空，不能可靠合并")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+entryColumns+`, b.node_id, b.local_path, b.bucket, b.bucket_id, b.object_key, b.local_deleted
		FROM gateway_log_archive_entries e JOIN gateway_log_archive_bundles b ON b.id = e.bundle_id
		WHERE e.session_id = $1 AND e.user_id = $2 AND e.api_key_id = $3
		ORDER BY e.started_at, e.id LIMIT $4`, anchor.SessionID, anchor.UserID, anchor.APIKeyID, maxSessionFragments+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []sessionFragment
	for rows.Next() {
		var f sessionFragment
		var crc int64
		var bucketID sql.NullInt64
		e, b := &f.Entry, &f.Bundle
		args := entryScanTargets(e, &crc)
		args = append(args, &b.NodeID, &b.LocalPath, &b.Bucket, &bucketID, &b.ObjectKey, &b.LocalDeleted)
		if err := rows.Scan(args...); err != nil {
			return nil, err
		}
		e.CRC = uint32(crc)
		b.ID, b.FileName, b.Day, b.Status = e.BundleID, e.FileName, e.Day, e.BundleStatus
		if bucketID.Valid {
			v := bucketID.Int64
			b.BucketID = &v
		}
		result = append(result, f)
	}
	if len(result) > maxSessionFragments {
		return nil, errors.New("会话片段超过导出上限，请离线合并")
	}
	return result, rows.Err()
}

type sessionExportSummary struct {
	Scope              string   `json:"scope"`
	FragmentCount      int      `json:"fragment_count"`
	NodeCount          int      `json:"node_count"`
	ExchangeCount      int      `json:"exchange_count"`
	DuplicateCount     int      `json:"duplicate_count"`
	AssistantTurns     int      `json:"assistant_turns"`
	ValidToolCalls     int      `json:"valid_tool_calls"`
	Signatures         int      `json:"signatures"`
	TruncatedExchanges int      `json:"truncated_exchanges"`
	Warnings           []string `json:"warnings"`
}

func (s *sessionExportSummary) warn(value string) {
	for _, w := range s.Warnings {
		if w == value {
			return
		}
	}
	s.Warnings = append(s.Warnings, value)
}

type exportExchange struct {
	Offset    int64
	Length    int64
	Start     time.Time
	Finish    time.Time
	RequestID string
	EntryID   int64
	Ordinal   int
}
type sessionExportFile struct {
	File    *os.File
	Summary sessionExportSummary
	cleanup func()
}

func (f *sessionExportFile) Close() { _ = f.File.Close(); f.cleanup() }

func (r *Runtime) ExportSession(ctx context.Context, anchor Entry) (*sessionExportFile, error) {
	if !r.exportBusy.CompareAndSwap(false, true) {
		return nil, errors.New("本节点已有会话正在合并导出，请稍后重试")
	}
	defer r.exportBusy.Store(false)
	fragments, err := r.Store.SessionFragments(ctx, anchor)
	if err != nil {
		return nil, err
	}
	return buildSessionExport(ctx, r.dir(), fragments, r.OpenEntry)
}

// A prepared file keeps HTTP error handling atomic and bounds memory to one
// exchange. Two private scratch files hold original exchanges and sorted output.
// Failures never return a partial export that looks complete.
func buildSessionExport(ctx context.Context, dir string, fragments []sessionFragment, open func(context.Context, Entry, Bundle) (io.ReadCloser, error)) (*sessionExportFile, error) {
	if len(fragments) == 0 {
		return nil, errors.New("没有可合并的会话片段")
	}
	if len(fragments) > maxSessionFragments {
		return nil, errors.New("会话片段超过导出上限")
	}
	var total int64
	nodes := map[string]bool{}
	var pending int
	for _, f := range fragments {
		if f.Bundle.Status != StatusUploaded {
			pending++
		}
		if f.Entry.USize < 0 || f.Entry.USize > maxSessionExportBytes-total {
			return nil, errors.New("会话超过 2 GB 在线导出上限，请离线合并；本次未截断导出")
		}
		total += f.Entry.USize
		nodes[f.Bundle.NodeID] = true
	}
	if pending > 0 {
		return nil, fmt.Errorf("该会话有 %d 个片段尚未上传（可能分布在其他节点），请等待各节点上传后再合并；未返回不完整结果", pending)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	spool, err := os.CreateTemp(dir, "session-export-source-*.tmp")
	if err != nil {
		return nil, err
	}
	defer func() { _ = spool.Close(); _ = os.Remove(spool.Name()) }()
	_ = spool.Chmod(0o600)
	summary := sessionExportSummary{Scope: "uploaded_indexed_fragments", FragmentCount: len(fragments), NodeCount: len(nodes), Warnings: []string{}}
	// Active spools, filtered/never captured requests and pruned indexes cannot
	// be proven absent, even if every currently indexed fragment is uploaded.
	summary.warn("active_or_unindexed_requests_not_included")
	seen := map[[sha256.Size]byte]bool{}
	var positions []exportExchange
	var offset int64
	for _, fragment := range fragments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rc, err := open(ctx, fragment.Entry, fragment.Bundle)
		if err != nil {
			return nil, fmt.Errorf("读取会话片段 #%d 失败：%w", fragment.Entry.ID, err)
		}
		count := 0
		err = readSessionExchanges(io.LimitReader(rc, fragment.Entry.USize+1), func(raw json.RawMessage) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			count++
			root := gjson.ParseBytes(raw)
			start, err := time.Parse(time.RFC3339Nano, root.Get("started_at").String())
			if err != nil {
				start = fragment.Entry.StartedAt
				summary.warn("exchange_timestamp_missing")
			}
			finish, _ := time.Parse(time.RFC3339Nano, root.Get("finished_at").String())
			requestID := root.Get("request_id").String()
			// Only byte-equivalent exchanges are deduplicated. The same request
			// id with a different response remains a distinct retry/attempt.
			sum := sha256.Sum256(raw)
			if seen[sum] {
				summary.DuplicateCount++
				return nil
			}
			seen[sum] = true
			if len(positions) >= maxSessionExchanges {
				return errors.New("会话超过 100000 轮在线导出上限，请离线合并")
			}
			n, err := spool.Write(raw)
			if err != nil {
				return err
			}
			positions = append(positions, exportExchange{Offset: offset, Length: int64(n), Start: start, Finish: finish, RequestID: requestID, EntryID: fragment.Entry.ID, Ordinal: count})
			offset += int64(n)
			return nil
		})
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("解析会话片段 #%d 失败：%w", fragment.Entry.ID, err)
		}
		if count != fragment.Entry.ExchangeCount {
			return nil, fmt.Errorf("会话片段 #%d 轮次不匹配，拒绝部分导出", fragment.Entry.ID)
		}
	}
	if len(positions) == 0 {
		return nil, errors.New("会话没有可导出的请求轮次")
	}
	sort.SliceStable(positions, func(i, j int) bool {
		a, b := positions[i], positions[j]
		if !a.Start.Equal(b.Start) {
			return a.Start.Before(b.Start)
		}
		if a.RequestID != b.RequestID {
			return a.RequestID < b.RequestID
		}
		if a.EntryID != b.EntryID {
			return a.EntryID < b.EntryID
		}
		return a.Ordinal < b.Ordinal
	})
	output, err := os.CreateTemp(dir, "session-export-*.json")
	if err != nil {
		return nil, err
	}
	_ = output.Chmod(0o600)
	ok := false
	defer func() {
		if !ok {
			_ = output.Close()
			_ = os.Remove(output.Name())
		}
	}()
	w := bufio.NewWriterSize(&exportLimitWriter{Writer: output, remaining: maxSessionExportBytes}, 64<<10)
	anchor := fragments[0].Entry
	meta := map[string]any{"format": "sub2api.gateway-session.v1", "correlation_id": sessionCorrelationID(anchor.APIKeyID, anchor.SessionID), "scope": "uploaded_indexed_fragments", "exchange_count": len(positions)}
	// The original session string may contain PII; use the same non-PII join key
	// exported by each new fragment instead of leaking SQL's raw session id.
	prefix := rawJSON(meta)
	if _, err = w.Write(append(prefix[:len(prefix)-1], `,"exchanges":[`...)); err != nil {
		return nil, err
	}
	calls, results, responseIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var previousIDs []string
	requestIDs := map[string]bool{}
	for i, pos := range positions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw := make([]byte, int(pos.Length))
		if _, err = spool.ReadAt(raw, pos.Offset); err != nil {
			return nil, err
		}
		a := analyzeExchange(raw)
		summary.AssistantTurns += a.AssistantTurns
		summary.Signatures += a.Signatures
		if a.CaptureTruncated {
			summary.TruncatedExchanges++
		}
		for _, warning := range a.Warnings {
			summary.warn(warning)
		}
		for _, call := range a.ToolCalls {
			if call.Valid {
				if call.ID == "" || !calls[call.ID] {
					summary.ValidToolCalls++
				}
				if call.ID != "" {
					calls[call.ID] = true
				}
			}
		}
		for _, id := range a.ToolResults {
			if id != "" {
				results[id] = true
			}
		}
		if a.ResponseID != "" {
			responseIDs[a.ResponseID] = true
		}
		if a.PreviousResponseID != "" {
			previousIDs = append(previousIDs, a.PreviousResponseID)
		}
		if pos.RequestID != "" {
			if requestIDs[pos.RequestID] {
				summary.warn("repeated_request_id_with_distinct_payload")
			}
			requestIDs[pos.RequestID] = true
		}
		if i > 0 && positions[i-1].Finish.After(pos.Start) {
			summary.warn("concurrent_exchanges_ordered_by_start_time")
		}
		if i > 0 {
			if err = w.WriteByte(','); err != nil {
				return nil, err
			}
		}
		if _, err = w.Write(addAnalysis(raw, a)); err != nil {
			return nil, err
		}
	}
	for id := range results {
		if !calls[id] {
			summary.warn("tool_result_without_matching_call")
		}
	}
	for id := range calls {
		if !results[id] {
			summary.warn("tool_call_without_result_in_snapshot")
		}
	}
	for _, id := range previousIDs {
		if !responseIDs[id] {
			summary.warn("previous_response_not_in_snapshot")
		}
	}
	summary.ExchangeCount = len(positions)
	if summary.AssistantTurns == 0 {
		summary.warn("no_assistant_output_observed")
	}
	if _, err = w.WriteString(`],"summary":`); err != nil {
		return nil, err
	}
	if _, err = w.Write(rawJSON(summary)); err != nil {
		return nil, err
	}
	if _, err = w.WriteString("}\n"); err != nil {
		return nil, err
	}
	if err = w.Flush(); err != nil {
		return nil, err
	}
	if _, err = output.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return &sessionExportFile{File: output, Summary: summary, cleanup: func() { _ = os.Remove(output.Name()) }}, nil
}

type exportLimitWriter struct {
	io.Writer
	remaining int64
}

func (w *exportLimitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, errors.New("合并后的文件超过 2 GB 在线导出上限，请离线合并；本次未截断导出")
	}
	n, err := w.Writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func readSessionExchanges(r io.Reader, visit func(json.RawMessage) error) error {
	d := json.NewDecoder(r)
	d.UseNumber()
	start, err := d.Token()
	if err != nil {
		return err
	}
	if start != json.Delim('{') {
		return errors.New("session document must be an object")
	}
	found := false
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return err
		}
		if key == "exchanges" {
			if found {
				return errors.New("duplicate exchanges field")
			}
			found = true
			token, err := d.Token()
			if err != nil {
				return err
			}
			if token != json.Delim('[') {
				return errors.New("exchanges must be an array")
			}
			for d.More() {
				var raw json.RawMessage
				if err := d.Decode(&raw); err != nil {
					return err
				}
				if !gjson.ParseBytes(raw).IsObject() {
					return errors.New("exchange must be an object")
				}
				if err := visit(raw); err != nil {
					return err
				}
			}
			if _, err := d.Token(); err != nil {
				return err
			}
		} else {
			var ignored json.RawMessage
			if err := d.Decode(&ignored); err != nil {
				return err
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if !found {
		return errors.New("session exchanges missing")
	}
	var tail json.RawMessage
	if err := d.Decode(&tail); err != io.EOF {
		return errors.New("trailing session data")
	}
	return nil
}

func sessionExportName(e Entry) string {
	return "session-" + sessionCorrelationID(e.APIKeyID, e.SessionID) + ".json"
}
