package ali

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const aliAudioTranscriptionPollInterval = time.Second
const aliAudioTranscriptionPollTimeout = 10 * time.Minute

type aliAudioTranscriptionInput struct {
	FileURLs []string `json:"file_urls"`
}

type aliAudioTranscriptionParameters struct {
	TimestampAlignmentEnabled bool     `json:"timestamp_alignment_enabled,omitempty"`
	LanguageHints             []string `json:"language_hints,omitempty"`
}

type aliAudioTranscriptionRequest struct {
	Model      string                          `json:"model"`
	Input      aliAudioTranscriptionInput      `json:"input"`
	Parameters aliAudioTranscriptionParameters `json:"parameters,omitempty"`
}

type aliAudioTranscriptionResult struct {
	FileURL          string `json:"file_url,omitempty"`
	TranscriptionURL string `json:"transcription_url,omitempty"`
	SubtaskStatus    string `json:"subtask_status,omitempty"`
	Code             string `json:"code,omitempty"`
	Message          string `json:"message,omitempty"`
}

type aliAudioTranscriptionResponse struct {
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Output    struct {
		TaskID     string                        `json:"task_id,omitempty"`
		TaskStatus string                        `json:"task_status,omitempty"`
		Results    []aliAudioTranscriptionResult `json:"results,omitempty"`
	} `json:"output"`
	Usage struct {
		Duration float64 `json:"duration,omitempty"`
	} `json:"usage,omitempty"`
}

type aliAudioTranscript struct {
	Properties struct {
		OriginalDurationInMilliseconds int `json:"original_duration_in_milliseconds,omitempty"`
	} `json:"properties,omitempty"`
	Transcripts []struct {
		Text      string `json:"text,omitempty"`
		Sentences []struct {
			BeginTime int    `json:"begin_time,omitempty"`
			EndTime   int    `json:"end_time,omitempty"`
			Text      string `json:"text,omitempty"`
		} `json:"sentences,omitempty"`
	} `json:"transcripts,omitempty"`
}

type aliOpenAITranscriptionSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type aliOpenAITranscriptionResponse struct {
	Text     string                          `json:"text"`
	Duration float64                         `json:"duration,omitempty"`
	Segments []aliOpenAITranscriptionSegment `json:"segments,omitempty"`
}

func buildAliAudioTranscriptionRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (aliAudioTranscriptionRequest, error) {
	modelName := strings.TrimSpace(request.Model)
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	fileURL := strings.TrimSpace(request.FileURL)
	if fileURL == "" {
		return aliAudioTranscriptionRequest{}, fmt.Errorf("file_url is required for Ali audio transcription")
	}
	return aliAudioTranscriptionRequest{
		Model: modelName,
		Input: aliAudioTranscriptionInput{FileURLs: []string{fileURL}},
		Parameters: aliAudioTranscriptionParameters{
			TimestampAlignmentEnabled: true,
		},
	}, nil
}

func convertAliAudioTranscriptionRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	aliRequest, err := buildAliAudioTranscriptionRequest(info, request)
	if err != nil {
		return nil, err
	}
	requestBody, err := common.Marshal(aliRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal Ali audio transcription request: %w", err)
	}
	return bytes.NewReader(requestBody), nil
}

