package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type factoryUsagePreflightChargeRequest struct {
	ResourceType string `json:"resource_type"`
	Model        string `json:"model"`
	Size         string `json:"size"`
	Quality      string `json:"quality"`
	N            int    `json:"n"`
	Duration     int    `json:"duration"`
	Seconds      string `json:"seconds"`
}

type factoryUsagePreflightChargePayload struct {
	Allowed                bool    `json:"allowed"`
	StrictPrecheck         bool    `json:"strict_precheck"`
	BillingMode            string  `json:"billing_mode"`
	Message                string  `json:"message"`
	NeedQuota              int     `json:"need_quota"`
	RemainQuota            int     `json:"remain_quota"`
	RemainQuotaAfterCharge int     `json:"remain_quota_after_charge"`
	UserQuota              int     `json:"user_quota"`
	UserQuotaAfterCharge   int     `json:"user_quota_after_charge"`
	TokenUnlimited         bool    `json:"token_unlimited"`
	OverdraftLimitQuota    int     `json:"overdraft_limit_quota"`
	QuotaPerUnit           float64 `json:"quota_per_unit"`
}

func parseFactoryUsageSpecificChannelID(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.GetHeader("X-AIGC-Factory-Channel-Id"))
	if raw == "" {
		return 0, nil
	}
	channelID, err := strconv.Atoi(raw)
	if err != nil || channelID <= 0 {
		return 0, fmt.Errorf("invalid factory channel id")
	}
	return channelID, nil
}

func pickFactoryUsagePricingGroup(groups []string, channelID int, modelName string) string {
	if channelID > 0 {
		for _, group := range groups {
			if model.IsChannelEnabledForGroupModel(group, modelName, channelID) {
				return group
			}
		}
	}
	if len(groups) > 0 {
		return groups[0]
	}
	return ""
}

func buildFactoryUsageRelayInfo(token *model.Token, modelName, group string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: modelName,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		TokenUnlimited:  token.UnlimitedQuota,
		TokenGroup:      token.Group,
		UserId:          token.UserId,
		UserGroup:       group,
		UsingGroup:      group,
	}
}

func applyFactoryUsageRelayContext(c *gin.Context, token *model.Token, group, modelName string, channel *model.Channel) {
	common.SetContextKey(c, constant.ContextKeyUserId, token.UserId)
	common.SetContextKey(c, constant.ContextKeyUserGroup, group)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
	common.SetContextKey(c, constant.ContextKeyTokenKey, token.Key)
	common.SetContextKey(c, constant.ContextKeyTokenUnlimited, token.UnlimitedQuota)
	c.Set("token_quota", token.RemainQuota)
	if channel != nil {
		common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
		common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
	}
}

func estimateFactoryUsageImageQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo, req factoryUsagePreflightChargeRequest) (int, string, error) {
	imageRequest := &dto.ImageRequest{
		Model:   relayInfo.OriginModelName,
		Prompt:  "factory preflight",
		Size:    strings.TrimSpace(req.Size),
		Quality: strings.TrimSpace(req.Quality),
	}
	n := uint(common.Max(req.N, 1))
	imageRequest.N = &n

	priceData, err := helper.ModelPriceHelper(c, relayInfo, 0, imageRequest.GetTokenCountMeta())
	if err != nil {
		return 0, "", err
	}
	relayInfo.UsePrice = priceData.UsePrice
	return priceData.QuotaToPreConsume, priceDataToBillingMode(priceData.UsePrice), nil
}

func estimateFactoryUsageVideoQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo, req factoryUsagePreflightChargeRequest, channel *model.Channel) (int, string, error) {
	if channel == nil {
		return 0, "", fmt.Errorf("video preflight requires a specific channel")
	}
	relayInfo.InitChannelMeta(c)
	relayInfo.Action = constant.TaskActionGenerate

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		return 0, "", err
	}
	relayInfo.PriceData = priceData
	relayInfo.UsePrice = priceData.UsePrice

	taskReq := relaycommon.TaskSubmitReq{
		Prompt:   "factory preflight",
		Model:    relayInfo.OriginModelName,
		Size:     strings.TrimSpace(req.Size),
		Duration: req.Duration,
		Seconds:  strings.TrimSpace(req.Seconds),
	}
	c.Set("task_request", taskReq)

	adaptor := relay.GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(channel.Type)))
	if adaptor == nil {
		return 0, "", fmt.Errorf("invalid video task adaptor for channel type %d", channel.Type)
	}
	adaptor.Init(relayInfo)
	if estimatedRatios := adaptor.EstimateBilling(c, relayInfo); len(estimatedRatios) > 0 {
		for key, ratio := range estimatedRatios {
			relayInfo.PriceData.AddOtherRatio(key, ratio)
		}
	}

	finalQuota := relayInfo.PriceData.Quota
	if !common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) {
		for _, ratio := range relayInfo.PriceData.OtherRatios {
			if ratio != 1.0 {
				finalQuota = int(float64(finalQuota) * ratio)
			}
		}
	}
	return finalQuota, priceDataToBillingMode(priceData.UsePrice), nil
}

