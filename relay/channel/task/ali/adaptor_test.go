package ali

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
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

func TestConvertToAliRequest_ModernI2VUsesMediaAndDefaultResolution(t *testing.T) {
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(&relaycommon.RelayInfo{}, relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.0-i2v",
		Prompt: "test prompt",
		Image:  "https://example.com/first-frame.png",
		Size:   "9x16",
	})
	require.NoError(t, err)
	require.Len(t, aliReq.Input.Media, 1)
	require.Equal(t, "first_frame", aliReq.Input.Media[0].Type)
	require.Equal(t, "https://example.com/first-frame.png", aliReq.Input.Media[0].URL)
	require.Empty(t, aliReq.Input.ImgURL)
	require.Equal(t, "1080P", aliReq.Parameters.Resolution)
	require.Empty(t, aliReq.Parameters.Ratio)
}

func TestConvertToAliRequest_ModernT2VMapsOpenAIRatio(t *testing.T) {
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(&relaycommon.RelayInfo{}, relaycommon.TaskSubmitReq{
		Model:  "wan2.7-t2v",
		Prompt: "test prompt",
		Size:   "9x16",
	})
	require.NoError(t, err)
	require.Equal(t, "9:16", aliReq.Parameters.Ratio)
	require.Equal(t, "1080P", aliReq.Parameters.Resolution)
	require.Empty(t, aliReq.Parameters.Size)
}

func TestConvertToAliRequest_ModernR2VMapsMetadataResolutionAndMedia(t *testing.T) {
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(&relaycommon.RelayInfo{}, relaycommon.TaskSubmitReq{
		Model:          "wan2.7-r2v",
		Prompt:         "test prompt",
		InputReference: "https://example.com/reference-image.png",
		Metadata: map[string]interface{}{
			"resolution":      "720p",
			"reference_video": "https://example.com/reference-video.mp4",
			"reference_voice": "https://example.com/reference-voice.mp3",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.ElementsMatch(t, []AliMediaItem{
		{ Type: "reference_image", URL: "https://example.com/reference-image.png" },
		{ Type: "reference_video", URL: "https://example.com/reference-video.mp4" },
		{ Type: "reference_voice", URL: "https://example.com/reference-voice.mp3" },
	}, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)
	require.Empty(t, aliReq.Input.VideoURL)
	require.Empty(t, aliReq.Input.ReferenceVoice)
}

func TestConvertToAliRequest_LegacyT2VMapsOpenAIRatioToSize(t *testing.T) {
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(&relaycommon.RelayInfo{}, relaycommon.TaskSubmitReq{
		Model:  "wan2.6-t2v",
		Prompt: "test prompt",
		Size:   "9x16",
	})
	require.NoError(t, err)
	require.Equal(t, "720*1280", aliReq.Parameters.Size)
	require.Empty(t, aliReq.Parameters.Resolution)
}

func TestPopulateAliMultipartImageUsesInputReferenceFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "wan2.7-i2v"))
	require.NoError(t, writer.WriteField("prompt", "test prompt"))
	part, err := writer.CreateFormFile("input_reference", "frame.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-png-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	taskReq := relaycommon.TaskSubmitReq{}
	require.NoError(t, populateAliMultipartImage(ctx, &taskReq))
	require.Contains(t, taskReq.InputReference, "data:")
	require.Contains(t, taskReq.InputReference, ";base64,")
	require.Equal(t, taskReq.InputReference, taskReq.Image)
}
