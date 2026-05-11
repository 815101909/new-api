package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

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

type factoryUsageOverviewDailyItem struct {
	Date      string  `json:"date"`
	Label     string  `json:"label"`
	Quota     int     `json:"quota"`
	Tokens    int     `json:"tokens"`
	AmountCny float64 `json:"amount_cny"`
}

type factoryUsageOverviewModelShareItem struct {
	ModelName string  `json:"model_name"`
	Quota     int     `json:"quota"`
	Tokens    int     `json:"tokens"`
	AmountCny float64 `json:"amount_cny"`
}

type factoryUsageOverviewPayload struct {
	TodayQuota         int                                  `json:"today_quota"`
	TodayTokens        int                                  `json:"today_tokens"`
	TodayAmountCny     float64                              `json:"today_amount_cny"`
	YesterdayQuota     int                                  `json:"yesterday_quota"`
	YesterdayTokens    int                                  `json:"yesterday_tokens"`
	YesterdayAmountCny float64                              `json:"yesterday_amount_cny"`
	Rpm                int                                  `json:"rpm"`
	Tpm                int                                  `json:"tpm"`
	Daily              []factoryUsageOverviewDailyItem      `json:"daily"`
	ModelShare         []factoryUsageOverviewModelShareItem `json:"model_share"`
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

func newFactoryUsageOverviewPayload(now time.Time) *factoryUsageOverviewPayload {
	overview := &factoryUsageOverviewPayload{
		Daily:      make([]factoryUsageOverviewDailyItem, 0, 7),
		ModelShare: make([]factoryUsageOverviewModelShareItem, 0),
	}
	location := now.Location()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	startDay := startOfToday.AddDate(0, 0, -6)
	for i := 0; i < 7; i++ {
		day := startDay.AddDate(0, 0, i)
		overview.Daily = append(overview.Daily, factoryUsageOverviewDailyItem{
			Date:      day.Format("2006-01-02"),
			Label:     day.Format("01-02"),
			Quota:     0,
			Tokens:    0,
			AmountCny: 0,
		})
	}
	return overview
}

func buildFactoryUsageOverview(tokenId int) (*factoryUsageOverviewPayload, error) {
	now := time.Now()
	overview := newFactoryUsageOverviewPayload(now)
	if tokenId <= 0 {
		return overview, nil
	}

	location := now.Location()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	startTimestamp := startOfToday.AddDate(0, 0, -6).Unix()
	endTimestamp := now.Unix()

	logs, err := model.ListTokenUsageOverviewLogsByTokenId(tokenId, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	stat, err := model.SumUsedQuotaByTokenId(tokenId, startTimestamp, endTimestamp, "", "", "")
	if err != nil {
		return nil, err
	}
	overview.Rpm = stat.Rpm
	overview.Tpm = stat.Tpm

	dailyMap := make(map[string]*factoryUsageOverviewDailyItem, len(overview.Daily))
	for index := range overview.Daily {
		item := &overview.Daily[index]
		dailyMap[item.Date] = item
	}

	modelMap := make(map[string]*factoryUsageOverviewModelShareItem)
	for _, log := range logs {
		quota := log.Quota
		tokens := log.PromptTokens + log.CompletionTokens
		amountCny := 0.0
		if common.QuotaPerUnit > 0 {
			amountCny = float64(quota) / common.QuotaPerUnit
		}

		dayKey := time.Unix(log.CreatedAt, 0).In(location).Format("2006-01-02")
		if item, ok := dailyMap[dayKey]; ok {
			item.Quota += quota
			item.Tokens += tokens
			item.AmountCny += amountCny
		}

		modelName := strings.TrimSpace(log.ModelName)
		if modelName == "" {
			modelName = "未知模型"
		}
		if _, ok := modelMap[modelName]; !ok {
			modelMap[modelName] = &factoryUsageOverviewModelShareItem{
				ModelName: modelName,
				Quota:     0,
				Tokens:    0,
				AmountCny: 0,
			}
		}
		modelMap[modelName].Quota += quota
		modelMap[modelName].Tokens += tokens
		modelMap[modelName].AmountCny += amountCny
	}

	if len(overview.Daily) > 0 {
		lastIndex := len(overview.Daily) - 1
		overview.TodayQuota = overview.Daily[lastIndex].Quota
		overview.TodayTokens = overview.Daily[lastIndex].Tokens
		overview.TodayAmountCny = overview.Daily[lastIndex].AmountCny
	}
	if len(overview.Daily) > 1 {
		yesterdayIndex := len(overview.Daily) - 2
		overview.YesterdayQuota = overview.Daily[yesterdayIndex].Quota
		overview.YesterdayTokens = overview.Daily[yesterdayIndex].Tokens
		overview.YesterdayAmountCny = overview.Daily[yesterdayIndex].AmountCny
	}

	modelShare := make([]factoryUsageOverviewModelShareItem, 0, len(modelMap))
	for _, item := range modelMap {
		if item.Tokens <= 0 {
			continue
		}
		modelShare = append(modelShare, *item)
	}
	sort.Slice(modelShare, func(i int, j int) bool {
		if modelShare[i].Tokens == modelShare[j].Tokens {
			return modelShare[i].ModelName < modelShare[j].ModelName
		}
		return modelShare[i].Tokens > modelShare[j].Tokens
	})
	if len(modelShare) > 5 {
		modelShare = modelShare[:5]
	}
	overview.ModelShare = modelShare

	return overview, nil
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
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("New API 额度换算配置异常")
	}
	quota := decimal.NewFromFloat(amountCny).
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

	factoryUserId := req.FactoryUserId
	factoryUsername := strings.TrimSpace(req.FactoryUsername)
	if binding != nil {
		if binding.FactoryUserId > 0 {
			factoryUserId = binding.FactoryUserId
		}
		if strings.TrimSpace(binding.FactoryUsername) != "" {
			factoryUsername = strings.TrimSpace(binding.FactoryUsername)
		}
	}
	content := fmt.Sprintf(
		"创作工厂令牌充值，创作工厂账号：%s，充值金额：¥%.2f，到账金额：¥%.2f",
		factoryUsername,
		req.AmountCny,
		req.AmountCny,
	)
	if remark := strings.TrimSpace(req.Remark); remark != "" {
		content = fmt.Sprintf("%s，备注：%s", content, remark)
	}
	audit := map[string]interface{}{
		"source":           "aigc_factory",
		"factory_user_id":  factoryUserId,
		"factory_username": factoryUsername,
		"new_api_user_id":  token.UserId,
		"new_api_username": newApiUsername,
		"amount_cny":       req.AmountCny,
		"quota":            quota,
		"quota_per_unit":   common.QuotaPerUnit,
	}
	log := &model.Log{
		UserId:    token.UserId,
		Username:  factoryUsername,
		CreatedAt: common.GetTimestamp(),
		Type:      model.LogTypeTopup,
		Content:   content,
		TokenName: token.Name,
		TokenId:   token.Id,
		Quota:     quota,
		Ip:        c.ClientIP(),
		RequestId: c.GetString(common.RequestIdKey),
		Other:     common.MapToJsonStr(audit),
	}
	if err := model.LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record factory token topup log: " + err.Error())
	}

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
	common.ApiSuccess(c, gin.H{
		"bound":          true,
		"binding":        payload,
		"quota":          quota,
		"amount_cny":     req.AmountCny,
		"quota_per_unit": common.QuotaPerUnit,
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
			"bound":          false,
			"binding":        nil,
			"quota_per_unit": common.QuotaPerUnit,
			"page":           pageInfo,
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
		"bound":          true,
		"binding":        target.Binding,
		"quota_per_unit": common.QuotaPerUnit,
		"page":           pageInfo,
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
		response := gin.H{
			"bound":          false,
			"binding":        nil,
			"quota":          0,
			"rpm":            0,
			"tpm":            0,
			"tokens":         0,
			"amount_cny":     0,
			"quota_per_unit": common.QuotaPerUnit,
		}
		includeOverview := strings.TrimSpace(c.Query("include_overview"))
		if includeOverview == "1" || strings.EqualFold(includeOverview, "true") {
			response["overview"] = newFactoryUsageOverviewPayload(time.Now())
		}
		common.ApiSuccess(c, response)
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
	amountCny := float64(stat.Quota) / common.QuotaPerUnit
	response := gin.H{
		"bound":          true,
		"binding":        target.Binding,
		"quota":          stat.Quota,
		"rpm":            stat.Rpm,
		"tpm":            stat.Tpm,
		"tokens":         stat.Tokens,
		"amount_cny":     amountCny,
		"quota_per_unit": common.QuotaPerUnit,
	}
	includeOverview := strings.TrimSpace(c.Query("include_overview"))
	if includeOverview == "1" || strings.EqualFold(includeOverview, "true") {
		overview, overviewErr := buildFactoryUsageOverview(target.TokenId)
		if overviewErr != nil {
			common.ApiError(c, overviewErr)
			return
		}
		response["overview"] = overview
	}
	common.ApiSuccess(c, response)
}
