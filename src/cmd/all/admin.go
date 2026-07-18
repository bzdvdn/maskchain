package main

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	adminhandler "github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task combined-binary: Build and wire admin server
func buildAdminServer(
	cfg *config.Config,
	logger *zap.Logger,
	serviceName string,
	pgPool *pgxpool.Pool,
	vkClient valkey.Client,
	promRegistry *prometheus.Registry,
	metricsHandler gin.HandlerFunc,
	otelShutdown func(context.Context) error,
) *api.AdminServer {
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
				targets = append(targets, bootstrap.ExtractHostPort(p.BaseURL))
			}
		}
		healthSvc.Register(health.NewEgressProbe(targets))
	}

	adminCfg := *cfg.Server
	adminCfg.Port = cfg.Server.AdminPort
	srv := api.NewAdminServer(&adminCfg, logger, serviceName+"-admin", healthSvc)

	if cfg.Tenants != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		tenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)

		cfgTenants := make(map[string]*entity.Tenant, len(cfg.Tenants))
		for slugStr, tc := range cfg.Tenants {
			slug, err := value.NewTenantSlug(slugStr)
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

		authMw := middleware.Auth(middleware.NewTenantProvider(dbTenants))
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterStaticFiles(ui.DistFiles)

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	srv.RegisterSwaggerUI()

	if pgPool != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)

		adminSessionStore := postgres.NewPostgresAdminSessionStore(pgPool)
		adminSessionUC := admin_session.NewAdminSessionUseCase(adminSessionStore)
		adminAuthHandler := adminhandler.NewAdminAuthHandler(adminSessionUC, cfg.Admin)
		srv.RegisterAdminSessionMiddleware(middleware.AdminSessionAuth(adminSessionUC))
		srv.RegisterAdminAuthRoutes(adminAuthHandler)
		logger.Info("admin auth registered", zap.String("username", cfg.Admin.Username))

		auditLogStore := postgres.NewAuditLogStore(pgPool, 100)
		defer auditLogStore.Shutdown()
		auditAdapter := &auditLogAdapter{store: auditLogStore}

		pgTenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)
		dictCache := dictionaryrepo.NewValkeyDictionaryCache(vkClient, 5*time.Minute)
		tenantHandler := adminhandler.NewTenantHandler(pgTenantRepo, dictCache, auditAdapter)
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

		auditHandler := adminhandler.NewAuditHandler(auditAdapter)
		srv.RegisterAuditHandler(auditHandler)

		healthChecker := adminhandler.NewProviderHealthChecker(5 * time.Second)
		if cfg.Routing != nil {
			var targets []adminhandler.ProviderTarget
			for _, p := range cfg.Routing.Providers {
				targets = append(targets, adminhandler.ProviderTarget{
					Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
				})
			}
			if len(targets) > 0 {
				healthChecker.StartBackgroundRefresh(context.Background(), 30*time.Second, targets)
			}
		}
		routingHandler := adminhandler.NewRoutingHandler(cfg.Routing, healthChecker)
		srv.RegisterRoutingHandler(routingHandler)

		sessionStore := sessionrepo.NewPostgresSessionStore(pgPool)
		sessionUseCase := session.NewSessionUseCase(sessionStore)
		sessionHandler := api.NewSessionHandler(sessionUseCase, cfg.Session)
		srv.RegisterSessionHandler(sessionHandler)

		pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
		analyticsHandler := analyticshandler.NewAnalyticsHandler(pgUsageStore)
		srv.RegisterAnalyticsHandler(analyticsHandler, cfg.Debug)
		logger.Info("analytics handler registered")

		if cfg.Session.CleanupEnabled {
			cleanupWorker := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
			cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
			go cleanupWorker.Run(cleanupCtx)
			logger.Info("session cleanup worker registered",
				zap.Duration("interval", cfg.Session.CleanupInterval),
			)
			defer cleanupCancel()
		} else {
			logger.Debug("session cleanup worker disabled")
		}
	}

	return srv
}

// @sk-task combined-binary: Adapter from postgres.AuditLogEntry to admin.AuditEvent
type auditLogAdapter struct {
	store *postgres.AuditLogStore
}

func (a *auditLogAdapter) Write(ctx context.Context, event *adminhandler.AuditEvent) error {
	return a.store.Write(ctx, &postgres.AuditLogEntry{
		AdminUsername: event.AdminUsername,
		Action:        event.Action,
		Target:        event.Target,
		Details:       event.Details,
		CreatedAt:     event.CreatedAt,
	})
}

func (a *auditLogAdapter) List(ctx context.Context, limit, offset int) ([]adminhandler.AuditEvent, error) {
	entries, err := a.store.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	events := make([]adminhandler.AuditEvent, len(entries))
	for i, e := range entries {
		events[i] = adminhandler.AuditEvent{
			AdminUsername: e.AdminUsername,
			Action:        e.Action,
			Target:        e.Target,
			Details:       e.Details,
			CreatedAt:     e.CreatedAt,
		}
	}
	return events, nil
}
