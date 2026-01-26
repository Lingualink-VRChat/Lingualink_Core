package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Auth       AuthConfig       `mapstructure:"auth"`
	ASR        ASRConfig        `mapstructure:"asr"`
	Correction CorrectionConfig `mapstructure:"correction"`
	Pipeline   PipelineConfig   `mapstructure:"pipeline"`
	Backends   BackendsConfig   `mapstructure:"backends"`
	Prompt     PromptConfig     `mapstructure:"prompt"`
	Logging    LoggingConfig    `mapstructure:"logging"`
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

// ASRConfig ASR 后端配置
type ASRConfig struct {
	Providers []ASRProvider `mapstructure:"providers"`
}

// ASRProvider ASR 提供者配置
type ASRProvider struct {
	Name       string                 `mapstructure:"name"`
	Type       string                 `mapstructure:"type"` // whisper / sensevoice / custom
	URL        string                 `mapstructure:"url"`
	Model      string                 `mapstructure:"model"`
	APIKey     string                 `mapstructure:"api_key"`
	Parameters map[string]interface{} `mapstructure:"parameters"`
}

// CorrectionConfig 纠错配置
type CorrectionConfig struct {
	Enabled              bool             `mapstructure:"enabled"`
	MergeWithTranslation bool             `mapstructure:"merge_with_translation"`
	GlobalDictionary     []DictionaryTerm `mapstructure:"global_dictionary"`
}

// DictionaryTerm represents a terminology mapping used during correction.
type DictionaryTerm struct {
	Term    string   `mapstructure:"term"`
	Aliases []string `mapstructure:"aliases"`
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
	Name       string                 `mapstructure:"name"`
	Type       string                 `mapstructure:"type"`
	Config     map[string]interface{} `mapstructure:"config"`
	URL        string                 `mapstructure:"url"`
	Model      string                 `mapstructure:"model"`
	APIKey     string                 `mapstructure:"api_key"`
	Parameters LLMParameters          `mapstructure:"parameters"`
}

// LLMParameters LLM模型参数配置
type LLMParameters struct {
	Temperature       *float64 `mapstructure:"temperature"`
	MaxTokens         *int     `mapstructure:"max_tokens"`
	TopP              *float64 `mapstructure:"top_p"`
	TopK              *int     `mapstructure:"top_k"`
	RepetitionPenalty *float64 `mapstructure:"repetition_penalty"`
	FrequencyPenalty  *float64 `mapstructure:"frequency_penalty"`
	PresencePenalty   *float64 `mapstructure:"presence_penalty"`
	Stop              []string `mapstructure:"stop"`
	Seed              *int     `mapstructure:"seed"`
	Stream            *bool    `mapstructure:"stream"`
}

// PromptConfig 提示词配置
type PromptConfig struct {
	Defaults  PromptDefaults `mapstructure:"defaults"`
	Languages []Language     `mapstructure:"languages"`
	Parsing   ParsingConfig  `mapstructure:"parsing"`
}

// PromptDefaults 提示词默认设置
type PromptDefaults struct {
	Task            string   `mapstructure:"task"`
	TargetLanguages []string `mapstructure:"target_languages"`
}

// Language 语言定义
type Language struct {
	Code      string            `mapstructure:"code"`
	Type      string            `mapstructure:"type"` // standard | fun (default: standard)
	Names     map[string]string `mapstructure:"names"`
	Aliases   []string          `mapstructure:"aliases"`
	StyleNote string            `mapstructure:"style_note"` // Optional (usually for fun languages)
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

func (c *Config) Validate() error {
	var errs []error

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid server port: %d", c.Server.Port))
	}

	if len(c.ASR.Providers) == 0 {
		errs = append(errs, fmt.Errorf("no asr providers configured"))
	}

	for _, provider := range c.ASR.Providers {
		if provider.Name == "" {
			errs = append(errs, fmt.Errorf("asr: missing name"))
		}
		if provider.Type == "" {
			errs = append(errs, fmt.Errorf("asr %s: missing type", provider.Name))
		}
		if provider.URL == "" {
			errs = append(errs, fmt.Errorf("asr %s: missing URL", provider.Name))
			continue
		}
		if _, err := url.ParseRequestURI(provider.URL); err != nil {
			errs = append(errs, fmt.Errorf("asr %s: invalid URL: %v", provider.Name, err))
		}
		if provider.Model == "" {
			errs = append(errs, fmt.Errorf("asr %s: missing model", provider.Name))
		}
	}

	if len(c.Backends.Providers) == 0 {
		errs = append(errs, fmt.Errorf("no backend providers configured"))
	}

	for _, provider := range c.Backends.Providers {
		if provider.Name == "" {
			errs = append(errs, fmt.Errorf("backend: missing name"))
		}
		if provider.Type == "" {
			errs = append(errs, fmt.Errorf("backend %s: missing type", provider.Name))
		}
		if provider.URL == "" {
			errs = append(errs, fmt.Errorf("backend %s: missing URL", provider.Name))
			continue
		}
		if _, err := url.ParseRequestURI(provider.URL); err != nil {
			errs = append(errs, fmt.Errorf("backend %s: invalid URL: %v", provider.Name, err))
		}
	}

	enabledStrategies := 0
	for _, strategy := range c.Auth.Strategies {
		if strategy.Enabled {
			enabledStrategies++
		}
	}
	if enabledStrategies == 0 {
		errs = append(errs, fmt.Errorf("no auth strategies enabled"))
	}

	return errors.Join(errs...)
}

