package wiring

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/gatewaylogarchive"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/setup"
	"github.com/gin-gonic/gin"
)

// ProvideGatewayLogArchive wires the archive runtime: host API key identity,
// the gateway's own session-id rules and the shared data directory.
func ProvideGatewayLogArchive(db *sql.DB, cipher service.SecretEncryptor, settingService *service.SettingService) *gatewaylogarchive.Handler {
	hooks := gatewaylogarchive.Hooks{
		Identity: func(c *gin.Context) gatewaylogarchive.Identity {
			apiKey, ok := middleware.GetAPIKeyFromContext(c)
			if !ok || apiKey == nil {
				return gatewaylogarchive.Identity{}
			}
			id := gatewaylogarchive.Identity{UserID: apiKey.UserID, APIKeyID: apiKey.ID}
			if apiKey.Group != nil {
				id.Platform = apiKey.Group.Platform
			}
			return id
		},
		HeaderSessionID: service.ExtractClientSessionID,
		MetadataSessionID: func(raw string) string {
			if parsed := service.ParseMetadataUserID(raw); parsed != nil {
				return parsed.SessionID
			}
			return ""
		},
	}
	runtime := gatewaylogarchive.NewRuntime(&gatewaylogarchive.Store{DB: db, Cipher: cipher}, settingService.CustomExtensions(), hooks, setup.GetDataDir())
	runtime.Start()
	return &gatewaylogarchive.Handler{Runtime: runtime}
}
