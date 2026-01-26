package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Load loads configuration from the default locations.
func Load() (*Config, error) {
	// --- Step 1: Setup user config reader ---
	userViper := viper.New()
	userViper.SetEnvPrefix("LINGUALINK")
	userViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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
	userViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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
	finalViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

// InitLogger initializes a logger based on config settings.
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

// GetConfigDir returns the default configuration directory.
func GetConfigDir() string {
	if dir := os.Getenv("LINGUALINK_CONFIG_DIR"); dir != "" {
		return dir
	}

	// 默认配置目录
	wd, _ := os.Getwd()
	return filepath.Join(wd, "config")
}
