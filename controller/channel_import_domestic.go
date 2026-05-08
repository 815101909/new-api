package controller

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type domesticChannelTemplate struct {
	VendorName string
	Name       string
	Type       int
	Models     []string
}

type domesticChannelProvider struct {
	Name string
	Type int
}

var domesticChannelProviders = []domesticChannelProvider{
	{Name: "DeepSeek", Type: constant.ChannelTypeDeepSeek},
	{Name: "阿里百炼", Type: constant.ChannelTypeAli},
	{Name: "智谱", Type: constant.ChannelTypeZhipu_v4},
	{Name: "Moonshot", Type: constant.ChannelTypeMoonshot},
	{Name: "火山方舟", Type: constant.ChannelTypeVolcEngine},
	{Name: "百度千帆", Type: constant.ChannelTypeBaiduV2},
	{Name: "腾讯混元", Type: constant.ChannelTypeTencent},
	{Name: "MiniMax", Type: constant.ChannelTypeMiniMax},
	{Name: "SiliconFlow", Type: constant.ChannelTypeSiliconFlow},
	{Name: "快手可灵", Type: constant.ChannelTypeKling},
	{Name: "即梦", Type: constant.ChannelTypeJimeng},
	{Name: "Vidu", Type: constant.ChannelTypeVidu},
}

func ImportDomesticDefaultChannels(c *gin.Context) {
	templates := getDomesticDefaultChannelTemplates()

	createdChannels := 0
	updatedChannels := 0
	skippedChannels := 0
	addedModels := 0
	createdList := make([]string, 0)
	updatedList := make([]string, 0)
	skippedList := make([]string, 0)

	for _, template := range templates {
		if len(template.Models) == 0 {
			skippedChannels++
			skippedList = append(skippedList, template.Name)
			continue
		}

		existing, err := getFirstChannelByType(template.Type)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		if existing == nil {
			channel := buildDomesticTemplateChannel(template)
			if err := channel.Insert(); err != nil {
				common.ApiError(c, err)
				return
			}
			createdChannels++
			addedModels += len(template.Models)
			createdList = append(createdList, template.Name)
			continue
		}

		mergedModels, added := mergeChannelModels(existing.Models, template.Models)
		if added == 0 {
			skippedChannels++
			skippedList = append(skippedList, existing.Name)
			continue
		}

		existing.Models = mergedModels
		if strings.TrimSpace(existing.Group) == "" {
			existing.Group = "default"
		}
		if existing.BaseURL == nil || strings.TrimSpace(*existing.BaseURL) == "" {
			baseURL := defaultChannelBaseURL(template.Type)
			existing.BaseURL = &baseURL
		}
		if err := existing.Update(); err != nil {
			common.ApiError(c, err)
			return
		}
		updatedChannels++
		addedModels += added
		updatedList = append(updatedList, existing.Name)
	}

	if createdChannels > 0 || updatedChannels > 0 {
		model.InitChannelCache()
		model.RefreshPricing()
		service.ResetProxyClientCache()
	}

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"created_channels": createdChannels,
			"updated_channels": updatedChannels,
			"skipped_channels": skippedChannels,
			"added_models":     addedModels,
			"created":          createdList,
			"updated":          updatedList,
			"skipped":          skippedList,
		},
	})
}

func getDomesticDefaultChannelTemplates() []domesticChannelTemplate {
	_, upstreamModels := getDomesticDefaultCatalog()

	modelsByVendor := make(map[string][]string)
	seenByVendor := make(map[string]map[string]struct{})
	for _, item := range upstreamModels {
		vendorName := strings.TrimSpace(item.VendorName)
		modelName := strings.TrimSpace(item.ModelName)
		if vendorName == "" || modelName == "" {
			continue
		}
		if seenByVendor[vendorName] == nil {
			seenByVendor[vendorName] = make(map[string]struct{})
		}
		if _, ok := seenByVendor[vendorName][modelName]; ok {
			continue
		}
		seenByVendor[vendorName][modelName] = struct{}{}
		modelsByVendor[vendorName] = append(modelsByVendor[vendorName], modelName)
	}

	templates := make([]domesticChannelTemplate, 0, len(domesticChannelProviders))
	for _, provider := range domesticChannelProviders {
		models := modelsByVendor[provider.Name]
		sort.Strings(models)
		templates = append(templates, domesticChannelTemplate{
			VendorName: provider.Name,
			Name:       provider.Name,
			Type:       provider.Type,
			Models:     models,
		})
	}
	return templates
}

func getFirstChannelByType(channelType int) (*model.Channel, error) {
	var channel model.Channel
	err := model.DB.Where("type = ?", channelType).Order("id ASC").First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func buildDomesticTemplateChannel(template domesticChannelTemplate) *model.Channel {
	baseURL := defaultChannelBaseURL(template.Type)
	priority := int64(0)
	weight := uint(0)
	autoBan := 1
	tag := "domestic-defaults"
	remark := "导入的国产渠道模板：填入官方 API Key 后再启用。"

	return &model.Channel{
		Type:        template.Type,
		Key:         "",
		Status:      common.ChannelStatusManuallyDisabled,
		Name:        template.Name,
		BaseURL:     &baseURL,
		Models:      strings.Join(template.Models, ","),
		Group:       "default",
		Priority:    &priority,
		Weight:      &weight,
		AutoBan:     &autoBan,
		Tag:         &tag,
		Remark:      &remark,
		CreatedTime: common.GetTimestamp(),
	}
}

func defaultChannelBaseURL(channelType int) string {
	if channelType >= 0 && channelType < len(constant.ChannelBaseURLs) {
		return constant.ChannelBaseURLs[channelType]
	}
	return ""
}

func mergeChannelModels(existing string, additions []string) (string, int) {
	models := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(existing, ",") {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}

	added := 0
	for _, item := range additions {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
		added++
	}

	return strings.Join(models, ","), added
}
