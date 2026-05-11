package ali

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestProcessAliOtherRatios_FlashAudioAndResolution(t *testing.T) {
	ratios, err := ProcessAliOtherRatios(&AliVideoRequest{
		Model: "wan2.6-r2v-flash",
		Parameters: &AliVideoParameters{
			Resolution: "1080p",
			Audio:      func() *bool { v := true; return &v }(),
		},
	})
	require.NoError(t, err)
	require.InDelta(t, 0.25/0.15, ratios["resolution-1080P"], 1e-9)
	require.Equal(t, 2.0, ratios["audio-true"])
}

func TestProcessAliOtherRatios_AnimateMode(t *testing.T) {
	ratios, err := ProcessAliOtherRatios(&AliVideoRequest{
		Model: "wan2.2-animate-mix",
		Parameters: &AliVideoParameters{
			Mode: "wan-pro",
		},
	})
	require.NoError(t, err)
	require.InDelta(t, 0.9/0.6, ratios["mode-wan-pro"], 1e-9)
}

func TestAdjustBillingOnComplete_UsesActualUsageDurationAndResolution(t *testing.T) {
	price := ratio_setting.GetDefaultModelPriceMap()["wan2.7-videoedit"]
	require.Greater(t, price, 0.0)

	task := &model.Task{
		Properties: model.Properties{
			OriginModelName: "wan2.7-videoedit",
		},
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelPrice:  price,
				GroupRatio:  1,
				OtherRatios: map[string]float64{"seconds": 5, "resolution-1080P": 1 / 0.6},
			},
		},
		Data: []byte(`{"output":{"task_status":"SUCCEEDED"},"usage":{"duration":10.04,"SR":720}}`),
	}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, nil)
	require.Equal(t, int(price*10.04*common.QuotaPerUnit), quota)
}
