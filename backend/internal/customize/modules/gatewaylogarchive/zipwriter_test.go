package gatewaylogarchive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func readZip(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err, f.Name)
		require.NoError(t, rc.Close())
		out[f.Name] = string(data)
	}
	return out
}

func TestZipAppenderProducesStandardArchive(t *testing.T) {
	for _, force := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "a.zip")
		z, err := createZip(path)
		require.NoError(t, err)
		z.forceZip64 = force
		mod := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
		big := strings.Repeat(`{"role":"user","content":"hello"}`, 50000)
		_, err = z.Append(EntryName(1), mod, strings.NewReader(`{"a":1}`))
		require.NoError(t, err)
		e2, err := z.Append(EntryName(2), mod, strings.NewReader(big))
		require.NoError(t, err)
		_, err = z.Seal()
		require.NoError(t, err)

		files := readZip(t, path)
		require.Equal(t, `{"a":1}`, files["数据_000001.json"])
		require.Equal(t, big, files["数据_000002.json"])

		f, err := os.Open(path)
		require.NoError(t, err)
		rc, err := readEntry(io.NewSectionReader(f, e2.Offset, 1<<30), e2.CSize)
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.Equal(t, big, string(data))
		require.NoError(t, f.Close())
	}
}

func TestZipAppenderResumesAfterInterruptedAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.zip")
	z, err := createZip(path)
	require.NoError(t, err)
	mod := time.Now()
	_, err = z.Append(EntryName(1), mod, strings.NewReader(`{"n":1}`))
	require.NoError(t, err)
	committed, entries := z.Size(), append([]zipEntry(nil), z.entries...)
	// A crash mid-append leaves garbage after the committed length.
	_, err = z.f.WriteAt([]byte("PK\x03\x04garbage"), committed)
	require.NoError(t, err)
	require.NoError(t, z.Close())

	z, err = reopenZip(path, committed, entries)
	require.NoError(t, err)
	_, err = z.Append(EntryName(2), mod, strings.NewReader(`{"n":2}`))
	require.NoError(t, err)
	_, err = z.Seal()
	require.NoError(t, err)
	files := readZip(t, path)
	require.Len(t, files, 2)
	require.Equal(t, `{"n":2}`, files[EntryName(2)])

	_, err = reopenZip(path, committed+1<<20, entries)
	require.Error(t, err, "a file shorter than its committed size is rejected")
}

func TestZipAppenderRemoveLastRestoresConsistency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.zip")
	z, err := createZip(path)
	require.NoError(t, err)
	_, err = z.Append(EntryName(1), time.Now(), strings.NewReader(`{"keep":true}`))
	require.NoError(t, err)
	_, err = z.Append(EntryName(2), time.Now(), strings.NewReader(`{"drop":true}`))
	require.NoError(t, err)
	z.RemoveLast()
	require.Equal(t, 1, z.Count())
	_, err = z.Seal()
	require.NoError(t, err)
	files := readZip(t, path)
	require.Equal(t, map[string]string{EntryName(1): `{"keep":true}`}, files)
}
