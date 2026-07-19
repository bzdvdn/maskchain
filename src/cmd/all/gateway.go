package main

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
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
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
)

// @sk-task combined-binary: Build and wire gateway server
func buildGatewayServer(
	cfg *config.Config,
	logger *zap.Logger,
	serviceName string,
	pgPool *pgxpool.Pool,
	vkClient valkey.Client,
	promRegistry *prometheus.Registry,
	metricsHandler gin.HandlerFunc,
	registry *routingSvc.ProviderRegistry,
	selector *routingSvc.RouteSelector,
	clients map[string]ports.ProviderClient,
	fallbackHandler *routingSvc.FallbackHandler,
	routingHandler *api.RoutingProxyHandler,
	otelShutdown func(context.Context) error,
) *api.Server {
	detectorRegistry := initDetectors(logger)

	maskTTL := time.Duration(cfg.Mask.CacheTTLSec) * time.Second
	pgRepo := maskrepo.NewPostgresMaskRepo(pgPool)
	vkRepo := maskrepo.NewValkeyMaskRepo(vkClient, maskTTL)
	maskStorage := maskrepo.NewCachedMaskRepo(pgRepo, vkRepo)
	maskUseCase := domainMask.NewMaskUseCase(detectorRegistry, maskStorage)
	maskHandler := api.NewMaskHandler(maskUseCase, detectorRegistry)

	sessionCacheTTL := cfg.Session.CacheTTL
	if sessionCacheTTL <= 0 {
		sessionCacheTTL = 5 * time.Minute
	}
	sessionPG := sessionrepo.NewPostgresSessionStore(pgPool)
	sessionVK := sessionrepo.NewValkeySessionCache(vkClient, sessionCacheTTL)
	sessionStore := sessionrepo.NewCachedSessionStore(sessionPG, sessionVK, logger)
	sessionUseCase := session.NewSessionUseCase(sessionStore)

	dictCache := dictionaryrepo.NewValkeyDictionaryCache(vkClient, 5*time.Minute)

	var rlRepo *budgetrepo.ValkeyRateLimitRepo
	var tbRepo *budgetrepo.ValkeyTokenBudgetRepo
	if cfg.RateLimit != nil {
		if vkClient == nil {
			logger.Warn("rate limit configured but Valkey unavailable — rate limiting disabled, requests will pass through")
		} else {
			rlRepo = budgetrepo.NewValkeyRateLimitRepo(vkClient)
			tbRepo = budgetrepo.NewValkeyTokenBudgetRepo(vkClient)
			logger.Info("rate limit repositories initialized")
		}
	} else {
		logger.Info("rate limit disabled — no ratelimit config section")
	}

	if cfg.Server == nil {
		cfg.Server = config.DefaultConfig().Server
	}
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

	srv := api.New(cfg.Server, logger, serviceName, healthSvc)

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

		for _, t := range dbTenants {
			slug := t.Slug().String()
			if dicts := t.Dictionaries(); len(dicts) > 0 {
				if err := dictCache.Set(context.Background(), slug, dicts); err != nil {
					logger.Warn("failed to warm dict cache at startup", zap.String("tenant", slug), zap.Error(err))
				}
			}
		}

		tenantProvider := middleware.NewTenantProvider(dbTenants)
		authMw := middleware.Auth(tenantProvider)
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))

		reloadCtx, reloadCancel := context.WithCancel(context.Background())
		defer reloadCancel()
		go func() {
			ticker := time.NewTicker(cfg.Server.TenantReloadInterval)
			defer ticker.Stop()
			for {
				select {
				case <-reloadCtx.Done():
					return
				case <-ticker.C:
					reloadCtx2, reloadCancel2 := context.WithTimeout(context.Background(), 10*time.Second)
					newTenants, err := tenantResolver.List(reloadCtx2)
					reloadCancel2()
					if err != nil {
						logger.Warn("tenant reload failed", zap.Error(err))
						continue
					}
					if len(newTenants) > 0 {
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
						tenantProvider.Update(newTenants)
						logger.Debug("tenants hot-reloaded", zap.Int("count", len(newTenants)))
					}
				}
			}
		}()
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	if cfg.RateLimit != nil && rlRepo != nil {
		rateLimitMw := middleware.RateLimit(rlRepo, cfg.RateLimit, tbRepo)
		srv.RegisterRateLimit(rateLimitMw)
		logger.Info("rate limit middleware registered")
	}

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)
	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterMaskHandler(maskHandler)

	pipelineFactory := appshield.NewScanPipelineFactory(detectorRegistry)
	scanUseCase := appshield.NewScanUseCase(pipelineFactory)
	shieldEngine := appshield.NewShieldEngine(scanUseCase)
	logger.Info("shield engine initialized")

	sessionMiddleware := middleware.SessionMiddleware(sessionUseCase, cfg.Session, logger)
	srv.RegisterSessionMiddleware(sessionMiddleware)
	logger.Info("session middleware registered")

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

	analyticsCtx, analyticsCancel := context.WithCancel(context.Background())
	defer analyticsCancel()
	if cfg.Analytics != nil && pgPool != nil {
		costRates := make([]*analytics.CostRate, 0, len(cfg.Analytics.CostRates))
		for _, cr := range cfg.Analytics.CostRates {
			rate, err := analytics.NewCostRate(cr.Model, cr.InputPricePer1K, cr.OutputPricePer1K)
			if err != nil {
				logger.Warn("analytics: invalid cost rate, skipping", zap.String("model", cr.Model), zap.Error(err))
				continue
			}
			costRates = append(costRates, rate)
		}
		costRegistry := analytics.NewCostRateRegistry(costRates)
		logger.Info("cost rate registry created", zap.Int("rates", len(costRates)))

		pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
		batchInterval, _ := time.ParseDuration(cfg.Analytics.BatchInterval)
		if batchInterval <= 0 {
			batchInterval = 5 * time.Second
		}
		asyncWorker := analyticsapp.NewAsyncWorker(pgUsageStore, 1000, batchInterval, logger)
		go asyncWorker.Run(analyticsCtx)
		logger.Info("analytics async worker started", zap.Duration("batch_interval", batchInterval))

		usageMw := middleware.NewUsageMiddleware(costRegistry, asyncWorker.Buffer(), logger)
		srv.RegisterUsageMiddleware(usageMw.Handler())
		logger.Info("usage middleware registered")

		retention := time.Duration(cfg.Analytics.RetentionDays) * 24 * time.Hour
		cleanupInterval := 10 * batchInterval
		aggWorker := analyticsapp.NewAggregationWorker(pgPool, batchInterval, logger)
		go aggWorker.Run(analyticsCtx)
		logger.Info("analytics aggregation worker started")

		cleanupWorker := analyticsapp.NewCleanupWorker(pgUsageStore, cleanupInterval, retention, logger)
		go cleanupWorker.Run(analyticsCtx)
		logger.Info("analytics cleanup worker started", zap.Duration("retention", retention))
	} else {
		logger.Debug("analytics pipeline disabled — no analytics config or no db pool")
	}

	srv.RegisterProxyRoute(middleware.ShieldMiddleware(shieldEngine, cfg.Shield, logger, sessionUseCase), routingHandler)
	logger.Info("gateway routes registered")

	return srv
}

// @sk-task combined-binary: Init detectors with CompositeDetector
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
