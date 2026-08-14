package vidu

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestViduVideoSubmitReturnsLocalTaskBeforeUpstreamRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader("request body"))
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeVidu,
			ChannelBaseUrl: "https://example.com",
			ApiKey:         "test-key",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_local",
		},
		RelayMode:       relayconstant.RelayModeVideoSubmit,
		OriginModelName: "viduq3-turbo",
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	resp, err := adaptor.DoRequest(c, info, strings.NewReader("upstream request"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"state":"created"`)

	value, exists := c.Get(relaycommon.ContextKeyAsyncTaskSubmitter)
	require.True(t, exists)
	require.IsType(t, relaycommon.AsyncTaskSubmitter(nil), value)
}

func TestViduBackgroundSubmitStoresUpstreamTaskID(t *testing.T) {
	service.InitHttpClient()

	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
	})
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	task := &model.Task{
		TaskID:   "task_local",
		Platform: constant.TaskPlatform("52"),
		Status:   model.TaskStatusSubmitted,
		Progress: "0%",
		PrivateData: model.TaskPrivateData{
			UpstreamSubmitting: true,
		},
	}
	require.NoError(t, task.Insert())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ent/v2/img2video", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"upstream-123","state":"created","model":"viduq3-turbo"}`))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(""))
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeVidu,
			ChannelBaseUrl: server.URL,
			ApiKey:         "test-key",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action:       constant.TaskActionGenerate,
			PublicTaskID: "task_local",
		},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	adaptor.submitUpstreamInBackground(c, info, []byte(`{"prompt":"move"}`))

	updatedTask, exists, err := model.GetByOnlyTaskId("task_local")
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, updatedTask.PrivateData.UpstreamSubmitting)
	require.Equal(t, model.TaskStatus(model.TaskStatusSubmitted), updatedTask.Status, updatedTask.FailReason)
	require.Equal(t, "upstream-123", updatedTask.PrivateData.UpstreamTaskID, updatedTask.FailReason)
}

func TestViduAuthorizationTypeAppliesToSubmitAndPolling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiKey: "test-key",
		ChannelOtherSettings: dto.ChannelOtherSettings{
			ViduAuthorizationType: dto.ViduAuthorizationTypeBearer,
		},
	}}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	require.NoError(t, adaptor.BuildRequestHeader(nil, req, info))
	require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))

	require.Equal(t, "Bearer polling-key", adaptor.authorizationHeader("polling-key"))
}

func TestViduInputReferenceUsesImageToVideoEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"prompt":"move the character","model":"viduq3-turbo","input_reference":"https://example.com/frame.png"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeVidu,
			ChannelBaseUrl: "https://yunwu.ai",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionGenerate, info.Action)

	requestURL, err := (&TaskAdaptor{baseURL: info.ChannelBaseUrl}).BuildRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://yunwu.ai/ent/v2/img2video", requestURL)
}

func TestViduEstimateCredits_Q3AndOffPeak(t *testing.T) {
	credits, ok := viduEstimateCredits(&requestPayload{
		Model:      "viduq3-pro",
		Duration:   5,
		Resolution: "720p",
	}, constant.TaskActionGenerate)
	require.True(t, ok)
	require.Equal(t, 100, credits)

	offPeak := true
	credits, ok = viduEstimateCredits(&requestPayload{
		Model:      "viduq3-pro",
		Duration:   5,
		Resolution: "720p",
		OffPeak:    &offPeak,
	}, constant.TaskActionGenerate)
	require.True(t, ok)
	require.Equal(t, 50, credits)
}

func TestViduEstimateCredits_Q2Variants(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		action     string
		duration   int
		resolution string
		expected   int
	}{
		{
			name:       "q2 pro fast image default",
			model:      "viduq2-pro-fast",
			action:     constant.TaskActionGenerate,
			duration:   5,
			resolution: "720p",
			expected:   16,
		},
		{
			name:       "q2 turbo image 720p",
			model:      "viduq2-turbo",
			action:     constant.TaskActionGenerate,
			duration:   5,
			resolution: "720p",
			expected:   48,
		},
		{
			name:       "q2 pro reference 720p",
			model:      "viduq2-pro",
			action:     constant.TaskActionReferenceGenerate,
			duration:   5,
			resolution: "720p",
			expected:   50,
		},
		{
			name:       "q2 text 720p",
			model:      "viduq2",
			action:     constant.TaskActionTextGenerate,
			duration:   5,
			resolution: "720p",
			expected:   35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credits, ok := viduEstimateCredits(&requestPayload{
				Model:      tt.model,
				Duration:   tt.duration,
				Resolution: tt.resolution,
			}, tt.action)
			require.True(t, ok)
			require.Equal(t, tt.expected, credits)
		})
	}
}

