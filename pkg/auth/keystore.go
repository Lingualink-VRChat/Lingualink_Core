package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// APIKeyStore API密钥存储
type APIKeyStore struct {
	Keys   map[string]APIKeyConfig `json:"keys"`
	logger *logrus.Logger
}

// APIKeyConfig API密钥配置
type APIKeyConfig struct {
	ID                string                 `json:"id"`
	Description       string                 `json:"description,omitempty"`
	RequestsPerMinute int                    `json:"requests_per_minute"`
	Enabled           bool                   `json:"enabled"`
	CreatedAt         string                 `json:"created_at,omitempty"`
	ExpiresAt         string                 `json:"expires_at,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// NewAPIKeyStore 创建API密钥存储
func NewAPIKeyStore(logger *logrus.Logger) *APIKeyStore {
	return &APIKeyStore{
		Keys:   make(map[string]APIKeyConfig),
		logger: logger,
	}
}

// LoadFromFile 从JSON文件加载密钥
func (store *APIKeyStore) LoadFromFile(filePath string) error {
	// 如果文件不存在，创建默认文件
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		store.logger.Warnf("API key file not found: %s, creating default file", filePath)
		return store.createDefaultKeyFile(filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read key file %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, store); err != nil {
		return fmt.Errorf("failed to parse key file %s: %w", filePath, err)
	}

	store.logger.Infof("Loaded %d API keys from %s", len(store.Keys), filePath)
	
	// 记录加载的密钥（掩码处理）
	for key, config := range store.Keys {
		if config.Enabled {
			maskedKey := key
			if len(maskedKey) > 8 {
				maskedKey = maskedKey[:8] + "***"
			}
			store.logger.Infof("Loaded API key: %s for user: %s, rate limit: %d req/min", 
				maskedKey, config.ID, config.RequestsPerMinute)
		}
	}

	return nil
}

// SaveToFile 保存密钥到JSON文件
func (store *APIKeyStore) SaveToFile(filePath string) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keys: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key file %s: %w", filePath, err)
	}

	store.logger.Infof("Saved %d API keys to %s", len(store.Keys), filePath)
	return nil
}

// GetKey 获取API密钥配置
func (store *APIKeyStore) GetKey(apiKey string) (APIKeyConfig, bool) {
	config, exists := store.Keys[apiKey]
	if !exists || !config.Enabled {
		return APIKeyConfig{}, false
	}

	// 检查是否过期
	if config.ExpiresAt != "" {
		expiryTime, err := time.Parse(time.RFC3339, config.ExpiresAt)
		if err == nil && time.Now().After(expiryTime) {
			store.logger.Warnf("API key expired: %s", apiKey[:8]+"***")
			return APIKeyConfig{}, false
		}
	}

	return config, true
}

// AddKey 添加新的API密钥
func (store *APIKeyStore) AddKey(apiKey string, config APIKeyConfig) {
	if config.CreatedAt == "" {
		config.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if !config.Enabled {
		config.Enabled = true // 默认启用
	}
	store.Keys[apiKey] = config
}

// RemoveKey 移除API密钥
func (store *APIKeyStore) RemoveKey(apiKey string) {
	delete(store.Keys, apiKey)
}

// DisableKey 禁用API密钥
func (store *APIKeyStore) DisableKey(apiKey string) {
	if config, exists := store.Keys[apiKey]; exists {
		config.Enabled = false
		store.Keys[apiKey] = config
	}
}

// ListKeys 列出所有密钥（掩码处理）
func (store *APIKeyStore) ListKeys() []string {
	var keys []string
	for key := range store.Keys {
		masked := key
		if len(masked) > 8 {
			masked = masked[:8] + "***"
		}
		keys = append(keys, masked)
	}
	return keys
}

// createDefaultKeyFile 创建默认密钥文件
func (store *APIKeyStore) createDefaultKeyFile(filePath string) error {
	// 创建默认密钥
	defaultKeys := map[string]APIKeyConfig{
		"dev-key-123": {
			ID:                "dev-user",
			Description:       "Development key",
			RequestsPerMinute: 100,
			Enabled:           true,
			CreatedAt:         time.Now().Format(time.RFC3339),
		},
		"lls-jm1Rg2Bt6HgCrDkzMou5Lu4t": {
			ID:                "enterprise-backend",
			Description:       "Enterprise backend key",
			RequestsPerMinute: -1, // 无限制
			Enabled:           true,
			CreatedAt:         time.Now().Format(time.RFC3339),
		},
	}

	store.Keys = defaultKeys
	
	if err := store.SaveToFile(filePath); err != nil {
		return err
	}

	store.logger.Infof("Created default API key file: %s", filePath)
	return nil
}

// GetKeyFilePath 获取密钥文件路径
func GetKeyFilePath() string {
	// 支持环境变量指定路径
	if path := os.Getenv("LINGUALINK_KEYS_FILE"); path != "" {
		return path
	}

	// 默认路径
	if configDir := os.Getenv("LINGUALINK_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "api_keys.json")
	}

	// 最后的默认路径
	return filepath.Join("config", "api_keys.json")
} 