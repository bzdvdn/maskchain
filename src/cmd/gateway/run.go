package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/zap"

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
	defer logger.Sync()

	b, err := bootstrap.InitBootstrap(context.Background(), cfg, logger, serviceName(cfg))
	if err != nil {
		logger.Fatal("bootstrap failed", zap.Error(err))
	}
	defer b.Close()

	if cfg.DB != nil && cfg.DB.DSN != "" {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	provDeps, err := initProviders(cfg.Routing, cfg.Egress)
	if err != nil {
		logger.Fatal("failed to init providers", zap.Error(err))
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

func serviceName(cfg *config.Config) string {
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		return cfg.OTel.ServiceName
	}
	return "maskchain-gateway"
}

func initSession(cfg *config.Config, pgPool *pgxpool.Pool, vkClient valkey.Client, logger *zap.Logger) *session.SessionUseCase {
	cacheTTL := cfg.Session.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	sessionPG := sessionrepo.NewPostgresSessionStore(pgPool)
	sessionVK := sessionrepo.NewValkeySessionCache(vkClient, cacheTTL)
	store := sessionrepo.NewCachedSessionStore(sessionPG, sessionVK, logger)
	return session.NewSessionUseCase(store)
}

func serve(cfg *config.Config, logger *zap.Logger, b *bootstrap.Bootstrap, srv *api.Server) {
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

func watchConfigReload(cfg *config.Config, pd *providerDeps, logger *zap.Logger) {
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
				logger.Error("config reload: routing registry update failed", zap.Error(err))
				return
			}
			newClients := make(map[string]ports.ProviderClient)
			if new.Egress != nil && new.Routing != nil {
				for i := range new.Routing.Providers {
					pcfg := &new.Routing.Providers[i]
					client, err := provider.NewProviderClient(pcfg, new.Egress)
					if err != nil {
						logger.Error("config reload: failed to create provider client", zap.String("provider", pcfg.Name), zap.Error(err))
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

func initTenants(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.Server, dictCache *dictionaryrepo.ValkeyDictionaryCache, logger *zap.Logger) {
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

	for _, t := range dbTenants {
		slug := t.Slug().String()
		if dicts := t.Dictionaries(); len(dicts) > 0 {
			if err := dictCache.Set(context.Background(), slug, dicts); err != nil {
				logger.Warn("failed to warm dict cache at startup", zap.String("tenant", slug), zap.Error(err))
			}
		}
	}

	tenantProvider := middleware.NewTenantProvider(dbTenants)
	srv.RegisterAuth(middleware.Auth(tenantProvider))
	logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))

	reloadCtx, reloadCancel := context.WithCancel(context.Background())
	_ = reloadCancel
	go startTenantReload(reloadCtx, cfg, tenantResolver, dictCache, tenantProvider, logger)
}

func startTenantReload(ctx context.Context, cfg *config.Config, resolver *resolver.DBFirstTenantResolver, dictCache *dictionaryrepo.ValkeyDictionaryCache, tp *middleware.TenantProvider, logger *zap.Logger) {
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
				logger.Warn("tenant reload failed", zap.Error(err))
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
						logger.Warn("failed to warm dict cache on reload", zap.String("tenant", slug), zap.Error(setErr))
					}
				}
			}
			tp.Update(newTenants)
			logger.Debug("tenants hot-reloaded", zap.Int("count", len(newTenants)))
		}
	}
}

func runSessionCleanup(cfg *config.Config, sessionUseCase *session.SessionUseCase, logger *zap.Logger) {
	if !cfg.Session.CleanupEnabled {
		logger.Debug("session cleanup worker disabled")
		return
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	_ = cleanupCancel
	w := worker.NewCleanupWorker(sessionUseCase, cfg.Session.CleanupInterval, logger)
	go w.Run(cleanupCtx)
	logger.Info("session cleanup worker registered", zap.Duration("interval", cfg.Session.CleanupInterval))
}

func runAnalytics(cfg *config.Config, pgPool *pgxpool.Pool, srv *api.Server, logger *zap.Logger) {
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
			logger.Warn("analytics: invalid cost rate, skipping", zap.String("model", cr.Model), zap.Error(err))
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

func initDetectors(log *zap.Logger) *detector.DetectorRegistry {
	registry := detector.NewDetectorRegistry()
	pii, err := detector.NewPIIDetector()
	if err != nil {
		log.Fatal("failed to create PII detector", zap.Error(err))
	}
	secrets, err := detector.NewSecretsDetector()
	if err != nil {
		log.Fatal("failed to create secrets detector", zap.Error(err))
	}
	financial, err := detector.NewFinancialDetector()
	if err != nil {
		log.Fatal("failed to create financial detector", zap.Error(err))
	}
	combined := detector.NewCompositeDetector(pii, secrets, financial)
	if err := registry.Register(entity.DetectorTypeRegex, combined); err != nil {
		log.Fatal("register composite regex detector", zap.Error(err))
	}
	placeholder := detector.NewDictionaryDetector(nil)
	if err := registry.Register(entity.DetectorTypeDictionary, placeholder); err != nil {
		log.Fatal("register dictionary detector", zap.Error(err))
	}
	promptInjection := detector.NewPromptInjectionDetector()
	if err := registry.Register(entity.DetectorTypePromptInjection, promptInjection); err != nil {
		log.Fatal("register prompt injection detector", zap.Error(err))
	}
	return registry
}
