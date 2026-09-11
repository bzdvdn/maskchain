package admin

import (
	"context"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

type KeyAtRestStatus struct {
	Configured bool   `json:"configured"`
	Cipher     string `json:"cipher,omitempty"`
}

type ConfigDiffStatus struct {
	Watched  bool     `json:"watched"`
	Sections []string `json:"sections,omitempty"`
}

type SystemStatus struct {
	Version    string                   `json:"version"`
	UptimeSec  int64                    `json:"uptime_seconds"`
	KeyAtRest  KeyAtRestStatus          `json:"key_at_rest"`
	Health     *health.AggregatedResult `json:"health"`
	ConfigDiff ConfigDiffStatus         `json:"config_diff"`
}

// StatusHandler serves the read-only admin status endpoint powering the
// Settings page: version, uptime, at-rest encryption state, aggregated store
// health and a live-vs-file config diff.
type StatusHandler struct {
	version   string
	cfg       *config.Config
	healthSvc *health.HealthService
	startedAt time.Time
	watchDir  string
}

func NewStatusHandler(version string, cfg *config.Config, healthSvc *health.HealthService) *StatusHandler {
	return &StatusHandler{
		version:   version,
		cfg:       cfg,
		healthSvc: healthSvc,
		startedAt: time.Now(),
		watchDir:  config.ConfigDirFromArgs(),
	}
}

func (h *StatusHandler) keyAtRest() KeyAtRestStatus {
	configured := os.Getenv(config.KeysKeyEnvVar) != ""
	if h.cfg != nil && h.cfg.Crypto != nil && h.cfg.Crypto.KeysKey != "" {
		configured = true
	}
	if !configured {
		return KeyAtRestStatus{Configured: false}
	}
	return KeyAtRestStatus{Configured: true, Cipher: "AES-256-GCM"}
}

func (h *StatusHandler) configDiff(ctx context.Context) ConfigDiffStatus {
	if h.watchDir == "" {
		return ConfigDiffStatus{Watched: false}
	}
	fresh, err := config.LoadConfigFromDir(h.watchDir)
	if err != nil {
		return ConfigDiffStatus{Watched: true}
	}
	changed := config.DiffSections(h.cfg, fresh)
	sections := make([]string, 0, len(changed))
	for s := range changed {
		if changed[s] {
			sections = append(sections, s)
		}
	}
	sort.Strings(sections)
	return ConfigDiffStatus{Watched: true, Sections: sections}
}

func (h *StatusHandler) HandleStatus(c *gin.Context) {
	healthRes := &health.AggregatedResult{Status: "unknown", Checks: map[string]health.CheckState{}}
	if h.healthSvc != nil {
		healthRes = h.healthSvc.CheckAll(c.Request.Context())
	}
	resp := &SystemStatus{
		Version:    h.version,
		UptimeSec:  int64(time.Since(h.startedAt).Seconds()),
		KeyAtRest:  h.keyAtRest(),
		Health:     healthRes,
		ConfigDiff: h.configDiff(c.Request.Context()),
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}
