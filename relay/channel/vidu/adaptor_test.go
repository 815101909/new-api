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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestWithMultipartReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image[]"; filename="ref.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("aspect_ratio", "3:4"))
	require.NoError(t, writer.WriteField("resolution", "4K"))
	require.NoError(t, writer.Close())

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	requestRaw, err := common.Marshal(map[string]any{
		"model":  "viduq2",
		"prompt": "keep the character identity",
		"size":   "3840x2160",
	})
	require.NoError(t, err)
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal(requestRaw, &request))

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "viduq2",
		},
	}
	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)

	payload, ok := converted.(*imageRequestPayload)
	require.True(t, ok)
	require.Equal(t, "viduq2", payload.Model)
	require.Equal(t, "3:4", payload.AspectRatio)
	require.Equal(t, "4K", payload.Resolution)
	require.Len(t, payload.Images, 1)
	require.True(t, strings.HasPrefix(payload.Images[0], "data:image/png;base64,"))
}

func TestViduImageAuthorizationType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiKey: "test-key",
		ChannelOtherSettings: dto.ChannelOtherSettings{
			ViduAuthorizationType: dto.ViduAuthorizationTypeBearer,
		},
	}}
	adaptor := &Adaptor{}
	header := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(context, &header, info))
	require.Equal(t, "Bearer test-key", header.Get("Authorization"))
}

func TestViduImageCreditsUseOfficialImageTiers(t *testing.T) {
	require.Equal(t, 10, estimateViduImageCredits("viduq2", 0, "4K"))
	require.Equal(t, 20, estimateViduImageCredits("viduq2", 3, "4K"))
	require.Equal(t, 30, estimateViduImageCredits("viduq2", 7, "4K"))
	require.Equal(t, 20, estimateViduImageCredits("viduq1", 1, "1080p"))
	require.InDelta(t, float64(20)/float64(35), viduImageCreditsRatio("viduq2", 20), 1e-9)
}

func TestDoResponsePollsTaskAndReturnsOpenAIImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ent/v2/tasks/task-1/creations", r.URL.Path)
		require.Equal(t, "Token test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","state":"success","credits":20,"creations":[{"id":"image-1","url":"https://example.com/vidu.png"}]}`))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesEdits,
		StartTime:       time.Unix(1700000000, 0),
		OriginModelName: "viduq2",
		PriceData: types.PriceData{
			UsePrice: true,
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ApiKey:            "test-key",
			UpstreamModelName: "viduq2",
		},
	}
	c.Set(contextKeyViduImageRequest, &imageRequestPayload{
		Model:      "viduq2",
		Images:     []string{"data:image/png;base64,AAAA"},
		Resolution: "4K",
	})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"task_id":"task-1","state":"created","model":"viduq2","credits":20}`)),
	}
	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	var imageResp dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResp))
	require.Len(t, imageResp.Data, 1)
	require.Equal(t, "https://example.com/vidu.png", imageResp.Data[0].Url)
	require.InDelta(t, float64(20)/float64(35), info.PriceData.OtherRatios["vidu-image-price"], 1e-9)
}

func TestDoResponseReturnsViduImageErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"err_code":"invalid token"}`)),
	}
	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	})

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	require.Contains(t, apiErr.ToOpenAIError().Message, "invalid token")
}
