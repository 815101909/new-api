package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetViduAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name     string
		settings ChannelOtherSettings
		expected string
	}{
		{name: "default token", expected: "Token test-key"},
		{
			name: "bearer",
			settings: ChannelOtherSettings{
				ViduAuthorizationType: ViduAuthorizationTypeBearer,
			},
			expected: "Bearer test-key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.settings.GetViduAuthorizationHeader("test-key"))
		})
	}
}
