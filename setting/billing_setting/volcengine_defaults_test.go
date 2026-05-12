package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVolcEngineDefaultBillingExprsSmoke(t *testing.T) {
	expr, ok := GetDefaultBillingExprMap()["doubao-seed-1-6-thinking-250715"]
	require.True(t, ok)
	require.NoError(t, SmokeTestExpr(expr))
}
