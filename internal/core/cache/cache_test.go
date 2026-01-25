package cache

import (
	"testing"
	"time"
)

func TestGenerateCacheKey_NormalizesTargets(t *testing.T) {
	t.Parallel()

	k1 := GenerateCacheKey("hello", "zh", []string{"EN", "ja"})
	k2 := GenerateCacheKey("hello", "ZH", []string{"ja", "en", "en"})
	if k1 != k2 {
		t.Fatalf("expected keys to match, got %q != %q", k1, k2)
	}
}

func TestInMemoryCache_TTL(t *testing.T) {
	c := NewInMemoryCache(10)
	c.Set("k", &CachedTranslation{
		Translations: map[string]string{"en": "hi"},
		CachedAt:     time.Now(),
	}, 5*time.Millisecond)

	if _, ok := c.Get("k"); !ok {
		t.Fatalf("expected cache hit")
	}

	time.Sleep(10 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatalf("expected cache miss after ttl")
	}
}

func TestInMemoryCache_GetReturnsCopy(t *testing.T) {
	t.Parallel()

	c := NewInMemoryCache(10)
	c.Set("k", &CachedTranslation{
		Translations: map[string]string{"en": "hi"},
		CachedAt:     time.Now(),
	}, time.Minute)

	got1, ok := c.Get("k")
	if !ok {
		t.Fatalf("expected cache hit")
	}
	got1.Translations["en"] = "mutated"

	got2, ok := c.Get("k")
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if got2.Translations["en"] != "hi" {
		t.Fatalf("expected cached value to be immutable, got %q", got2.Translations["en"])
	}
}
