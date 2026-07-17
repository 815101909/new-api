package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViduDefaultVideoPrices(t *testing.T) {
	resetModelPricingState()

	tests := map[string]float64{
		"viduq3-pro":      3.125,
		"viduq3-turbo":    1.875,
		"viduq3-pro-fast": 1.875,
		"viduq2-pro-fast": 0.5,
		"viduq2-pro":      1.71875,
		"viduq2-turbo":    1.5,
		"viduq2":          1.09375,
		"viduq1":          2.5,
		"viduq1-classic":  2.5,
		"vidu2.0":         0.625,
	}

	for modelName, expected := range tests {
		price, ok := GetModelPrice(modelName, false)
		require.True(t, ok, modelName)
		require.InDelta(t, expected, price, 1e-9, modelName)
	}
}
