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
	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	analyticsapp "github.com/bzdvdn/maskchain/src/internal/app/analytics"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/bzdvdn/maskchain/src/pkg/version"
)

func run() {
	cfg, logger := initConfigLog()

	b, err := bootstrap.InitBootstrap(context.Background(), cfg, logger, serviceName(cfg))
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
	}

	provDeps, err := initProviders(cfg.Routing, cfg.Egress)
	if err != nil {
		logger.Error("failed to init providers", slog.String("error", err.Error()))
		os.Exit(1)
	}
	routingHandler := api.NewRoutingProxyHandler(provDeps.selector, provDeps.fallbackHandler)
	watchConfigReload(cfg, provDeps, logger)

	if cfg.Routing != nil {
		pollCtx, pollCancel := context.WithCancel(context.Background())
		defer pollCancel()
		healthChecker := routingSvc.NewHealthChecker(provDeps.registry, nil)
		go healthChecker.Start(pollCtx, 30*time.Second)
		logger.Info("provider health checker started")
	}

	srv := api.New(cfg.Server, logger, serviceName(cfg), b.HealthSvc)

	var rlRepo *budgetrepo.ValkeyRateLimitRepo
	var tbRepo *budgetrepo.ValkeyTokenBudgetRepo
	if cfg.RateLimit != nil && b.ValkeyClient != nil {
		rlRepo = budgetrepo.NewValkeyRateLimitRepo(b.ValkeyClient)
		tbRepo = budgetrepo.NewValkeyTokenBudgetRepo(b.ValkeyClient)
	} else if cfg.RateLimit != nil {
		logger.Warn("rate limit configured but Valkey unavailable — rate limiting disabled")
	}

	detectorRegistry := initDetectors(logger)

	maskTTL := time.Duration(cfg.Mask.CacheTTLSec) * time.Second
	pgMaskRepo := maskrepo.NewPostgresMaskRepo(b.PGPool)
	vkMaskRepo := maskrepo.NewValkeyMaskRepo(b.ValkeyClient, maskTTL)
	maskStorage := maskrepo.NewCachedMaskRepo(pgMaskRepo, vkMaskRepo)
	maskUseCase := domainMask.NewMaskUseCase(detectorRegistry, maskStorage)
	maskHandler := api.NewMaskHandler(maskUseCase, detectorRegistry)

	dictCache := dictionaryrepo.NewValkeyDictionaryCache(b.ValkeyClient, 5*time.Minute)
	initTenants(cfg, b.PGPool, srv, dictCache, logger)

	sessionUseCase := initSession(cfg, b.PGPool, b.ValkeyClient, logger)
	srv.RegisterSessionMiddleware(middleware.SessionMiddleware(sessionUseCase, cfg.Session, logger))

	if cfg.RateLimit != nil && rlRepo != nil {
		srv.RegisterRateLimit(middleware.RateLimit(rlRepo, cfg.RateLimit, tbRepo))
	}
	srv.RegisterDebugRoutes(middleware.AdminAuth(cfg.Debug))
	srv.RegisterMetricsRoute(metrics.Handler(b.PromRegistry))
	srv.RegisterVersionRoute(version.Info())
	srv.RegisterMaskHandler(maskHandler)

	pipelineFactory := appshield.NewScanPipelineFactory(detectorRegistry)
	shieldEngine := appshield.NewShieldEngine(appshield.NewScanUseCase(pipelineFactory))
	logger.Info("shield engine initialized")

	runSessionCleanup(cfg, sessionUseCase, logger)
	runAnalytics(cfg, b.PGPool, srv, logger)

	srv.RegisterProxyRoute(middleware.ShieldMiddleware(shieldEngine, cfg.Shield, logger, sessionUseCase), routingHandler)
	logger.Info("proxy routes registered")

	serve(cfg, logger, b, srv)
}

func initConfigLog() (*config.Config, *slog.Logger) {
	cfg := config.MustLoadConfig()
	logger := bootstrap.BuildLogger(cfg.Log.Level)
	logger.Debug("config loaded", slog.Any("config", cfg))
	return cfg, logger
}

