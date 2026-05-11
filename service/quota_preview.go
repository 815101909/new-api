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
	defaultTokenOverdraftLimitCNY = 1.0
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
	return relayInfo != nil && !relayInfo.UsePrice && !relayInfo.ForcePreConsume
}

func getMinAllowedRemainingQuota(relayInfo *relaycommon.RelayInfo) int {
	if !IsTokenOverdraftAllowed(relayInfo) {
		return 0
	}
	return -GetTokenOverdraftQuotaLimit()
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
	if IsTokenOverdraftAllowed(relayInfo) {
		preview.BillingMode = "token_overdraft"
	}
	preview.UserQuotaAfterCharge = userQuota - needQuota
	if tokenUnlimited {
		preview.TokenQuotaAfterCharge = tokenQuota
	} else {
		preview.TokenQuotaAfterCharge = tokenQuota - needQuota
	}
	preview.Allowed = preview.UserQuotaAfterCharge >= preview.MinAllowedRemainingQuota
	if preview.Allowed && !tokenUnlimited {
		preview.Allowed = preview.TokenQuotaAfterCharge >= preview.MinAllowedRemainingQuota
	}
	return preview
}

func (preview ChargeQuotaPreview) FailureKind() ChargeQuotaFailureKind {
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
