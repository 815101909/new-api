package ali

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// AliVideoRequest 阿里通义万相视频生成请求
type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

// AliVideoInput 视频输入参数
type AliVideoInput struct {
	Prompt         string         `json:"prompt,omitempty"`          // 文本提示词
	ImgURL         string         `json:"img_url,omitempty"`         // 首帧图像URL或Base64（图生视频）
	ImageURL       string         `json:"image_url,omitempty"`       // 角色替换/角色动画输入图片
	VideoURL       string         `json:"video_url,omitempty"`       // 参考生视频/视频编辑输入视频
	ReferenceVoice string         `json:"reference_voice,omitempty"` // 参考音色URL（wan2.7）
	Media          []AliMediaItem `json:"media,omitempty"`           // 2.7 / HappyHorse 媒体输入
	FirstFrameURL  string         `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string         `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	AudioURL       string         `json:"audio_url,omitempty"`       // 音频URL（wan2.5支持）
	NegativePrompt string         `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string         `json:"template,omitempty"`        // 视频特效模板
}

type AliMediaItem struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// AliVideoParameters 视频参数
type AliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P（图生视频、首尾帧生视频）
	Size         string `json:"size,omitempty"`          // 尺寸: 如 "832*480"（文生视频）
	Ratio        string `json:"ratio,omitempty"`         // 宽高比: 16:9/9:16/1:1/4:3/3:4（wan2.7/HappyHorse）
	Mode         string `json:"mode,omitempty"`          // 模式: wan-std/wan-pro（角色动画）
	Duration     int    `json:"duration,omitempty"`      // 时长: 3-10秒
	PromptExtend bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    bool   `json:"watermark"`               // 是否添加水印
	Audio        *bool  `json:"audio,omitempty"`         // 是否添加音频（wan2.5）
	Seed         int    `json:"seed,omitempty"`          // 随机数种子
}

// AliVideoResponse 阿里通义万相响应
type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

// AliVideoOutput 输出信息
type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

// AliUsage 使用统计
type AliUsage struct {
	Duration            float64 `json:"duration,omitempty"`
	InputVideoDuration  float64 `json:"input_video_duration,omitempty"`
	OutputVideoDuration float64 `json:"output_video_duration,omitempty"`
	VideoCount          int     `json:"video_count,omitempty"`
	SR                  int     `json:"SR,omitempty"`
}

type AliMetadata struct {
	// Input 相关
	AudioURL       string `json:"audio_url,omitempty"`       // 音频URL
	ImgURL         string `json:"img_url,omitempty"`         // 图片URL（图生视频）
	FirstFrameURL  string `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	NegativePrompt string `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string `json:"template,omitempty"`        // 视频特效模板

	// Parameters 相关
	Resolution   *string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P
	Size         *string `json:"size,omitempty"`          // 尺寸: 如 "832*480"
	Duration     *int    `json:"duration,omitempty"`      // 时长
	PromptExtend *bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    *bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool   `json:"audio,omitempty"`         // 是否添加音频
	Seed         *int    `json:"seed,omitempty"`          // 随机数种子
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// ValidateMultipartDirect 负责解析并将原始 TaskSubmitReq 存入 context
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v1/services/aigc/video-generation/video-synthesis", a.baseURL), nil
}

// BuildRequestHeader sets required headers for Ali API
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable") // 阿里异步任务必须设置
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}
	if err := populateAliMultipartImage(c, &taskReq); err != nil {
		return nil, errors.Wrap(err, "populate_multipart_image_failed")
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := common.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p = []string{
		"832*480",
		"480*832",
		"624*624",
	}
	size720p = []string{
		"1280*720",
		"720*1280",
		"960*960",
		"1088*832",
		"832*1088",
	}
	size1080p = []string{
		"1920*1080",
		"1080*1920",
		"1440*1440",
		"1632*1248",
		"1248*1632",
	}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	} else if lo.Contains(size720p, size) {
		return "720P", nil
	} else if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func aliResolutionRatios(model string) map[string]float64 {
	switch model {
	case "wan2.7-t2v", "wan2.7-t2v-2026-04-25", "wan2.7-i2v", "wan2.7-i2v-2026-04-25", "wan2.7-r2v", "wan2.7-videoedit",
		"wan2.6-t2v", "wan2.6-i2v", "wan2.6-r2v":
		return map[string]float64{"720P": 1, "1080P": 1 / 0.6}
	case "wan2.6-i2v-flash", "wan2.6-r2v-flash":
		return map[string]float64{"720P": 1, "1080P": 0.25 / 0.15}
	case "wan2.5-t2v-preview", "wan2.5-i2v-preview":
		return map[string]float64{"480P": 1, "720P": 2, "1080P": 1 / 0.3}
	case "wan2.2-t2v-plus", "wan2.2-i2v-plus":
		return map[string]float64{"480P": 1, "1080P": 0.7 / 0.14}
	case "wan2.2-i2v-flash", "wan2.2-kf2v-flash":
		return map[string]float64{"480P": 1, "720P": 2, "1080P": 4.8}
	case "wanx2.1-t2v-plus", "wanx2.1-i2v-plus", "wanx2.1-kf2v-plus", "wanx2.1-vace-plus":
		return map[string]float64{"720P": 1}
	case "wanx2.1-t2v-turbo", "wanx2.1-i2v-turbo":
		return map[string]float64{"480P": 1, "720P": 1}
	case "happyhorse-1.0-t2v", "happyhorse-1.0-i2v", "happyhorse-1.0-r2v", "happyhorse-1.0-video-edit":
		return map[string]float64{"720P": 1, "1080P": 1.6 / 0.9}
	default:
		return nil
	}
}

func aliModeRatios(model string) map[string]float64 {
	switch model {
	case "wan2.2-animate-move":
		return map[string]float64{"WAN-STD": 1, "WAN-PRO": 0.6 / 0.4}
	case "wan2.2-animate-mix":
		return map[string]float64{"WAN-STD": 1, "WAN-PRO": 0.9 / 0.6}
	default:
		return nil
	}
}

func normalizeAliResolution(resolution string) string {
	resolution = strings.ToUpper(strings.TrimSpace(resolution))
	if resolution == "" {
		return ""
	}
	if !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	return resolution
}

func normalizeAliMode(mode string) string {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode == "" {
		return ""
	}
	return mode
}

func normalizeAliRatio(size string) string {
	normalized := strings.TrimSpace(strings.ToLower(size))
	if normalized == "" {
		return ""
	}
	replacer := strings.NewReplacer("*", "x", ":", "x", "：", "x", " ", "")
	normalized = replacer.Replace(normalized)
	switch normalized {
	case "16x9":
		return "16:9"
	case "9x16":
		return "9:16"
	case "1x1":
		return "1:1"
	case "4x3":
		return "4:3"
	case "3x4":
		return "3:4"
	default:
		return ""
	}
}

func aliMetadataStringValue(metadata map[string]interface{}, keys ...string) string {
	if metadata == nil {
		return ""
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if value, ok := metadata[key]; ok {
			text := strings.TrimSpace(fmt.Sprintf("%v", value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func hasAliMediaType(media []AliMediaItem, mediaType string) bool {
	normalizedType := strings.TrimSpace(mediaType)
	if normalizedType == "" {
		return false
	}
	for _, item := range media {
		if strings.EqualFold(strings.TrimSpace(item.Type), normalizedType) {
			return true
		}
	}
	return false
}

func appendAliMediaItem(media []AliMediaItem, mediaType, url string) []AliMediaItem {
	normalizedType := strings.TrimSpace(mediaType)
	normalizedURL := strings.TrimSpace(url)
	if normalizedType == "" || normalizedURL == "" || hasAliMediaType(media, normalizedType) {
		return media
	}
	return append(media, AliMediaItem{
		Type: normalizedType,
		URL:  normalizedURL,
	})
}

func usesAliModernMediaInput(model string) bool {
	return strings.HasPrefix(model, "wan2.7") || strings.HasPrefix(model, "happyhorse-1.0")
}

func isAliImageToVideoModel(model string) bool {
	return strings.Contains(model, "i2v")
}

func isAliReferenceToVideoModel(model string) bool {
	return strings.Contains(model, "r2v")
}

func isAliVideoEditModel(model string) bool {
	return strings.Contains(model, "videoedit") || strings.Contains(model, "video-edit")
}

func isAliTextToVideoModel(model string) bool {
	return strings.Contains(model, "t2v")
}

func firstAliImageInput(req relaycommon.TaskSubmitReq) string {
	if strings.TrimSpace(req.InputReference) != "" {
		return strings.TrimSpace(req.InputReference)
	}
	if strings.TrimSpace(req.Image) != "" {
		return strings.TrimSpace(req.Image)
	}
	for _, image := range req.Images {
		if strings.TrimSpace(image) != "" {
			return strings.TrimSpace(image)
		}
	}
	return ""
}

func legacyAliSizeFromRatio(model, ratio string) string {
	switch ratio {
	case "16:9":
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			return "1920*1080"
		}
		return "1280*720"
	case "9:16":
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			return "1080*1920"
		}
		return "720*1280"
	case "1:1":
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			return "1440*1440"
		}
		return "960*960"
	case "4:3":
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			return "1632*1248"
		}
		return "1088*832"
	case "3:4":
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			return "1248*1632"
		}
		return "832*1088"
	default:
		return ""
	}
}

func applyAliDefaultOutputSpec(model string, params *AliVideoParameters) {
	if params == nil {
		return
	}
	if params.Size != "" || params.Resolution != "" {
		return
	}
	if isAliTextToVideoModel(model) {
		if usesAliModernMediaInput(model) {
			params.Resolution = "1080P"
			return
		}
		if strings.HasPrefix(model, "wan2.5") || strings.HasPrefix(model, "wan2.2") {
			params.Size = "1920*1080"
		} else {
			params.Size = "1280*720"
		}
		return
	}
	switch {
	case strings.HasPrefix(model, "wan2.7"):
		params.Resolution = "1080P"
	case strings.HasPrefix(model, "happyhorse-1.0"):
		params.Resolution = "1080P"
	case strings.HasPrefix(model, "wan2.6"):
		params.Resolution = "1080P"
	case strings.HasPrefix(model, "wan2.5"):
		params.Resolution = "1080P"
	case strings.HasPrefix(model, "wan2.2-i2v-flash"):
		params.Resolution = "720P"
	case strings.HasPrefix(model, "wan2.2-i2v-plus"):
		params.Resolution = "1080P"
	default:
		params.Resolution = "720P"
	}
}

func populateAliMultipartImage(c *gin.Context, req *relaycommon.TaskSubmitReq) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(firstAliImageInput(*req)) != "" {
		return nil
	}
	mf, err := c.MultipartForm()
	if err != nil {
		return nil
	}
	files, exists := mf.File["input_reference"]
	if !exists || len(files) == 0 {
		return nil
	}
	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(fileBytes)
	}
	req.InputReference = fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes))
	if strings.TrimSpace(req.Image) == "" {
		req.Image = req.InputReference
	}
	return nil
}

func aliOtherRatiosForRequest(modelName, resolution, mode string, audio *bool) map[string]float64 {
	otherRatios := make(map[string]float64)
	if resolutionRatios := aliResolutionRatios(modelName); resolutionRatios != nil {
		if ratio, ok := resolutionRatios[normalizeAliResolution(resolution)]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", normalizeAliResolution(resolution))] = ratio
		}
	}
	if modeRatios := aliModeRatios(modelName); modeRatios != nil {
		if ratio, ok := modeRatios[normalizeAliMode(mode)]; ok {
			otherRatios[fmt.Sprintf("mode-%s", strings.ToLower(normalizeAliMode(mode)))] = ratio
		}
	}
	if audio != nil && *audio {
		switch modelName {
		case "wan2.6-i2v-flash", "wan2.6-r2v-flash":
			otherRatios["audio-true"] = 2
		}
	}
	return otherRatios
}

func aliResolutionFromRequest(aliReq *AliVideoRequest) (string, error) {
	if aliReq.Parameters == nil {
		return "", nil
	}
	if aliReq.Parameters.Size != "" {
		return sizeToResolution(aliReq.Parameters.Size)
	}
	return normalizeAliResolution(aliReq.Parameters.Resolution), nil
}

func ProcessAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	resolution, err := aliResolutionFromRequest(aliReq)
	if err != nil {
		return nil, err
	}
	if aliReq.Parameters == nil {
		return map[string]float64{}, nil
	}
	return aliOtherRatiosForRequest(aliReq.Model, resolution, aliReq.Parameters.Mode, aliReq.Parameters.Audio), nil
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info != nil && info.ChannelMeta != nil && info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	imageInput := firstAliImageInput(req)
	ratioInput := normalizeAliRatio(req.Size)
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
			ImgURL: imageInput,
		},
		Parameters: &AliVideoParameters{
			PromptExtend: true, // 默认开启智能改写
			Watermark:    false,
		},
	}
	if aliReq.Parameters == nil {
		aliReq.Parameters = &AliVideoParameters{}
	}

	// 处理分辨率映射
	if req.Size != "" {
		// text to video size must be contained *
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			if ratioInput == "" {
				return nil, fmt.Errorf("invalid size: %s, example: %s", req.Size, "1920*1080")
			}
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else if ratioInput != "" {
			if usesAliModernMediaInput(upstreamModel) {
				aliReq.Parameters.Ratio = ratioInput
			} else if isAliTextToVideoModel(upstreamModel) {
				aliReq.Parameters.Size = legacyAliSizeFromRatio(upstreamModel, ratioInput)
			}
		} else {
			resolution := strings.ToUpper(req.Size)
			// 支持 480p, 720p, 1080p 或 480P, 720P, 1080P
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	}

	// 处理时长
	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			return nil, errors.Wrap(err, "convert seconds to int failed")
		} else {
			aliReq.Parameters.Duration = seconds
		}
	} else {
		aliReq.Parameters.Duration = 5 // 默认5秒
	}

	// 从 metadata 中提取额外参数
	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			err = common.Unmarshal(metadataBytes, aliReq)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}
	if aliReq.Parameters == nil {
		aliReq.Parameters = &AliVideoParameters{}
	}
	aliReq.Parameters.Watermark = false
	if aliReq.Parameters.Resolution == "" {
		aliReq.Parameters.Resolution = normalizeAliResolution(
			aliMetadataStringValue(req.Metadata, "resolution", "output_resolution", "outputResolution"),
		)
	} else {
		aliReq.Parameters.Resolution = normalizeAliResolution(aliReq.Parameters.Resolution)
	}
	if normalizedRatio := normalizeAliRatio(aliReq.Parameters.Ratio); normalizedRatio != "" {
		aliReq.Parameters.Ratio = normalizedRatio
	}

	applyAliDefaultOutputSpec(upstreamModel, aliReq.Parameters)

	if usesAliModernMediaInput(upstreamModel) {
		referenceVideoInput := aliMetadataStringValue(req.Metadata, "reference_video", "referenceVideo")
		referenceVoiceInput := aliMetadataStringValue(req.Metadata, "reference_voice", "referenceVoice")
		if referenceVideoInput == "" {
			referenceVideoInput = strings.TrimSpace(aliReq.Input.VideoURL)
		}
		if referenceVoiceInput == "" {
			referenceVoiceInput = strings.TrimSpace(aliReq.Input.ReferenceVoice)
		}
		if referenceVoiceInput == "" && strings.HasPrefix(upstreamModel, "wan2.7") {
			referenceVoiceInput = strings.TrimSpace(aliReq.Input.AudioURL)
		}
		if referenceVoiceInput == "" && strings.HasPrefix(upstreamModel, "wan2.7") {
			referenceVoiceInput = aliMetadataStringValue(req.Metadata, "voice_url", "voiceUrl")
		}
		switch {
		case isAliImageToVideoModel(upstreamModel):
			aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "first_frame", imageInput)
			aliReq.Input.ImgURL = ""
			aliReq.Input.ReferenceVoice = ""
			if aliReq.Parameters != nil {
				aliReq.Parameters.Size = ""
				aliReq.Parameters.Ratio = ""
			}
		case isAliReferenceToVideoModel(upstreamModel):
			aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_image", imageInput)
			if strings.HasPrefix(upstreamModel, "wan2.7") {
				aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_video", referenceVideoInput)
				aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_voice", referenceVoiceInput)
			}
			aliReq.Input.ImgURL = ""
			aliReq.Input.VideoURL = ""
			aliReq.Input.ReferenceVoice = ""
			aliReq.Input.AudioURL = ""
			if aliReq.Parameters != nil {
				aliReq.Parameters.Size = ""
			}
		case isAliVideoEditModel(upstreamModel):
			aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_image", imageInput)
			if strings.HasPrefix(upstreamModel, "wan2.7") {
				aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_video", referenceVideoInput)
				aliReq.Input.Media = appendAliMediaItem(aliReq.Input.Media, "reference_voice", referenceVoiceInput)
			}
			aliReq.Input.ImgURL = ""
			aliReq.Input.VideoURL = ""
			aliReq.Input.ReferenceVoice = ""
			aliReq.Input.AudioURL = ""
			if aliReq.Parameters != nil {
				aliReq.Parameters.Size = ""
			}
		case isAliTextToVideoModel(upstreamModel):
			if aliReq.Parameters != nil {
				aliReq.Parameters.Size = ""
			}
		}
	}

	return aliReq, nil
}

// EstimateBilling 根据用户请求参数计算 OtherRatios（时长、分辨率等）。
// 在 ValidateRequestAndSetAction 之后、价格计算之前调用。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil
	}

	otherRatios := map[string]float64{
		"seconds": float64(aliReq.Parameters.Duration),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	return otherRatios
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, _ *relaycommon.TaskInfo) int {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(task.Data, &aliResp); err != nil {
		return 0
	}
	if aliResp.Usage == nil {
		return 0
	}

	duration := aliResp.Usage.Duration
	if duration <= 0 {
		if aliResp.Usage.OutputVideoDuration > 0 {
			duration = aliResp.Usage.OutputVideoDuration
		} else {
			return 0
		}
	}

	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
	}

	modelPrice := bc.ModelPrice
	if modelPrice <= 0 {
		price, ok := ratio_setting.GetModelPrice(task.Properties.OriginModelName, false)
		if !ok {
			return 0
		}
		modelPrice = price
	}

	otherMultiplier := 1.0
	for key, value := range bc.OtherRatios {
		if value <= 0 || value == 1 {
			continue
		}
		if key == "seconds" || strings.HasPrefix(key, "resolution-") {
			continue
		}
		otherMultiplier *= value
	}

	if resolutionRatios := aliResolutionRatios(task.Properties.OriginModelName); resolutionRatios != nil {
		resolutionKey := normalizeAliResolution(fmt.Sprintf("%d", aliResp.Usage.SR))
		if ratio, ok := resolutionRatios[resolutionKey]; ok {
			otherMultiplier *= ratio
		} else {
			for key, value := range bc.OtherRatios {
				if strings.HasPrefix(key, "resolution-") && value > 0 {
					otherMultiplier *= value
					break
				}
			}
		}
	}

	return int(modelPrice * duration * otherMultiplier * common.QuotaPerUnit * bc.GroupRatio)
}

// DoRequest delegates to common helper
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// 解析阿里响应
	var aliResp AliVideoResponse
	if err := common.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 检查错误
	if aliResp.Code != "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}

	if aliResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 转换为 OpenAI 格式响应
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()

	// 返回 OpenAI 格式
	c.JSON(http.StatusOK, openAIResp)

	return aliResp.Output.TaskID, responseBody, nil
}

// FetchTask 查询任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ParseTaskResult 解析任务结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// 状态映射
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		// 阿里直接返回视频URL，不需要额外的代理端点
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(task.Data, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal ali response failed")
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt

	// 设置视频URL（核心字段）
	openAIResp.SetMetadata("url", aliResp.Output.VideoURL)

	// 错误处理
	if aliResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Code,
			Message: aliResp.Message,
		}
	} else if aliResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Output.Code,
			Message: aliResp.Output.Message,
		}
	}

	return common.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}
