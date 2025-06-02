package metrics

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	RecordLatency(name string, duration time.Duration, tags map[string]string)
	RecordCounter(name string, value int64, tags map[string]string)
	RecordGauge(name string, value float64, tags map[string]string)
	GetMetrics() map[string]interface{}
}

// SimpleMetricsCollector 简单的指标收集器实现
type SimpleMetricsCollector struct {
	counters map[string]int64
	gauges   map[string]float64
	latency  map[string][]time.Duration
	mu       sync.RWMutex
	logger   *logrus.Logger
}

// NewSimpleMetricsCollector 创建简单指标收集器
func NewSimpleMetricsCollector(logger *logrus.Logger) *SimpleMetricsCollector {
	return &SimpleMetricsCollector{
		counters: make(map[string]int64),
		gauges:   make(map[string]float64),
		latency:  make(map[string][]time.Duration),
		logger:   logger,
	}
}

// RecordLatency 记录延迟
func (c *SimpleMetricsCollector) RecordLatency(name string, duration time.Duration, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.buildKey(name, tags)
	c.latency[key] = append(c.latency[key], duration)

	// 保持最近100个记录
	if len(c.latency[key]) > 100 {
		c.latency[key] = c.latency[key][1:]
	}
}

// RecordCounter 记录计数器
func (c *SimpleMetricsCollector) RecordCounter(name string, value int64, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.buildKey(name, tags)
	c.counters[key] += value
}

// RecordGauge 记录仪表
func (c *SimpleMetricsCollector) RecordGauge(name string, value float64, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.buildKey(name, tags)
	c.gauges[key] = value
}

// GetMetrics 获取所有指标
func (c *SimpleMetricsCollector) GetMetrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})

	// 计数器
	counters := make(map[string]int64)
	for k, v := range c.counters {
		counters[k] = v
	}
	result["counters"] = counters

	// 仪表
	gauges := make(map[string]float64)
	for k, v := range c.gauges {
		gauges[k] = v
	}
	result["gauges"] = gauges

	// 延迟统计
	latencyStats := make(map[string]map[string]float64)
	for k, durations := range c.latency {
		if len(durations) == 0 {
			continue
		}

		stats := calculateLatencyStats(durations)
		latencyStats[k] = stats
	}
	result["latency"] = latencyStats

	return result
}

// buildKey 构建指标键
func (c *SimpleMetricsCollector) buildKey(name string, tags map[string]string) string {
	key := name
	for k, v := range tags {
		key += ":" + k + "=" + v
	}
	return key
}

// calculateLatencyStats 计算延迟统计
func calculateLatencyStats(durations []time.Duration) map[string]float64 {
	if len(durations) == 0 {
		return make(map[string]float64)
	}

	// 转换为毫秒
	values := make([]float64, len(durations))
	total := 0.0
	for i, d := range durations {
		ms := float64(d.Nanoseconds()) / 1000000.0
		values[i] = ms
		total += ms
	}

	// 计算统计值
	avg := total / float64(len(values))

	// 简单的百分位数计算（排序后取位置）
	// 这里为了简单起见，不进行完整排序
	min := values[0]
	max := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return map[string]float64{
		"count": float64(len(values)),
		"avg":   avg,
		"min":   min,
		"max":   max,
	}
}
