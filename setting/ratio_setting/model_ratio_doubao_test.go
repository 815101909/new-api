package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func resetModelPricingState() {
	modelPriceMap.Clear()
	modelRatioMap.Clear()
	completionRatioMap.Clear()
	InitRatioSettings()
}

func TestDoubaoDefaultImagePrices(t *testing.T) {
	resetModelPricingState()

	price40, ok := GetModelPrice("doubao-seedream-4-0-250828", false)
	require.True(t, ok)
	require.InDelta(t, 0.2, price40, 1e-9)

	price45, ok := GetModelPrice("doubao-seedream-4-5-251128", false)
	require.True(t, ok)
	require.InDelta(t, 0.25, price45, 1e-9)
}

func TestDoubaoDefaultTextRatios(t *testing.T) {
	resetModelPricingState()

	ratioPro, ok, _ := GetModelRatio("Doubao-pro-32k")
	require.True(t, ok)
	require.InDelta(t, cnyPerMillionTokenRatio(0.8)*CNYPriceMigrationRate, ratioPro, 1e-9)
	require.InDelta(t, 2.5, GetCompletionRatio("Doubao-pro-32k"), 1e-9)

	ratioLite, ok, _ := GetModelRatio("Doubao-lite-32k")
	require.True(t, ok)
	require.InDelta(t, cnyPerMillionTokenRatio(0.3)*CNYPriceMigrationRate, ratioLite, 1e-9)
	require.InDelta(t, 2.0, GetCompletionRatio("Doubao-lite-32k"), 1e-9)
}

func TestDoubaoDefaultVideoRatios(t *testing.T) {
	resetModelPricingState()

	ratio15, ok, _ := GetModelRatio("doubao-seedance-1-5-pro-251215")
	require.True(t, ok)
	require.InDelta(t, cnyPerMillionTokenRatio(8)*CNYPriceMigrationRate, ratio15, 1e-9)

	ratio20, ok, _ := GetModelRatio("doubao-seedance-2-0-260128")
	require.True(t, ok)
	require.InDelta(t, cnyPerMillionTokenRatio(46)*CNYPriceMigrationRate, ratio20, 1e-9)
}

func TestUpdateModelPriceByJSONStringMergesMissingDefaults(t *testing.T) {
	resetModelPricingState()

	err := UpdateModelPriceByJSONString(`{"custom-image-model":1.23}`)
	require.NoError(t, err)

	customPrice, ok := GetModelPrice("custom-image-model", false)
	require.True(t, ok)
	require.InDelta(t, 1.23, customPrice, 1e-9)

	defaultPrice, ok := GetModelPrice("doubao-seedream-4-0-250828", false)
	require.True(t, ok)
	require.InDelta(t, 0.2, defaultPrice, 1e-9)
}

func TestUpdateModelRatioByJSONStringMergesMissingDefaults(t *testing.T) {
	resetModelPricingState()

	err := UpdateModelRatioByJSONString(`{"custom-video-model":3.21}`)
	require.NoError(t, err)

	customRatio, ok, _ := GetModelRatio("custom-video-model")
	require.True(t, ok)
	require.InDelta(t, 3.21, customRatio, 1e-9)

	defaultRatio, ok, _ := GetModelRatio("doubao-seedance-2-0-260128")
	require.True(t, ok)
	require.InDelta(t, cnyPerMillionTokenRatio(46)*CNYPriceMigrationRate, defaultRatio, 1e-9)
}

func TestUpdateCompletionRatioByJSONStringMergesMissingDefaults(t *testing.T) {
	resetModelPricingState()

	err := UpdateCompletionRatioByJSONString(`{"custom-text-model":6.54}`)
	require.NoError(t, err)

	require.InDelta(t, 6.54, GetCompletionRatio("custom-text-model"), 1e-9)
	require.InDelta(t, 2.5, GetCompletionRatio("Doubao-pro-32k"), 1e-9)
}
