package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type syncUpstreamTestResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Conflicts []struct {
			ModelName string `json:"model_name"`
			Fields    []struct {
				Field string `json:"field"`
			} `json:"fields"`
		} `json:"conflicts"`
		CreatedModels int `json:"created_models"`
		UpdatedModels int `json:"updated_models"`
	} `json:"data"`
}

func performModelSyncTestRequest(method string, target string, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body == "" {
		c.Request = httptest.NewRequest(method, target, nil)
	} else {
		c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
	}
	handler(c)
	return w
}

func TestSyncUpstreamModelsCreatesDomesticDefaultsAsSyncOfficial(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	w := performModelSyncTestRequest(http.MethodPost, "/api/models/sync_upstream", `{"source":"domestic","locale":"zh"}`, SyncUpstreamModels)
	require.Equal(t, http.StatusOK, w.Code)

	var response syncUpstreamTestResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Positive(t, response.Data.CreatedModels)

	var viduQ2 model.Model
	require.NoError(t, db.Where("model_name = ?", "viduq2").First(&viduQ2).Error)
	require.Equal(t, 1, viduQ2.SyncOfficial)
}

func TestDomesticPreviewIncludesSyncDisabledModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	insertSyncDisabledViduModel(t, db, "viduq2")

	w := performModelSyncTestRequest(http.MethodGet, "/api/models/sync_upstream/preview?source=domestic&locale=zh", "", SyncUpstreamPreview)
	require.Equal(t, http.StatusOK, w.Code)

	var response syncUpstreamTestResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.True(t, hasConflictField(response, "viduq2", "endpoints"))
}

func TestDomesticOverwriteUpdatesSyncDisabledModel(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	insertSyncDisabledViduModel(t, db, "viduq2")

	body := `{"source":"domestic","locale":"zh","overwrite":[{"model_name":"viduq2","fields":["endpoints"]}]}`
	w := performModelSyncTestRequest(http.MethodPost, "/api/models/sync_upstream", body, SyncUpstreamModels)
	require.Equal(t, http.StatusOK, w.Code)

	var response syncUpstreamTestResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 1, response.Data.UpdatedModels)

	var viduQ2 model.Model
	require.NoError(t, db.Where("model_name = ?", "viduq2").First(&viduQ2).Error)
	require.Contains(t, viduQ2.Endpoints, string(constant.EndpointTypeImageGeneration))
	require.Equal(t, 0, viduQ2.SyncOfficial)
}

func insertSyncDisabledViduModel(t *testing.T, db *gorm.DB, modelName string) {
	t.Helper()

	vendor := model.Vendor{Name: "Vidu", Status: 1}
	require.NoError(t, vendor.Insert())

	oldEndpoints := `{"openai-video":{"method":"POST","path":"/v1/video/generations"}}`
	item := model.Model{
		ModelName:    modelName,
		Description:  "旧 Vidu 本地模型",
		Endpoints:    oldEndpoints,
		Icon:         "Vidu.Color",
		Tags:         "国产,视频",
		VendorID:     vendor.Id,
		Status:       1,
		SyncOfficial: 0,
	}
	require.NoError(t, item.Insert())
}

func hasConflictField(response syncUpstreamTestResponse, modelName string, field string) bool {
	for _, conflict := range response.Data.Conflicts {
		if conflict.ModelName != modelName {
			continue
		}
		for _, item := range conflict.Fields {
			if item.Field == field {
				return true
			}
		}
	}
	return false
}
