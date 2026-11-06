package gatewaylogarchive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// memStore is an in-memory archiveStore with the SQL store's semantics.
type memStore struct {
	mu      sync.Mutex
	nextID  int64
	bundles map[int64]*Bundle
	entries map[int64][]Entry
}

func newMemStore() *memStore {
	return &memStore{bundles: map[int64]*Bundle{}, entries: map[int64][]Entry{}}
}

func (m *memStore) CreateBundle(_ context.Context, node, day, modelKey string, pathFor func(string) string) (Bundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	part := 1
	for _, b := range m.bundles {
		if b.Day == day && b.ModelKey == modelKey && b.Part >= part {
			part = b.Part + 1
		}
	}
	m.nextID++
	file := BundleFileName(modelKey, day, part)
	b := &Bundle{ID: m.nextID, NodeID: node, Day: day, ModelKey: modelKey, Part: part, FileName: file, LocalPath: pathFor(file), Status: StatusOpen, CreatedAt: time.Now()}
	m.bundles[b.ID] = b
	return *b, nil
}

func (m *memStore) CommitEntry(_ context.Context, size int64, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.bundles[e.BundleID]
	b.SizeBytes = size
	b.EntryCount++
	e.ID = int64(len(m.entries[e.BundleID]) + 1)
	m.entries[e.BundleID] = append(m.entries[e.BundleID], e)
	return nil
}

func (m *memStore) list(match func(*Bundle) bool) []Bundle {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Bundle
	for _, b := range m.bundles {
		if match(b) {
			out = append(out, *b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *memStore) OpenBundles(_ context.Context, node string) ([]Bundle, error) {
	return m.list(func(b *Bundle) bool { return b.Status == StatusOpen && b.NodeID == node }), nil
}

func (m *memStore) BundleEntries(_ context.Context, id int64) ([]zipEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []zipEntry
	for _, e := range m.entries[id] {
		out = append(out, zipEntry{Name: e.Name, Offset: e.Offset, CRC: e.CRC, CSize: e.CSize, USize: e.USize, Mod: e.EndedAt.UTC().Truncate(time.Second)})
	}
	return out, nil
}

func (m *memStore) update(id int64, fn func(*Bundle)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.bundles[id]; b != nil {
		fn(b)
	}
	return nil
}

func (m *memStore) MarkSealed(_ context.Context, id, size int64) error {
	return m.update(id, func(b *Bundle) { now := time.Now(); b.Status, b.SizeBytes, b.SealedAt = StatusSealed, size, &now })
}
func (m *memStore) MarkBroken(_ context.Context, id int64, reason string) error {
	return m.update(id, func(b *Bundle) { b.Status, b.LastError = StatusFailed, reason })
}
func (m *memStore) ResetUploading(_ context.Context, node string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.bundles {
		if b.Status == StatusUploading && b.NodeID == node {
			b.Status = StatusSealed
		}
	}
	return nil
}
func (m *memStore) ClaimUpload(_ context.Context, node string, _ time.Time) (*Bundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []int64
	for id, b := range m.bundles {
		if b.NodeID == node && b.Status == StatusSealed && !b.LocalDeleted {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	b := m.bundles[ids[0]]
	b.Status = StatusUploading
	b.Attempts++
	copied := *b
	return &copied, nil
}
func (m *memStore) ObjectKeyTaken(_ context.Context, bucket, key string, except int64) (bool, error) {
	return len(m.list(func(b *Bundle) bool { return b.Bucket == bucket && b.ObjectKey == key && b.ID != except })) > 0, nil
}
func (m *memStore) MarkUploaded(_ context.Context, id, bucketID int64, bucket, key string) error {
	return m.update(id, func(b *Bundle) {
		now := time.Now()
		bid := bucketID
		b.Status, b.Bucket, b.BucketID, b.ObjectKey, b.UploadedAt = StatusUploaded, bucket, &bid, key, &now
	})
}
func (m *memStore) MarkUploadFailed(_ context.Context, id int64, reason string, _ time.Time) error {
	return m.update(id, func(b *Bundle) { b.Status, b.LastError = StatusFailed, reason })
}
func (m *memStore) MarkLocalDeleted(_ context.Context, id int64) error {
	return m.update(id, func(b *Bundle) { b.LocalDeleted = true })
}
func (m *memStore) LocalExpired(context.Context, string, time.Time) ([]Bundle, error) {
	return nil, nil
}
func (m *memStore) Prune(context.Context, time.Time) error { return nil }

func (m *memStore) bundleList() []Bundle { return m.list(func(*Bundle) bool { return true }) }

// memObjects keeps uploaded objects in memory.
type memObjects struct {
	mu      sync.Mutex
	objects map[string][]byte
	fail    error
	name    string
	uploads int
}

func newMemObjects() *memObjects { return &memObjects{objects: map[string][]byte{}} }

func (o *memObjects) Bucket() string {
	if o.name != "" {
		return o.name
	}
	return "test-bucket"
}
func (o *memObjects) Exists(_ context.Context, key string) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.objects[key]
	return ok, nil
}
func (o *memObjects) Upload(_ context.Context, key, path string, progress func(int64)) error {
	o.mu.Lock()
	o.uploads++
	o.mu.Unlock()
	if o.fail != nil {
		return o.fail
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	o.mu.Lock()
	o.objects[key] = data
	o.mu.Unlock()
	progress(int64(len(data)))
	return nil
}
func (o *memObjects) Presign(context.Context, string, string, time.Duration) (string, error) {
	return "https://example.invalid/x", nil
}
func (o *memObjects) OpenRange(_ context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	data, ok := o.objects[key]
	if !ok {
		return nil, errors.New("missing")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), nil
}
func (o *memObjects) Test(context.Context) []TestStep { return []TestStep{{Name: "ok", OK: true}} }
