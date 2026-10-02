package mediagateway

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"net/http"
)

const ModuleID = "media-gateway"

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}
type Operation string

const (
	Submit         Operation = "submit"
	ReadExisting   Operation = "read-existing"
	CancelExisting Operation = "cancel-existing"
)

// CheckAdmission is called once before scheduling/forwarding. Draining an existing
// task never reads the flag: outages or disablement must not strand its billing.
func CheckAdmission(ctx context.Context, states StateReader, operation Operation) error {
	switch operation {
	case ReadExisting, CancelExisting:
		return nil
	case Submit:
	default:
		return infraerrors.BadRequest("INVALID_MEDIA_OPERATION", "unknown media operation")
	}
	if states == nil {
		return infraerrors.ServiceUnavailable("CUSTOM_EXTENSION_STATE_UNAVAILABLE", "extension state is unavailable; please retry")
	}
	enabled, err := states.Enabled(ctx, ModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return infraerrors.Forbidden("CUSTOM_EXTENSION_DISABLED", "extension media-gateway is disabled; existing tasks remain accessible")
	}
	return nil
}

func VoiceOperation(method string) Operation {
	switch method {
	case http.MethodGet, http.MethodHead:
		return ReadExisting
	case http.MethodDelete:
		return CancelExisting
	default:
		return Submit
	}
}

func GeminiOperation(action string) Operation {
	if action == "predictLongRunning" {
		return Submit
	}
	return ReadExisting // native text/model/token APIs are not owned by this plugin
}