func serviceName(cfg *config.Config) string {
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		return cfg.OTel.ServiceName
	}
	return "maskchain-gateway"
}

func initSession(cfg *config.Config, pgPool *pgxpool.Pool, vkClient valkey.Client, logger *slog.Logger) *session.SessionUseCase {
	cacheTTL := cfg.Session.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	sessionPG := sessionrepo.NewPostgresSessionStore(pgPool)
	sessionVK := sessionrepo.NewValkeySessionCache(vkClient, cacheTTL)
	store := sessionrepo.NewCachedSessionStore(sessionPG, sessionVK, logger)
	return session.NewSessionUseCase(store)
}

func serve(cfg *config.Config, logger *slog.Logger, b *bootstrap.Bootstrap, srv *api.Server) {
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

func watchConfigReload(cfg *config.Config, pd *providerDeps, logger *slog.Logger) {
	cfgDir := config.ConfigDirFromArgs()
	if cfgDir == "" {
		return
	}
	reloadCtx, reloadCancel := context.WithCancel(context.Background())
	_ = reloadCancel
	config.WatchConfigDir(reloadCtx, cfgDir, func(old, new *config.Config) {
		changed := config.DiffSections(old, new)
		if changed["routing"] && pd.registry != nil {
			if err := pd.registry.UpdateConfig(toDomainRoutingConfig(new.Routing)); err != nil {
				logger.Error("config reload: routing registry update failed", slog.String("error", err.Error()))
				return
			}
			newClients := make(map[string]ports.ProviderClient)
			if new.Egress != nil && new.Routing != nil {
				for i := range new.Routing.Providers {
					pcfg := &new.Routing.Providers[i]
					client, err := provider.NewProviderClient(pcfg, new.Egress)
					if err != nil {
						logger.Error("config reload: failed to create provider client", slog.String("provider", pcfg.Name), slog.String("error", err.Error()))
						continue
					}
					newClients[pcfg.Name] = client
				}
			}
			pd.fallbackHandler.UpdateClients(newClients)
			logger.Info("config reloaded: routing changed")
		}
		if changed["shield"] && cfg.Shield != nil && new.Shield != nil {
			*cfg.Shield = *new.Shield
		}
		if changed["ratelimit"] && cfg.RateLimit != nil && new.RateLimit != nil {
			*cfg.RateLimit = *new.RateLimit
		}
		if changed["debug"] && cfg.Debug != nil && new.Debug != nil {
			*cfg.Debug = *new.Debug
		}
	})
}

func initTenants(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.Server, dictCache *dictionaryrepo.ValkeyDictionaryCache, logger *slog.Logger) {
	if cfg.Tenants == nil {
		logger.Warn("no tenants configured, auth disabled")
		return
	}

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
		cfgTenants[slugStr] = entity.NewTenant(slug, tc.Name, tc.AuthHeader, tc.APIKeys, opts...)
	}
	tenantResolver := resolver.NewDBFirstTenantResolver(tenantRepo, cfgTenants)

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

	for _, t := range dbTenants {
		slug := t.Slug().String()
		if dicts := t.Dictionaries(); len(dicts) > 0 {
			if err := dictCache.Set(context.Background(), slug, dicts); err != nil {
				logger.Warn("failed to warm dict cache at startup", slog.String("tenant", slug), slog.String("error", err.Error()))
			}
		}
	}

	tenantProvider := middleware.NewTenantProvider(dbTenants)
	srv.RegisterAuth(middleware.Auth(tenantProvider))
	logger.Info("auth middleware registered", slog.Int("tenants", len(dbTenants)))

	reloadCtx, reloadCancel := context.WithCancel(context.Background())
	_ = reloadCancel
	go startTenantReload(reloadCtx, cfg, tenantResolver, dictCache, tenantProvider, logger)
}

