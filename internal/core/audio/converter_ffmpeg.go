package audio

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// IsFFmpegAvailable checks ffmpeg availability (cached).
func (c *AudioConverter) IsFFmpegAvailable() bool {
	c.ffmpegCheckOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "ffmpeg", "-version")
		err := cmd.Run()
		c.ffmpegAvailable = (err == nil)

		if c.ffmpegAvailable {
			c.logger.Info("FFmpeg is available for audio conversion")
		} else {
			c.logger.Warn("FFmpeg is not available, audio conversion capabilities limited")
		}
	})
	return c.ffmpegAvailable
}

// convertWithFFmpegOptimized converts audio to WAV using ffmpeg.
func (c *AudioConverter) convertWithFFmpegOptimized(inputData []byte, inputFormat string) ([]byte, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), c.conversionTimeout)
	defer cancel()

	// 构建优化的ffmpeg命令
	inputFormatArg := c.getFFmpegInputFormat(inputFormat)
	args := []string{
		"-hide_banner",       // 减少输出
		"-loglevel", "error", // 只显示错误
		"-f", inputFormatArg,
		"-i", "pipe:0", // 从stdin读取
		"-f", "wav",
		"-ar", "16000", // 采样率16kHz（适合语音识别）
		"-ac", "1", // 单声道（减少数据量）
		"-acodec", "pcm_s16le", // 16位PCM编码
		"-compression_level", "6", // 适中的压缩级别
		"pipe:1", // 输出到stdout
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// 使用更高效的输入方式
	cmd.Stdin = bytes.NewReader(inputData)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()

	// 检查上下文是否被取消（超时）
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("audio conversion timeout after %v", c.conversionTimeout)
	}

	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"error":        err.Error(),
			"stderr":       stderr.String(),
			"input_format": inputFormat,
			"input_size":   len(inputData),
		}).Error("ffmpeg conversion failed")
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	outputData := stdout.Bytes()

	// 验证输出数据
	if len(outputData) == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty output")
	}

	// 基本的WAV格式验证
	if len(outputData) < 44 || !bytes.HasPrefix(outputData, []byte("RIFF")) {
		return nil, fmt.Errorf("ffmpeg produced invalid WAV output")
	}

	c.logger.WithFields(logrus.Fields{
		"input_format": inputFormat,
		"input_size":   len(inputData),
		"output_size":  len(outputData),
		"compression":  fmt.Sprintf("%.2f%%", (1.0-float64(len(outputData))/float64(len(inputData)))*100),
	}).Info("Audio conversion completed successfully")

	return outputData, nil
}

// getFFmpegInputFormat maps input format to ffmpeg demuxer/format.
func (c *AudioConverter) getFFmpegInputFormat(format string) string {
	switch strings.ToLower(format) {
	case "opus":
		return "ogg"
	case "mp3":
		return "mp3"
	case "m4a", "aac":
		return "m4a"
	case "flac":
		return "flac"
	case "wav":
		return "wav"
	case "ogg":
		return "ogg"
	case "wma":
		return "asf"
	case "amr":
		return "amr"
	case "3gp":
		return "3gp"
	default:
		// 让ffmpeg自动检测
		return format
	}
}
