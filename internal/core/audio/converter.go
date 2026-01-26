package audio

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AudioConverter converts input audio into WAV for ASR.
type AudioConverter struct {
	logger            *logrus.Logger
	concurrencyLimit  int              // 并发转换限制
	conversionTimeout time.Duration    // 转换超时时间
	semaphore         chan struct{}    // 并发控制信号量
	ffmpegAvailable   bool             // ffmpeg可用性缓存
	ffmpegCheckOnce   sync.Once        // 只检查一次ffmpeg
	conversionStats   *ConversionStats // 转换统计
}

// ConversionStats aggregates conversion statistics.
type ConversionStats struct {
	mu                    sync.RWMutex
	TotalConversions      int64         // 总转换次数
	SuccessfulConversions int64         // 成功转换次数
	FailedConversions     int64         // 失败转换次数
	TotalProcessingTime   time.Duration // 总处理时间
	AverageSize           int64         // 平均文件大小
}

// ConversionMetrics represents per-conversion metrics.
type ConversionMetrics struct {
	InputSize      int64         `json:"input_size"`
	OutputSize     int64         `json:"output_size"`
	ProcessingTime time.Duration `json:"processing_time"`
	Compression    float64       `json:"compression"`
	Success        bool          `json:"success"`
	Format         string        `json:"format"`
}

// NewAudioConverter creates a new AudioConverter.
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

// getConcurrencyLimit returns a conversion concurrency limit.
func getConcurrencyLimit() int {
	// 可以根据CPU核心数、内存等来动态设置
	// 这里简化为固定值，实际部署时可以通过环境变量配置
	return 4
}

// ConvertToWAV converts audio to WAV (16kHz, mono) via ffmpeg when needed.
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

// GetSupportedFormats returns formats the converter can validate/handle.
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

// IsConversionNeeded returns whether input must be converted to WAV.
func (c *AudioConverter) IsConversionNeeded(format string) bool {
	return strings.ToLower(format) != "wav"
}

// UpdateConcurrencyLimit updates the conversion concurrency limit at runtime.
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
