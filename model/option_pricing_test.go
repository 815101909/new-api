package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func ensureOptionMapForTest() {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
}

func TestUpdateOptionMapModelPriceExposesMergedDefaults(t *testing.T) {
	ensureOptionMapForTest()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	t.Cleanup(func() {
		_ = ratio_setting.UpdateModelPriceByJSONString(`{}`)
		common.OptionMapRWMutex.Lock()
		delete(common.OptionMap, "ModelPrice")
		common.OptionMapRWMutex.Unlock()
	})

	err := updateOptionMap("ModelPrice", `{"custom-image-model":1.23}`)
	require.NoError(t, err)

	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap["ModelPrice"]
	common.OptionMapRWMutex.RUnlock()

	var prices map[string]float64
	require.NoError(t, common.UnmarshalJsonStr(raw, &prices))
	require.InDelta(t, 1.23, prices["custom-image-model"], 1e-9)
	require.InDelta(t, 3.125, prices["viduq3-pro"], 1e-9)
	require.InDelta(t, 0.5, prices["viduq2-pro-fast"], 1e-9)
}
