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
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	shvalue "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/logging"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task 100-admin-control-plane#T2.2: Admin binary entrypoint with UI, profile/incident handlers (AC-002, AC-004, AC-006, AC-007, AC-010)
// @sk-task tenant-profile-sync#T2.1: Wire TenantResolver and new Auth middleware (AC-002, AC-005)
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

	_ = initValkey(cfg.Valkey, logger)

	srv := api.NewAdminServer(cfg.Server, logger, serviceName)

	if cfg.Tenants != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		tenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)

		cfgTenants := make(map[string]*entity.Tenant, len(cfg.Tenants))
		for slugStr, tc := range cfg.Tenants {
			slug, err := shvalue.NewTenantSlug(slugStr)
			if err != nil {
				logger.Fatal("invalid tenant slug", zap.String("tenant", slugStr), zap.Error(err))
			}
			cfgTenants[slugStr] = entity.NewTenant(slug, tc.Name, tc.AuthHeader, tc.APIKeys)
		}
		tenantResolver := resolver.NewDBFirstTenantResolver(tenantRepo, cfgTenants)

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := tenantResolver.SyncConfig(syncCtx, cfgTenants); err != nil {
			syncCancel()
			logger.Fatal("failed to sync tenants from config", zap.Error(err))
		}
		syncCancel()

		loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Second)
		dbTenants, err := tenantResolver.List(loadCtx)
		loadCancel()
		if err != nil {
			logger.Fatal("failed to load tenants from db", zap.Error(err))
		}

		authMw := middleware.Auth(dbTenants)
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterStaticFiles(ui.DistFiles)

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	if pgPool != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)

		// @sk-task tenant-profile-sync#T2.2: Wire TenantHandler in admin (AC-001, AC-005, AC-008)
		pgTenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)
		tenantHandler := admin.NewTenantHandler(pgTenantRepo)
		srv.RegisterTenantHandler(tenantHandler)

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
