package operation_setting

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

// ---------------------------------------------------------------------------
// Tool call prices (CNY/1K calls, admin-configurable)
// DB key: tool_price_setting.prices
//
// Key format:
//   - "tool_name"              → default price for all models
//   - "tool_name:model_prefix*" → override for models matching the prefix
//
// Lookup order: longest prefix match → default → hardcoded fallback → 0
// ---------------------------------------------------------------------------

var defaultToolPrices = map[string]float64{
	"web_search":         73.0,  // OpenAI web search (all models) / Claude web search
	"web_search_preview": 73.0,  // OpenAI web search preview (default: reasoning models)
	"file_search":        18.25, // OpenAI file search (Responses API)
	"google_search":      102.2, // Gemini Grounding with Google Search
}

var defaultToolPriceOverrides = map[string]float64{
	"web_search_preview:gpt-4o*":       182.5, // non-reasoning models
	"web_search_preview:gpt-4.1*":      182.5,
	"web_search_preview:gpt-4o-mini*":  182.5,
	"web_search_preview:gpt-4.1-mini*": 182.5,
}

// ToolPriceSetting is managed by config.GlobalConfig.Register.
type ToolPriceSetting struct {
	Prices map[string]float64 `json:"prices"`
}

var toolPriceSetting = ToolPriceSetting{
	Prices: func() map[string]float64 {
		m := make(map[string]float64, len(defaultToolPrices)+len(defaultToolPriceOverrides))
		for k, v := range defaultToolPrices {
			m[k] = v
		}
		for k, v := range defaultToolPriceOverrides {
			m[k] = v
		}
		return m
	}(),
}

func init() {
	config.GlobalConfig.Register("tool_price_setting", &toolPriceSetting)
	RebuildToolPriceIndex()
}

// ---------------------------------------------------------------------------
// Precomputed price index (atomic, lock-free on read path)
// ---------------------------------------------------------------------------

type prefixEntry struct {
	prefix string
	price  float64
}

type toolPriceIndex struct {
	defaults map[string]float64
	prefixes map[string][]prefixEntry
}

var currentIndex atomic.Pointer[toolPriceIndex]

// RebuildToolPriceIndex rebuilds the lookup index from the current config.
// Called on init and after config updates. Not on the billing hot path.
func RebuildToolPriceIndex() {
	merged := make(map[string]float64, len(defaultToolPrices)+len(defaultToolPriceOverrides)+len(toolPriceSetting.Prices))
	for k, v := range defaultToolPrices {
		merged[k] = v
	}
	for k, v := range defaultToolPriceOverrides {
		merged[k] = v
	}
	for k, v := range toolPriceSetting.Prices {
		merged[k] = v
	}

	idx := &toolPriceIndex{
		defaults: make(map[string]float64),
		prefixes: make(map[string][]prefixEntry),
	}

	for key, price := range merged {
		colonIdx := strings.IndexByte(key, ':')
		if colonIdx < 0 {
			idx.defaults[key] = price
			continue
		}
		toolName := key[:colonIdx]
		modelPart := key[colonIdx+1:]
		prefix := strings.TrimSuffix(modelPart, "*")
		idx.prefixes[toolName] = append(idx.prefixes[toolName], prefixEntry{prefix: prefix, price: price})
	}

	for tool := range idx.prefixes {
		entries := idx.prefixes[tool]
		sort.Slice(entries, func(i, j int) bool {
			return len(entries[i].prefix) > len(entries[j].prefix)
		})
		idx.prefixes[tool] = entries
	}

	currentIndex.Store(idx)
}

// GetToolPriceForModel returns the price (CNY/1K calls) for a tool given a model name.
// Lookup: longest prefix match → tool default → 0.
func GetToolPriceForModel(toolName, modelName string) float64 {
	idx := currentIndex.Load()
	if idx == nil {
		if v, ok := defaultToolPrices[toolName]; ok {
			return v
		}
		return 0
	}

	if entries, ok := idx.prefixes[toolName]; ok && modelName != "" {
		for _, e := range entries {
			if strings.HasPrefix(modelName, e.prefix) {
				return e.price
			}
		}
	}

	if p, ok := idx.defaults[toolName]; ok {
		return p
	}
	return 0
}

// GetToolPrice is a convenience wrapper when no model name is needed.
func GetToolPrice(toolName string) float64 {
	return GetToolPriceForModel(toolName, "")
}

// ---------------------------------------------------------------------------
// GPT Image 1 per-call pricing (special: depends on quality + size)
// ---------------------------------------------------------------------------

const (
	GPTImage1Low1024x1024    = 0.0803
	GPTImage1Low1024x1536    = 0.1168
	GPTImage1Low1536x1024    = 0.1168
	GPTImage1Medium1024x1024 = 0.3066
	GPTImage1Medium1024x1536 = 0.4599
	GPTImage1Medium1536x1024 = 0.4599
	GPTImage1High1024x1024   = 1.2191
	GPTImage1High1024x1536   = 1.825
	GPTImage1High1536x1024   = 1.825
)

func GetGPTImage1PriceOnceCall(quality string, size string) float64 {
	prices := map[string]map[string]float64{
		"low": {
			"1024x1024": GPTImage1Low1024x1024,
			"1024x1536": GPTImage1Low1024x1536,
			"1536x1024": GPTImage1Low1536x1024,
		},
		"medium": {
			"1024x1024": GPTImage1Medium1024x1024,
			"1024x1536": GPTImage1Medium1024x1536,
			"1536x1024": GPTImage1Medium1536x1024,
		},
		"high": {
			"1024x1024": GPTImage1High1024x1024,
			"1024x1536": GPTImage1High1024x1536,
			"1536x1024": GPTImage1High1536x1024,
		},
	}

	if qualityMap, exists := prices[quality]; exists {
		if price, exists := qualityMap[size]; exists {
			return price
		}
	}

	return GPTImage1High1024x1024
}

// ---------------------------------------------------------------------------
// Gemini audio input pricing (per-million tokens, model-specific)
// ---------------------------------------------------------------------------

const (
	Gemini25FlashPreviewInputAudioPrice     = 7.30
	Gemini25FlashProductionInputAudioPrice  = 7.30
	Gemini25FlashLitePreviewInputAudioPrice = 3.65
	Gemini25FlashNativeAudioInputAudioPrice = 21.90
	Gemini20FlashInputAudioPrice            = 5.11
	GeminiRoboticsER15InputAudioPrice       = 7.30
)

func GetGeminiInputAudioPricePerMillionTokens(modelName string) float64 {
	if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-native-audio") {
		return Gemini25FlashNativeAudioInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-lite") {
		return Gemini25FlashLitePreviewInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash-preview") {
		return Gemini25FlashPreviewInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash") {
		return Gemini25FlashProductionInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.0-flash") {
		return Gemini20FlashInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-robotics-er-1.5") {
		return GeminiRoboticsER15InputAudioPrice
	}
	return 0
}
