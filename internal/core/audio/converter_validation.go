package audio

import (
	"bytes"
	"fmt"
	"strings"
)

// ValidateAudioData validates input audio data size and performs basic format checks.
func (c *AudioConverter) ValidateAudioData(data []byte, format string) error {
	if len(data) == 0 {
		return fmt.Errorf("empty audio data")
	}

	// 检查文件大小限制（32MB）
	maxSize := 32 * 1024 * 1024
	if len(data) > maxSize {
		return fmt.Errorf("audio file too large: %d bytes (max: %d bytes)", len(data), maxSize)
	}

	// 最小文件大小检查
	minSize := 100 // 至少100字节
	if len(data) < minSize {
		return fmt.Errorf("audio file too small: %d bytes (min: %d bytes)", len(data), minSize)
	}

	// 格式特定的验证
	switch strings.ToLower(format) {
	case "wav":
		return c.validateWAV(data)
	case "mp3":
		return c.validateMP3(data)
	case "opus":
		return c.validateOpus(data)
	case "flac":
		return c.validateFLAC(data)
	case "m4a", "aac":
		return c.validateM4A(data)
	default:
		// 对于其他格式，只做基本检查
		c.logger.Debugf("Basic validation for format: %s", format)
		return nil
	}
}

// validateWAV validates WAV headers.
func (c *AudioConverter) validateWAV(data []byte) error {
	if len(data) < 44 {
		return fmt.Errorf("invalid WAV file: too short (need at least 44 bytes)")
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) {
		return fmt.Errorf("invalid WAV file: missing RIFF header")
	}
	if !bytes.Contains(data[8:16], []byte("WAVE")) {
		return fmt.Errorf("invalid WAV file: missing WAVE format")
	}
	return nil
}

// validateMP3 validates basic MP3 header patterns.
func (c *AudioConverter) validateMP3(data []byte) error {
	if len(data) < 10 {
		return fmt.Errorf("invalid MP3 file: too short")
	}

	// MP3可能以ID3标签开始或直接以帧同步开始
	if bytes.HasPrefix(data, []byte("ID3")) {
		return nil // ID3标签
	}

	// 检查MP3帧同步模式
	syncPatterns := [][]byte{
		{0xFF, 0xFB}, {0xFF, 0xFA}, {0xFF, 0xF3}, {0xFF, 0xF2},
		{0xFF, 0xE3}, {0xFF, 0xE2},
	}

	for _, pattern := range syncPatterns {
		if bytes.HasPrefix(data, pattern) {
			return nil
		}
	}

	return fmt.Errorf("invalid MP3 file: missing valid header or sync pattern")
}

// validateOpus validates the OGG container header for OPUS.
func (c *AudioConverter) validateOpus(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid OPUS file: too short")
	}
	// OPUS通常在OGG容器中
	if !bytes.HasPrefix(data, []byte("OggS")) {
		return fmt.Errorf("invalid OPUS file: missing OGG container header")
	}
	return nil
}

// validateFLAC validates the fLaC header.
func (c *AudioConverter) validateFLAC(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("invalid FLAC file: too short")
	}
	if !bytes.HasPrefix(data, []byte("fLaC")) {
		return fmt.Errorf("invalid FLAC file: missing fLaC header")
	}
	return nil
}

// validateM4A validates a basic MP4 container signature.
func (c *AudioConverter) validateM4A(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid M4A file: too short")
	}

	// M4A文件通常以ftyp box开始
	if bytes.Contains(data[:20], []byte("ftyp")) {
		return nil
	}

	// 有些M4A文件可能以其他box开始
	validBoxes := [][]byte{
		[]byte("ftyp"), []byte("mdat"), []byte("moov"), []byte("free"),
	}

	for _, box := range validBoxes {
		if bytes.Contains(data[:20], box) {
			return nil
		}
	}

	return fmt.Errorf("invalid M4A file: missing valid box header")
}
