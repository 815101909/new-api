package vidu

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/pkg/errors"
)

const viduMaxReferenceImageSize = 50 * 1024 * 1024

// ============================
// Request / Response structures
// ============================

type requestPayload struct {
	Model             string   `json:"model"`
	Images            []string `json:"images"`
	Prompt            string   `json:"prompt,omitempty"`
	Audio             *bool    `json:"audio,omitempty"`
	AudioType         string   `json:"audio_type,omitempty"`
	VoiceID           string   `json:"voice_id,omitempty"`
	IsRec             *bool    `json:"is_rec,omitempty"`
	Duration          int      `json:"duration,omitempty"`
	Seed              int      `json:"seed,omitempty"`
	Resolution        string   `json:"resolution,omitempty"`
	MovementAmplitude string   `json:"movement_amplitude,omitempty"`
	Bgm               bool     `json:"bgm,omitempty"`
	Payload           string   `json:"payload,omitempty"`
	OffPeak           *bool    `json:"off_peak,omitempty"`
	Watermark         *bool    `json:"watermark,omitempty"`
	WmPosition        int      `json:"wm_position,omitempty"`
	WmURL             string   `json:"wm_url,omitempty"`
	MetaData          string   `json:"meta_data,omitempty"`
	CallbackUrl       string   `json:"callback_url,omitempty"`
}

type responsePayload struct {
	TaskId            string   `json:"task_id"`
	State             string   `json:"state"`
	Model             string   `json:"model"`
	Images            []string `json:"images"`
	Prompt            string   `json:"prompt"`
	Duration          int      `json:"duration"`
	Seed              int      `json:"seed"`
	Resolution        string   `json:"resolution"`
	Bgm               bool     `json:"bgm"`
	MovementAmplitude string   `json:"movement_amplitude"`
	Payload           string   `json:"payload"`
	OffPeak           bool     `json:"off_peak"`
	Credits           int      `json:"credits"`
	Watermark         bool     `json:"watermark"`
	CreatedAt         string   `json:"created_at"`
}

type taskResultResponse struct {
	State     string     `json:"state"`
	ErrCode   string     `json:"err_code"`
	Credits   int        `json:"credits"`
	Payload   string     `json:"payload"`
	Creations []creation `json:"creations"`
}

