package service

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	defaultTokenOverdraftLimitCNY = 5.0
)

type ChargeQuotaFailureKind string

const (
	ChargeQuotaFailureNone  ChargeQuotaFailureKind = ""
	ChargeQuotaFailureUser  ChargeQuotaFailureKind = "user"
	ChargeQuotaFailureToken ChargeQuotaFailureKind = "token"
)

type ChargeQuotaPreview struct {
	Allowed                  bool
	BillingMode              string
	NeedQuota                int
	UserQuota                int
	TokenQuota               int
	UserQuotaAfterCharge     int
	TokenQuotaAfterCharge    int
	TokenUnlimited           bool
	MinAllowedRemainingQuota int
	CurrentFailureKind       ChargeQuotaFailureKind
}

func parseTokenOverdraftLimitCNY() float64 {
	candidates := []string{
		strings.TrimSpace(os.Getenv("AIGC_FACTORY_TOKEN_OVERDRAFT_CNY")),
		strings.TrimSpace(os.Getenv("NEW_API_TOKEN_OVERDRAFT_CNY")),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		value, err := strconv.ParseFloat(candidate, 64)
		if err == nil && value > 0 {
			return value
		}
	}
	return defaultTokenOverdraftLimitCNY
}

func GetTokenOverdraftQuotaLimit() int {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return int(math.Round(parseTokenOverdraftLimitCNY() * common.QuotaPerUnit))
}

func IsTokenOverdraftAllowed(relayInfo *relaycommon.RelayInfo) bool {
	return relayInfo != nil && !relayInfo.UsePrice && !relayInfo.ForcePreConsume && isFactoryTokenBound(relayInfo)
}

func isFactoryTokenBound(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil || relayInfo.IsPlayground || relayInfo.IsChannelTest {
		return false
	}
	if relayInfo.FactoryTokenBound == nil {
		bound, err := model.IsEnabledFactoryTokenBound(relayInfo.UserId, relayInfo.TokenId)
		if err != nil {
			common.SysLog("failed to check factory token binding: " + err.Error())
			bound = false
		}
		relayInfo.FactoryTokenBound = &bound
	}
	return *relayInfo.FactoryTokenBound
}

func getMinAllowedRemainingQuota(relayInfo *relaycommon.RelayInfo) int {
	if !IsTokenOverdraftAllowed(relayInfo) {
		return 0
	}
	return -GetTokenOverdraftQuotaLimit()
}

func getCurrentQuotaFailureKind(relayInfo *relaycommon.RelayInfo, userQuota, tokenQuota int, tokenUnlimited bool) ChargeQuotaFailureKind {
	if !IsTokenOverdraftAllowed(relayInfo) {
		if userQuota <= 0 {
			return ChargeQuotaFailureUser
		}
		return ChargeQuotaFailureNone
	}
	if userQuota < 0 {
		return ChargeQuotaFailureUser
	}
	if !tokenUnlimited && tokenQuota < 0 {
		return ChargeQuotaFailureToken
	}
	return ChargeQuotaFailureNone
}

func BuildChargeQuotaPreview(relayInfo *relaycommon.RelayInfo, userQuota, tokenQuota, needQuota int, tokenUnlimited bool) ChargeQuotaPreview {
	preview := ChargeQuotaPreview{
		NeedQuota:                needQuota,
		UserQuota:                userQuota,
		TokenQuota:               tokenQuota,
		TokenUnlimited:           tokenUnlimited,
		MinAllowedRemainingQuota: getMinAllowedRemainingQuota(relayInfo),
		BillingMode:              "prepaid",
	}
	if relayInfo != nil && relayInfo.IsVideoSubmitRelay() {
		// 视频任务只校验当前绑定令牌是否仍有正余额，实际额度在任务完成后再结算。
		preview.BillingMode = "video_post_consume"
		preview.UserQuotaAfterCharge = userQuota - needQuota
		if userQuota <= 0 && !isFactoryTokenBound(relayInfo) {
			preview.CurrentFailureKind = ChargeQuotaFailureUser
			return preview
		}
		if tokenUnlimited {
			preview.TokenQuotaAfterCharge = tokenQuota
			preview.Allowed = true
			return preview
		}
		preview.TokenQuotaAfterCharge = tokenQuota - needQuota
		preview.Allowed = tokenQuota > 0
		if !preview.Allowed {
			preview.CurrentFailureKind = ChargeQuotaFailureToken
		}
		return preview
	}
	if IsTokenOverdraftAllowed(relayInfo) {
		preview.BillingMode = "token_overdraft"
	}
	preview.UserQuotaAfterCharge = userQuota - needQuota
	if tokenUnlimited {
		preview.TokenQuotaAfterCharge = tokenQuota
	} else {
		preview.TokenQuotaAfterCharge = tokenQuota - needQuota
	}
	preview.CurrentFailureKind = getCurrentQuotaFailureKind(relayInfo, userQuota, tokenQuota, tokenUnlimited)
	if preview.CurrentFailureKind != ChargeQuotaFailureNone {
		preview.Allowed = false
		return preview
	}
	preview.Allowed = preview.UserQuotaAfterCharge >= preview.MinAllowedRemainingQuota
	if preview.Allowed && !tokenUnlimited {
		preview.Allowed = preview.TokenQuotaAfterCharge >= preview.MinAllowedRemainingQuota
	}
	return preview
}

func (preview ChargeQuotaPreview) FailureKind() ChargeQuotaFailureKind {
	if preview.CurrentFailureKind != ChargeQuotaFailureNone {
		return preview.CurrentFailureKind
	}
	if preview.UserQuotaAfterCharge < preview.MinAllowedRemainingQuota {
		return ChargeQuotaFailureUser
	}
	if !preview.TokenUnlimited && preview.TokenQuotaAfterCharge < preview.MinAllowedRemainingQuota {
		return ChargeQuotaFailureToken
	}
	return ChargeQuotaFailureNone
}

func BuildChargeQuotaPreviewError(preview ChargeQuotaPreview) error {
	switch preview.FailureKind() {
	case ChargeQuotaFailureUser:
		return fmt.Errorf(
			"user quota is not enough, user quota: %s, need quota: %s",
			logger.FormatQuota(preview.UserQuota),
			logger.FormatQuota(preview.NeedQuota),
		)
	case ChargeQuotaFailureToken:
		return fmt.Errorf(
			"token quota is not enough, token remain quota: %s, need quota: %s",
			logger.FormatQuota(preview.TokenQuota),
			logger.FormatQuota(preview.NeedQuota),
		)
	default:
		return nil
	}
}

func GetCurrentTokenQuota(relayInfo *relaycommon.RelayInfo) (int, error) {
	if relayInfo == nil || relayInfo.TokenUnlimited || relayInfo.IsPlayground {
		return 0, nil
	}
	token, err := model.GetTokenByKey(relayInfo.TokenKey, false)
	if err != nil {
		return 0, err
	}
	return token.RemainQuota, nil
}
