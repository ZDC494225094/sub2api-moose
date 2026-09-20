package handler

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRequestsAcceptUpstreamPlatforms(t *testing.T) {
	for _, platform := range []string{"kimi", "zhipu", "deepseek", "minimax", "opencode_go", "composite"} {
		t.Run(platform, func(t *testing.T) {
			require.NoError(t, binding.Validator.ValidateStruct(&CreateAPIKeyRequest{Name: "upstream", Platform: platform}))
			require.NoError(t, binding.Validator.ValidateStruct(&UpdateAPIKeyRequest{Platform: platform}))
		})
	}
	require.Error(t, binding.Validator.ValidateStruct(&CreateAPIKeyRequest{Name: "invalid", Platform: "unsupported"}))
	require.Error(t, binding.Validator.ValidateStruct(&UpdateAPIKeyRequest{Platform: "unsupported"}))
}
