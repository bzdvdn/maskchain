package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/profile"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	profilerepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/profile"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/tenant"
	tenantrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/tenant"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/logging"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task 100-admin-control-plane#T2.2: Admin binary entrypoint with UI, profile/incident handlers (AC-002, AC-004, AC-006, AC-007, AC-010)
func main() {
	cfg := config.MustLoadConfig()

	logger, err := buildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	slogLogger := logging.NewLogger(io.Discard, slog.LevelInfo)

	serviceName := "maskchain-admin"
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		serviceName = cfg.OTel.ServiceName + "-admin"
	}

	otelShutdown := noopShutdown
	if cfg.OTel != nil {
		shutdown, err := telemetry.InitProvider(
			context.Background(),
			cfg.OTel.Endpoint,
			serviceName,
			cfg.OTel.Environment,
			cfg.OTel.SamplingRatio,
			slogLogger,
		)
		if err != nil {
			slogLogger.Warn("telemetry init", "error", err)
		}
		otelShutdown = shutdown
	}

	promRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(promRegistry)
	metricsHandler := metrics.Handler(promRegistry)

	pgPool, err := initPG(context.Background(), cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to init postgres", zap.Error(err))
	}

	if pgPool != nil {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
		metrics.RegisterPGPoolCollector(promRegistry, pgPool)
	}

	vkClient := initValkey(cfg.Valkey, logger)

	srv := api.NewAdminServer(cfg.Server, logger, serviceName)

	if cfg.Tenants != nil {
		tenants := make([]*tenant.Tenant, 0, len(cfg.Tenants))
		for slug, tc := range cfg.Tenants {
			apiKeys := make([]tenant.APIKey, 0, len(tc.APIKeys))
			for _, k := range tc.APIKeys {
				ak, err := tenant.NewAPIKey(k)
				if err != nil {
					logger.Fatal("invalid api key", zap.String("tenant", slug), zap.Error(err))
				}
				apiKeys = append(apiKeys, ak)
			}
			tenants = append(tenants, tenant.NewTenant(slug, tc.Name, tc.ProfileSlug, apiKeys, tc.AuthHeader, tc.AuthScheme))
		}
		tenantRepo, err := tenantrepo.NewInMemoryRepository(tenants)
		if err != nil {
			logger.Fatal("failed to build tenant repository", zap.Error(err))
		}
		authMw := middleware.Auth(tenantRepo)
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(tenants)))
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterStaticFiles(ui.DistFiles)

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	if pgPool != nil {
		dictRepo := postgres.NewPostgresDictionaryRepo(pgPool)
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		pgProfileRepo := postgres.NewPostgresProfileRepo(pgPool, dictRepo, txMgr)

		// @sk-task 102-profile-cache#T2.5: Wire ProfileCache in admin (AC-002, AC-009)
		profileValkeyTTL := time.Duration(cfg.ProfileCache.ValkeyTTLSec) * time.Second
		pvkRepo := profilerepo.NewProfileValkeyRepo(vkClient, profileValkeyTTL)
		pLru := profilerepo.NewProfileLRUCache(cfg.ProfileCache.LRUSize)
		dictLoader := profilerepo.NewDictLoader(func(ctx context.Context, slug string) (*dictionary.Dictionary, error) {
			return dictRepo.FindByProfileSlug(ctx, slug)
		})
		versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
			var v int
			err := pgPool.QueryRow(ctx, "SELECT version FROM profiles WHERE slug = $1 AND tenant_id = $2", slug, tenantID).Scan(&v)
			return v, err
		}
		cacheMetrics := profilerepo.NewPromCacheMetrics(
			metrics.ProfileCacheHitsTotal,
			metrics.ProfileCacheMissesTotal,
			metrics.ProfileCacheStaleTotal,
			metrics.ProfileCacheInvalidationsTotal,
		)
		profileRepo := profilerepo.NewProfileCache(pgProfileRepo, pvkRepo, pLru, dictLoader, slogLogger, versionFunc, cacheMetrics, nil)
		profileHandler := profile.New(profileRepo)
		srv.RegisterProfileHandler(profileHandler)

		incidentRepo := postgres.NewPostgresIncidentRepo(pgPool)
		incidentHandler := incident.New(incidentRepo)
		srv.RegisterIncidentHandler(incidentHandler)
	}

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("admin server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("admin shutting down", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := otelShutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("admin server stopped")
}

func noopShutdown(_ context.Context) error { return nil }

func initPG(ctx context.Context, dbCfg *config.DatabaseConfig, log *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("init pool: %w", err)
	}
	if pool == nil {
		log.Warn("no database configured, persistence disabled")
	}
	return pool, nil
}

func initValkey(vkCfg *config.ValkeyConfig, log *zap.Logger) valkey.Client {
	if vkCfg == nil || vkCfg.Addr == "" {
		log.Warn("no valkey configured, admin cache disabled")
		return nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkCfg.Addr},
		Password:    vkCfg.Password,
	})
	if err != nil {
		log.Warn("valkey client init failed", zap.Error(err))
		return nil
	}
	return client
}

func buildLogger(level string) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return zapCfg.Build()
}
