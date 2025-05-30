package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Backends BackendsConfig `mapstructure:"backends"`
	Prompt   PromptConfig   `mapstructure:"prompt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Mode string `mapstructure:"mode"`
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Strategies []AuthStrategy `mapstructure:"strategies"`
}

// AuthStrategy 认证策略
type AuthStrategy struct {
	Type     string                 `mapstructure:"type"`
	Enabled  bool                   `mapstructure:"enabled"`
	Config   map[string]interface{} `mapstructure:"config"`
	Endpoint string                 `mapstructure:"endpoint"`
}

// BackendsConfig LLM后端配置
type BackendsConfig struct {
	LoadBalancer LoadBalancerConfig `mapstructure:"load_balancer"`
	Providers    []BackendProvider  `mapstructure:"providers"`
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy string `mapstructure:"strategy"`
}

// BackendProvider 后端提供者配置
type BackendProvider struct {
	Name   string                 `mapstructure:"name"`
	Type   string                 `mapstructure:"type"`
	Config map[string]interface{} `mapstructure:"config"`
	URL    string                 `mapstructure:"url"`
	Model  string                 `mapstructure:"model"`
	APIKey string                 `mapstructure:"api_key"`
}

// PromptConfig 提示词配置
type PromptConfig struct {
	Defaults    PromptDefaults `mapstructure:"defaults"`
	TemplateDir string         `mapstructure:"template_dir"`
	Languages   []Language     `mapstructure:"languages"`
	Parsing     ParsingConfig  `mapstructure:"parsing"`
}

// PromptDefaults 提示词默认设置
type PromptDefaults struct {
	Task            string   `mapstructure:"task"`
	TargetLanguages []string `mapstructure:"target_languages"`
	Template        string   `mapstructure:"template"`
}

// Language 语言定义
type Language struct {
	Code    string            `mapstructure:"code"`
	Names   map[string]string `mapstructure:"names"`
	Aliases []string          `mapstructure:"aliases"`
}

// ParsingConfig 解析配置
type ParsingConfig struct {
	Separators []string         `mapstructure:"separators"`
	StrictMode bool             `mapstructure:"strict_mode"`
	Validation ValidationConfig `mapstructure:"validation"`
}

// ValidationConfig 验证配置
type ValidationConfig struct {
	RequiredSections []string `mapstructure:"required_sections"`
	MinContentLength int      `mapstructure:"min_content_length"`
	MaxContentLength int      `mapstructure:"max_content_length"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load 加载配置
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 配置文件搜索路径
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// 环境变量设置
	viper.AutomaticEnv()
	viper.SetEnvPrefix("LINGUALINK")

	// 默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 如果配置文件不存在，使用默认配置
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		log.Println("Config file not found, using defaults and environment variables")
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	// 服务器默认配置
	viper.SetDefault("server.mode", "development")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")

	// 认证默认配置
	viper.SetDefault("auth.strategies", []map[string]interface{}{
		{
			"type":    "api_key",
			"enabled": true,
		},
	})

	// 后端默认配置
	viper.SetDefault("backends.load_balancer.strategy", "round_robin")
	viper.SetDefault("backends.providers", []map[string]interface{}{
		{
			"name":  "default",
			"type":  "openai",
			"url":   "http://localhost:8000/v1",
			"model": "gpt-3.5-turbo",
		},
	})

	// 提示词默认配置
	viper.SetDefault("prompt.defaults.task", "both")
	viper.SetDefault("prompt.defaults.target_languages", []string{"英文", "日文", "中文"})
	viper.SetDefault("prompt.defaults.template", "default")
	viper.SetDefault("prompt.template_dir", "./templates")

	// 日志默认配置
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
}

// InitLogger 初始化日志
func InitLogger(cfg *Config) *logrus.Logger {
	logger := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	return logger
}

// GetConfigDir 获取配置目录
func GetConfigDir() string {
	if dir := os.Getenv("LINGUALINK_CONFIG_DIR"); dir != "" {
		return dir
	}

	// 默认配置目录
	wd, _ := os.Getwd()
	return filepath.Join(wd, "config")
}
