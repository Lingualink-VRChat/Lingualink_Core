package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

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
