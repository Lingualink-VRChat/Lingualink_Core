package config

import "github.com/spf13/viper"

// setDefaults sets default values on the given viper instance.
func setDefaults(v *viper.Viper) {
	// 服务器默认配置
	v.SetDefault("server.mode", "development")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")

	// 认证默认配置
	v.SetDefault("auth.strategies", []map[string]interface{}{
		{
			"type":    "api_key",
			"enabled": true,
		},
	})

	// 后端默认配置
	v.SetDefault("backends.load_balancer.strategy", "round_robin")
	v.SetDefault("backends.providers", []map[string]interface{}{
		{
			"name":  "default",
			"type":  "openai",
			"url":   "http://localhost:8000/v1",
			"model": "gpt-3.5-turbo",
		},
	})

	// ASR 默认配置
	v.SetDefault("asr.providers", []map[string]interface{}{
		{
			"name":  "default",
			"type":  "whisper",
			"url":   "http://localhost:8000/v1",
			"model": "whisper-1",
			"parameters": map[string]interface{}{
				"response_format": "verbose_json",
				"temperature":     0.0,
			},
		},
	})

	// 纠错默认配置
	v.SetDefault("correction.enabled", true)
	v.SetDefault("correction.merge_with_translation", true)
	v.SetDefault("correction.global_dictionary", []map[string]interface{}{})

	// Pipeline 默认配置
	v.SetDefault("pipeline.tool_calling.enabled", true)
	v.SetDefault("pipeline.tool_calling.allow_thinking", false)

	// 提示词默认配置
	v.SetDefault("prompt.defaults.task", "translate")
	v.SetDefault("prompt.defaults.target_languages", []string{"en", "ja", "zh"})
	// 语言配置需在 config.yaml 的 prompt.languages 中显式配置

	// 日志默认配置
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}
