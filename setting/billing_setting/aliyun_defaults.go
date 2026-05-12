package billing_setting

import (
	"fmt"
	"strings"
)

type aliTier struct {
	Name           string
	MaxLen         int
	InputPrice     float64
	OutputPrice    float64
	ThinkingOutput float64
}

const (
	aliCacheModeNone               = "none"
	aliCacheModeExplicitOnly       = "explicit_only"
	aliCacheModeExplicitOrImplicit = "explicit_or_implicit"
)

const aliExplicitCachePredicate = `has(param("messages"), "cache_control") || has(param("input"), "cache_control")`

var aliThinkingEnabledChecks = []string{
	`param("enable_thinking") == true`,
	`param("enable_thinking") == "true"`,
	`param("extra_body.enable_thinking") == true`,
	`param("extra_body.enable_thinking") == "true"`,
	`param("thinking.type") == "enabled"`,
	`param("reasoning.type") == "enabled"`,
	`param("thinking") == true`,
	`param("thinking") == "true"`,
	`param("think") == true`,
	`param("think") == "true"`,
}

var aliThinkingDisabledChecks = []string{
	`param("enable_thinking") == false`,
	`param("enable_thinking") == "false"`,
	`param("extra_body.enable_thinking") == false`,
	`param("extra_body.enable_thinking") == "false"`,
	`param("thinking.type") == "disabled"`,
	`param("reasoning.type") == "disabled"`,
	`param("thinking") == false`,
	`param("thinking") == "false"`,
	`param("think") == false`,
	`param("think") == "false"`,
}

func aliThinkingEnabledExpr(defaultEnabled bool) string {
	if defaultEnabled {
		return fmt.Sprintf("!(%s)", strings.Join(aliThinkingDisabledChecks, " || "))
	}
	return fmt.Sprintf("(%s)", strings.Join(aliThinkingEnabledChecks, " || "))
}

func aliCacheExpr(inputPrice float64, mode string) string {
	switch mode {
	case aliCacheModeExplicitOnly:
		return fmt.Sprintf(" + cc * %g + cr * %g", inputPrice*1.25, inputPrice*0.1)
	case aliCacheModeExplicitOrImplicit:
		return fmt.Sprintf(" + cc * %g + cr * ((%s) ? %g : %g)", inputPrice*1.25, aliExplicitCachePredicate, inputPrice*0.1, inputPrice*0.2)
	default:
		return ""
	}
}

func aliTierBodyExpr(tier aliTier, thinkingExpr, cacheMode string) string {
	outputExpr := fmt.Sprintf("%g", tier.OutputPrice)
	if thinkingExpr != "" && tier.ThinkingOutput > 0 && tier.ThinkingOutput != tier.OutputPrice {
		outputExpr = fmt.Sprintf("((%s) ? %g : %g)", thinkingExpr, tier.ThinkingOutput, tier.OutputPrice)
	}
	return fmt.Sprintf("p * %g + c * %s%s", tier.InputPrice, outputExpr, aliCacheExpr(tier.InputPrice, cacheMode))
}

func aliTieredExpr(tiers []aliTier, thinkingExpr, cacheMode string) string {
	if len(tiers) == 0 {
		return ""
	}
	if len(tiers) == 1 {
		return fmt.Sprintf(`tier("%s", %s)`, tiers[0].Name, aliTierBodyExpr(tiers[0], thinkingExpr, cacheMode))
	}
	var builder strings.Builder
	for i, tier := range tiers {
		body := fmt.Sprintf(`tier("%s", %s)`, tier.Name, aliTierBodyExpr(tier, thinkingExpr, cacheMode))
		if i == len(tiers)-1 {
			builder.WriteString(body)
			break
		}
		builder.WriteString(fmt.Sprintf("len <= %d ? %s : ", tier.MaxLen, body))
	}
	return builder.String()
}

func defaultBillingModeMap() map[string]string {
	modeMap := make(map[string]string)
	for _, model := range []string{
		"qwen3.6-max-preview",
		"qwen3.6-plus",
		"qwen3.6-plus-2026-04-02",
		"qwen3.6-flash",
		"qwen3.6-flash-2026-04-16",
		"qwen3-max",
		"qwen3-max-preview",
		"qwen-flash",
		"qwen-turbo",
		"qwen-plus",
		"qwen-max",
		"qwen3-235b-a22b",
		"qwen3-coder-plus",
		"qwen3-coder-flash",
		"qwen3-coder-next",
	} {
		modeMap[model] = BillingModeTieredExpr
	}
	for model, mode := range defaultVolcEngineBillingModeMap() {
		modeMap[model] = mode
	}
	return modeMap
}

