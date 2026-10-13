package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
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
	bound := true
	preview := BuildChargeQuotaPreview(&relaycommon.RelayInfo{
		RelayMode:         relayconstant.RelayModeVideoSubmit,
		FactoryTokenBound: &bound,
	}, -2000, 500, 3000, false)

	require.True(t, preview.Allowed)
	require.Equal(t, "video_post_consume", preview.BillingMode)
	require.Equal(t, ChargeQuotaFailureNone, preview.FailureKind())
	require.Equal(t, -2500, preview.TokenQuotaAfterCharge)
	require.Equal(t, -5000, preview.UserQuotaAfterCharge)
}

func TestFactoryOverdraftRequiresMatchingEnabledBinding(t *testing.T) {
	truncate(t)
	seedUser(t, 901, 0)
	seedToken(t, 901, 901, "factory-overdraft-test", 5000)
	binding := &model.FactoryAccountBinding{
		FactoryUserId: 901, NewApiUserId: 901, TokenId: 901,
		Status: model.FactoryAccountBindingStatusEnabled, FactoryStatus: 1,
	}
	require.NoError(t, model.DB.Create(binding).Error)
	for _, tc := range []struct {
		name                                   string
		userId, tokenId, status, factoryStatus int
		allowed                                bool
	}{
		{"matching", 901, 901, 1, 1, true},
		{"different user", 902, 901, 1, 1, false},
		{"different token", 901, 902, 1, 1, false},
		{"disabled binding", 901, 901, 2, 1, false},
		{"disabled factory account", 901, 901, 1, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, model.DB.Model(binding).Updates(map[string]interface{}{
				"status": tc.status, "factory_status": tc.factoryStatus,
			}).Error)
			info := &relaycommon.RelayInfo{UserId: tc.userId, TokenId: tc.tokenId}
			require.Equal(t, tc.allowed, IsTokenOverdraftAllowed(info))
		})
	}
}

func TestOrdinaryUserZeroQuotaRejected(t *testing.T) {
	truncate(t)
	seedUser(t, 902, 0)
	seedToken(t, 902, 902, "ordinary-zero-test", 500000)
	ctx, _ := gin.CreateTestContext(nil)
	for _, mode := range []int{relayconstant.RelayModeChatCompletions, relayconstant.RelayModeVideoSubmit} {
		info := &relaycommon.RelayInfo{
			UserId: 902, TokenId: 902, TokenKey: "ordinary-zero-test",
			TokenUnlimited: true, RelayMode: mode,
		}
		preview := BuildChargeQuotaPreview(info, 0, 500000, 0, true)
		require.False(t, preview.Allowed)
		require.Equal(t, ChargeQuotaFailureUser, preview.FailureKind())
		session, apiErr := NewBillingSession(ctx, info, 0)
		require.Nil(t, session)
		require.NotNil(t, apiErr)
		require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
		require.NotNil(t, PreConsumeQuota(ctx, 0, info))
		require.Equal(t, 0, getUserQuota(t, 902))
		require.Equal(t, 500000, getTokenRemainQuota(t, 902))
	}
}

func TestOrdinaryUserPositiveQuotaStillAllowed(t *testing.T) {
	truncate(t)
	seedUser(t, 904, 1000)
	seedToken(t, 904, 904, "ordinary-positive-test", 5000)
	ctx, _ := gin.CreateTestContext(nil)
	for _, mode := range []int{relayconstant.RelayModeChatCompletions, relayconstant.RelayModeVideoSubmit} {
		info := &relaycommon.RelayInfo{
			UserId: 904, TokenId: 904, TokenKey: "ordinary-positive-test",
			TokenUnlimited: true, RelayMode: mode,
		}
		require.False(t, IsTokenOverdraftAllowed(info))
		require.True(t, BuildChargeQuotaPreview(info, 1000, 5000, 500, true).Allowed)
		session, apiErr := NewBillingSession(ctx, info, 0)
		require.Nil(t, apiErr)
		require.NotNil(t, session)
	}
}

func TestFactoryBoundZeroQuotaStillAllowsOverdraft(t *testing.T) {
	truncate(t)
	seedUser(t, 903, 0)
	seedToken(t, 903, 903, "factory-zero-test", 5000)
	require.NoError(t, model.DB.Create(&model.FactoryAccountBinding{
		FactoryUserId: 903, NewApiUserId: 903, TokenId: 903,
		Status: 1, FactoryStatus: 1,
	}).Error)
	info := &relaycommon.RelayInfo{UserId: 903, TokenId: 903, TokenKey: "factory-zero-test"}
	preview := BuildChargeQuotaPreview(info, 0, 5000, 500, false)
	require.True(t, preview.Allowed)
	ctx, _ := gin.CreateTestContext(nil)
	session, apiErr := NewBillingSession(ctx, info, 0)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, 0, getUserQuota(t, 903))
	require.Equal(t, 5000, getTokenRemainQuota(t, 903))
}

func TestBuildChargeQuotaPreviewRejectsVideoSubmitWithEmptyToken(t *testing.T) {
	preview := BuildChargeQuotaPreview(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeVideoSubmit,
	}, 10000, 0, 3000, false)

	require.False(t, preview.Allowed)
	require.Equal(t, "video_post_consume", preview.BillingMode)
	require.Equal(t, ChargeQuotaFailureToken, preview.FailureKind())
}
