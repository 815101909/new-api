package controller

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type factoryUsageBindingPayload struct {
	FactoryUserId   int    `json:"factory_user_id"`
	FactoryUsername string `json:"factory_username"`
	NewApiUserId    int    `json:"new_api_user_id"`
	NewApiUsername  string `json:"new_api_username"`
	TokenId         int    `json:"token_id"`
	TokenName       string `json:"token_name"`
}

type factoryUsageTarget struct {
	TokenId int
	Bound   bool
	Binding *factoryUsageBindingPayload
}

type factoryUsageBindTokenRequest struct {
	FactoryUserId   int    `json:"factory_user_id"`
	FactoryUsername string `json:"factory_username"`
	ApiKey          string `json:"api_key"`
}

const defaultFactoryUsageSyncSecret = "dev-new-api-sync-secret"

func getFactoryUsageSyncSecret() string {
	if secret := strings.TrimSpace(os.Getenv("AIGC_FACTORY_NEW_API_SYNC_SECRET")); secret != "" {
		return secret
	}
	if secret := strings.TrimSpace(os.Getenv("AIGC_FACTORY_SYNC_SECRET")); secret != "" {
		return secret
	}
	return defaultFactoryUsageSyncSecret
}

func assertFactoryUsageSecret(c *gin.Context) bool {
	expected := getFactoryUsageSyncSecret()
	actual := strings.TrimSpace(c.GetHeader("X-New-API-Sync-Secret"))
	if expected == "" || actual != expected {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "New API 用量同步密钥无效",
		})
		return false
	}
	return true
}

func parseFactoryUsageTimestamp(value string) int64 {
	timestamp, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if timestamp > 1000000000000 {
		return timestamp / 1000
	}
	return timestamp
}

func normalizeFactoryUsageApiKey(value string) string {
	apiKey := strings.TrimSpace(value)
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	apiKey = strings.TrimPrefix(apiKey, "bearer ")
	apiKey = strings.TrimPrefix(apiKey, "sk-")
	if strings.Contains(apiKey, "-") {
		return strings.Split(apiKey, "-")[0]
	}
	return apiKey
}

func buildFactoryUsageBindingPayload(binding *model.FactoryAccountBinding) factoryUsageBindingPayload {
	if binding == nil {
		return factoryUsageBindingPayload{}
	}
	return factoryUsageBindingPayload{
		FactoryUserId:   binding.FactoryUserId,
		FactoryUsername: binding.FactoryUsername,
		NewApiUserId:    binding.NewApiUserId,
		NewApiUsername:  binding.NewApiUsername,
		TokenId:         binding.TokenId,
		TokenName:       getFactoryUsageTokenName(binding.TokenId),
	}
}

func getFactoryUsageTokenName(tokenId int) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil || token == nil {
		return ""
	}
	return token.Name
}

func getFactoryUsageTarget(c *gin.Context) (*factoryUsageTarget, bool) {
	factoryUserId, _ := strconv.Atoi(c.Query("factory_user_id"))
	if factoryUserId <= 0 {
		common.ApiErrorMsg(c, "factory_user_id 不能为空")
		return nil, false
	}
	binding, err := model.GetFactoryAccountBindingByFactoryUserId(factoryUserId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
			return nil, false
		}
		return getFactoryUsageTargetByTokenName(c, factoryUserId)
	}
	if binding.TokenId <= 0 {
		return getFactoryUsageTargetByTokenName(c, factoryUserId)
	}
	payload := buildFactoryUsageBindingPayload(binding)
	return &factoryUsageTarget{
		TokenId: binding.TokenId,
		Bound:   true,
		Binding: &payload,
	}, true
}

func getFactoryUsageTargetByTokenName(c *gin.Context, factoryUserId int) (*factoryUsageTarget, bool) {
	factoryUsername := strings.TrimSpace(c.Query("factory_username"))
	if factoryUsername == "" {
		return nil, true
	}
	token, err := model.GetTokenByName(factoryUsername)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true
		}
		common.ApiError(c, err)
		return nil, false
	}
	newApiUsername, _ := model.GetUsernameById(token.UserId, false)
	payload := factoryUsageBindingPayload{
		FactoryUserId:   factoryUserId,
		FactoryUsername: factoryUsername,
		NewApiUserId:    token.UserId,
		NewApiUsername:  newApiUsername,
		TokenId:         token.Id,
		TokenName:       token.Name,
	}
	return &factoryUsageTarget{
		TokenId: token.Id,
		Bound:   true,
		Binding: &payload,
	}, true
}

