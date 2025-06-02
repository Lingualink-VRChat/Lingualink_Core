package audio

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AudioConverter 音频格式转换器
type AudioConverter struct {
	logger            *logrus.Logger
	concurrencyLimit  int              // 并发转换限制
	conversionTimeout time.Duration    // 转换超时时间
	semaphore         chan struct{}    // 并发控制信号量
	ffmpegAvailable   bool             // ffmpeg可用性缓存
	ffmpegCheckOnce   sync.Once        // 只检查一次ffmpeg
	conversionStats   *ConversionStats // 转换统计
}

// ConversionStats 转换统计信息
type ConversionStats struct {
	mu                    sync.RWMutex
	TotalConversions      int64         // 总转换次数
	SuccessfulConversions int64         // 成功转换次数
	FailedConversions     int64         // 失败转换次数
	TotalProcessingTime   time.Duration // 总处理时间
	AverageSize           int64         // 平均文件大小
}

// ConversionMetrics 转换指标
type ConversionMetrics struct {
	InputSize      int64         `json:"input_size"`
	OutputSize     int64         `json:"output_size"`
	ProcessingTime time.Duration `json:"processing_time"`
	Compression    float64       `json:"compression"`
	Success        bool          `json:"success"`
	Format         string        `json:"format"`
}

// NewAudioConverter 创建音频转换器
func NewAudioConverter(logger *logrus.Logger) *AudioConverter {
	// 根据系统资源设置合理的并发限制
	concurrencyLimit := 4 // 默认4个并发转换
	if limit := getConcurrencyLimit(); limit > 0 {
		concurrencyLimit = limit
	}

	return &AudioConverter{
		logger:            logger,
		concurrencyLimit:  concurrencyLimit,
		conversionTimeout: 60 * time.Second, // 增加到60秒
		semaphore:         make(chan struct{}, concurrencyLimit),
		conversionStats:   &ConversionStats{},
	}
}

// getConcurrencyLimit 根据系统资源获取并发限制
func getConcurrencyLimit() int {
	// 可以根据CPU核心数、内存等来动态设置
	// 这里简化为固定值，实际部署时可以通过环境变量配置
	return 4
}

// IsFFmpegAvailable 检查ffmpeg是否可用（带缓存）
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

// ConvertToWAV 将音频转换为WAV格式（优化版本）
func (c *AudioConverter) ConvertToWAV(inputData []byte, inputFormat string) ([]byte, error) {
	startTime := time.Now()

	// 如果已经是WAV格式，直接返回
	if strings.ToLower(inputFormat) == "wav" {
		c.recordConversion(len(inputData), len(inputData), time.Since(startTime), true, inputFormat)
		return inputData, nil
	}

	c.logger.WithFields(logrus.Fields{
		"input_format": inputFormat,
		"input_size":   len(inputData),
		"queue_length": len(c.semaphore),
	}).Info("Starting audio conversion")

	// 检查ffmpeg是否可用
	if !c.IsFFmpegAvailable() {
		c.recordConversion(len(inputData), 0, time.Since(startTime), false, inputFormat)
		return nil, fmt.Errorf("ffmpeg not available, cannot convert %s to wav", inputFormat)
	}

	// 并发控制：获取信号量
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-time.After(30 * time.Second):
		c.recordConversion(len(inputData), 0, time.Since(startTime), false, inputFormat)
		return nil, fmt.Errorf("conversion queue timeout, too many concurrent conversions")
	}

	// 执行转换
	result, err := c.convertWithFFmpegOptimized(inputData, inputFormat)

	// 记录统计信息
	success := err == nil
	outputSize := 0
	if result != nil {
		outputSize = len(result)
	}
	c.recordConversion(len(inputData), outputSize, time.Since(startTime), success, inputFormat)

	return result, err
}

// convertWithFFmpegOptimized 使用ffmpeg进行优化转换
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

// getFFmpegInputFormat 获取ffmpeg输入格式参数（扩展支持）
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

// GetSupportedFormats 获取支持的格式（扩展列表）
func (c *AudioConverter) GetSupportedFormats() []string {
	baseFormats := []string{"wav", "mp3", "m4a", "flac"}

	if c.IsFFmpegAvailable() {
		// ffmpeg可用时支持更多格式
		return []string{
			"wav", "mp3", "m4a", "flac", "opus",
			"aac", "wma", "ogg", "amr", "3gp",
		}
	}

	return baseFormats
}

