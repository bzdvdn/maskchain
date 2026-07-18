package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	shvalue "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/pkg/version"
	"github.com/bzdvdn/maskchain/ui"
)

func run() {
	cfg, logger := initConfigLog()
	defer logger.Sync()

	b, err := bootstrap.InitBootstrap(context.Background(), cfg, logger, adminServiceName(cfg))
	if err != nil {
		logger.Fatal("bootstrap failed", zap.Error(err))
	}
	defer b.Close()

	if cfg.DB != nil && cfg.DB.DSN != "" {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	watchAdminConfigReload(cfg, logger)
	srv := api.NewAdminServer(cfg.Server, logger, adminServiceName(cfg), b.HealthSvc)
	srv.RegisterMetricsRoute(metrics.Handler(b.PromRegistry))
	srv.RegisterVersionRoute(version.Info())
	srv.RegisterStaticFiles(ui.DistFiles)
	srv.RegisterDebugRoutes(middleware.AdminAuth(cfg.Debug))

	initAdminTenants(cfg, b.PGPool, srv, logger)

	if b.PGPool != nil {
		txMgr := postgres.NewPGXTransactionManager(b.PGPool)

		adminSessionStore := postgres.NewPostgresAdminSessionStore(b.PGPool)
		adminSessionUC := admin_session.NewAdminSessionUseCase(adminSessionStore)
		adminAuthHandler := admin.NewAdminAuthHandler(adminSessionUC, cfg.Admin)
		srv.RegisterAdminSessionMiddleware(middleware.AdminSessionAuth(adminSessionUC))
		srv.RegisterAdminAuthRoutes(adminAuthHandler)

		auditLogStore := postgres.NewAuditLogStore(b.PGPool, 100)
		defer auditLogStore.Shutdown()
		auditAdapter := &auditLogAdapter{store: auditLogStore}

		dictCache := dictionaryrepo.NewValkeyDictionaryCache(b.ValkeyClient, 5*time.Minute)
		pgTenantRepo := postgres.NewPostgresTenantRepo(b.PGPool, txMgr)
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

		auditHandler := admin.NewAuditHandler(auditAdapter)
		srv.RegisterAuditHandler(auditHandler)

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

		sessionStore := sessionrepo.NewPostgresSessionStore(b.PGPool)
		sessionUseCase := session.NewSessionUseCase(sessionStore)
		srv.RegisterSessionHandler(api.NewSessionHandler(sessionUseCase, cfg.Session))

		pgUsageStore := analyticsrepo.NewPgUsageStore(b.PGPool)
		analyticsHandler := analyticshandler.NewAnalyticsHandler(pgUsageStore)
		srv.RegisterAnalyticsHandler(analyticsHandler, cfg.Debug)

		if cfg.Session.CleanupEnabled {
			cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
			_ = cleanupCancel
			w := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
			go w.Run(cleanupCtx)
		}
	}

	adminServe(cfg, logger, b, srv)
}

func initConfigLog() (*config.Config, *zap.Logger) {
	cfg := config.MustLoadConfig()
	logger, err := bootstrap.BuildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	logger.Debug("config loaded", zap.Object("config", cfg))
	return cfg, logger
}

func adminServiceName(cfg *config.Config) string {
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		return cfg.OTel.ServiceName + "-admin"
	}
	return "maskchain-admin"
}

func watchAdminConfigReload(cfg *config.Config, logger *zap.Logger) {
	cfgDir := config.ConfigDirFromArgs()
	if cfgDir == "" {
		return
	}
	reloadCtx, reloadCancel := context.WithCancel(context.Background())
	_ = reloadCancel
	config.WatchConfigDir(reloadCtx, cfgDir, func(old, new *config.Config) {
		changed := config.DiffSections(old, new)
		if changed["tenants"] {
			logger.Info("config reloaded: tenants changed")
		}
		if changed["debug"] {
			logger.Info("config reloaded: debug changed")
		}
	})
}

func initAdminTenants(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.AdminServer, logger *zap.Logger) {
	if cfg.Tenants == nil {
		logger.Warn("no tenants configured, auth disabled")
		return
	}

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

	srv.RegisterAuth(middleware.Auth(middleware.NewTenantProvider(dbTenants)))
	logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))
}

func adminServe(cfg *config.Config, logger *zap.Logger, b *bootstrap.Bootstrap, srv *api.AdminServer) {
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
		logger.Info("shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("server error", zap.Error(err))
		return
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := b.OTelShutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
}

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