type creation struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	CoverURL string `json:"cover_url"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType       int
	baseURL           string
	authorizationType dto.ViduAuthorizationType
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.authorizationType = info.ChannelOtherSettings.ViduAuthorizationType
}

func (a *TaskAdaptor) authorizationHeader(apiKey string) string {
	settings := dto.ChannelOtherSettings{ViduAuthorizationType: a.authorizationType}
	return settings.GetViduAuthorizationHeader(apiKey)
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "get_task_request_failed", http.StatusBadRequest)
	}
	action := constant.TaskActionTextGenerate
	if meatAction, ok := req.Metadata["action"]; ok {
		action, _ = meatAction.(string)
	} else if req.HasImage() {
		action = constant.TaskActionGenerate
		if info.ChannelType == constant.ChannelTypeVidu {
			// vidu 增加 首尾帧生视频和参考图生视频
			if len(req.Images) == 2 {
				action = constant.TaskActionFirstTailGenerate
			} else if len(req.Images) > 2 {
				action = constant.TaskActionReferenceGenerate
			}
		}
	}
	info.Action = action
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)
	if err := populateViduMultipartImages(c, &req, info); err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}

	if info.Action == constant.TaskActionReferenceGenerate {
		if body.Model == "viduq2-pro-fast" || body.Model == "viduq2-turbo" {
			// 参考生视频当前支持 viduq2/viduq2-pro；不支持的 Q2 变体沿用旧兼容逻辑降级到 viduq2。
			body.Model = "viduq2"
		}
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.Action {
	case constant.TaskActionGenerate:
		path = "/img2video"
	case constant.TaskActionFirstTailGenerate:
		path = "/start-end2video"
	case constant.TaskActionReferenceGenerate:
		path = "/reference2video"
	default:
		path = "/text2video"
	}
	return fmt.Sprintf("%s/ent/v2%s", a.baseURL, path), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", info.ChannelOtherSettings.GetViduAuthorizationHeader(info.ApiKey))
	return nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	var vResp responsePayload
	err = common.Unmarshal(responseBody, &vResp)
	if err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrap(err, fmt.Sprintf("%s", responseBody)), "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}

	if vResp.State == "failed" {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("task failed"), "task_failed", http.StatusBadRequest)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return vResp.TaskId, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	url := fmt.Sprintf("%s/ent/v2/tasks/%s/creations", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", a.authorizationHeader(key))

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"viduq3-turbo",
		"viduq3-pro",
		"viduq3-pro-fast",
		"viduq2-pro-fast",
		"viduq2-pro",
		"viduq2-turbo",
		"viduq2",
		"viduq1",
		"viduq1-classic",
		"vidu2.0",
		"vidu1.5",
	}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "vidu"
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil
	}
	if info.Action == constant.TaskActionReferenceGenerate && (body.Model == "viduq2-pro-fast" || body.Model == "viduq2-turbo") {
		body.Model = "viduq2"
	}

	credits, ok := viduEstimateCredits(body, info.Action)
	if !ok {
		return nil
	}
	return viduCreditsRatio(body.Model, credits)
}

func (a *TaskAdaptor) AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64 {
	var resp responsePayload
	if err := common.Unmarshal(taskData, &resp); err != nil {
		return nil
	}
	if resp.Credits <= 0 {
		return nil
	}
	modelName := resp.Model
	if modelName == "" && info != nil && info.ChannelMeta != nil {
		modelName = info.UpstreamModelName
	}
	return viduCreditsRatio(modelName, resp.Credits)
}

// ============================
// helpers
// ============================

const viduCreditPriceCNY = 0.03125

func viduDefaultModelCredits(model string) (int, bool) {
	switch strings.TrimSpace(strings.ToLower(model)) {
	case "viduq3-pro":
		return 100, true
	case "viduq3-turbo", "viduq3-pro-fast":
		return 60, true
	case "viduq2-pro-fast":
		return 16, true
	case "viduq2-pro":
		return 55, true
	case "viduq2-turbo":
		return 48, true
	case "viduq2":
		return 35, true
	case "viduq1", "viduq1-classic":
		return 80, true
	case "vidu2.0":
		return 20, true
	default:
		return 0, false
	}
}

func viduCreditsRatio(model string, credits int) map[string]float64 {
	defaultCredits, ok := viduDefaultModelCredits(model)
	if !ok || defaultCredits <= 0 || credits <= 0 {
		return nil
	}
	return map[string]float64{
		"vidu-price": float64(credits) / float64(defaultCredits),
	}
}

func viduCreditsToCNY(credits int) float64 {
	return float64(credits) * viduCreditPriceCNY
}

func viduDefaultModelPriceCNY(model string) (float64, bool) {
	credits, ok := viduDefaultModelCredits(model)
	if !ok {
		return 0, false
	}
	return viduCreditsToCNY(credits), true
}

func normalizeViduResolution(resolution string) string {
	value := strings.ToLower(strings.TrimSpace(resolution))
	switch {
	case strings.Contains(value, "1080"):
		return "1080p"
	case strings.Contains(value, "720"):
		return "720p"
	case strings.Contains(value, "540"):
		return "540p"
	case strings.Contains(value, "360"):
		return "360p"
	default:
		return ""
	}
}

func viduDefaultDuration(model string) int {
	if strings.EqualFold(strings.TrimSpace(model), "vidu2.0") {
		return 4
	}
	return 5
}

func viduDefaultResolution(model string, duration int) string {
	model = strings.TrimSpace(strings.ToLower(model))
	switch {
	case model == "vidu2.0":
		if duration == 8 {
			return "720p"
		}
		return "360p"
	case model == "viduq1" || model == "viduq1-classic":
		return "1080p"
	case strings.HasPrefix(model, "viduq3") || strings.HasPrefix(model, "viduq2"):
		return "720p"
	default:
		return "1080p"
	}
}

func viduBoolValue(value *bool) bool {
	return value != nil && *value
}

func populateViduMultipartImages(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) error {
	if c == nil || req == nil || c.Request == nil || !strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "multipart/form-data") {
		return nil
	}
	if strings.TrimSpace(req.InputReference) != "" && len(req.Images) == 0 {
		req.Images = []string{req.InputReference}
	}
	if len(req.Images) > 0 {
		adjustViduActionByImages(info, req.Images)
		return nil
	}

	mf, err := c.MultipartForm()
	if err != nil {
		return nil
	}
	if mf == nil || mf.File == nil {
		return nil
	}

	files := append(mf.File["input_reference"], mf.File["input_reference[]"]...)
	if len(files) == 0 {
		return nil
	}

	images := make([]string, 0, len(files))
	for _, fileHeader := range files {
		if fileHeader == nil {
			continue
		}
		if fileHeader.Size > viduMaxReferenceImageSize {
			return fmt.Errorf("reference image %s exceeds 50MB limit", fileHeader.Filename)
		}
		file, err := fileHeader.Open()
		if err != nil {
			return err
		}
		fileBytes, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return readErr
		}
		mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = http.DetectContentType(fileBytes)
		}
		images = append(images, fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes)))
	}
	req.Images = images
	if len(images) > 0 {
		req.InputReference = images[0]
	}
	adjustViduActionByImages(info, req.Images)
	return nil
}

func adjustViduActionByImages(info *relaycommon.RelayInfo, images []string) {
	if info == nil || info.ChannelType != constant.ChannelTypeVidu || len(images) == 0 {
		return
	}
	switch {
	case len(images) == 2:
		info.Action = constant.TaskActionFirstTailGenerate
	case len(images) > 2:
		info.Action = constant.TaskActionReferenceGenerate
	default:
		info.Action = constant.TaskActionGenerate
	}
}

func normalizeViduRequestMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	normalized := make(map[string]any, len(metadata))
	for key, value := range metadata {
		switch strings.TrimSpace(strings.ToLower(key)) {
		case "audio", "is_rec", "off_peak", "watermark", "bgm":
			if boolValue, ok := parseViduMetadataBool(value); ok {
				normalized[key] = boolValue
				continue
			}
		}
		normalized[key] = value
	}
	return normalized
}

func parseViduMetadataBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		normalized := strings.TrimSpace(strings.ToLower(v))
		switch normalized {
		case "true", "1", "yes", "y", "on":
			return true, true
		case "false", "0", "no", "n", "off", "":
			return false, true
		default:
			if parsed, err := strconv.ParseBool(normalized); err == nil {
				return parsed, true
			}
		}
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case float32:
		return v != 0, true
	}
	return false, false
}

func viduEstimateCredits(body *requestPayload, action string) (int, bool) {
	modelName := strings.TrimSpace(strings.ToLower(body.Model))
	duration := body.Duration
	if duration <= 0 {
		duration = viduDefaultDuration(modelName)
	}
	resolution := normalizeViduResolution(body.Resolution)
	if resolution == "" {
		resolution = viduDefaultResolution(modelName, duration)
	}

	audio := viduBoolValue(body.Audio)
	offPeak := viduBoolValue(body.OffPeak)
	applyOffPeak := offPeak
	if !strings.HasPrefix(modelName, "viduq3") && audio {
		applyOffPeak = false
	}

	var (
		credits int
		ok      bool
	)
	switch {
	case strings.HasPrefix(modelName, "viduq3"):
		credits, ok = viduQ3Credits(modelName, action, duration, resolution, applyOffPeak)
	case strings.HasPrefix(modelName, "viduq2"):
		credits, ok = viduQ2Credits(modelName, action, duration, resolution, applyOffPeak)
	case modelName == "viduq1" || modelName == "viduq1-classic":
		credits, ok = viduQ1Credits(applyOffPeak)
	case modelName == "vidu2.0":
		credits, ok = vidu2Credits(action, duration, resolution, applyOffPeak)
	}
	if !ok {
		return 0, false
	}
	if viduBoolValue(body.IsRec) {
		credits += 10
	}
	if strings.HasPrefix(modelName, "viduq2") && audio && (action == constant.TaskActionGenerate || action == constant.TaskActionReferenceGenerate) {
		credits += 15
	}
	return credits, true
}

func viduQ3Credits(modelName string, action string, duration int, resolution string, offPeak bool) (int, bool) {
	var rates map[string][2]int
	if action == constant.TaskActionReferenceGenerate {
		switch modelName {
		case "viduq3-turbo":
			rates = map[string][2]int{"540p": {4, 2}, "720p": {10, 5}, "1080p": {13, 7}}
		default:
			return 0, false
		}
	} else {
		switch modelName {
		case "viduq3-pro":
			rates = map[string][2]int{"540p": {9, 5}, "720p": {20, 10}, "1080p": {24, 12}}
		case "viduq3-turbo":
			rates = map[string][2]int{"540p": {7, 4}, "720p": {12, 6}, "1080p": {13, 7}}
		case "viduq3-pro-fast":
			rates = map[string][2]int{"720p": {12, 6}, "1080p": {15, 8}}
		default:
			return 0, false
		}
	}
	return viduRateCredits(duration, resolution, rates, offPeak)
}

func viduQ2Credits(modelName string, action string, duration int, resolution string, offPeak bool) (int, bool) {
	var tiers map[string][3]int
	switch action {
	case constant.TaskActionReferenceGenerate:
		if modelName == "viduq2-pro" {
			tiers = map[string][3]int{"540p": {20, 0, 5}, "720p": {30, 0, 5}, "1080p": {85, 0, 10}}
		} else {
			tiers = map[string][3]int{"540p": {15, 0, 5}, "720p": {25, 0, 5}, "1080p": {75, 0, 10}}
		}
	case constant.TaskActionGenerate, constant.TaskActionFirstTailGenerate:
		switch modelName {
		case "viduq2-turbo":
			tiers = map[string][3]int{"540p": {6, 0, 2}, "720p": {8, 10, 10}, "1080p": {35, 0, 10}}
		case "viduq2-pro":
			tiers = map[string][3]int{"540p": {8, 10, 5}, "720p": {15, 0, 10}, "1080p": {55, 0, 15}}
		case "viduq2-pro-fast":
			tiers = map[string][3]int{"720p": {8, 0, 2}, "1080p": {16, 0, 4}}
		default:
			tiers = map[string][3]int{"540p": {10, 0, 2}, "720p": {15, 0, 5}, "1080p": {20, 0, 10}}
		}
	default:
		tiers = map[string][3]int{"540p": {10, 0, 2}, "720p": {15, 0, 5}, "1080p": {20, 0, 10}}
	}
	credits, ok := viduTierCredits(duration, resolution, tiers)
	if !ok {
		return 0, false
	}
	if offPeak {
		credits = int(math.Ceil(float64(credits) / 2))
	}
	return credits, true
}

func viduQ1Credits(offPeak bool) (int, bool) {
	if offPeak {
		return 40, true
	}
	return 80, true
}

func vidu2Credits(action string, duration int, resolution string, offPeak bool) (int, bool) {
	var credits int
	if action == constant.TaskActionReferenceGenerate {
		switch resolution {
		case "360p", "720p":
			credits = 80
		default:
			return 0, false
		}
	} else if duration == 8 {
		if resolution != "720p" {
			return 0, false
		}
		credits = 100
	} else {
		switch resolution {
		case "360p":
			credits = 20
		case "720p":
			credits = 40
		case "1080p":
			credits = 100
		default:
			return 0, false
		}
	}
	if offPeak {
		credits = int(math.Ceil(float64(credits) / 2))
	}
	return credits, true
}

func viduRateCredits(duration int, resolution string, rates map[string][2]int, offPeak bool) (int, bool) {
	if duration <= 0 {
		return 0, false
	}
	ratePair, ok := rates[resolution]
	if !ok {
		return 0, false
	}
	rate := ratePair[0]
	if offPeak {
		rate = ratePair[1]
	}
	return rate * duration, true
}

func viduTierCredits(duration int, resolution string, tiers map[string][3]int) (int, bool) {
	if duration <= 0 {
		return 0, false
	}
	tier, ok := tiers[resolution]
	if !ok {
		return 0, false
	}
	credits := tier[0]
	if duration == 1 {
		return credits, true
	}
	secondSecondCredits := tier[1]
	if secondSecondCredits == 0 {
		secondSecondCredits = tier[2]
	}
	credits += secondSecondCredits
	if duration > 2 {
		credits += (duration - 2) * tier[2]
	}
	return credits, true
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	modelName := taskcommon.DefaultString(info.UpstreamModelName, "viduq1")
	r := requestPayload{
		Model:             modelName,
		Images:            req.Images,
		Prompt:            req.Prompt,
		Duration:          req.Duration,
		Resolution:        req.Size,
		MovementAmplitude: "auto",
		Bgm:               false,
	}
	if err := taskcommon.UnmarshalMetadata(normalizeViduRequestMetadata(req.Metadata), &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	if r.Duration <= 0 {
		r.Duration = viduDefaultDuration(r.Model)
	}
	r.Resolution = normalizeViduResolution(r.Resolution)
	if r.Resolution == "" {
		r.Resolution = viduDefaultResolution(r.Model, r.Duration)
	}
	if r.MovementAmplitude == "" {
		r.MovementAmplitude = "auto"
	}
	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	taskInfo := &relaycommon.TaskInfo{}

	var taskResp taskResultResponse
	err := common.Unmarshal(respBody, &taskResp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	state := taskResp.State
	switch state {
	case "created", "queueing":
		taskInfo.Status = model.TaskStatusSubmitted
	case "processing":
		taskInfo.Status = model.TaskStatusInProgress
	case "success":
		taskInfo.Status = model.TaskStatusSuccess
		if len(taskResp.Creations) > 0 {
			taskInfo.Url = taskResp.Creations[0].URL
		}
	case "failed":
		taskInfo.Status = model.TaskStatusFailure
		if taskResp.ErrCode != "" {
			taskInfo.Reason = taskResp.ErrCode
		}
	default:
		return nil, fmt.Errorf("unknown task state: %s", state)
	}

	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var viduResp taskResultResponse
	if err := common.Unmarshal(originTask.Data, &viduResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal vidu task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt

	if len(viduResp.Creations) > 0 && viduResp.Creations[0].URL != "" {
		openAIVideo.SetMetadata("url", viduResp.Creations[0].URL)
	}

	if viduResp.State == "failed" && viduResp.ErrCode != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: viduResp.ErrCode,
			Code:    viduResp.ErrCode,
		}
	}

	return common.Marshal(openAIVideo)
}
