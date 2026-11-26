package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIPFromContext(t *testing.T) {
	ctx := WithSessionBinding(context.Background(), &SessionBinding{IP: " 203.0.113.7 "})
	addr, ok := ClientIPFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "203.0.113.7", addr)

	_, ok = ClientIPFromContext(context.Background())
	require.False(t, ok)

	ctx = WithSessionBinding(context.Background(), &SessionBinding{IP: "not-an-ip"})
	_, ok = ClientIPFromContext(ctx)
	require.False(t, ok)

	ctx = WithSessionBinding(context.Background(), &SessionBinding{IP: "2001:db8::1"})
	addr, ok = ClientIPFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "2001:db8::1", addr)
}
