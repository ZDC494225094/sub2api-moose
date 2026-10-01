package service

import (
	"context"
	"testing"

	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProvidePluginManagerWiresAccountDirectoryBeforeStart(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	manager := ProvidePluginManager(nil, nil, nil, PluginHostInfo{}, newFakePluginKVStore(), gateway)
	require.Same(t, gateway, manager.accountDirectory)
	require.False(t, manager.started, "directory must be available before plugin runtimes start")

	authorized := &PluginInstallation{PluginKey: "test.authorized", Manifest: PluginManifest{
		Capabilities: []PluginCapability{{ID: PluginCapabilityOpenAIOAuthOutbound, Platform: PlatformOpenAI, AccountType: AccountTypeOAuth}},
	}}
	server := manager.buildHostServices(authorized)
	require.NotNil(t, server)
	_, err := server.ListAccounts(context.Background(), &pluginv1.ListAccountsRequest{Platform: PlatformOpenAI, AccountType: AccountTypeOAuth})
	require.NoError(t, err, "authorized plugin must reach the wired directory, not Unavailable")

	unauthorized := &PluginInstallation{PluginKey: "test.other"}
	server = manager.buildHostServices(unauthorized)
	require.NotNil(t, server)
	_, err = server.ListAccounts(context.Background(), &pluginv1.ListAccountsRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err), "wiring must not broaden capability authorization")
	_, err = server.KVGet(context.Background(), &pluginv1.KVGetRequest{Namespace: "state", Key: "key"})
	require.NoError(t, err, "ordinary plugin storage must remain available")
}

func TestProvidePluginManagerDoesNotWireNilAccountDirectory(t *testing.T) {
	manager := ProvidePluginManager(nil, nil, nil, PluginHostInfo{}, nil, nil)
	require.Nil(t, manager.accountDirectory, "a typed nil gateway must not advertise account-directory availability")
}
