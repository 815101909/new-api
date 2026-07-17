package ali

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type aliAudioSpeechInput struct {
	Text          string   `json:"text"`
	Voice         string   `json:"voice"`
	Format        string   `json:"format,omitempty"`
	SampleRate    *int     `json:"sample_rate,omitempty"`
	Volume        *int     `json:"volume,omitempty"`
	Rate          *float64 `json:"rate,omitempty"`
	Pitch         *float64 `json:"pitch,omitempty"`
	LanguageType  string   `json:"language_type,omitempty"`
	LanguageHints []string `json:"language_hints,omitempty"`
	Instruction   string   `json:"instruction,omitempty"`
}

type aliAudioSpeechRequest struct {
	Model string              `json:"model"`
	Input aliAudioSpeechInput `json:"input"`
}

type aliAudioSpeechResponse struct {
	StatusCode int    `json:"status_code,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	Output     struct {
		FinishReason string `json:"finish_reason,omitempty"`
		Audio        struct {
			Data      string `json:"data,omitempty"`
			URL       string `json:"url,omitempty"`
			ID        string `json:"id,omitempty"`
			ExpiresAt int64  `json:"expires_at,omitempty"`
		} `json:"audio"`
	} `json:"output"`
	Usage struct {
		Characters int `json:"characters,omitempty"`
	} `json:"usage"`
}

func isAliQwenTTSSpeechModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "qwen3-tts-")
}

func buildAliAudioSpeechRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) aliAudioSpeechRequest {
	modelName := strings.TrimSpace(request.Model)
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}

	input := aliAudioSpeechInput{
		Text:  request.Input,
		Voice: request.Voice,
	}
	if isAliQwenTTSSpeechModel(modelName) {
		input.LanguageType = request.LanguageType
	} else {
		input.Format = request.ResponseFormat
		input.SampleRate = request.SampleRate
		input.Volume = request.Volume
		input.Rate = request.Speed
		input.Pitch = request.Pitch
		input.LanguageHints = request.LanguageHints
		input.Instruction = request.Instructions
	}

	return aliAudioSpeechRequest{
		Model: modelName,
		Input: input,
	}
}

func convertAliAudioSpeechRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	requestBody, err := common.Marshal(buildAliAudioSpeechRequest(info, request))
	if err != nil {
		return nil, fmt.Errorf("marshal Ali audio speech request: %w", err)
	}
	return bytes.NewReader(requestBody), nil
}

func aliAudioSpeechError(message string, code types.ErrorCode, statusCode int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New(message), code, statusCode)
}

func aliAudioSpeechHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if resp == nil {
		return nil, aliAudioSpeechError("Ali audio speech response is nil", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("read Ali audio speech response: %v", err), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}

	var response aliAudioSpeechResponse
	if err = common.Unmarshal(responseBody, &response); err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("decode Ali audio speech response: %v", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if response.StatusCode != 0 && response.StatusCode != http.StatusOK {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = fmt.Sprintf("Ali audio speech failed with status %d", response.StatusCode)
		}
		return nil, aliAudioSpeechError(message, types.ErrorCodeBadResponse, response.StatusCode)
	}
	if strings.TrimSpace(response.Code) != "" {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = response.Code
		}
		return nil, aliAudioSpeechError(message, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	audioURL := strings.TrimSpace(response.Output.Audio.URL)
	if audioURL == "" {
		return nil, aliAudioSpeechError("Ali audio speech response does not contain an audio URL", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}

	downloadRequest, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("create Ali audio download request: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	downloadResponse, err := service.GetHttpClient().Do(downloadRequest)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("download Ali audio result: %v", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(downloadResponse)
	if downloadResponse.StatusCode < http.StatusOK || downloadResponse.StatusCode >= http.StatusMultipleChoices {
		return nil, aliAudioSpeechError(fmt.Sprintf("download Ali audio result failed with status %d", downloadResponse.StatusCode), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	}
	audioBytes, err := io.ReadAll(downloadResponse.Body)
	if err != nil {
		return nil, aliAudioSpeechError(fmt.Sprintf("read Ali audio result: %v", err), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	if len(audioBytes) == 0 {
		return nil, aliAudioSpeechError("downloaded Ali audio result is empty", types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}

	downloadResponse.StatusCode = http.StatusOK
	service.IOCopyBytesGracefully(c, downloadResponse, audioBytes)

	characters := response.Usage.Characters
	if characters <= 0 && info != nil {
		if audioRequest, ok := info.Request.(*dto.AudioRequest); ok {
			characters = utf8.RuneCountInString(audioRequest.Input)
		}
	}
	usage := &dto.Usage{
		PromptTokens: characters,
		TotalTokens:  characters,
	}
	usage.PromptTokensDetails.TextTokens = characters
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	return usage, nil
}
