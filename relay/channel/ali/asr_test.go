package ali

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAudioTranscriptionMultipartBindsFileURL(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "paraformer-v2"))
	require.NoError(t, writer.WriteField("file_url", "https://example.com/audio.mp3"))
	fileWriter, err := writer.CreateFormFile("file", "audio.mp3")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("test audio"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	request, err := relayhelper.GetAndValidAudioRequest(context, relayconstant.RelayModeAudioTranscription)
	require.NoError(t, err)
	require.Equal(t, "paraformer-v2", request.Model)
	require.Equal(t, "https://example.com/audio.mp3", request.FileURL)
}

func TestBuildAliAudioTranscriptionRequest(t *testing.T) {
	request, err := buildAliAudioTranscriptionRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "paraformer-v2"},
		},
		dto.AudioRequest{
			Model:   "mapped-model",
			FileURL: "https://example.com/audio.mp3",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "paraformer-v2", request.Model)
	require.Equal(t, []string{"https://example.com/audio.mp3"}, request.Input.FileURLs)
	require.True(t, request.Parameters.TimestampAlignmentEnabled)
}

func TestConvertAliAudioTranscriptionRequestRequiresFileURL(t *testing.T) {
	_, err := convertAliAudioTranscriptionRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "fun-asr"},
		},
		dto.AudioRequest{Model: "fun-asr"},
	)
	require.ErrorContains(t, err, "file_url is required")
}

func TestConvertAliAudioTranscriptionRequest(t *testing.T) {
	reader, err := convertAliAudioTranscriptionRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "fun-asr"},
		},
		dto.AudioRequest{
			Model:   "fun-asr",
			FileURL: "https://example.com/audio.wav",
		},
	)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(body), `"model":"fun-asr"`)
	require.Contains(t, string(body), `"file_urls":["https://example.com/audio.wav"]`)
}

func TestAppendAliAudioTranscript(t *testing.T) {
	transcript := &aliAudioTranscript{}
	transcript.Properties.OriginalDurationInMilliseconds = 2800
	transcript.Transcripts = append(transcript.Transcripts, struct {
		Text      string `json:"text,omitempty"`
		Sentences []struct {
			BeginTime int    `json:"begin_time,omitempty"`
			EndTime   int    `json:"end_time,omitempty"`
			Text      string `json:"text,omitempty"`
		} `json:"sentences,omitempty"`
	}{Text: "你好世界"})
	transcript.Transcripts[0].Sentences = append(transcript.Transcripts[0].Sentences, struct {
		BeginTime int    `json:"begin_time,omitempty"`
		EndTime   int    `json:"end_time,omitempty"`
		Text      string `json:"text,omitempty"`
	}{BeginTime: 100, EndTime: 2800, Text: "你好世界"})

	response := &aliOpenAITranscriptionResponse{}
	appendAliAudioTranscript(response, transcript)
	require.Equal(t, "你好世界", response.Text)
	require.Equal(t, 2.8, response.Duration)
	require.Len(t, response.Segments, 1)
	require.Equal(t, 0.1, response.Segments[0].Start)
	require.Equal(t, 2.8, response.Segments[0].End)
}

func TestAliAudioTranscriptionRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	requestURL, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://dashscope.aliyuncs.com",
		},
		RelayMode: relayconstant.RelayModeAudioTranscription,
	})
	require.NoError(t, err)
	require.Equal(t, "https://dashscope.aliyuncs.com/api/v1/services/audio/asr/transcription", requestURL)
}
