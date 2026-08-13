package vidu

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	contextKeyViduImageRequest = "vidu_image_request"
	viduImagePollInterval      = 2 * time.Second
	viduImageMaxPollAttempts   = 180
)

type imageRequestPayload struct {
	Model       string   `json:"model"`
	Images      []string `json:"images,omitempty"`
	Prompt      string   `json:"prompt"`
	Seed        int      `json:"seed,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	Payload     string   `json:"payload,omitempty"`
	CallbackURL string   `json:"callback_url,omitempty"`
}

type imageSubmitResponse struct {
	TaskID      string   `json:"task_id"`
	State       string   `json:"state"`
	Model       string   `json:"model"`
	Images      []string `json:"images"`
	Prompt      string   `json:"prompt"`
	Seed        int      `json:"seed"`
	AspectRatio string   `json:"aspect_ratio"`
	Resolution  string   `json:"resolution"`
	Payload     string   `json:"payload"`
	Credits     int      `json:"credits"`
	ErrCode     string   `json:"err_code"`
	CreatedAt   string   `json:"created_at"`
}

type imageTaskResponse struct {
	ID        string          `json:"id"`
	State     string          `json:"state"`
	ErrCode   string          `json:"err_code"`
	Credits   int             `json:"credits"`
	Payload   string          `json:"payload"`
	Creations []imageCreation `json:"creations"`
}

type imageCreation struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	CoverURL       string `json:"cover_url"`
	WatermarkedURL string `json:"watermarked_url"`
}

func oaiImage2ViduImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (*imageRequestPayload, error) {
	imageCount := uint(1)
	if request.N != nil {
		imageCount = *request.N
	}
	if imageCount > 1 {
		return nil, fmt.Errorf("vidu image generation only supports n=1")
	}

	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}
	if modelName == "" {
		modelName = "viduq2"
	}

	images, err := collectViduRequestImages(c, request)
	if err != nil {
		return nil, err
	}
	if len(images) > 7 {
		return nil, fmt.Errorf("vidu image generation supports at most 7 reference images")
	}

	normalizedModel := strings.ToLower(modelName)
	if normalizedModel == "viduq1" && len(images) == 0 {
		return nil, fmt.Errorf("viduq1 requires at least one reference image")
	}

	payload := &imageRequestPayload{
		Model:       modelName,
		Images:      images,
		Prompt:      request.Prompt,
		AspectRatio: resolveViduAspectRatio(request),
		Resolution:  resolveViduResolution(request, modelName),
	}
	applyViduImageExtraFields(payload, request)
	applyViduImageFormFields(c, payload)
	payload.Resolution = normalizeViduResolution(payload.Resolution, request.Size, modelName)
	if payload.AspectRatio == "" {
		payload.AspectRatio = resolveViduAspectRatio(request)
	}

	c.Set(contextKeyViduImageRequest, payload)
	return payload, nil
}

func applyViduImageExtraFields(payload *imageRequestPayload, request dto.ImageRequest) {
	if payload == nil {
		return
	}
	if len(request.ExtraFields) > 0 {
		var extra imageRequestPayload
		if err := common.Unmarshal(request.ExtraFields, &extra); err == nil {
			mergeViduImageExtra(payload, extra)
		}
	}
	if raw, ok := request.Extra["seed"]; ok {
		var seed int
		if err := common.Unmarshal(raw, &seed); err == nil {
			payload.Seed = seed
		}
	}
	if raw, ok := request.Extra["aspect_ratio"]; ok {
		var aspectRatio string
		if err := common.Unmarshal(raw, &aspectRatio); err == nil && strings.TrimSpace(aspectRatio) != "" {
			payload.AspectRatio = strings.TrimSpace(aspectRatio)
		}
	}
	if raw, ok := request.Extra["resolution"]; ok {
		var resolution string
		if err := common.Unmarshal(raw, &resolution); err == nil && strings.TrimSpace(resolution) != "" {
			payload.Resolution = strings.TrimSpace(resolution)
		}
	}
	if raw, ok := request.Extra["payload"]; ok {
		var passthrough string
		if err := common.Unmarshal(raw, &passthrough); err == nil {
			payload.Payload = passthrough
		}
	}
	if raw, ok := request.Extra["callback_url"]; ok {
		var callbackURL string
		if err := common.Unmarshal(raw, &callbackURL); err == nil {
			payload.CallbackURL = callbackURL
		}
	}
}

func mergeViduImageExtra(payload *imageRequestPayload, extra imageRequestPayload) {
	if extra.Seed != 0 {
		payload.Seed = extra.Seed
	}
	if strings.TrimSpace(extra.AspectRatio) != "" {
		payload.AspectRatio = strings.TrimSpace(extra.AspectRatio)
	}
	if strings.TrimSpace(extra.Resolution) != "" {
		payload.Resolution = strings.TrimSpace(extra.Resolution)
	}
	if strings.TrimSpace(extra.Payload) != "" {
		payload.Payload = strings.TrimSpace(extra.Payload)
	}
	if strings.TrimSpace(extra.CallbackURL) != "" {
		payload.CallbackURL = strings.TrimSpace(extra.CallbackURL)
	}
}

func applyViduImageFormFields(c *gin.Context, payload *imageRequestPayload) {
	if payload == nil || !isViduMultipartFormRequest(c) {
		return
	}
	form := c.Request.PostForm
	if form == nil {
		if _, err := c.MultipartForm(); err != nil {
			return
		}
		form = c.Request.PostForm
	}
	if form == nil {
		return
	}
	if seedText := strings.TrimSpace(form.Get("seed")); seedText != "" {
		if seed, err := strconv.Atoi(seedText); err == nil {
			payload.Seed = seed
		}
	}
	if aspectRatio := strings.TrimSpace(form.Get("aspect_ratio")); aspectRatio != "" {
		payload.AspectRatio = aspectRatio
	}
	if resolution := strings.TrimSpace(form.Get("resolution")); resolution != "" {
		payload.Resolution = resolution
	}
	if passthrough := strings.TrimSpace(form.Get("payload")); passthrough != "" {
		payload.Payload = passthrough
	}
	if callbackURL := strings.TrimSpace(form.Get("callback_url")); callbackURL != "" {
		payload.CallbackURL = callbackURL
	}
}

func collectViduRequestImages(c *gin.Context, request dto.ImageRequest) ([]string, error) {
	images := make([]string, 0, 7)
	appendRawImageValues(&images, request.Images)
	appendRawImageValues(&images, request.Image)
	images = append(images, collectViduMultipartImageValues(c)...)

	multipartImages, err := collectViduMultipartImages(c)
	if err != nil {
		return nil, err
	}
	images = append(images, multipartImages...)

	seen := make(map[string]struct{}, len(images))
	deduped := make([]string, 0, len(images))
	for _, image := range images {
		normalized := strings.TrimSpace(image)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		deduped = append(deduped, normalized)
	}
	return deduped, nil
}

func appendRawImageValues(images *[]string, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var list []string
	if err := common.Unmarshal(raw, &list); err == nil {
		*images = append(*images, list...)
		return
	}
	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		*images = append(*images, single)
	}
}

func collectViduMultipartImageValues(c *gin.Context) []string {
	if !isViduMultipartFormRequest(c) {
		return nil
	}
	form := c.Request.PostForm
	if form == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil
		}
		form = c.Request.PostForm
	}
	if form == nil {
		return nil
	}

	values := make([]string, 0)
	values = append(values, form["image"]...)
	values = append(values, form["images"]...)
	values = append(values, form["image[]"]...)
	values = append(values, form["images[]"]...)

	keys := make([]string, 0, len(form))
	for key := range form {
		if (strings.HasPrefix(key, "image[") || strings.HasPrefix(key, "images[")) && key != "image[]" && key != "images[]" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		values = append(values, form[key]...)
	}
	return values
}

func collectViduMultipartImages(c *gin.Context) ([]string, error) {
	if !isViduMultipartFormRequest(c) {
		return nil, nil
	}
	mf := c.Request.MultipartForm
	if mf == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
		}
		mf = c.Request.MultipartForm
	}
	if mf == nil || len(mf.File) == 0 {
		return nil, nil
	}

	fileHeaders := collectViduImageFileHeaders(mf)
	images := make([]string, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		image, err := imageFileHeaderToDataURI(fileHeader)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, nil
}

func isViduMultipartFormRequest(c *gin.Context) bool {
	return c != nil &&
		c.Request != nil &&
		strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "multipart/form-data")
}

func collectViduImageFileHeaders(mf *multipart.Form) []*multipart.FileHeader {
	if mf == nil || mf.File == nil {
		return nil
	}
	fileHeaders := make([]*multipart.FileHeader, 0)
	fileHeaders = append(fileHeaders, mf.File["image"]...)
	fileHeaders = append(fileHeaders, mf.File["image[]"]...)

	keys := make([]string, 0, len(mf.File))
	for key := range mf.File {
		if strings.HasPrefix(key, "image[") && key != "image[]" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fileHeaders = append(fileHeaders, mf.File[key]...)
	}
	return fileHeaders
}

func imageFileHeaderToDataURI(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", fmt.Errorf("image file is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open reference image %s: %w", fileHeader.Filename, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read reference image %s: %w", fileHeader.Filename, err)
	}
	mimeType := detectViduImageMimeType(fileHeader, data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

func detectViduImageMimeType(fileHeader *multipart.FileHeader, data []byte) string {
	if fileHeader != nil {
		if contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type")); contentType != "" {
			return contentType
		}
		switch strings.ToLower(filepath.Ext(fileHeader.Filename)) {
		case ".jpg", ".jpeg":
			return "image/jpeg"
		case ".webp":
			return "image/webp"
		case ".png":
			return "image/png"
		}
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "image/png"
}

func resolveViduAspectRatio(request dto.ImageRequest) string {
	if raw, ok := request.Extra["aspect_ratio"]; ok {
		var aspectRatio string
		if err := common.Unmarshal(raw, &aspectRatio); err == nil && strings.TrimSpace(aspectRatio) != "" {
			return strings.TrimSpace(aspectRatio)
		}
	}
	width, height, ok := parseViduSize(request.Size)
	if !ok {
		return "16:9"
	}
	ratio := reduceViduAspectRatio(width, height)
	switch ratio {
	case "16:9", "9:16", "1:1", "3:4", "4:3", "21:9", "2:3", "3:2":
		return ratio
	default:
		return "16:9"
	}
}

func resolveViduResolution(request dto.ImageRequest, model string) string {
	if raw, ok := request.Extra["resolution"]; ok {
		var resolution string
		if err := common.Unmarshal(raw, &resolution); err == nil && strings.TrimSpace(resolution) != "" {
			return normalizeViduResolution(resolution, request.Size, model)
		}
	}
	return normalizeViduResolution("", request.Size, model)
}

func normalizeViduResolution(resolution string, size string, model string) string {
	if strings.EqualFold(strings.TrimSpace(model), "viduq1") {
		return "1080p"
	}
	normalized := strings.ToLower(strings.TrimSpace(resolution))
	switch normalized {
	case "4k":
		return "4K"
	case "2k":
		return "2K"
	case "1080p", "1080":
		return "1080p"
	case "1k":
		return "1080p"
	}
	width, height, ok := parseViduSize(size)
	if !ok {
		return "1080p"
	}
	longSide := width
	if height > longSide {
		longSide = height
	}
	switch {
	case longSide >= 3000:
		return "4K"
	case longSide >= 1500:
		return "2K"
	default:
		return "1080p"
	}
}

func parseViduSize(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func reduceViduAspectRatio(width int, height int) string {
	divisor := viduGCD(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func viduGCD(a int, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func viduImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, viduImageOpenAIError(extractViduImageErrorMessage(responseBody), resp.StatusCode)
	}

	var submitResp imageSubmitResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if strings.EqualFold(submitResp.State, "failed") {
		return nil, viduImageOpenAIError(firstNonEmpty(submitResp.ErrCode, "vidu image task failed"), resp.StatusCode)
	}
	if strings.TrimSpace(submitResp.TaskID) == "" {
		return nil, types.NewError(fmt.Errorf("vidu image response missing task_id"), types.ErrorCodeBadResponseBody)
	}

	taskResp, _, waitErr := waitViduImageTask(c, info, submitResp.TaskID)
	if waitErr != nil {
		return nil, types.NewError(waitErr, types.ErrorCodeBadResponse)
	}
	if !strings.EqualFold(taskResp.State, "success") {
		return nil, viduImageOpenAIError(firstNonEmpty(taskResp.ErrCode, "vidu image task failed"), http.StatusBadGateway)
	}

	openAIResponse, err := responseVidu2OpenAIImage(taskResp, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	applyViduImageBillingRatio(c, info, taskResp)

	jsonResponse, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	resp.Header.Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return &dto.Usage{}, nil
}

func waitViduImageTask(c *gin.Context, info *relaycommon.RelayInfo, taskID string) (*imageTaskResponse, []byte, error) {
	var lastState string
	for attempt := 0; attempt < viduImageMaxPollAttempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(viduImagePollInterval)
			select {
			case <-timer.C:
			case <-c.Request.Context().Done():
				timer.Stop()
				return nil, nil, c.Request.Context().Err()
			}
		}
		taskResp, body, err := fetchViduImageTask(info, taskID)
		if err != nil {
			return nil, nil, err
		}
		lastState = taskResp.State
		switch strings.ToLower(strings.TrimSpace(taskResp.State)) {
		case "success":
			return taskResp, body, nil
		case "failed":
			return taskResp, body, nil
		case "created", "queueing", "processing", "":
			continue
		default:
			return nil, body, fmt.Errorf("unknown vidu image task state: %s", taskResp.State)
		}
	}
	return nil, nil, fmt.Errorf("vidu image task polling timeout, last state: %s", lastState)
}

func fetchViduImageTask(info *relaycommon.RelayInfo, taskID string) (*imageTaskResponse, []byte, error) {
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = strings.TrimRight("https://api.vidu.cn", "/")
	}
	requestURL := fmt.Sprintf("%s/ent/v2/tasks/%s/creations", baseURL, url.PathEscape(taskID))
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", info.ChannelOtherSettings.GetViduAuthorizationHeader(info.ApiKey))

	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, body, fmt.Errorf("vidu image task query failed, status %d: %s", resp.StatusCode, string(body))
	}
	var taskResp imageTaskResponse
	if err := common.Unmarshal(body, &taskResp); err != nil {
		return nil, body, err
	}
	return &taskResp, body, nil
}

func extractViduImageErrorMessage(responseBody []byte) string {
	bodyText := strings.TrimSpace(string(responseBody))
	if len(responseBody) == 0 {
		return "vidu image request failed"
	}
	var payload map[string]any
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return firstNonEmpty(bodyText, "vidu image request failed")
	}
	return firstNonEmpty(
		stringValueFromMap(payload, "message"),
		stringValueFromMap(payload, "msg"),
		stringValueFromMap(payload, "error"),
		stringValueFromMap(payload, "err_msg"),
		stringValueFromMap(payload, "err_code"),
		stringValueFromMap(payload, "code"),
		stringValueFromMap(payload, "detail"),
		nestedStringValueFromMap(payload, "error", "message"),
		nestedStringValueFromMap(payload, "error", "code"),
		bodyText,
		"vidu image request failed",
	)
}

func stringValueFromMap(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	}
	return ""
}

func nestedStringValueFromMap(payload map[string]any, parent string, key string) string {
	if payload == nil {
		return ""
	}
	nested, ok := payload[parent].(map[string]any)
	if !ok {
		return ""
	}
	return stringValueFromMap(nested, key)
}

func responseVidu2OpenAIImage(response *imageTaskResponse, info *relaycommon.RelayInfo) (*dto.ImageResponse, error) {
	imageResponse := &dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}
	for _, creation := range response.Creations {
		imageURL := strings.TrimSpace(creation.URL)
		if imageURL == "" {
			continue
		}
		imageResponse.Data = append(imageResponse.Data, dto.ImageData{Url: imageURL})
	}
	if len(imageResponse.Data) == 0 {
		return nil, fmt.Errorf("vidu image task succeeded without image url")
	}
	return imageResponse, nil
}

func applyViduImageBillingRatio(c *gin.Context, info *relaycommon.RelayInfo, taskResp *imageTaskResponse) {
	if info == nil || taskResp == nil {
		return
	}
	credits := taskResp.Credits
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	if credits <= 0 {
		if payload, ok := c.Get(contextKeyViduImageRequest); ok {
			if req, ok := payload.(*imageRequestPayload); ok {
				modelName = firstNonEmpty(req.Model, modelName)
				credits = estimateViduImageCredits(req.Model, len(req.Images), req.Resolution)
			}
		}
	}
	if credits <= 0 {
		return
	}
	if ratio := viduImageCreditsRatio(modelName, credits); ratio > 0 {
		info.PriceData.AddOtherRatio("vidu-image-price", ratio)
	}
}

func viduImageCreditsRatio(model string, credits int) float64 {
	baseCredits := viduImageBaseCredits(model)
	if baseCredits <= 0 || credits <= 0 {
		return 0
	}
	return float64(credits) / float64(baseCredits)
}

func viduImageBaseCredits(model string) int {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "viduq2":
		return 35
	case "viduq1":
		return 80
	default:
		return 0
	}
}

func estimateViduImageCredits(model string, imageCount int, resolution string) int {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	normalizedResolution := normalizeViduResolution(resolution, "", normalizedModel)
	if normalizedModel == "viduq1" {
		return 20
	}
	if normalizedModel != "viduq2" {
		return 0
	}

	switch {
	case imageCount <= 0:
		switch normalizedResolution {
		case "4K":
			return 10
		case "2K":
			return 8
		default:
			return 6
		}
	case imageCount <= 3:
		switch normalizedResolution {
		case "4K":
			return 20
		case "2K":
			return 12
		default:
			return 8
		}
	default:
		switch normalizedResolution {
		case "4K":
			return 30
		case "2K":
			return 16
		default:
			return 10
		}
	}
}

func viduImageOpenAIError(message string, statusCode int) *types.NewAPIError {
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	return types.WithOpenAIError(types.OpenAIError{
		Message: message,
		Type:    "vidu_image_error",
		Code:    "vidu_image_error",
	}, statusCode)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
