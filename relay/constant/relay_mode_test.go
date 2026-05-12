package constant

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath2RelayModeDetectsVideoRoutes(t *testing.T) {
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/v1/videos"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/v1/videos/task_123"))
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/v1/video/generations"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/v1/video/generations/task_123"))
}
