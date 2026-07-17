package common

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func buildTestWAV(t *testing.T, pcmSizeHeader uint32, pcmData []byte) *bytes.Reader {
	t.Helper()

	const (
		sampleRate    = uint32(24000)
		channels      = uint16(1)
		bitsPerSample = uint16(16)
	)
	blockAlign := channels * bitsPerSample / 8
	byteRate := sampleRate * uint32(blockAlign)

	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("RIFF")
	if err := binary.Write(buffer, binary.LittleEndian, uint32(36)+pcmSizeHeader); err != nil {
		t.Fatal(err)
	}
	buffer.WriteString("WAVE")
	buffer.WriteString("fmt ")
	for _, value := range []any{
		uint32(16), uint16(1), channels, sampleRate, byteRate, blockAlign, bitsPerSample,
	} {
		if err := binary.Write(buffer, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	buffer.WriteString("data")
	if err := binary.Write(buffer, binary.LittleEndian, pcmSizeHeader); err != nil {
		t.Fatal(err)
	}
	buffer.Write(pcmData)
	return bytes.NewReader(buffer.Bytes())
}

func TestGetWAVDurationUsesActualSizeForStreamingPlaceholder(t *testing.T) {
	const actualPCMSize = 1850880
	reader := buildTestWAV(t, 0x7fffff9b, make([]byte, actualPCMSize))

	duration, err := getWAVDuration(reader)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(duration-38.56) > 0.000001 {
		t.Fatalf("duration = %f, want 38.56 seconds", duration)
	}
}

func TestGetWAVDurationUsesDeclaredPCMSizeWhenValid(t *testing.T) {
	const declaredPCMSize = 24000 * 2
	pcmData := make([]byte, declaredPCMSize+1024)
	reader := buildTestWAV(t, declaredPCMSize, pcmData)

	duration, err := getWAVDuration(reader)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(duration-1) > 0.000001 {
		t.Fatalf("duration = %f, want 1 second", duration)
	}
}
