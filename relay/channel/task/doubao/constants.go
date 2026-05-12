package doubao

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
}

var ChannelName = "doubao-video"

// videoInputRatioMap 视频输入折扣比率（含视频单价 / 不含视频单价）。
// 管理员应将 ModelRatio 设置为"不含视频"的较高费率，
// 系统在检测到视频输入时自动乘以此折扣。
var videoInputRatioMap = map[string]float64{
	"doubao-seedance-2-0-260128":      28.0 / 46.0, // ~0.6087
	"doubao-seedance-2-0-fast-260128": 22.0 / 37.0, // ~0.5946
}

// audioEnabledRatioMap 音频启用时的倍率（有声价 / 无声价）。
// 以无声视频单价为基础倍率，在检测到启用音频后乘以对应倍率。
var audioEnabledRatioMap = map[string]float64{
	"doubao-seedance-1-5-pro-251215": 16.0 / 8.0,
}

func GetVideoInputRatio(modelName string) (float64, bool) {
	r, ok := videoInputRatioMap[modelName]
	return r, ok
}

func GetAudioEnabledRatio(modelName string) (float64, bool) {
	r, ok := audioEnabledRatioMap[modelName]
	return r, ok
}
