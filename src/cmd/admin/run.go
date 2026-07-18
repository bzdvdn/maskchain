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
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
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
func run() {
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

	// @sk-task config-hot-reload#T3.1: Start config watcher for admin hot-reload (AC-001, AC-002)
	if cfgDir := config.ConfigDirFromArgs(); cfgDir != "" {
		reloadCtx, reloadCancel := context.WithCancel(context.Background())
		defer reloadCancel()
		config.WatchConfigDir(reloadCtx, cfgDir, func(old, new *config.Config) {
			changed := config.DiffSections(old, new)
			if changed["tenants"] {
				logger.Info("config reloaded: tenants changed (requires manual auth refresh)")
			}
			if changed["debug"] {
				logger.Info("config reloaded: debug changed")
			}
		})
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

		tenantProvider := middleware.NewTenantProvider(dbTenants)
		authMw := middleware.Auth(tenantProvider)
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))

		// @sk-task config-hot-reload#T3.3: Capture tenantProvider for hot-reload (AC-002)
		_ = tenantProvider
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterStaticFiles(ui.DistFiles)

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	if pgPool != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)

		// @sk-task admin-ui-design#T2.3: Wire admin auth handler and middleware (AC-001, AC-004)
		adminSessionStore := postgres.NewPostgresAdminSessionStore(pgPool)
		adminSessionUC := admin_session.NewAdminSessionUseCase(adminSessionStore)
		adminAuthHandler := admin.NewAdminAuthHandler(adminSessionUC, cfg.Admin)
		// IMPORTANT: adminSessionMw must be registered before routes that use it
		srv.RegisterAdminSessionMiddleware(middleware.AdminSessionAuth(adminSessionUC))
		srv.RegisterAdminAuthRoutes(adminAuthHandler)
		logger.Info("admin auth registered", zap.String("username", cfg.Admin.Username))

		// @sk-task admin-ui-design#T2.3: Wire audit log store (AC-005)
		auditLogStore := postgres.NewAuditLogStore(pgPool, 100)
		defer auditLogStore.Shutdown()
		auditAdapter := &auditLogAdapter{store: auditLogStore}

		// @sk-task admin-ui-design#T3.2: Wire TenantHandler with audit logger (AC-005)
		// @sk-task seed-tenant-fix#T1.1: Use combined middleware for seed script compat (AC-001)
		pgTenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)
		dictCache := dictionaryrepo.NewValkeyDictionaryCache(vkClient, 5*time.Minute)
		tenantHandler := admin.NewTenantHandler(pgTenantRepo, dictCache, auditAdapter)
		tenantMw := middleware.AdminSessionOrTokenAuth(adminSessionUC, cfg.Debug, func(ctx context.Context, apiKey string) bool {
			tenants, err := pgTenantRepo.List(ctx)
			if err != nil {
				logger.Warn("tenant list for API key check", zap.Error(err))
				return false
			}
			for _, t := range tenants {
				for _, k := range t.APIKeys() {
					if k == apiKey {
						return true
					}
				}
			}
			return false
		})
		srv.RegisterTenantHandler(tenantHandler, tenantMw)

		// @sk-task admin-ui-design#T3.2: Wire AuditHandler (AC-005)
		auditHandler := admin.NewAuditHandler(auditAdapter)
		srv.RegisterAuditHandler(auditHandler)

		// @sk-task admin-ui-design#T4.3: Wire RoutingHandler with health checker (AC-006)
		healthChecker := admin.NewProviderHealthChecker(5 * time.Second)
		if cfg.Routing != nil {
			var targets []admin.ProviderTarget
			for _, p := range cfg.Routing.Providers {
				targets = append(targets, admin.ProviderTarget{
					Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
				})
			}
			if len(targets) > 0 {
				healthChecker.StartBackgroundRefresh(context.Background(), 30*time.Second, targets)
			}
		}
		routingHandler := admin.NewRoutingHandler(cfg.Routing, healthChecker)
		srv.RegisterRoutingHandler(routingHandler)

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

// @sk-task admin-ui-design#T3.2: Adapter from postgres.AuditLogEntry to admin.AuditEvent (AC-005)
type auditLogAdapter struct {
	store *postgres.AuditLogStore
}

func (a *auditLogAdapter) Write(ctx context.Context, event *admin.AuditEvent) error {
	return a.store.Write(ctx, &postgres.AuditLogEntry{
		AdminUsername: event.AdminUsername,
		Action:        event.Action,
		Target:        event.Target,
		Details:       event.Details,
		CreatedAt:     event.CreatedAt,
	})
}

func (a *auditLogAdapter) List(ctx context.Context, limit, offset int) ([]admin.AuditEvent, error) {
	entries, err := a.store.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	events := make([]admin.AuditEvent, len(entries))
	for i, e := range entries {
		events[i] = admin.AuditEvent{
			AdminUsername: e.AdminUsername,
			Action:        e.Action,
			Target:        e.Target,
			Details:       e.Details,
			CreatedAt:     e.CreatedAt,
		}
	}
	return events, nil
}
