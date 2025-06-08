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
	Defaults                   PromptDefaults `mapstructure:"defaults"`
	Languages                  []Language     `mapstructure:"languages"`
	Parsing                    ParsingConfig  `mapstructure:"parsing"`
	LanguageManagementStrategy string         `mapstructure:"language_management_strategy"`
}

// PromptDefaults 提示词默认设置
type PromptDefaults struct {
	Task            string   `mapstructure:"task"`
	TargetLanguages []string `mapstructure:"target_languages"`
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
	// --- Step 1: Setup user config reader ---
	userViper := viper.New()
	userViper.SetEnvPrefix("LINGUALINK")
	userViper.AutomaticEnv()

	// Set paths for user config
	if configFile := os.Getenv("LINGUALINK_CONFIG_FILE"); configFile != "" {
		userViper.SetConfigFile(configFile)
		log.Printf("Using config file from environment variable: %s", configFile)
	} else {
		configDir := GetConfigDir()
		userViper.SetConfigName("config")
		userViper.SetConfigType("yaml")
		userViper.AddConfigPath(configDir)
		userViper.AddConfigPath(".")
		log.Printf("Using default config file search in: %s", configDir)
	}

	// --- Step 2: Read user config ---
	if err := userViper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read user config: %w", err)
		}
		log.Println("User config file not found, using defaults.")
	}

	// --- Step 3: Determine the final Viper instance based on strategy ---
	finalViper := viper.New()
	finalViper.SetEnvPrefix("LINGUALINK")
	finalViper.AutomaticEnv()
	setDefaults() // Set defaults on the final viper instance first

	// Get strategy from user config, default to "merge"
	langStrategy := userViper.GetString("prompt.language_management_strategy")
	if langStrategy == "" {
		langStrategy = "merge"
	}
	log.Printf("Language management strategy: %s", langStrategy)

	// If merge, load defaults first
	if langStrategy == "merge" {
		defaultLangFile := filepath.Join(GetConfigDir(), "languages.default.yaml")
		if _, err := os.Stat(defaultLangFile); err == nil {
			defaultLangViper := viper.New()
			defaultLangViper.SetConfigFile(defaultLangFile)
			if err := defaultLangViper.ReadInConfig(); err == nil {
				// Merge default languages into the final viper instance
				if err := finalViper.MergeConfigMap(defaultLangViper.AllSettings()); err != nil {
					log.Printf("Warning: failed to merge default languages map: %v", err)
				} else {
					log.Printf("Default languages loaded for merging from %s", defaultLangFile)
				}
			} else {
				log.Printf("Warning: failed to read default languages config: %v", err)
			}
		} else {
			log.Printf("Default languages file not found: %s, skipping", defaultLangFile)
		}
	}

	// Merge user settings on top. This will override defaults.
	if err := finalViper.MergeConfigMap(userViper.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to merge user settings: %w", err)
	}

	// --- Step 4: Unmarshal the final configuration ---
	var cfg Config
	if err := finalViper.Unmarshal(&cfg); err != nil {
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
	viper.SetDefault("prompt.defaults.task", "translate")
	viper.SetDefault("prompt.defaults.target_languages", []string{"en", "ja", "zh"})
	viper.SetDefault("prompt.language_management_strategy", "merge")

	// 语言配置现在从外部文件加载，不再在这里设置默认值

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
