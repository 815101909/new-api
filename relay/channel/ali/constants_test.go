package ali

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelListIncludesAudioModels(t *testing.T) {
	modelSet := make(map[string]struct{}, len(ModelList))
	for _, modelName := range ModelList {
		modelSet[modelName] = struct{}{}
	}

	for _, modelName := range []string{
		"paraformer-v2",
		"fun-asr",
		"cosyvoice-v3.5-plus",
		"cosyvoice-v3.5-flash",
		"cosyvoice-v3-plus",
		"cosyvoice-v3-flash",
		"qwen3-tts-vc-2026-01-22",
	} {
		require.Contains(t, modelSet, modelName)
	}
}
