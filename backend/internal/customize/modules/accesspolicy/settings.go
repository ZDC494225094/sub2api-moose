package accesspolicy

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const ModuleID = "access-policy"
const (
	EmailWhitelistKey      = "registration_email_suffix_whitelist"
	DomainQuotaKey         = "registration_email_domain_quota_enabled"
	ProofEnabledKey        = "registration_proof_enabled"
	ProofDifficultyKey     = "registration_proof_difficulty"
	MainlandRestrictionKey = "mainland_china_access_restriction_enabled"
)

var ErrSettingsUnavailable = infraerrors.ServiceUnavailable("ACCESS_POLICY_SETTINGS_UNAVAILABLE", "security settings are unavailable; please retry")
var ErrExtensionDisabled = infraerrors.Forbidden("CUSTOM_EXTENSION_DISABLED", "extension access-policy is disabled; existing security rules remain enforced")

// SettingsReader is intentionally independent of the host settings service.
type SettingsReader interface {
	GetMultiple(context.Context, []string) (map[string]string, error)
}
type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

type Settings struct {
	EmailWhitelist      []string
	DomainQuota         bool
	ProofEnabled        bool
	ProofDifficulty     int
	MainlandRestriction bool
}

func SettingsKeys() []string {
	return []string{EmailWhitelistKey, DomainQuotaKey, ProofEnabledKey, ProofDifficultyKey, MainlandRestrictionKey}
}
func ConfigurationKeys() []string {
	return []string{DomainQuotaKey, ProofEnabledKey, ProofDifficultyKey, MainlandRestrictionKey}
}

// Missing keys retain native defaults; failed reads and corrupt stored values
// must never be interpreted as permission to bypass an existing security rule.
func ReadSettings(ctx context.Context, reader SettingsReader) (Settings, error) {
	if reader == nil {
		return Settings{}, ErrSettingsUnavailable
	}
	values, err := reader.GetMultiple(ctx, SettingsKeys())
	if err != nil {
		return Settings{}, ErrSettingsUnavailable
	}
	return ParseSettings(values)
}

func ParseSettings(values map[string]string) (Settings, error) {
	out := Settings{ProofDifficulty: DefaultRegistrationProofDifficulty}
	for key, dest := range map[string]*bool{DomainQuotaKey: &out.DomainQuota, ProofEnabledKey: &out.ProofEnabled, MainlandRestrictionKey: &out.MainlandRestriction} {
		if value, exists := values[key]; exists {
			switch value {
			case "true":
				*dest = true
			case "false":
			default:
				return Settings{}, ErrSettingsUnavailable
			}
		}
	}
	if raw := values[ProofDifficultyKey]; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < RegistrationProofMinDifficulty || value > RegistrationProofMaxDifficulty {
			return Settings{}, ErrSettingsUnavailable
		}
		out.ProofDifficulty = value
	}
	if raw := strings.TrimSpace(values[EmailWhitelistKey]); raw != "" {
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return Settings{}, ErrSettingsUnavailable
		}
		var err error
		out.EmailWhitelist, err = NormalizeRegistrationEmailSuffixWhitelist(items)
		if err != nil {
			return Settings{}, ErrSettingsUnavailable
		}
	}
	return out, nil
}

// CheckConfigurationChange gates only new custom configuration, never enforcement.
// Native whitelist edits stay with the host. Omitted and unchanged values are
// allowed so ordinary settings saves cannot clear historical security rules.
func CheckConfigurationChange(ctx context.Context, reader SettingsReader, states StateReader, updates map[string]string) error {
	keys := ConfigurationKeys()
	relevant := false
	for _, key := range keys {
		if _, ok := updates[key]; ok {
			relevant = true
		}
	}
	if !relevant {
		return nil
	}
	if reader == nil {
		return ErrSettingsUnavailable
	}
	current, err := reader.GetMultiple(ctx, keys)
	if err != nil {
		return ErrSettingsUnavailable
	}
	changed := false
	for _, key := range keys {
		after, present := updates[key]
		if !present {
			continue
		}
		before, exists := current[key]
		if !exists {
			before = "false"
			if key == ProofDifficultyKey {
				before = strconv.Itoa(DefaultRegistrationProofDifficulty)
			}
		}
		if before != after {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if states == nil {
		return ErrExtensionDisabled
	}
	enabled, err := states.Enabled(ctx, ModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrExtensionDisabled
	}
	return nil
}
