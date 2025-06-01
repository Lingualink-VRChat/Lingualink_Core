package audio

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// AudioConverter 音频格式转换器
type AudioConverter struct {
	logger *logrus.Logger
}

// NewAudioConverter 创建音频转换器
func NewAudioConverter(logger *logrus.Logger) *AudioConverter {
	return &AudioConverter{
		logger: logger,
	}
}

// IsFFmpegAvailable 检查ffmpeg是否可用
func (c *AudioConverter) IsFFmpegAvailable() bool {
	cmd := exec.Command("ffmpeg", "-version")
	err := cmd.Run()
	return err == nil
}

// ConvertToWAV 将音频转换为WAV格式
func (c *AudioConverter) ConvertToWAV(inputData []byte, inputFormat string) ([]byte, error) {
	// 如果已经是WAV格式，直接返回
	if strings.ToLower(inputFormat) == "wav" {
		return inputData, nil
	}

	c.logger.WithFields(logrus.Fields{
		"input_format": inputFormat,
		"input_size":   len(inputData),
	}).Info("Converting audio to WAV format")

	// 检查ffmpeg是否可用
	if !c.IsFFmpegAvailable() {
		return nil, fmt.Errorf("ffmpeg not available, cannot convert %s to wav", inputFormat)
	}

	// 使用ffmpeg进行转换
	return c.convertWithFFmpeg(inputData, inputFormat)
}

// convertWithFFmpeg 使用ffmpeg进行转换
func (c *AudioConverter) convertWithFFmpeg(inputData []byte, inputFormat string) ([]byte, error) {
	// 构建ffmpeg命令
	inputFormatArg := c.getFFmpegInputFormat(inputFormat)

	args := []string{
		"-f", inputFormatArg,
		"-i", "pipe:0", // 从stdin读取
		"-f", "wav",
		"-ar", "16000", // 采样率16kHz
		"-ac", "1", // 单声道
		"-acodec", "pcm_s16le", // 16位PCM编码
		"pipe:1", // 输出到stdout
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(inputData)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 设置超时
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			c.logger.WithFields(logrus.Fields{
				"error":  err.Error(),
				"stderr": stderr.String(),
			}).Error("ffmpeg conversion failed")
			return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
		}

		outputData := stdout.Bytes()
		c.logger.WithFields(logrus.Fields{
			"input_size":  len(inputData),
			"output_size": len(outputData),
		}).Info("Audio conversion completed")

		return outputData, nil

	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("audio conversion timeout")
	}
}

// getFFmpegInputFormat 获取ffmpeg输入格式参数
func (c *AudioConverter) getFFmpegInputFormat(format string) string {
	switch strings.ToLower(format) {
	case "opus":
		return "ogg"
	case "mp3":
		return "mp3"
	case "m4a":
		return "m4a"
	case "flac":
		return "flac"
	case "wav":
		return "wav"
	default:
		// 尝试原格式
		return format
	}
}

// GetSupportedFormats 获取支持的格式
func (c *AudioConverter) GetSupportedFormats() []string {
	baseFormats := []string{"wav", "mp3", "m4a", "flac"}

	if c.IsFFmpegAvailable() {
		// 如果ffmpeg可用，支持更多格式
		return append(baseFormats, "opus", "aac", "wma", "ogg")
	}

	// 只支持基本格式
	return baseFormats
}

// IsConversionNeeded 检查是否需要转换
func (c *AudioConverter) IsConversionNeeded(format string) bool {
	return strings.ToLower(format) != "wav"
}

// ValidateAudioData 验证音频数据
func (c *AudioConverter) ValidateAudioData(data []byte, format string) error {
	if len(data) == 0 {
		return fmt.Errorf("empty audio data")
	}

	// 基本的格式检查
	switch strings.ToLower(format) {
	case "wav":
		if len(data) < 44 {
			return fmt.Errorf("invalid WAV file: too short")
		}
		if !bytes.HasPrefix(data, []byte("RIFF")) {
			return fmt.Errorf("invalid WAV file: missing RIFF header")
		}
	case "mp3":
		if len(data) < 10 {
			return fmt.Errorf("invalid MP3 file: too short")
		}
		// MP3文件可能以ID3标签开始或直接以帧开始
		if !bytes.HasPrefix(data, []byte("ID3")) && !bytes.HasPrefix(data[:2], []byte{0xFF, 0xFB}) &&
			!bytes.HasPrefix(data[:2], []byte{0xFF, 0xFA}) && !bytes.HasPrefix(data[:2], []byte{0xFF, 0xF3}) &&
			!bytes.HasPrefix(data[:2], []byte{0xFF, 0xF2}) {
			return fmt.Errorf("invalid MP3 file: missing header")
		}
	case "opus":
		if len(data) < 8 {
			return fmt.Errorf("invalid OPUS file: too short")
		}
		// OPUS通常在OGG容器中
		if !bytes.HasPrefix(data, []byte("OggS")) {
			return fmt.Errorf("invalid OPUS file: missing OGG header")
		}
	case "flac":
		if len(data) < 4 {
			return fmt.Errorf("invalid FLAC file: too short")
		}
		if !bytes.HasPrefix(data, []byte("fLaC")) {
			return fmt.Errorf("invalid FLAC file: missing header")
		}
	}

	return nil
}
