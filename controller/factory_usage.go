package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type factoryUsageBindingPayload struct {
	FactoryUserId   int    `json:"factory_user_id"`
	FactoryUsername string `json:"factory_username"`
	NewApiUserId    int    `json:"new_api_user_id"`
	NewApiUsername  string `json:"new_api_username"`
	TokenId         int    `json:"token_id"`
	TokenName       string `json:"token_name"`
	TokenStatus     int    `json:"token_status"`
	RemainQuota     int    `json:"remain_quota"`
	UsedQuota       int    `json:"used_quota"`
	TotalQuota      int    `json:"total_quota"`
	UnlimitedQuota  bool   `json:"unlimited_quota"`
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

type factoryUsageTopupTokenRequest struct {
	FactoryUserId   int     `json:"factory_user_id"`
	FactoryUsername string  `json:"factory_username"`
	AmountCny       float64 `json:"amount_cny"`
	Remark          string  `json:"remark"`
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
	payload := factoryUsageBindingPayload{
		FactoryUserId:   binding.FactoryUserId,
		FactoryUsername: binding.FactoryUsername,
		NewApiUserId:    binding.NewApiUserId,
		NewApiUsername:  binding.NewApiUsername,
		TokenId:         binding.TokenId,
	}
	fillFactoryUsageTokenInfo(&payload, binding.TokenId)
	return payload
}

func fillFactoryUsageTokenInfo(payload *factoryUsageBindingPayload, tokenId int) {
	if payload == nil || tokenId <= 0 {
		return
	}
	token, err := model.GetTokenById(tokenId)
	if err != nil || token == nil {
		return
	}
	payload.TokenName = token.Name
	payload.TokenStatus = token.Status
	payload.RemainQuota = token.RemainQuota
	payload.UsedQuota = token.UsedQuota
	payload.TotalQuota = token.RemainQuota + token.UsedQuota
	payload.UnlimitedQuota = token.UnlimitedQuota
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
		TokenStatus:     token.Status,
		RemainQuota:     token.RemainQuota,
		UsedQuota:       token.UsedQuota,
		TotalQuota:      token.RemainQuota + token.UsedQuota,
		UnlimitedQuota:  token.UnlimitedQuota,
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

func convertFactoryUsageCnyToQuota(amountCny float64) (int, error) {
	if amountCny <= 0 {
		return 0, errors.New("充值金额必须大于 0")
	}
	if operation_setting.USDExchangeRate <= 0 || common.QuotaPerUnit <= 0 {
		return 0, errors.New("New API 额度换算配置异常")
	}
	quota := decimal.NewFromFloat(amountCny).
		Div(decimal.NewFromFloat(operation_setting.USDExchangeRate)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0).
		IntPart()
	if quota <= 0 {
		return 0, errors.New("充值金额过低，无法换算为有效额度")
	}
	if quota > int64(getMaxTokenQuotaValue()) {
		return 0, errors.New("充值金额超过有效范围")
	}
	return int(quota), nil
}

func getFactoryUsageTopupToken(req factoryUsageTopupTokenRequest) (*model.Token, *model.FactoryAccountBinding, string, error) {
	factoryUserId := req.FactoryUserId
	factoryUsername := strings.TrimSpace(req.FactoryUsername)
	if factoryUserId <= 0 || factoryUsername == "" {
		return nil, nil, "", errors.New("创作工厂账号信息不能为空")
	}

	binding, err := model.GetFactoryAccountBindingByFactoryUserId(factoryUserId)
	if err == nil && binding != nil && binding.TokenId > 0 {
		token, tokenErr := model.GetTokenById(binding.TokenId)
		if tokenErr != nil {
			return nil, nil, "", tokenErr
		}
		newApiUsername, _ := model.GetUsernameById(token.UserId, false)
		return token, binding, newApiUsername, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, "", err
	}

	token, tokenErr := model.GetTokenByName(factoryUsername)
	if tokenErr != nil {
		if errors.Is(tokenErr, gorm.ErrRecordNotFound) {
			return nil, nil, "", errors.New("当前创作工厂账号尚未绑定 New API 令牌")
		}
		return nil, nil, "", tokenErr
	}
	newApiUsername, _ := model.GetUsernameById(token.UserId, false)
	return token, nil, newApiUsername, nil
}

func TopupFactoryUsageToken(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	req := factoryUsageTopupTokenRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	token, binding, newApiUsername, err := getFactoryUsageTopupToken(req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if token.UnlimitedQuota {
		common.ApiErrorMsg(c, "当前令牌为无限额度，无需充值")
		return
	}
	quota, err := convertFactoryUsageCnyToQuota(req.AmountCny)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if token.RemainQuota > getMaxTokenQuotaValue()-quota {
		common.ApiErrorMsg(c, "充值后令牌额度超过有效范围")
		return
	}

	token.RemainQuota += quota
	if token.Status == common.TokenStatusExhausted && token.RemainQuota > 0 {
		token.Status = common.TokenStatusEnabled
	}
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	factoryUsername := strings.TrimSpace(req.FactoryUsername)
	content := fmt.Sprintf(
		"创作工厂令牌充值，创作工厂账号：%s，充值金额：¥%.2f，充值额度：%s",
		factoryUsername,
		req.AmountCny,
		logger.FormatQuota(quota),
	)
	if remark := strings.TrimSpace(req.Remark); remark != "" {
		content = fmt.Sprintf("%s，备注：%s", content, remark)
	}
	log := &model.Log{
		UserId:    token.UserId,
		Username:  newApiUsername,
		CreatedAt: common.GetTimestamp(),
		Type:      model.LogTypeTopup,
		Content:   content,
		TokenName: token.Name,
		TokenId:   token.Id,
		Quota:     quota,
		Ip:        c.ClientIP(),
		RequestId: c.GetString(common.RequestIdKey),
		Other:     common.MapToJsonStr(map[string]interface{}{"source": "aigc_factory"}),
	}
	if err := model.LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record factory token topup log: " + err.Error())
	}

	payload := factoryUsageBindingPayload{
		FactoryUserId:   req.FactoryUserId,
		FactoryUsername: factoryUsername,
		NewApiUserId:    token.UserId,
		NewApiUsername:  newApiUsername,
		TokenId:         token.Id,
		TokenName:       token.Name,
		TokenStatus:     token.Status,
		RemainQuota:     token.RemainQuota,
		UsedQuota:       token.UsedQuota,
		TotalQuota:      token.RemainQuota + token.UsedQuota,
		UnlimitedQuota:  token.UnlimitedQuota,
	}
	if binding != nil {
		payload.FactoryUsername = binding.FactoryUsername
		payload.FactoryUserId = binding.FactoryUserId
	}
	common.ApiSuccess(c, gin.H{
		"bound":             true,
		"binding":           payload,
		"quota":             quota,
		"amount_cny":        req.AmountCny,
		"amount_usd":        req.AmountCny / operation_setting.USDExchangeRate,
		"quota_per_unit":    common.QuotaPerUnit,
		"usd_exchange_rate": operation_setting.USDExchangeRate,
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
