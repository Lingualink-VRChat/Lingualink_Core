package config

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

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
