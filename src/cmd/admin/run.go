package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	conversationrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/conversation"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	conversationhandler "github.com/bzdvdn/maskchain/src/internal/api/handler/conversation"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	shvalue "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/pkg/version"
	"github.com/bzdvdn/maskchain/ui"
)

func run() {
	cfg, logger := initConfigLog()

	b, err := bootstrap.InitBootstrap(context.Background(), cfg, logger, adminServiceName(cfg))
	if err != nil {
		logger.Error("bootstrap failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer b.Close()

	if cfg.DB != nil && cfg.DB.DSN != "" {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Error("failed to run migrations", slog.String("error", err.Error()))
			os.Exit(1)
		}
		// @sk-task 403-key-at-rest-encryption#T2.2: Seal legacy plaintext provider secrets post-migration (AC-002)
		if cfg.Routing != nil {
			key := os.Getenv(config.KeysKeyEnvVar)
			enc, kerr := crypto.New(key)
			if kerr != nil {
				logger.Error("at-rest encryption key unavailable — refusing to start (fail-closed)",
					slog.String("env", config.KeysKeyEnvVar), slog.String("error", kerr.Error()))
				os.Exit(1)
			}
			reencCtx, reencCancel := context.WithTimeout(context.Background(), 60*time.Second)
			rerr := postgres.ReencryptProviderSecrets(reencCtx, b.PGPool, enc)
			reencCancel()
			if rerr != nil {
				logger.Error("failed to re-encrypt legacy provider secrets", slog.String("error", rerr.Error()))
				os.Exit(1)
			}
			logger.Info("provider secret re-encryption check complete")
		}
	}

	watchAdminConfigReload(cfg, logger)
	srv := api.NewAdminServer(cfg.Server, logger, adminServiceName(cfg), b.HealthSvc)
	srv.RegisterMetricsRoute(metrics.Handler(b.PromRegistry))
	srv.RegisterVersionRoute(version.Info())
	srv.RegisterStatusHandler(admin.NewStatusHandler(version.Info(), cfg, b.HealthSvc))
	if err := srv.RegisterStaticFiles(ui.DistFiles); err != nil {
		logger.Error("register static files", slog.String("error", err.Error()))
		os.Exit(1)
	}
	srv.RegisterDebugRoutes(middleware.AdminAuth(cfg.Debug))

	if err := srv.RegisterSwaggerUI(); err != nil {
		logger.Error("register swagger ui", slog.String("error", err.Error()))
	}

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

		backfillCtx, backfillCancel := context.WithTimeout(context.Background(), 10*time.Second)
		bootstrap.BackfillVirtualKeys(backfillCtx, cfg, b.PGPool, logger)
		backfillCancel()

		dictCache := dictionaryrepo.NewValkeyDictionaryCache(b.ValkeyClient, 5*time.Minute)
		pgTenantRepo := postgres.NewPostgresTenantRepo(b.PGPool, txMgr)
		tenantHandler := admin.NewTenantHandler(pgTenantRepo, dictCache, auditAdapter)
		vkRepo := postgres.NewPostgresVirtualKeyRepository(b.PGPool)
		tenantMw := middleware.AdminSessionOrTokenAuth(adminSessionUC, cfg.Debug, func(ctx context.Context, apiKey string) bool {
			if apiKey == "" {
				return false
			}
			vk, err := vkRepo.FindByKeyHash(ctx, virtualkey.KeyHash(apiKey))
			return err == nil && vk != nil
		})
		srv.RegisterTenantHandler(tenantHandler, tenantMw)

		complianceRegistry := bootstrap.LoadComplianceRegistry(cfg, b.Logger)
		if complianceRegistry != nil {
			complianceHandler := admin.NewComplianceHandler(complianceRegistry, pgTenantRepo)
			srv.RegisterComplianceHandler(complianceHandler, tenantMw)
		}

		vkHandler := admin.NewVirtualKeyHandler(vkRepo, auditAdapter)
		srv.RegisterVirtualKeyHandler(vkHandler)

		// @sk-task shield-detector-catalog#T2.2: expose the shield catalog (AC-001, AC-002)
		srv.RegisterCatalogHandler(admin.NewCatalogHandler(
			bootstrap.ReferenceDetectorRegistry().Types(),
			complianceRegistry,
		))

		gatewayURL := ""
		if cfg.Admin != nil {
			gatewayURL = cfg.Admin.GatewayURL
		}
		srv.RegisterPlaygroundHandler(admin.NewPlaygroundHandler(gatewayURL))

		auditHandler := admin.NewAuditHandler(auditAdapter)
		srv.RegisterAuditHandler(auditHandler)

		healthChecker := admin.NewProviderHealthChecker(5 * time.Second)
		loadCtx, loadCancel := context.WithTimeout(context.Background(), 10*time.Second)
		registryProviders, _, _ := bootstrap.LoadRoutingFromDB(loadCtx, cfg.Routing, b.PGPool, logger)
		loadCancel()
		var targets []admin.ProviderTarget
		for _, p := range registryProviders {
			targets = append(targets, admin.ProviderTarget{
				Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
			})
		}
		if len(targets) > 0 {
			healthChecker.StartBackgroundRefresh(context.Background(), 30*time.Second, targets)
		}
		// @sk-task routing-ia#T2.3: pass cost rates and tx runner for atomic provider+models (AC-004)
		costRateStore := analyticsrepo.NewPostgresCostRateStore(b.PGPool)
		routingHandler := admin.NewRoutingHandler(postgres.NewPostgresRegistryRepository(b.PGPool), costRateStore, txMgr, provider.NewModelDiscoverer(cfg.Egress), healthChecker, auditAdapter)
		srv.RegisterRoutingHandler(routingHandler)

		costRateHandler := admin.NewCostRateHandler(costRateStore, auditAdapter)
		srv.RegisterCostRateHandler(costRateHandler)

		// @sk-task 301-budget-enforcement#T3.3: Register budget CRUD handler (AC-006)
		budgetHandler := admin.NewBudgetHandler(postgres.NewPostgresBudgetRepository(b.PGPool), bootstrap.NewBudgetCounter(b.ValkeyClient), auditAdapter)
		srv.RegisterBudgetHandler(budgetHandler)

		sessionStore := sessionrepo.NewPostgresSessionStore(b.PGPool)
		sessionUseCase := session.NewSessionUseCase(sessionStore)
		srv.RegisterSessionHandler(api.NewSessionHandler(sessionUseCase, cfg.Session))

		pgUsageStore := analyticsrepo.NewPgUsageStore(b.PGPool)
		analyticsHandler := analyticshandler.NewAnalyticsHandlerWithCostRates(pgUsageStore, analyticsrepo.NewPostgresCostRateStore(b.PGPool))
		srv.RegisterAnalyticsHandler(analyticsHandler, cfg.Debug)

		// @sk-task conversation-logging#T3.1: Register conversation read API in standalone admin (AC-005, AC-006)
		if cfg.Conversations != nil && cfg.Conversations.Enabled {
			key := os.Getenv(crypto.KeyEnvVar)
			enc, err := crypto.New(key)
			if err != nil {
				logger.Error("conversation logging enabled but encryption key unavailable — conversation API disabled",
					slog.String("env", crypto.KeyEnvVar), slog.String("error", err.Error()))
			} else {
				convStore := conversationrepo.NewPgConversationStore(b.PGPool)
				srv.RegisterConversationHandler(conversationhandler.NewConversationHandler(convStore, enc))
				logger.Info("conversation handler registered")
			}
		}

		if cfg.Session.CleanupEnabled {
			cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
			_ = cleanupCancel
			w := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
			go w.Run(cleanupCtx)
		}
	}

	adminServe(cfg, logger, b, srv)
}

func initConfigLog() (*config.Config, *slog.Logger) {
	cfg := config.MustLoadConfig()
	logger := bootstrap.BuildLogger(cfg.Log.Level)
	logger.Debug("config loaded", slog.Any("config", cfg))
	return cfg, logger
}

func adminServiceName(cfg *config.Config) string {
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		return cfg.OTel.ServiceName + "-admin"
	}
	return "maskchain-admin"
}

func watchAdminConfigReload(cfg *config.Config, logger *slog.Logger) {
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

func initAdminTenants(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.AdminServer, logger *slog.Logger) {
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
	if mode, err := shvalue.ParseRetentionMode(cfg.DefaultRetentionMode()); err == nil {
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
	dbTenants, err := tenantResolver.List(loadCtx)
	loadCancel()
	if err != nil {
		logger.Error("failed to load tenants from db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("auth middleware registered", slog.Int("tenants", len(dbTenants)))
}

func adminServe(cfg *config.Config, logger *slog.Logger, b *bootstrap.Bootstrap, srv *api.AdminServer) {
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
		logger.Info("shutting down", slog.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("server error", slog.String("error", err.Error()))
		return
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := b.OTelShutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", slog.String("error", err.Error()))
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
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