// Load 加载配置
func Load() (*Config, error) {
	// --- Step 1: Setup user config reader ---
	userViper := viper.New()
	userViper.SetEnvPrefix("LINGUALINK")
	userViper.AutomaticEnv()

	configDir := GetConfigDir()

	// Set paths for user config
	if configFile := os.Getenv("LINGUALINK_CONFIG_FILE"); configFile != "" {
		userViper.SetConfigFile(configFile)
		configDir = filepath.Dir(configFile)
		log.Printf("Using config file from environment variable: %s", configFile)

		if err := userViper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read user config: %w", err)
		}
	} else {
		userViper.SetConfigName("config")
		userViper.SetConfigType("yaml")
		userViper.AddConfigPath(configDir)
		userViper.AddConfigPath(".")
		log.Printf("Using default config file search in: %s", configDir)

		// --- Step 2: Read user config ---
		if err := userViper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read user config: %w", err)
			}
			log.Println("User config file not found, using defaults.")
		}
	}

	return loadFromUserViper(userViper)
}

// LoadFromFile loads the configuration from an explicit file path.
func LoadFromFile(path string) (*Config, error) {
	userViper := viper.New()
	userViper.SetEnvPrefix("LINGUALINK")
	userViper.AutomaticEnv()
	userViper.SetConfigFile(path)

	if err := userViper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	return loadFromUserViper(userViper)
}

func loadFromUserViper(userViper *viper.Viper) (*Config, error) {
	// --- Step 3: Build the final Viper instance ---
	finalViper := viper.New()
	finalViper.SetEnvPrefix("LINGUALINK")
	finalViper.AutomaticEnv()
	setDefaults(finalViper) // Set defaults on the final viper instance first

	// Merge user settings on top. This will override defaults.
	if err := finalViper.MergeConfigMap(userViper.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to merge user settings: %w", err)
	}

	// --- Step 4: Unmarshal the final configuration ---
	var cfg Config
	if err := finalViper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	normalizePromptLanguages(&cfg.Prompt)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func normalizePromptLanguages(cfg *PromptConfig) {
	for i := range cfg.Languages {
		cfg.Languages[i].Code = strings.TrimSpace(cfg.Languages[i].Code)
		cfg.Languages[i].Type = strings.ToLower(strings.TrimSpace(cfg.Languages[i].Type))
		if cfg.Languages[i].Type == "" {
			cfg.Languages[i].Type = "standard"
		}
		cfg.Languages[i].StyleNote = strings.TrimSpace(cfg.Languages[i].StyleNote)
	}
}

// setDefaults 设置默认值
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

// ConfigWatcher watches a config file and reloads it on changes.
type ConfigWatcher struct {
	config   *Config
	onChange func(*Config)
	mu       sync.RWMutex
	logger   *logrus.Logger
}

// NewConfigWatcher creates a watcher that reloads configuration when the given file changes.
func NewConfigWatcher(cfg *Config, onChange func(*Config), logger *logrus.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		config:   cfg,
		onChange: onChange,
		logger:   logger,
	}
}

func (w *ConfigWatcher) Get() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

func (w *ConfigWatcher) Watch(path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(path); err != nil {
		_ = watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}

				// slight delay to avoid reading partially-written files
				time.Sleep(50 * time.Millisecond)

				cfg, err := LoadFromFile(path)
				if err != nil {
					if w.logger != nil {
						w.logger.WithError(err).WithField("path", path).Warn("Failed to reload config")
					}
					continue
				}

				w.mu.Lock()
				w.config = cfg
				w.mu.Unlock()

				if w.onChange != nil {
					w.onChange(cfg)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if w.logger != nil {
					w.logger.WithError(err).WithField("path", path).Warn("Config watcher error")
				}
			}
		}
	}()

	return nil
}