func TestViduEstimateCredits_Q2AudioAndIsRec(t *testing.T) {
	audio := true
	isRec := true
	credits, ok := viduEstimateCredits(&requestPayload{
		Model:      "viduq2-turbo",
		Duration:   5,
		Resolution: "720p",
		Audio:      &audio,
		IsRec:      &isRec,
	}, constant.TaskActionGenerate)

	require.True(t, ok)
	require.Equal(t, 73, credits)
}

func TestViduEstimateCredits_Q1AndVidu2(t *testing.T) {
	credits, ok := viduEstimateCredits(&requestPayload{
		Model:      "viduq1-classic",
		Duration:   5,
		Resolution: "1080p",
	}, constant.TaskActionGenerate)
	require.True(t, ok)
	require.Equal(t, 80, credits)

	credits, ok = viduEstimateCredits(&requestPayload{
		Model:      "vidu2.0",
		Duration:   4,
		Resolution: "360p",
	}, constant.TaskActionGenerate)
	require.True(t, ok)
	require.Equal(t, 20, credits)

	credits, ok = viduEstimateCredits(&requestPayload{
		Model:      "vidu2.0",
		Duration:   8,
		Resolution: "720p",
	}, constant.TaskActionFirstTailGenerate)
	require.True(t, ok)
	require.Equal(t, 100, credits)
}

func TestViduCreditsRatioUsesDefaultModelCredits(t *testing.T) {
	ratios := viduCreditsRatio("viduq3-pro", 50)
	require.InDelta(t, 0.5, ratios["vidu-price"], 1e-9)

	price, ok := viduDefaultModelPriceCNY("viduq3-pro")
	require.True(t, ok)
	require.Equal(t, 3.125, price)
}

func TestAdjustBillingOnSubmitUsesUpstreamCredits(t *testing.T) {
	taskData, err := common.Marshal(responsePayload{
		Model:   "viduq3-pro",
		Credits: 50,
	})
	require.NoError(t, err)

	ratios := (&TaskAdaptor{}).AdjustBillingOnSubmit(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "viduq3-pro",
		},
	}, taskData)

	require.InDelta(t, 0.5, ratios["vidu-price"], 1e-9)
}

func TestConvertToRequestPayloadAcceptsStringBooleans(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:   "move the character",
		Images:   []string{"https://example.com/ref.png"},
		Duration: 5,
		Size:     "720p",
		Metadata: map[string]interface{}{
			"watermark": "false",
			"audio":     "true",
			"off_peak":  "0",
			"is_rec":    "1",
			"bgm":       "false",
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "viduq2",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, payload.Watermark)
	require.False(t, *payload.Watermark)
	require.NotNil(t, payload.Audio)
	require.True(t, *payload.Audio)
	require.NotNil(t, payload.OffPeak)
	require.False(t, *payload.OffPeak)
	require.NotNil(t, payload.IsRec)
	require.True(t, *payload.IsRec)
	require.False(t, payload.Bgm)
}

func TestConvertToRequestPayloadIgnoresOpenAIVideoAspectSizeAsResolution(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:   "move the character",
		Images:   []string{"https://example.com/ref.png"},
		Duration: 5,
		Size:     "16x9",
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "viduq2",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "720p", payload.Resolution)
}

func TestBuildRequestBodyReadsMultipartInputReference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("watermark", "false"))
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="input_reference"; filename="frame.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "move the character",
		Model:  "viduq2",
		Size:   "720p",
		Metadata: map[string]interface{}{
			"watermark": "false",
		},
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVidu,
			UpstreamModelName: "viduq2",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: constant.TaskActionTextGenerate,
		},
	}

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	payloadBytes, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(payloadBytes, &payload))
	require.Len(t, payload.Images, 1)
	require.Contains(t, payload.Images[0], "data:image/png;base64,")
	require.Equal(t, constant.TaskActionGenerate, info.Action)
	require.NotNil(t, payload.Watermark)
	require.False(t, *payload.Watermark)
}
