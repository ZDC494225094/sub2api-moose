package gatewaylogarchive

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJSONLAppenderKeepsJSONValuesAndOneLinePerSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.jsonl")
	j, err := createJSONL(path)
	require.NoError(t, err)
	first := "{\n  \"n\": 9007199254740993, \"text\": \"line1\\nline2\\\"\\\\\", \"n\": 1.2300e+05\n}"
	e1, err := j.Append(EntryName(1), time.Now(), strings.NewReader(first))
	require.NoError(t, err)
	// A line larger than bufio.Scanner's default must remain a single record.
	second := `{"text":"` + strings.Repeat("原始 <>& ", 20000) + `"}`
	e2, err := j.Append(EntryName(2), time.Now(), strings.NewReader(second))
	require.NoError(t, err)
	size, err := j.Seal()
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, int64(len(raw)), size)
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	require.Len(t, lines, 2)
	var expected bytes.Buffer
	require.NoError(t, json.Compact(&expected, []byte(first)))
	require.Equal(t, expected.String(), string(lines[0]))
	require.Equal(t, second, string(lines[1]))
	require.Equal(t, int64(len(lines[0])+1), e2.Offset)
	require.Equal(t, e1.CSize, e1.USize)
	require.Equal(t, lines[1], raw[e2.Offset:e2.Offset+e2.USize])
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("source interrupted") }

func TestJSONLAppenderRecoversCommittedLengthAndRollsBackErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.jsonl")
	j, err := createJSONL(path)
	require.NoError(t, err)
	_, err = j.Append(EntryName(1), time.Now(), strings.NewReader(`{"keep":true}`))
	require.NoError(t, err)
	committed := j.Size()
	_, err = j.Append(EntryName(2), time.Now(), io.MultiReader(strings.NewReader(`{"partial":`), failingReader{}))
	require.Error(t, err)
	require.Equal(t, committed, j.Size())
	_, err = j.Append(EntryName(2), time.Now(), strings.NewReader(`{"rollback":true}`))
	require.NoError(t, err)
	j.RemoveLast()
	require.Equal(t, 1, j.Count())
	require.Equal(t, committed, j.Size())
	_, err = j.f.WriteAt([]byte(`{"crash":`), committed)
	require.NoError(t, err)
	require.NoError(t, j.Close())
	j, err = reopenJSONL(path, committed, 1)
	require.NoError(t, err)
	_, err = j.Append(EntryName(2), time.Now(), strings.NewReader(`{"resume":true}`))
	require.NoError(t, err)
	_, err = j.Seal()
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{\"keep\":true}\n{\"resume\":true}\n", string(raw))
	_, err = reopenJSONL(path, int64(len(raw)+1), 2)
	require.Error(t, err)
}
