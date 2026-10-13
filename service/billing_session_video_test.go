package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBillingSessionVideoSubmitSkipsPreConsumeAndSettlesByActualQuota(t *testing.T) {
	truncate(t)

	const userID = 11
	const tokenID = 11
	const tokenRemain = 1000
	const actualQuota = 5000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-video-test", tokenRemain)
	require.NoError(t, model.DB.Create(&model.FactoryAccountBinding{
		FactoryUserId: userID, NewApiUserId: userID, TokenId: tokenID,
		Status: 1, FactoryStatus: 1,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("token_quota", tokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:    userID,
		TokenId:   tokenID,
		TokenKey:  "sk-video-test",
		RelayMode: relayconstant.RelayModeVideoSubmit,
		RequestId: "video-post-consume-test",
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, actualQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, 0, session.GetPreConsumedQuota())
	assert.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
	assert.Equal(t, 0, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	require.NoError(t, session.Settle(actualQuota))

	assert.Equal(t, -actualQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-actualQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
}
