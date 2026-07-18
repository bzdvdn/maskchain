package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// @sk-task config-hot-reload#T1.1: WatchConfigDir with fsnotify + debounce (AC-004)
// ConfigDirFromArgs extracts --config-dir from os.Args or CONFIG_DIR env.
func ConfigDirFromArgs() string {
	if dir := os.Getenv("CONFIG_DIR"); dir != "" {
		return dir
	}
	args := os.Args[1:]
	for i, a := range args {
		if a == "--config-dir" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, "--config-dir=") {
			return strings.TrimPrefix(a, "--config-dir=")
		}
	}
	return ""
}

// @sk-task config-hot-reload#T1.1: WatchConfigDir with fsnotify + debounce (AC-004)
func WatchConfigDir(ctx context.Context, dir string, onReload func(old, new *Config)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("config: failed to create fsnotify watcher", "error", err)
		return
	}
	if err := watcher.Add(dir); err != nil {
		slog.Error("config: failed to watch config dir", "dir", dir, "error", err)
		watcher.Close()
		return
	}

	current, err := LoadConfigFromDir(dir)
	if err != nil {
		slog.Warn("config: initial LoadConfigFromDir failed", "dir", dir, "error", err)
	}
	var storeMu sync.Mutex
	var reloadMu sync.Mutex

	go func() {
		defer watcher.Close()
		var debounce *time.Timer

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if ext := filepath.Ext(event.Name); ext != ".yaml" && ext != ".yml" {
					continue
				}

				// @sk-task config-hot-reload#T2.2: Debounce 100ms, error handling — invalid config skips reload with log (AC-003, AC-004)
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(100*time.Millisecond, func() {
					reloadMu.Lock()
					defer reloadMu.Unlock()

					newCfg, loadErr := LoadConfigFromDir(dir)
					if loadErr != nil {
						slog.Error("config: reload error", "error", loadErr)
						return
					}
					storeMu.Lock()
					old := current
					current = newCfg
					storeMu.Unlock()
					onReload(old, newCfg)
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("config: fsnotify error", "error", err)

			case <-ctx.Done():
				if debounce != nil {
					debounce.Stop()
				}
				return
			}
		}
	}()
}
