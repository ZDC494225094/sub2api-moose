package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type groupBillingRateSyncAccountRepo struct {
	AccountRepository
	accounts map[int64]*Account
}

func (r *groupBillingRateSyncAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if account := r.accounts[id]; account != nil {
		return account, nil
	}
	return nil, ErrAccountNotFound
}

func TestNormalizeGroupBillingRateSyncConfig(t *testing.T) {
	apiKeyID := int64(17)
	oauthID := int64(18)
	svc := &adminServiceImpl{accountRepo: &groupBillingRateSyncAccountRepo{accounts: map[int64]*Account{
		apiKeyID: {ID: apiKeyID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		oauthID:  {ID: oauthID, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	}}}

	accountID, markup, err := svc.normalizeGroupBillingRateSyncConfig(context.Background(), PlatformOpenAI, &apiKeyID, 0.1)
	require.NoError(t, err)
	require.Equal(t, &apiKeyID, accountID)
	require.Equal(t, 0.1, markup)

	_, _, err = svc.normalizeGroupBillingRateSyncConfig(context.Background(), PlatformOpenAI, &oauthID, 0.1)
	require.ErrorContains(t, err, "reference account must be an OpenAI API key account")

	_, _, err = svc.normalizeGroupBillingRateSyncConfig(context.Background(), PlatformOpenAI, &apiKeyID, -0.1)
	require.ErrorContains(t, err, "billing_rate_markup must be a finite number >= 0")

	accountID, markup, err = svc.normalizeGroupBillingRateSyncConfig(context.Background(), PlatformAnthropic, &apiKeyID, 0.1)
	require.NoError(t, err)
	require.Nil(t, accountID)
	require.Zero(t, markup)
}
