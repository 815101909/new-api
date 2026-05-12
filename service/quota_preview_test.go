package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestGetTokenOverdraftQuotaLimitUsesFiveCNYDefault(t *testing.T) {
	t.Setenv("AIGC_FACTORY_TOKEN_OVERDRAFT_CNY", "")
	t.Setenv("NEW_API_TOKEN_OVERDRAFT_CNY", "")

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	require.Equal(t, 2500000, GetTokenOverdraftQuotaLimit())
}

func TestGetTokenOverdraftQuotaLimitPrefersEnvOverride(t *testing.T) {
	t.Setenv("AIGC_FACTORY_TOKEN_OVERDRAFT_CNY", "3.5")
	t.Setenv("NEW_API_TOKEN_OVERDRAFT_CNY", "")

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	require.Equal(t, 1750000, GetTokenOverdraftQuotaLimit())
}

func TestBuildChargeQuotaPreviewAllowsVideoSubmitWithPositiveToken(t *testing.T) {
	preview := BuildChargeQuotaPreview(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeVideoSubmit,
	}, -2000, 500, 3000, false)

	require.True(t, preview.Allowed)
	require.Equal(t, "video_post_consume", preview.BillingMode)
	require.Equal(t, ChargeQuotaFailureNone, preview.FailureKind())
	require.Equal(t, -2500, preview.TokenQuotaAfterCharge)
	require.Equal(t, -5000, preview.UserQuotaAfterCharge)
}

func TestBuildChargeQuotaPreviewRejectsVideoSubmitWithEmptyToken(t *testing.T) {
	preview := BuildChargeQuotaPreview(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeVideoSubmit,
	}, 10000, 0, 3000, false)

	require.False(t, preview.Allowed)
	require.Equal(t, "video_post_consume", preview.BillingMode)
	require.Equal(t, ChargeQuotaFailureToken, preview.FailureKind())
}
