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
}

func getFactoryUsageSyncSecret() string {
	if secret := strings.TrimSpace(os.Getenv("AIGC_FACTORY_NEW_API_SYNC_SECRET")); secret != "" {
		return secret
	}
	return strings.TrimSpace(os.Getenv("AIGC_FACTORY_SYNC_SECRET"))
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
	}
}

func getFactoryUsageBinding(c *gin.Context) (*model.FactoryAccountBinding, bool) {
	factoryUserId, _ := strconv.Atoi(c.Query("factory_user_id"))
	if factoryUserId <= 0 {
		common.ApiErrorMsg(c, "factory_user_id 不能为空")
		return nil, false
	}
	binding, err := model.GetFactoryAccountBindingByFactoryUserId(factoryUserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true
		}
		common.ApiError(c, err)
		return nil, false
	}
	if binding.TokenId <= 0 {
		return nil, true
	}
	return binding, true
}

func GetFactoryUsageLogs(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	pageInfo := common.GetPageQuery(c)
	binding, ok := getFactoryUsageBinding(c)
	if !ok {
		return
	}
	if binding == nil {
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
		binding.TokenId,
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
		"binding":           buildFactoryUsageBindingPayload(binding),
		"quota_per_unit":    common.QuotaPerUnit,
		"usd_exchange_rate": operation_setting.USDExchangeRate,
		"page":              pageInfo,
	})
}

func GetFactoryUsageStat(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	binding, ok := getFactoryUsageBinding(c)
	if !ok {
		return
	}
	if binding == nil {
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
	stat, err := model.SumUsedQuotaByTokenId(binding.TokenId, startTimestamp, endTimestamp, modelName, group, requestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	amountUsd := float64(stat.Quota) / common.QuotaPerUnit
	common.ApiSuccess(c, gin.H{
		"bound":             true,
		"binding":           buildFactoryUsageBindingPayload(binding),
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
