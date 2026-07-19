package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/logging"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
)

type Bootstrap struct {
	Cfg          *config.Config
	Logger       *slog.Logger
	PGPool       *pgxpool.Pool
	ValkeyClient valkey.Client
	PromRegistry *prometheus.Registry
	OTelShutdown func(context.Context) error
	HealthSvc    *health.HealthService
}

func HealthCheck(cfg *config.ServerConfig) int {
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func NoopShutdown(_ context.Context) error { return nil }

func BuildLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return logging.NewLogger(os.Stdout, l)
}

func ExtractHostPort(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

func InitBootstrap(ctx context.Context, cfg *config.Config, log *slog.Logger, serviceName string) (*Bootstrap, error) {
	var pgPool *pgxpool.Pool
	if cfg.DB != nil && cfg.DB.DSN != "" {
		var err error
		pgPool, err = postgres.NewPool(ctx, cfg.DB)
		if err != nil {
			return nil, fmt.Errorf("init PG pool: %w", err)
		}
	}

	var vkClient valkey.Client
	if cfg.Valkey != nil && cfg.Valkey.Addr != "" {
		var err error
		vkClient, err = valkey.NewClient(valkey.ClientOption{
			InitAddress: []string{cfg.Valkey.Addr},
			Password:    cfg.Valkey.Password,
		})
		if err != nil {
			log.Warn("Valkey init failed, continuing without cache", slog.String("error", err.Error()))
		}
	}

	promRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(promRegistry)

	otelShutdown := NoopShutdown
	if cfg.OTel != nil {
		shutdown, err := telemetry.InitProvider(ctx, cfg.OTel.Endpoint, serviceName, cfg.OTel.Environment, cfg.OTel.SamplingRatio, log)
		if err != nil {
			log.Warn("OTel init failed, continuing without telemetry", slog.String("error", err.Error()))
		} else {
			otelShutdown = shutdown
		}
	}

	if cfg.Server.HealthCheck == nil {
		cfg.Server.HealthCheck = &config.HealthCheckConfig{CriticalDeps: []string{"database"}}
	}
	healthSvc := health.NewService(cfg.Server.HealthCheck.CriticalDeps)
	healthSvc.Register(health.NewPGProbe(pgPool))
	healthSvc.Register(health.NewValkeyProbe(vkClient))
	if cfg.Routing != nil {
		var targets []string
		for _, p := range cfg.Routing.Providers {
			if p.BaseURL != "" {
				targets = append(targets, ExtractHostPort(p.BaseURL))
			}
		}
		if len(targets) > 0 {
			healthSvc.Register(health.NewEgressProbe(targets))
		}
	}

	if pgPool != nil {
		metrics.RegisterPGPoolCollector(promRegistry, pgPool)
	}

	return &Bootstrap{
		Cfg:          cfg,
		Logger:       log,
		PGPool:       pgPool,
		ValkeyClient: vkClient,
		PromRegistry: promRegistry,
		OTelShutdown: otelShutdown,
		HealthSvc:    healthSvc,
	}, nil
}

// InitPG — deprecated, use InitBootstrap.
func InitPG(ctx context.Context, dbCfg *config.DatabaseConfig, log *slog.Logger) (*pgxpool.Pool, error) {
	return postgres.NewPool(ctx, dbCfg)
}

// InitValkey — deprecated, use InitBootstrap.
func InitValkey(vkCfg *config.ValkeyConfig, log *slog.Logger) (valkey.Client, error) {
	if vkCfg == nil || vkCfg.Addr == "" {
		return nil, nil
	}
	return valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkCfg.Addr},
		Password:    vkCfg.Password,
	})
}

func (b *Bootstrap) Close() {
	if b.PGPool != nil {
		b.PGPool.Close()
	}
	if b.ValkeyClient != nil {
		b.ValkeyClient.Close()
	}
}
