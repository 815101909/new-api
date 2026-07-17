package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/require"
)

func TestBuildOptionResponseValueBillingSettingIncludesDefaultPricing(t *testing.T) {
	withTieredBillingConfig(t, map[string]string{
		"custom-model": "ratio",
	}, map[string]string{
		"custom-model": `tier("custom", p * 1)`,
	})

	rawMode := buildOptionResponseValue("billing_setting.billing_mode", `{"custom-model":"ratio"}`)
	var modes map[string]string
	require.NoError(t, common.UnmarshalJsonStr(rawMode, &modes))
	require.Equal(t, "ratio", modes["custom-model"])
	require.Equal(t, billing_setting.BillingModeTieredExpr, modes["paraformer-v2"])

	rawExpr := buildOptionResponseValue("billing_setting.billing_expr", `{"custom-model":"tier(\"custom\", p * 1)"}`)
	var exprs map[string]string
	require.NoError(t, common.UnmarshalJsonStr(rawExpr, &exprs))
	require.Equal(t, `tier("custom", p * 1)`, exprs["custom-model"])
	require.NotEmpty(t, exprs["paraformer-v2"])
}