func priceDataToBillingMode(usePrice bool) string {
	if usePrice {
		return "prepaid"
	}
	return "token_overdraft"
}

func buildFactoryUsagePreflightPayload(preview service.ChargeQuotaPreview) factoryUsagePreflightChargePayload {
	payload := factoryUsagePreflightChargePayload{
		Allowed:                preview.Allowed,
		StrictPrecheck:         preview.BillingMode == "prepaid" || !preview.Allowed,
		BillingMode:            preview.BillingMode,
		NeedQuota:              preview.NeedQuota,
		RemainQuota:            preview.TokenQuota,
		RemainQuotaAfterCharge: preview.TokenQuotaAfterCharge,
		UserQuota:              preview.UserQuota,
		UserQuotaAfterCharge:   preview.UserQuotaAfterCharge,
		TokenUnlimited:         preview.TokenUnlimited,
		OverdraftLimitQuota:    service.GetTokenOverdraftQuotaLimit(),
		QuotaPerUnit:           common.QuotaPerUnit,
	}
	if !preview.Allowed {
		payload.Message = "余额不足，请充值"
	}
	return payload
}

func PreflightFactoryUsageCharge(c *gin.Context) {
	if !assertFactoryUsageSecret(c) {
		return
	}

	token, _, groups, ok := getFactoryUsageToken(c)
	if !ok {
		return
	}

	req := factoryUsagePreflightChargeRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	resourceType := strings.ToLower(strings.TrimSpace(req.ResourceType))
	modelName := strings.TrimSpace(req.Model)
	if resourceType != "image" && resourceType != "video" {
		common.ApiErrorMsg(c, "resource_type must be image or video")
		return
	}
	if modelName == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}
	if !factoryTokenAllowsModel(token, modelName) {
		common.ApiErrorMsg(c, fmt.Sprintf("token has no access to model %s", modelName))
		return
	}

	channelID, err := parseFactoryUsageSpecificChannelID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	group := pickFactoryUsagePricingGroup(groups, channelID, modelName)
	if group == "" {
		common.ApiErrorMsg(c, "no usable group found for current token")
		return
	}

	var channel *model.Channel
	if channelID > 0 {
		channel, err = model.GetChannelById(channelID, true)
		if err != nil {
			common.ApiErrorMsg(c, "invalid channel id")
			return
		}
		if channel.Status != common.ChannelStatusEnabled {
			common.ApiErrorMsg(c, "channel is disabled")
			return
		}
		if !model.IsChannelEnabledForGroupModel(group, modelName, channelID) {
			common.ApiErrorMsg(c, "specified channel does not support the current model or group")
			return
		}
	}

	applyFactoryUsageRelayContext(c, token, group, modelName, channel)
	relayInfo := buildFactoryUsageRelayInfo(token, modelName, group)

	userQuota, err := model.GetUserQuota(token.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	needQuota := 0
	billingMode := "prepaid"
	switch resourceType {
	case "image":
		needQuota, billingMode, err = estimateFactoryUsageImageQuota(c, relayInfo, req)
	case "video":
		needQuota, billingMode, err = estimateFactoryUsageVideoQuota(c, relayInfo, req, channel)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if billingMode == "prepaid" {
		relayInfo.UsePrice = true
	} else {
		relayInfo.UsePrice = false
	}
	preview := service.BuildChargeQuotaPreview(relayInfo, userQuota, token.RemainQuota, needQuota, token.UnlimitedQuota)
	common.ApiSuccess(c, buildFactoryUsagePreflightPayload(preview))
}