func BindFactoryUsageToken(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	req := factoryUsageBindTokenRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	factoryUserId := req.FactoryUserId
	factoryUsername := strings.TrimSpace(req.FactoryUsername)
	apiKey := normalizeFactoryUsageApiKey(req.ApiKey)
	if factoryUserId <= 0 || factoryUsername == "" {
		common.ApiErrorMsg(c, "创作工厂账号信息不能为空")
		return
	}
	if apiKey == "" {
		common.ApiErrorMsg(c, "New API 令牌不能为空")
		return
	}
	token, err := model.ValidateUserToken(apiKey)
	if err != nil || token == nil {
		common.ApiErrorMsg(c, "New API 令牌无效或不可用")
		return
	}
	newApiUsername, err := model.GetUsernameById(token.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Name != factoryUsername {
		token.Name = factoryUsername
		if err := token.Update(); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	binding := &model.FactoryAccountBinding{
		FactoryUserId:      factoryUserId,
		FactoryUsername:    factoryUsername,
		FactoryDisplayName: factoryUsername,
		FactoryStatus:      1,
		NewApiUserId:       token.UserId,
		NewApiUsername:     newApiUsername,
		TokenId:            token.Id,
		Status:             model.FactoryAccountBindingStatusEnabled,
		LastSyncedAt:       common.GetTimestamp(),
	}
	if err := model.SaveFactoryAccountBinding(binding); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"bound":   true,
		"binding": buildFactoryUsageBindingPayload(binding),
	})
}

func GetFactoryUsageLogs(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	pageInfo := common.GetPageQuery(c)
	target, ok := getFactoryUsageTarget(c)
	if !ok {
		return
	}
	if target == nil || !target.Bound {
		pageInfo.SetTotal(0)
		pageInfo.SetItems([]*model.Log{})
		common.ApiSuccess(c, gin.H{
			"bound":             false,
			"binding":           nil,
			"quota_per_unit":    common.QuotaPerUnit,
			"usd_exchange_rate": operation_setting.USDExchangeRate,
			"page":              pageInfo,
		})
		return
	}

	logType := model.LogTypeConsume
	if typeValue := strings.TrimSpace(c.Query("type")); typeValue != "" {
		logType, _ = strconv.Atoi(typeValue)
	}
	startTimestamp := parseFactoryUsageTimestamp(c.Query("start_timestamp"))
	endTimestamp := parseFactoryUsageTimestamp(c.Query("end_timestamp"))
	modelName := strings.TrimSpace(c.Query("model_name"))
	group := strings.TrimSpace(c.Query("group"))
	requestId := strings.TrimSpace(c.Query("request_id"))
	logs, total, err := model.GetLogsByTokenId(
		target.TokenId,
		logType,
		startTimestamp,
		endTimestamp,
		modelName,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		group,
		requestId,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, gin.H{
		"bound":             true,
		"binding":           target.Binding,
		"quota_per_unit":    common.QuotaPerUnit,
		"usd_exchange_rate": operation_setting.USDExchangeRate,
		"page":              pageInfo,
	})
}

func GetFactoryUsageStat(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	target, ok := getFactoryUsageTarget(c)
	if !ok {
		return
	}
	if target == nil || !target.Bound {
		common.ApiSuccess(c, gin.H{
			"bound":             false,
			"binding":           nil,
			"quota":             0,
			"rpm":               0,
			"tpm":               0,
			"tokens":            0,
			"amount_usd":        0,
			"amount_cny":        0,
			"quota_per_unit":    common.QuotaPerUnit,
			"usd_exchange_rate": operation_setting.USDExchangeRate,
		})
		return
	}

	startTimestamp := parseFactoryUsageTimestamp(c.Query("start_timestamp"))
	endTimestamp := parseFactoryUsageTimestamp(c.Query("end_timestamp"))
	modelName := strings.TrimSpace(c.Query("model_name"))
	group := strings.TrimSpace(c.Query("group"))
	requestId := strings.TrimSpace(c.Query("request_id"))
	stat, err := model.SumUsedQuotaByTokenId(target.TokenId, startTimestamp, endTimestamp, modelName, group, requestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	amountUsd := float64(stat.Quota) / common.QuotaPerUnit
	common.ApiSuccess(c, gin.H{
		"bound":             true,
		"binding":           target.Binding,
		"quota":             stat.Quota,
		"rpm":               stat.Rpm,
		"tpm":               stat.Tpm,
		"tokens":            stat.Tokens,
		"amount_usd":        amountUsd,
		"amount_cny":        amountUsd * operation_setting.USDExchangeRate,
		"quota_per_unit":    common.QuotaPerUnit,
		"usd_exchange_rate": operation_setting.USDExchangeRate,
	})
}
