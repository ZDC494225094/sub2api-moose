package handler

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func openAIAccountSelectionUnavailableMessage(err error) string {
	if err == nil {
		return "Service temporarily unavailable"
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "Service temporarily unavailable"
	}

	messageLower := strings.ToLower(message)
	if errors.Is(err, service.ErrNoAvailableAccounts) ||
		errors.Is(err, service.ErrNoAvailableCompactAccounts) ||
		strings.Contains(messageLower, "no available openai accounts") ||
		strings.Contains(messageLower, "no available accounts supporting model") {
		if message == service.ErrNoAvailableAccounts.Error() {
			return "No available accounts"
		}
		return "No available accounts: " + message
	}

	return "Service temporarily unavailable"
}