func startTenantReload(ctx context.Context, cfg *config.Config, resolver *resolver.DBFirstTenantResolver, dictCache *dictionaryrepo.ValkeyDictionaryCache, tp *middleware.TenantProvider, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.Server.TenantReloadInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
			newTenants, err := resolver.List(rCtx)
			rCancel()
			if err != nil {
				logger.Warn("tenant reload failed", slog.String("error", err.Error()))
				continue
			}
			if len(newTenants) == 0 {
				continue
			}
			for _, t := range newTenants {
				slug := t.Slug().String()
				cachedDicts, cacheErr := dictCache.Get(context.Background(), slug)
				if cacheErr == nil && cachedDicts != nil {
					t.SetDictionaries(cachedDicts)
				} else if dicts := t.Dictionaries(); len(dicts) > 0 {
					if setErr := dictCache.Set(context.Background(), slug, dicts); setErr != nil {
						logger.Warn("failed to warm dict cache on reload", slog.String("tenant", slug), slog.String("error", setErr.Error()))
					}
				}
			}
			tp.Update(newTenants)
			logger.Debug("tenants hot-reloaded", slog.Int("count", len(newTenants)))
		}
	}
}

func runSessionCleanup(cfg *config.Config, sessionUseCase *session.SessionUseCase, logger *slog.Logger) {
	if !cfg.Session.CleanupEnabled {
		logger.Debug("session cleanup worker disabled")
		return
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	_ = cleanupCancel
	w := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
	go w.Run(cleanupCtx)
	logger.Info("session cleanup worker registered", slog.Duration("interval", cfg.Session.CleanupInterval))
}

func runAnalytics(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.Server, logger *slog.Logger) {
	if cfg.Analytics == nil || pgPool == nil {
		logger.Debug("analytics pipeline disabled")
		return
	}
	analyticsCtx, analyticsCancel := context.WithCancel(context.Background())
	_ = analyticsCancel

	costRates := make([]*analytics.CostRate, 0, len(cfg.Analytics.CostRates))
	for _, cr := range cfg.Analytics.CostRates {
		rate, err := analytics.NewCostRate(cr.Model, cr.InputPricePer1K, cr.OutputPricePer1K)
		if err != nil {
			logger.Warn("analytics: invalid cost rate, skipping", slog.String("model", cr.Model), slog.String("error", err.Error()))
			continue
		}
		costRates = append(costRates, rate)
	}

	pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
	batchInterval, _ := time.ParseDuration(cfg.Analytics.BatchInterval)
	if batchInterval <= 0 {
		batchInterval = 5 * time.Second
	}
	asyncWorker := analyticsapp.NewAsyncWorker(pgUsageStore, 1000, batchInterval, logger)
	go asyncWorker.Run(analyticsCtx)

	srv.RegisterUsageMiddleware(middleware.NewUsageMiddleware(
		analytics.NewCostRateRegistry(costRates),
		asyncWorker.Buffer(),
		logger,
	).Handler())

	go analyticsapp.NewAggregationWorker(pgPool, batchInterval, logger).Run(analyticsCtx)

	retention := time.Duration(cfg.Analytics.RetentionDays) * 24 * time.Hour
	go analyticsapp.NewCleanupWorker(pgUsageStore, 10*batchInterval, retention, logger).Run(analyticsCtx)
	logger.Info("analytics pipeline started")
}

func initDetectors(log *slog.Logger) *detector.DetectorRegistry {
	registry := detector.NewDetectorRegistry()
	pii, err := detector.NewPIIDetector()
	if err != nil {
		log.Error("failed to create PII detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	secrets, err := detector.NewSecretsDetector()
	if err != nil {
		log.Error("failed to create secrets detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	financial, err := detector.NewFinancialDetector()
	if err != nil {
		log.Error("failed to create financial detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	combined := detector.NewCompositeDetector(pii, secrets, financial)
	if err := registry.Register(entity.DetectorTypeRegex, combined); err != nil {
		log.Error("register composite regex detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	placeholder := detector.NewDictionaryDetector(nil)
	if err := registry.Register(entity.DetectorTypeDictionary, placeholder); err != nil {
		log.Error("register dictionary detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	promptInjection := detector.NewPromptInjectionDetector()
	if err := registry.Register(entity.DetectorTypePromptInjection, promptInjection); err != nil {
		log.Error("register prompt injection detector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	return registry
}
