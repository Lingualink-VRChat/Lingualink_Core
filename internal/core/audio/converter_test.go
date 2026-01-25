package audio

import (
	"bytes"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestAudioConverter_ConvertToWAV_WAVPassthrough(t *testing.T) {
	logger := testutil.NewTestLogger()
	converter := NewAudioConverter(logger)

	input := testutil.LoadTestAudio(t, "test.wav")
	out, err := converter.ConvertToWAV(input, "wav")
	if err != nil {
		t.Fatalf("ConvertToWAV: %v", err)
	}
	if !bytes.Equal(out, input) {
		t.Fatalf("expected output to match input")
	}
}

func TestAudioConverter_ValidateAudioData(t *testing.T) {
	logger := testutil.NewTestLogger()
	converter := NewAudioConverter(logger)

	if err := converter.ValidateAudioData(nil, "wav"); err == nil {
		t.Fatalf("expected error for empty data")
	}

	if err := converter.ValidateAudioData([]byte("too small"), "wav"); err == nil {
		t.Fatalf("expected error for too small data")
	}

	badWav := make([]byte, 200)
	if err := converter.ValidateAudioData(badWav, "wav"); err == nil {
		t.Fatalf("expected error for invalid wav header")
	}

	goodWav := testutil.LoadTestAudio(t, "test.wav")
	if err := converter.ValidateAudioData(goodWav, "wav"); err != nil {
		t.Fatalf("expected valid wav, got: %v", err)
	}
}

func TestAudioConverter_ConvertToWAV_OpusToWAV(t *testing.T) {
	logger := testutil.NewTestLogger()
	converter := NewAudioConverter(logger)

	if !converter.IsFFmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	input := testutil.LoadTestAudio(t, "test.opus")
	out, err := converter.ConvertToWAV(input, "opus")
	if err != nil {
		t.Fatalf("ConvertToWAV: %v", err)
	}
	if len(out) < 4 || !bytes.HasPrefix(out, []byte("RIFF")) {
		t.Fatalf("expected wav output, got prefix: %q", out[:minInt(len(out), 16)])
	}
}

func TestAudioConverter_ConvertToWAV_FFmpegUnavailable(t *testing.T) {
	t.Setenv("PATH", "")

	logger := testutil.NewTestLogger()
	converter := NewAudioConverter(logger)

	_, err := converter.ConvertToWAV([]byte("not wav"), "mp3")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