func defaultBillingExprMap() map[string]string {
	thinkingOnByDefault := aliThinkingEnabledExpr(true)
	thinkingOffByDefault := aliThinkingEnabledExpr(false)

	exprMap := map[string]string{
		"qwen3.6-max-preview": aliTieredExpr([]aliTier{
			{Name: "0-128k", MaxLen: 128000, InputPrice: 9, OutputPrice: 54},
			{Name: "128k-256k", InputPrice: 15, OutputPrice: 90},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3.6-plus": aliTieredExpr([]aliTier{
			{Name: "0-256k", MaxLen: 256000, InputPrice: 2, OutputPrice: 12},
			{Name: "256k-1m", InputPrice: 8, OutputPrice: 48},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3.6-plus-2026-04-02": aliTieredExpr([]aliTier{
			{Name: "0-256k", MaxLen: 256000, InputPrice: 2, OutputPrice: 12},
			{Name: "256k-1m", InputPrice: 8, OutputPrice: 48},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3.6-flash": aliTieredExpr([]aliTier{
			{Name: "0-256k", MaxLen: 256000, InputPrice: 1.2, OutputPrice: 7.2},
			{Name: "256k-1m", InputPrice: 4.8, OutputPrice: 28.8},
		}, "", aliCacheModeExplicitOnly),
		"qwen3.6-flash-2026-04-16": aliTieredExpr([]aliTier{
			{Name: "0-256k", MaxLen: 256000, InputPrice: 1.2, OutputPrice: 7.2},
			{Name: "256k-1m", InputPrice: 4.8, OutputPrice: 28.8},
		}, "", aliCacheModeExplicitOnly),
		"qwen3-max": aliTieredExpr([]aliTier{
			{Name: "0-32k", MaxLen: 32000, InputPrice: 2.5, OutputPrice: 10},
			{Name: "32k-128k", MaxLen: 128000, InputPrice: 4, OutputPrice: 16},
			{Name: "128k-256k", InputPrice: 7, OutputPrice: 28},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3-max-preview": aliTieredExpr([]aliTier{
			{Name: "0-32k", MaxLen: 32000, InputPrice: 6, OutputPrice: 24},
			{Name: "32k-128k", MaxLen: 128000, InputPrice: 10, OutputPrice: 40},
			{Name: "128k-256k", InputPrice: 15, OutputPrice: 60},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen-flash": aliTieredExpr([]aliTier{
			{Name: "0-128k", MaxLen: 128000, InputPrice: 0.15, OutputPrice: 1.5, ThinkingOutput: 1.5},
			{Name: "128k-256k", MaxLen: 256000, InputPrice: 0.6, OutputPrice: 6, ThinkingOutput: 6},
			{Name: "256k-1m", InputPrice: 1.2, OutputPrice: 12, ThinkingOutput: 12},
		}, thinkingOffByDefault, aliCacheModeExplicitOrImplicit),
		"qwen-turbo": aliTieredExpr([]aliTier{
			{Name: "flat", InputPrice: 0.3, OutputPrice: 0.6, ThinkingOutput: 3},
		}, thinkingOffByDefault, aliCacheModeExplicitOrImplicit),
		"qwen-plus": aliTieredExpr([]aliTier{
			{Name: "0-128k", MaxLen: 128000, InputPrice: 0.8, OutputPrice: 2, ThinkingOutput: 8},
			{Name: "128k-256k", MaxLen: 256000, InputPrice: 2.4, OutputPrice: 20, ThinkingOutput: 24},
			{Name: "256k-1m", InputPrice: 4.8, OutputPrice: 48, ThinkingOutput: 48},
		}, thinkingOffByDefault, aliCacheModeExplicitOrImplicit),
		"qwen-max": aliTieredExpr([]aliTier{
			{Name: "flat", InputPrice: 2.4, OutputPrice: 9.6},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3-235b-a22b": aliTieredExpr([]aliTier{
			{Name: "flat", InputPrice: 2, OutputPrice: 8, ThinkingOutput: 20},
		}, thinkingOnByDefault, aliCacheModeNone),
		"qwen3-coder-plus": aliTieredExpr([]aliTier{
			{Name: "0-32k", MaxLen: 32000, InputPrice: 4, OutputPrice: 16},
			{Name: "32k-128k", MaxLen: 128000, InputPrice: 6, OutputPrice: 24},
			{Name: "128k-256k", MaxLen: 256000, InputPrice: 10, OutputPrice: 40},
			{Name: "256k-1m", InputPrice: 20, OutputPrice: 200},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3-coder-flash": aliTieredExpr([]aliTier{
			{Name: "0-32k", MaxLen: 32000, InputPrice: 1, OutputPrice: 4},
			{Name: "32k-128k", MaxLen: 128000, InputPrice: 1.5, OutputPrice: 6},
			{Name: "128k-256k", MaxLen: 256000, InputPrice: 2.5, OutputPrice: 10},
			{Name: "256k-1m", InputPrice: 5, OutputPrice: 25},
		}, "", aliCacheModeExplicitOrImplicit),
		"qwen3-coder-next": aliTieredExpr([]aliTier{
			{Name: "0-32k", MaxLen: 32000, InputPrice: 1, OutputPrice: 4},
			{Name: "32k-128k", MaxLen: 128000, InputPrice: 1.5, OutputPrice: 6},
			{Name: "128k-256k", InputPrice: 2.5, OutputPrice: 10},
		}, "", aliCacheModeNone),
	}
	for model, expr := range defaultVolcEngineBillingExprMap() {
		exprMap[model] = expr
	}
	return exprMap
}

func GetDefaultBillingModeMap() map[string]string {
	return defaultBillingModeMap()
}

func GetDefaultBillingExprMap() map[string]string {
	return defaultBillingExprMap()
}
