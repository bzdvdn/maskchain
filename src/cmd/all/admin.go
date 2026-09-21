package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	conversationrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/conversation"
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
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
	"github.com/bzdvdn/maskchain/src/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	adminhandler "github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	conversationhandler "github.com/bzdvdn/maskchain/src/internal/api/handler/conversation"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task combined-binary: Build and wire admin server
func buildAdminServer(
	cfg *config.Config,
	logger *slog.Logger,
	serviceName string,
	pgPool *pgxpool.Pool,
	vkClient valkey.Client,
	promRegistry *prometheus.Registry,
	metricsHandler gin.HandlerFunc,
	otelShutdown func(context.Context) error,
	vkCache *middleware.VirtualKeyCache,
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
	srv.RegisterStatusHandler(adminhandler.NewStatusHandler(version.Info(), cfg, healthSvc))

	if cfg.Tenants != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		tenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)

		cfgTenants := make(map[string]*entity.Tenant, len(cfg.Tenants))
		for slugStr, tc := range cfg.Tenants {
			slug, err := value.NewTenantSlug(slugStr)
			if err != nil {
				logger.Error("invalid tenant slug", slog.String("tenant", slugStr), slog.String("error", err.Error()))
				os.Exit(1)
			}
			opts := []entity.TenantOption{entity.WithTenantDictionaries(nil)}
			if tc.PIIConfig != nil {
				opts = append(opts, entity.WithTenantPIIConfig(*tc.PIIConfig))
			}
			cfgTenants[slugStr] = entity.NewTenant(slug, tc.Name, tc.AuthHeader, opts...)
		}
		tenantResolver := resolver.NewDBFirstTenantResolver(tenantRepo, cfgTenants)
		if mode, err := value.ParseRetentionMode(cfg.DefaultRetentionMode()); err == nil {
			tenantResolver.SetDefaultRetentionMode(mode)
		}

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := tenantResolver.SyncConfig(syncCtx, cfgTenants); err != nil {
			syncCancel()
			logger.Error("failed to sync tenants from config", slog.String("error", err.Error()))
			os.Exit(1)
		}
		syncCancel()

		loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := tenantResolver.List(loadCtx)
		loadCancel()
		if err != nil {
			logger.Error("failed to load tenants from db", slog.String("error", err.Error()))
			os.Exit(1)
		}
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	srv.RegisterMetricsRoute(metricsHandler)
	if err := srv.RegisterStaticFiles(ui.DistFiles); err != nil {
		logger.Error("register static files", slog.String("error", err.Error()))
		os.Exit(1)
	}

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	if err := srv.RegisterSwaggerUI(); err != nil {
		logger.Error("register swagger ui", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if pgPool != nil {
		txMgr := postgres.NewPGXTransactionManager(pgPool)

		adminSessionStore := postgres.NewPostgresAdminSessionStore(pgPool)
		adminSessionUC := admin_session.NewAdminSessionUseCase(adminSessionStore)
		adminAuthHandler := adminhandler.NewAdminAuthHandler(adminSessionUC, cfg.Admin)
		srv.RegisterAdminSessionMiddleware(middleware.AdminSessionAuth(adminSessionUC))
		srv.RegisterAdminAuthRoutes(adminAuthHandler)
		logger.Info("admin auth registered", slog.String("username", cfg.Admin.Username))

		auditLogStore := postgres.NewAuditLogStore(pgPool, 100)
		defer auditLogStore.Shutdown()
		auditAdapter := &auditLogAdapter{store: auditLogStore}

		backfillCtx, backfillCancel := context.WithTimeout(context.Background(), 10*time.Second)
		bootstrap.BackfillVirtualKeys(backfillCtx, cfg, pgPool, logger)
		backfillCancel()

		pgTenantRepo := postgres.NewPostgresTenantRepo(pgPool, txMgr)
		dictCache := dictionaryrepo.NewValkeyDictionaryCache(vkClient, 5*time.Minute)
		tenantHandler := adminhandler.NewTenantHandler(pgTenantRepo, dictCache, auditAdapter)
		vkRepo := postgres.NewPostgresVirtualKeyRepository(pgPool)
		tenantMw := middleware.AdminSessionOrTokenAuth(adminSessionUC, cfg.Debug, func(ctx context.Context, apiKey string) bool {
			if apiKey == "" {
				return false
			}
			vk, err := vkRepo.FindByKeyHash(ctx, virtualkey.KeyHash(apiKey))
			return err == nil && vk != nil
		})
		srv.RegisterTenantHandler(tenantHandler, tenantMw)

		complianceRegistry := bootstrap.LoadComplianceRegistry(cfg, logger)
		if complianceRegistry != nil {
			complianceHandler := adminhandler.NewComplianceHandler(complianceRegistry, pgTenantRepo)
			srv.RegisterComplianceHandler(complianceHandler, tenantMw)
		}

		// @sk-task 403-key-at-rest-encryption#T3.3: Wire auth cache into key handler for invalidation (AC-005)
		vkHandler := adminhandler.NewVirtualKeyHandler(vkRepo, auditAdapter, vkCache)
		srv.RegisterVirtualKeyHandler(vkHandler)

		// @sk-task shield-detector-catalog#T2.2: expose the shield catalog (AC-001, AC-002)
		srv.RegisterCatalogHandler(adminhandler.NewCatalogHandler(
			bootstrap.ReferenceDetectorRegistry().Types(),
			complianceRegistry,
		))

		gatewayURL := ""
		if cfg.Admin != nil {
			gatewayURL = cfg.Admin.GatewayURL
		}
		srv.RegisterPlaygroundHandler(adminhandler.NewPlaygroundHandler(gatewayURL))

		auditHandler := adminhandler.NewAuditHandler(auditAdapter)
		srv.RegisterAuditHandler(auditHandler)

		healthChecker := adminhandler.NewProviderHealthChecker(5 * time.Second)
		loadCtx, loadCancel := context.WithTimeout(context.Background(), 10*time.Second)
		registryProviders, _ := bootstrap.LoadRoutingFromDB(loadCtx, cfg.Routing, pgPool, logger)
		loadCancel()
		var targets []adminhandler.ProviderTarget
		for _, p := range registryProviders {
			targets = append(targets, adminhandler.ProviderTarget{
				Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
			})
		}
		if len(targets) > 0 {
			healthChecker.StartBackgroundRefresh(context.Background(), 30*time.Second, targets)
		}
		// @sk-task routing-ia#T2.3: pass cost rates and tx runner for atomic provider+models (AC-004)
		costRateStore := analyticsrepo.NewPostgresCostRateStore(pgPool)
		routingHandler := adminhandler.NewRoutingHandler(postgres.NewPostgresRegistryRepository(pgPool), costRateStore, txMgr, provider.NewModelDiscoverer(cfg.Egress), healthChecker, auditAdapter)
		srv.RegisterRoutingHandler(routingHandler)

		costRateHandler := adminhandler.NewCostRateHandler(costRateStore, auditAdapter)
		srv.RegisterCostRateHandler(costRateHandler)

		// @sk-task 301-budget-enforcement#T3.3: Register budget CRUD handler (AC-006)
		budgetHandler := adminhandler.NewBudgetHandler(postgres.NewPostgresBudgetRepository(pgPool), bootstrap.NewBudgetCounter(vkClient), auditAdapter)
		srv.RegisterBudgetHandler(budgetHandler)

		sessionStore := sessionrepo.NewPostgresSessionStore(pgPool)
		sessionUseCase := session.NewSessionUseCase(sessionStore)
		sessionHandler := api.NewSessionHandler(sessionUseCase, cfg.Session)
		srv.RegisterSessionHandler(sessionHandler)

		pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
		analyticsHandler := analyticshandler.NewAnalyticsHandlerWithCostRates(pgUsageStore, analyticsrepo.NewPostgresCostRateStore(pgPool))
		srv.RegisterAnalyticsHandler(analyticsHandler, cfg.Debug)
		logger.Info("analytics handler registered")

		// @sk-task conversation-logging#T3.1: Register conversation read API in combined admin (AC-005, AC-006)
		if cfg.Conversations != nil && cfg.Conversations.Enabled {
			key := os.Getenv(crypto.KeyEnvVar)
			enc, err := crypto.New(key)
			if err != nil {
				logger.Error("conversation logging enabled but encryption key unavailable — conversation API disabled",
					slog.String("env", crypto.KeyEnvVar), slog.String("error", err.Error()))
			} else {
				convStore := conversationrepo.NewPgConversationStore(pgPool)
				srv.RegisterConversationHandler(conversationhandler.NewConversationHandler(convStore, enc))
				logger.Info("conversation handler registered")
			}
		}

		if cfg.Session.CleanupEnabled {
			cleanupWorker := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
			// Context is intentionally never cancelled: worker runs for the
			// process lifetime, matching the single-binary bootstrap pattern.
			cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
			_ = cleanupCancel
			go cleanupWorker.Run(cleanupCtx)
			logger.Info("session cleanup worker registered",
				slog.Duration("interval", cfg.Session.CleanupInterval),
			)
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
