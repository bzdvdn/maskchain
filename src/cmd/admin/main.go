package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	shvalue "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task 100-admin-control-plane#T2.2: Admin binary entrypoint with UI, profile/incident handlers (AC-002, AC-004, AC-006, AC-007, AC-010)
// @sk-task tenant-profile-sync#T2.1: Wire TenantResolver and new Auth middleware (AC-002, AC-005)
func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(bootstrap.HealthCheck(config.MustLoadConfig().Server))
	}

	cfg := config.MustLoadConfig()

	logger, err := bootstrap.BuildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	serviceName := "maskchain-admin"
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		serviceName = cfg.OTel.ServiceName + "-admin"
	}

	otelShutdown := bootstrap.NoopShutdown
	if cfg.OTel != nil {
		shutdown, err := telemetry.InitProvider(
			context.Background(),
			cfg.OTel.Endpoint,
			serviceName,
			cfg.OTel.Environment,
			cfg.OTel.SamplingRatio,
			logger,
		)
		if err != nil {
			logger.Warn("telemetry init", zap.Error(err))
		}
		otelShutdown = shutdown
	}

	promRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(promRegistry)
	metricsHandler := metrics.Handler(promRegistry)

	pgPool, err := bootstrap.InitPG(context.Background(), cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to init postgres", zap.Error(err))
	}

	if pgPool != nil {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
		metrics.RegisterPGPoolCollector(promRegistry, pgPool)
	}

	vkClient, err := bootstrap.InitValkey(cfg.Valkey, logger)
	if err != nil {
		logger.Warn("valkey init failed", zap.Error(err))
	}

	// @sk-task 114-real-health-probes#T2.3: Create health service and wire into admin server (AC-001, AC-005, AC-008)
	if cfg.Server.HealthCheck == nil {
		cfg.Server.HealthCheck = &config.HealthCheckConfig{CriticalDeps: []string{"database"}}
	}
	healthSvc := health.NewService(cfg.Server.HealthCheck.CriticalDeps)

	// @sk-task 114-real-health-probes#T3.2: Register concrete probes (AC-002, AC-003, AC-004)
	healthSvc.Register(health.NewPGProbe(pgPool))
	healthSvc.Register(health.NewValkeyProbe(vkClient))
	if cfg.Routing != nil {
		var targets []string
		for _, p := range cfg.Routing.Providers {
			if p.BaseURL != "" {
				targets = append(targets, bootstrap.ExtractHostPort(p.BaseURL))
			}
		}
		healthSvc.Register(health.NewEgressProbe(targets))
	}

	srv := api.NewAdminServer(cfg.Server, logger, serviceName, healthSvc)

	if cfg.Tenants != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		tenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)

		cfgTenants := make(map[string]*entity.Tenant, len(cfg.Tenants))
		for slugStr, tc := range cfg.Tenants {
			slug, err := shvalue.NewTenantSlug(slugStr)
			if err != nil {
				logger.Fatal("invalid tenant slug", zap.String("tenant", slugStr), zap.Error(err))
			}
			opts := []entity.TenantOption{entity.WithTenantDictionaries(nil)}
			if tc.PIIConfig != nil {
				opts = append(opts, entity.WithTenantPIIConfig(*tc.PIIConfig))
			}
			cfgTenants[slugStr] = entity.NewTenant(slug, tc.Name, tc.AuthHeader, tc.APIKeys, opts...)
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

		// @sk-task remove-audit-incidents#T3.4: Incident handler wiring removed from admin (AC-009)

		// @sk-task sessions#T2.3: Wire SessionHandler in admin (AC-001)
		sessionStore := sessionrepo.NewPostgresSessionStore(pgPool)
		sessionUseCase := session.NewSessionUseCase(sessionStore)
		sessionHandler := api.NewSessionHandler(sessionUseCase, cfg.Session)
		srv.RegisterSessionHandler(sessionHandler)

		// @sk-task 132-analytics-api#T2.3: Wire AnalyticsHandler in admin (AC-001, AC-002, AC-003, AC-004)
		pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
		analyticsHandler := analyticshandler.NewAnalyticsHandler(pgUsageStore)
		srv.RegisterAnalyticsHandler(analyticsHandler, cfg.Debug)
		logger.Info("analytics handler registered")

		// @sk-task sessions#T5.2: Wire CleanupWorker in admin (AC-007)
		if cfg.Session.CleanupEnabled {
			cleanupWorker := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
			cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
			go cleanupWorker.Run(cleanupCtx)
			logger.Info("session cleanup worker registered",
				zap.Duration("interval", cfg.Session.CleanupInterval),
			)
			// @sk-task sessions#T5.2: Cancel cleanup context on shutdown (AC-007)
			defer cleanupCancel()
		} else {
			logger.Debug("session cleanup worker disabled")
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var sig os.Signal
	select {
	case sig = <-quit:
		logger.Info("admin shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("admin server error", zap.Error(err))
		os.Exit(1)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := otelShutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}

	if pgPool != nil {
		pgPool.Close()
	}
	if vkClient != nil {
		vkClient.Close()
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("admin server stopped")
}


