package mediagateway

import (
	"context"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

type stateFunc func(context.Context, string) (bool, error)

func (f stateFunc) Enabled(ctx context.Context, id string) (bool, error) { return f(ctx, id) }
func TestAdmissionFailClosedAndDraining(t *testing.T) {
	for _, op := range []Operation{ReadExisting, CancelExisting} {
		require.NoError(t, CheckAdmission(context.Background(), stateFunc(func(context.Context, string) (bool, error) { t.Fatal("drain read live flag"); return false, nil }), op))
		require.NoError(t, CheckAdmission(context.Background(), nil, op))
	}
	require.Equal(t, 503, infraerrors.Code(CheckAdmission(context.Background(), nil, Submit)))
	require.Equal(t, 400, infraerrors.Code(CheckAdmission(context.Background(), nil, "unknown")))
	for _, enabled := range []bool{false, true} {
		err := CheckAdmission(context.Background(), stateFunc(func(_ context.Context, id string) (bool, error) { require.Equal(t, ModuleID, id); return enabled, nil }), Submit)
		if enabled {
			require.NoError(t, err)
		} else {
			require.Equal(t, 403, infraerrors.Code(err))
		}
	}
	boom := errors.New("store unavailable")
	require.ErrorIs(t, CheckAdmission(context.Background(), stateFunc(func(context.Context, string) (bool, error) { return true, boom }), Submit), boom)
}
func TestEndpointRegistry(t *testing.T) {
	for _, e := range []Endpoint{ImagesGenerations, ImagesEdits, VideosGenerations, VideosEdits, VideosExtensions, SeedanceCreate} {
		require.Equal(t, Submit, e.Operation())
		require.True(t, e.RequiresRequestBody())
		require.True(t, e.IsGenerationRequest())
		require.Equal(t, http.MethodPost, e.HTTPMethod())
	}
	for _, e := range []Endpoint{VideoStatus, VideoContent, SeedanceStatus} {
		require.Equal(t, ReadExisting, e.Operation())
		require.False(t, e.RequiresRequestBody())
		require.False(t, e.IsGenerationRequest())
		require.Equal(t, http.MethodGet, e.HTTPMethod())
	}
	require.Equal(t, CancelExisting, SeedanceDelete.Operation())
	require.Equal(t, http.MethodDelete, SeedanceDelete.HTTPMethod())
	require.Equal(t, Operation("unknown"), Endpoint("unknown").Operation())
	require.True(t, SeedanceCreate.IsSeedance())
	require.False(t, ImagesEdits.IsSeedance())
	for _, method := range []string{"GET", "HEAD"} {
		require.Equal(t, ReadExisting, VoiceOperation(method))
	}
	require.Equal(t, CancelExisting, VoiceOperation("DELETE"))
	for _, method := range []string{"POST", "PATCH", "PUT", ""} {
		require.Equal(t, Submit, VoiceOperation(method))
	}
	require.Equal(t, Submit, GeminiOperation("predictLongRunning"))
	for _, action := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
		require.Equal(t, ReadExisting, GeminiOperation(action))
	}
}