func requestAliAudioTranscriptionTask(c *gin.Context, info *relaycommon.RelayInfo, taskID string) (*aliAudioTranscriptionResponse, *types.NewAPIError) {
	requestURL := fmt.Sprintf("%s/api/v1/tasks/%s", strings.TrimRight(info.ChannelBaseUrl, "/"), taskID)
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("create Ali transcription task query: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	request.Header.Set("Authorization", "Bearer "+info.ApiKey)
	response, err := service.GetHttpClient().Do(request)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("query Ali transcription task: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(response)
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("read Ali transcription task: %v", err), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var taskResponse aliAudioTranscriptionResponse
	if err = common.Unmarshal(responseBody, &taskResponse); err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("decode Ali transcription task: %v", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || strings.TrimSpace(taskResponse.Code) != "" {
		message := strings.TrimSpace(taskResponse.Message)
		if message == "" {
			message = fmt.Sprintf("Ali transcription task query failed with status %d", response.StatusCode)
		}
		return nil, aliAudioSpeechError(message, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	return &taskResponse, nil
}

func waitAliAudioTranscriptionTask(c *gin.Context, info *relaycommon.RelayInfo, taskID string) (*aliAudioTranscriptionResponse, *types.NewAPIError) {
	deadline := time.Now().Add(aliAudioTranscriptionPollTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-c.Request.Context().Done():
			return nil, aliAudioSpeechError("Ali transcription task was cancelled", types.ErrorCodeBadResponse, http.StatusGatewayTimeout)
		case <-time.After(aliAudioTranscriptionPollInterval):
		}
		taskResponse, relayErr := requestAliAudioTranscriptionTask(c, info, taskID)
		if relayErr != nil {
			return nil, relayErr
		}
		switch strings.ToUpper(strings.TrimSpace(taskResponse.Output.TaskStatus)) {
		case "SUCCEEDED":
			return taskResponse, nil
		case "FAILED", "CANCELED", "UNKNOWN":
			return nil, aliAudioSpeechError("Ali audio transcription task failed", types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
	}
	return nil, aliAudioSpeechError("Ali audio transcription task timed out", types.ErrorCodeBadResponse, http.StatusGatewayTimeout)
}

func downloadAliAudioTranscript(c *gin.Context, transcriptionURL string) (*aliAudioTranscript, *types.NewAPIError) {
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, transcriptionURL, nil)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("create Ali transcription result request: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	response, err := service.GetHttpClient().Do(request)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("download Ali transcription result: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(response)
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("read Ali transcription result: %v", err), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var transcript aliAudioTranscript
	if err = common.Unmarshal(responseBody, &transcript); err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("decode Ali transcription result: %v", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	return &transcript, nil
}

func appendAliAudioTranscript(response *aliOpenAITranscriptionResponse, transcript *aliAudioTranscript) {
	if response == nil || transcript == nil {
		return
	}
	texts := make([]string, 0, len(transcript.Transcripts))
	for _, item := range transcript.Transcripts {
		if text := strings.TrimSpace(item.Text); text != "" {
			texts = append(texts, text)
		}
		for _, sentence := range item.Sentences {
			text := strings.TrimSpace(sentence.Text)
			if text == "" {
				continue
			}
			response.Segments = append(response.Segments, aliOpenAITranscriptionSegment{
				ID:    len(response.Segments),
				Start: float64(sentence.BeginTime) / 1000,
				End:   float64(sentence.EndTime) / 1000,
				Text:  text,
			})
		}
	}
	if len(texts) > 0 {
		if response.Text != "" {
			response.Text += "\n"
		}
		response.Text += strings.Join(texts, "\n")
	}
	duration := float64(transcript.Properties.OriginalDurationInMilliseconds) / 1000
	if duration > response.Duration {
		response.Duration = duration
	}
}

func aliAudioTranscriptionHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if resp == nil {
		return nil, aliAudioSpeechError("Ali audio transcription response is nil", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("read Ali audio transcription response: %v", err), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var taskResponse aliAudioTranscriptionResponse
	if err = common.Unmarshal(responseBody, &taskResponse); err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("decode Ali audio transcription response: %v", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || strings.TrimSpace(taskResponse.Code) != "" {
		message := strings.TrimSpace(taskResponse.Message)
		if message == "" {
			message = fmt.Sprintf("Ali audio transcription failed with status %d", resp.StatusCode)
		}
		return nil, aliAudioSpeechError(message, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if strings.ToUpper(strings.TrimSpace(taskResponse.Output.TaskStatus)) != "SUCCEEDED" {
		taskID := strings.TrimSpace(taskResponse.Output.TaskID)
		if taskID == "" {
			return nil, aliAudioSpeechError("Ali audio transcription response does not contain a task ID", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
		}
		completedTask, relayErr := waitAliAudioTranscriptionTask(c, info, taskID)
		if relayErr != nil {
			return nil, relayErr
		}
		taskResponse = *completedTask
	}

	openAIResponse := aliOpenAITranscriptionResponse{}
	for _, result := range taskResponse.Output.Results {
		if strings.ToUpper(strings.TrimSpace(result.SubtaskStatus)) == "FAILED" {
			message := strings.TrimSpace(result.Message)
			if message == "" {
				message = strings.TrimSpace(result.Code)
			}
			return nil, aliAudioSpeechError(message, types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		transcriptionURL := strings.TrimSpace(result.TranscriptionURL)
		if transcriptionURL == "" {
			continue
		}
		transcript, relayErr := downloadAliAudioTranscript(c, transcriptionURL)
		if relayErr != nil {
			return nil, relayErr
		}
		appendAliAudioTranscript(&openAIResponse, transcript)
	}
	if strings.TrimSpace(openAIResponse.Text) == "" {
		return nil, aliAudioSpeechError("Ali audio transcription result is empty", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}
	openAIResponseBody, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("marshal OpenAI transcription response: %v", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	response.Header.Set("Content-Type", gin.MIMEJSON)
	service.IOCopyBytesGracefully(c, response, openAIResponseBody)

	estimatedTokens := 0
	if info != nil {
		estimatedTokens = info.GetEstimatePromptTokens()
	}
	usage := &dto.Usage{
		PromptTokens: estimatedTokens,
		TotalTokens:  estimatedTokens,
	}
	usage.PromptTokensDetails.AudioTokens = estimatedTokens
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	return usage, nil
}