// IsConversionNeeded 检查是否需要转换
func (c *AudioConverter) IsConversionNeeded(format string) bool {
	return strings.ToLower(format) != "wav"
}

// ValidateAudioData 验证音频数据（增强版本）
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

// validateWAV WAV格式验证
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

// validateMP3 MP3格式验证
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

// validateOpus OPUS格式验证
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

// validateFLAC FLAC格式验证
func (c *AudioConverter) validateFLAC(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("invalid FLAC file: too short")
	}
	if !bytes.HasPrefix(data, []byte("fLaC")) {
		return fmt.Errorf("invalid FLAC file: missing fLaC header")
	}
	return nil
}

// validateM4A M4A/AAC格式验证
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

// recordConversion 记录转换统计信息
func (c *AudioConverter) recordConversion(inputSize, outputSize int, duration time.Duration, success bool, format string) {
	c.conversionStats.mu.Lock()
	defer c.conversionStats.mu.Unlock()

	c.conversionStats.TotalConversions++
	c.conversionStats.TotalProcessingTime += duration

	if success {
		c.conversionStats.SuccessfulConversions++
	} else {
		c.conversionStats.FailedConversions++
	}

	// 计算平均大小
	if c.conversionStats.TotalConversions > 0 {
		c.conversionStats.AverageSize = (c.conversionStats.AverageSize*(c.conversionStats.TotalConversions-1) + int64(inputSize)) / c.conversionStats.TotalConversions
	}

	// 记录详细日志
	c.logger.WithFields(logrus.Fields{
		"format":            format,
		"input_size":        inputSize,
		"output_size":       outputSize,
		"duration":          duration,
		"success":           success,
		"queue_length":      len(c.semaphore),
		"total_conversions": c.conversionStats.TotalConversions,
	}).Debug("Conversion completed")
}

// GetStats 获取转换统计信息
func (c *AudioConverter) GetStats() ConversionStats {
	c.conversionStats.mu.RLock()
	defer c.conversionStats.mu.RUnlock()

	return ConversionStats{
		TotalConversions:      c.conversionStats.TotalConversions,
		SuccessfulConversions: c.conversionStats.SuccessfulConversions,
		FailedConversions:     c.conversionStats.FailedConversions,
		TotalProcessingTime:   c.conversionStats.TotalProcessingTime,
		AverageSize:           c.conversionStats.AverageSize,
	}
}

// GetMetrics 获取性能指标
func (c *AudioConverter) GetMetrics() map[string]interface{} {
	stats := c.GetStats()

	var avgProcessingTime float64
	var successRate float64

	if stats.TotalConversions > 0 {
		avgProcessingTime = float64(stats.TotalProcessingTime.Milliseconds()) / float64(stats.TotalConversions)
		successRate = float64(stats.SuccessfulConversions) / float64(stats.TotalConversions) * 100
	}

	return map[string]interface{}{
		"total_conversions":      stats.TotalConversions,
		"successful_conversions": stats.SuccessfulConversions,
		"failed_conversions":     stats.FailedConversions,
		"success_rate":           successRate,
		"avg_processing_time_ms": avgProcessingTime,
		"avg_file_size_bytes":    stats.AverageSize,
		"concurrency_limit":      c.concurrencyLimit,
		"current_queue_length":   len(c.semaphore),
		"ffmpeg_available":       c.ffmpegAvailable,
		"conversion_timeout_sec": c.conversionTimeout.Seconds(),
	}
}

// UpdateConcurrencyLimit 更新并发限制（运行时调整）
func (c *AudioConverter) UpdateConcurrencyLimit(newLimit int) {
	if newLimit <= 0 || newLimit > 20 { // 合理范围
		c.logger.Warnf("Invalid concurrency limit: %d, ignoring", newLimit)
		return
	}

	c.logger.Infof("Updating concurrency limit from %d to %d", c.concurrencyLimit, newLimit)

	// 创建新的信号量
	oldSemaphore := c.semaphore
	c.semaphore = make(chan struct{}, newLimit)
	c.concurrencyLimit = newLimit

	// 等待旧的转换完成
	go func() {
		for i := 0; i < cap(oldSemaphore); i++ {
			oldSemaphore <- struct{}{}
		}
	}()
}
