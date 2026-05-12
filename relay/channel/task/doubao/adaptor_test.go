package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestAudioEnabledInMetadataDefaultsToTrue(t *testing.T) {
	require.True(t, audioEnabledInMetadata(map[string]interface{}{}))
}

func TestAudioEnabledInMetadataSupportsBoolValue(t *testing.T) {
	require.False(t, audioEnabledInMetadata(map[string]interface{}{
		"generate_audio": false,
	}))

	require.False(t, audioEnabledInMetadata(map[string]interface{}{
		"generate_audio": "false",
	}))

	value := dto.BoolValue(true)
	require.True(t, audioEnabledInMetadata(map[string]interface{}{
		"generate_audio": &value,
	}))
}

func TestGetAudioEnabledRatio(t *testing.T) {
	ratio, ok := GetAudioEnabledRatio("doubao-seedance-1-5-pro-251215")
	require.True(t, ok)
	require.InDelta(t, 2.0, ratio, 1e-9)
}
