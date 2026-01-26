package audio

import (
	"time"

	"github.com/sirupsen/logrus"
)

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

// GetStats returns a snapshot of conversion stats.
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

// GetMetrics returns converter performance metrics.
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
