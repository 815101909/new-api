package volcengine

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestForcesDoubaoSeedreamWatermarkFalse(t *testing.T) {
	adaptor := &Adaptor{}
	explicitWatermark := true
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "doubao-seedream-4-5-251128",
		},
	}

	converted, err := adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{
		Model:     "mapped-image-model",
		Prompt:    "test prompt",
		Watermark: &explicitWatermark,
	})
	require.NoError(t, err)

	request, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	require.NotNil(t, request.Watermark)
	require.False(t, *request.Watermark)

	payload, err := common.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(payload, &body))
	require.Contains(t, body, "watermark")
	require.Equal(t, false, body["watermark"])
}

func TestConvertImageRequestLeavesNonSeedreamWatermarkUnset(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "Doubao-pro-32k",
		},
	}

	converted, err := adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{
		Model:  "other-image-model",
		Prompt: "test prompt",
	})
	require.NoError(t, err)

	request, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	require.Nil(t, request.Watermark)

	payload, err := common.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(payload, &body))
	require.NotContains(t, body, "watermark")
}
