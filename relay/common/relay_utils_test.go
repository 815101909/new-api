package common

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateBasicTaskRequestNormalizesInputReference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		multipart bool
	}{
		{
			name: "json",
		},
		{
			name:      "multipart text field",
			multipart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"prompt":"move the character","model":"viduq3-turbo","input_reference":"https://example.com/frame.png"}`)
			contentType := "application/json"
			if tt.multipart {
				buffer := &bytes.Buffer{}
				writer := multipart.NewWriter(buffer)
				require.NoError(t, writer.WriteField("prompt", "move the character"))
				require.NoError(t, writer.WriteField("model", "viduq3-turbo"))
				require.NoError(t, writer.WriteField("input_reference", "https://example.com/frame.png"))
				require.NoError(t, writer.Close())
				body = buffer.Bytes()
				contentType = writer.FormDataContentType()
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", contentType)
			bodyStorage, err := rootcommon.GetBodyStorage(c)
			require.NoError(t, err)
			c.Request.Body = io.NopCloser(bodyStorage)
			info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

			taskErr := ValidateBasicTaskRequest(c, info, "textGenerate")
			require.Nil(t, taskErr)
			req, err := GetTaskRequest(c)
			require.NoError(t, err)
			require.Equal(t, "https://example.com/frame.png", req.InputReference)
			require.Equal(t, []string{"https://example.com/frame.png"}, req.Images)
			require.True(t, req.HasImage())
		})
	}
}

func TestValidateBasicTaskRequestAcceptsMultipartInputReferenceFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	require.NoError(t, writer.WriteField("prompt", "move the character"))
	require.NoError(t, writer.WriteField("model", "viduq3-turbo"))
	require.NoError(t, writer.WriteField("seconds", "5"))
	require.NoError(t, writer.WriteField("metadata", `{"watermark":false}`))
	fileWriter, err := writer.CreateFormFile("input_reference", "frame.png")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake png data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(buffer.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	bodyStorage, err := rootcommon.GetBodyStorage(c)
	require.NoError(t, err)
	c.Request.Body = io.NopCloser(bodyStorage)
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateBasicTaskRequest(c, info, "textGenerate")
	require.Nil(t, taskErr)
	req, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "move the character", req.Prompt)
	require.Equal(t, "viduq3-turbo", req.Model)
	require.Equal(t, "5", req.Seconds)
	require.Equal(t, 5, req.Duration)
	require.Equal(t, false, req.Metadata["watermark"])
	require.Empty(t, req.InputReference)
	require.Empty(t, req.Images)
	require.False(t, req.HasImage())
	multipartForm, err := c.MultipartForm()
	require.NoError(t, err)
	require.Len(t, multipartForm.File["input_reference"], 1)
}

func TestValidateBasicTaskRequestKeepsImagesPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"prompt":"move","images":["https://example.com/images.png"],"input_reference":"https://example.com/input.png"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	taskErr := ValidateBasicTaskRequest(c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}, "textGenerate")
	require.Nil(t, taskErr)
	req, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/images.png"}, req.Images)
}
