package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

// CachedTranslation represents a cached translation result.
type CachedTranslation struct {
	Translations map[string]string
	CachedAt     time.Time
}

// TranslationCache stores translation results keyed by a stable request key.
type TranslationCache interface {
	Get(key string) (*CachedTranslation, bool)
	Set(key string, value *CachedTranslation, ttl time.Duration)
}

type cacheEntry struct {
	value     *CachedTranslation
	expiresAt time.Time
}

// InMemoryCache is a simple in-memory implementation of TranslationCache with TTL support.
type InMemoryCache struct {
	mu      sync.RWMutex
	store   map[string]cacheEntry
	maxSize int
}

// NewInMemoryCache creates an InMemoryCache with an optional size limit.
func NewInMemoryCache(maxSize int) *InMemoryCache {
	if maxSize < 0 {
		maxSize = 0
	}
	return &InMemoryCache{
		store:   make(map[string]cacheEntry),
		maxSize: maxSize,
	}
}

func (c *InMemoryCache) Get(key string) (*CachedTranslation, bool) {
	c.mu.RLock()
	entry, ok := c.store[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.store, key)
		c.mu.Unlock()
		return nil, false
	}

	return cloneCachedTranslation(entry.value), true
}

func (c *InMemoryCache) Set(key string, value *CachedTranslation, ttl time.Duration) {
	if value == nil {
		return
	}

	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSize > 0 && len(c.store) >= c.maxSize {
		c.evictLocked()
	}

	c.store[key] = cacheEntry{
		value:     cloneCachedTranslation(value),
		expiresAt: expiresAt,
	}
}

func (c *InMemoryCache) evictLocked() {
	now := time.Now()
	for k, v := range c.store {
		if !v.expiresAt.IsZero() && now.After(v.expiresAt) {
			delete(c.store, k)
		}
	}
	if c.maxSize <= 0 || len(c.store) < c.maxSize {
		return
	}

	for k := range c.store {
		delete(c.store, k)
		break
	}
}

func cloneCachedTranslation(in *CachedTranslation) *CachedTranslation {
	if in == nil {
		return nil
	}

	out := &CachedTranslation{
		CachedAt: in.CachedAt,
	}
	if in.Translations != nil {
		out.Translations = make(map[string]string, len(in.Translations))
		for k, v := range in.Translations {
			out.Translations[k] = v
		}
	}
	return out
}

// GenerateCacheKey creates a stable cache key for a translation request.
// It lowercases and sorts target languages so equivalent sets map to the same key.
func GenerateCacheKey(text string, sourceLanguage string, targetLangs []string) string {
	normalizedTargets := make([]string, 0, len(targetLangs))
	seen := make(map[string]struct{}, len(targetLangs))
	for _, lang := range targetLangs {
		lang = strings.ToLower(strings.TrimSpace(lang))
		if lang == "" {
			continue
		}
		if _, ok := seen[lang]; ok {
			continue
		}
		seen[lang] = struct{}{}
		normalizedTargets = append(normalizedTargets, lang)
	}
	sort.Strings(normalizedTargets)

	payload := strings.Join([]string{
		strings.ToLower(strings.TrimSpace(sourceLanguage)),
		strings.Join(normalizedTargets, ","),
		text,
	}, "|")

	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
