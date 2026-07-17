package ali

import (
	"io"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestBuildAliAudioSpeechRequestForCosyVoice(t *testing.T) {
	speed := 1.2
	volume := 60
	pitch := 0.9
	sampleRate := 22050
	request := buildAliAudioSpeechRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "cosyvoice-v3.5-plus"},
		},
		dto.AudioRequest{
			Model:          "mapped-model",
			Input:          "测试文本",
			Voice:          "voice-id",
			Instructions:   "温柔地朗读",
			ResponseFormat: "mp3",
			Speed:          &speed,
			SampleRate:     &sampleRate,
			Volume:         &volume,
			Pitch:          &pitch,
			LanguageHints:  []string{"zh"},
		},
	)

	require.Equal(t, "cosyvoice-v3.5-plus", request.Model)
	require.Equal(t, "测试文本", request.Input.Text)
	require.Equal(t, "voice-id", request.Input.Voice)
	require.Equal(t, "mp3", request.Input.Format)
	require.Equal(t, "温柔地朗读", request.Input.Instruction)
	require.Equal(t, []string{"zh"}, request.Input.LanguageHints)
	require.Equal(t, speed, *request.Input.Rate)
	require.Equal(t, volume, *request.Input.Volume)
	require.Equal(t, pitch, *request.Input.Pitch)
	require.Equal(t, sampleRate, *request.Input.SampleRate)
}

func TestBuildAliAudioSpeechRequestForQwenVoiceClone(t *testing.T) {
	request := buildAliAudioSpeechRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "qwen3-tts-vc-2026-01-22"},
		},
		dto.AudioRequest{
			Input:          "hello",
			Voice:          "voice-id",
			LanguageType:   "English",
			ResponseFormat: "wav",
		},
	)

	require.Equal(t, "qwen3-tts-vc-2026-01-22", request.Model)
	require.Equal(t, "English", request.Input.LanguageType)
	require.Empty(t, request.Input.Format)
	require.Nil(t, request.Input.Rate)
}

func TestConvertAliAudioSpeechRequest(t *testing.T) {
	reader, err := convertAliAudioSpeechRequest(
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "cosyvoice-v3-flash"},
		},
		dto.AudioRequest{Input: "hello", Voice: "voice-id"},
	)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(body), `"model":"cosyvoice-v3-flash"`)
	require.Contains(t, string(body), `"text":"hello"`)
}
