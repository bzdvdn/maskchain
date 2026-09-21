package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	conversationrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/conversation"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	analyticsapp "github.com/bzdvdn/maskchain/src/internal/app/analytics"
	budgetapp "github.com/bzdvdn/maskchain/src/internal/app/budget"
	conversationapp "github.com/bzdvdn/maskchain/src/internal/app/conversation"
	logexportapp "github.com/bzdvdn/maskchain/src/internal/app/logexport"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/bzdvdn/maskchain/src/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
)

// @sk-task combined-binary: Build and wire gateway server
func buildGatewayServer(
	cfg *config.Config,
	logger *slog.Logger,
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
	vkCache *middleware.VirtualKeyCache,
) *api.Server {
	// @sk-task 403-key-at-rest-encryption#T2.2: Seal legacy plaintext provider secrets post-migration (AC-002)
	if pgPool != nil && cfg.Routing != nil {
		key := os.Getenv(config.KeysKeyEnvVar)
		enc, kerr := crypto.New(key)
		if kerr != nil {
			logger.Error("at-rest encryption key unavailable — refusing to start (fail-closed)",
				slog.String("env", config.KeysKeyEnvVar), slog.String("error", kerr.Error()))
			os.Exit(1)
		}
		reencCtx, reencCancel := context.WithTimeout(context.Background(), 60*time.Second)
		rerr := postgres.ReencryptProviderSecrets(reencCtx, pgPool, enc)
		reencCancel()
		if rerr != nil {
			logger.Error("failed to re-encrypt legacy provider secrets", slog.String("error", rerr.Error()))
			os.Exit(1)
		}
		logger.Info("provider secret re-encryption check complete")
	}

	detectorRegistry := initDetectors(logger)

	// @sk-task usage-accounting-integrity#T2.3: request provider usage on streams (AC-001)
	if routingHandler != nil && cfg.Analytics != nil {
		routingHandler.WithStreamUsage(cfg.Analytics.StreamUsage)
	}

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
		if pgPool != nil {
			bootstrap.BackfillVirtualKeys(context.Background(), cfg, pgPool, logger)
			vkRepo := bootstrap.NewVirtualKeyRepo(pgPool)
			// @sk-task 403-key-at-rest-encryption#T3.2: Register auth against the shared cache (AC-005, AC-006)
			authMw := middleware.VirtualKeyAuth(vkRepo, tenantProvider, vkCache)
			srv.RegisterAuth(authMw)
			srv.RegisterModelAccess(middleware.ModelAccess())
			logger.Info("virtual key auth middleware registered", slog.Int("tenants", len(dbTenants)))
		} else {
			logger.Warn("virtual key auth requires a database; auth disabled", slog.Int("tenants", len(dbTenants)))
		}

		// Context is intentionally never cancelled: reloader runs for the
		// process lifetime, matching the single-binary bootstrap pattern (cmd/gateway/run.go).
		reloadCtx, reloadCancel := context.WithCancel(context.Background())
		_ = reloadCancel
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
						logger.Warn("tenant reload failed", slog.String("error", err.Error()))
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
									logger.Warn("failed to warm dict cache on reload", slog.String("tenant", slug), slog.String("error", setErr.Error()))
								}
							}
						}
						tenantProvider.Update(newTenants)
						logger.Debug("tenants hot-reloaded", slog.Int("count", len(newTenants)))
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
	srv.RegisterSelfHandler(api.NewSelfHandler(registry, version.Info()))
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
		// Context is intentionally never cancelled: worker runs for the process
		// lifetime, matching the single-binary bootstrap pattern (cmd/gateway/run.go).
		cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
		_ = cleanupCancel
		go cleanupWorker.Run(cleanupCtx)
		logger.Info("session cleanup worker registered",
			slog.Duration("interval", cfg.Session.CleanupInterval),
		)
	} else {
		logger.Debug("session cleanup worker disabled")
	}

	// Context is intentionally never cancelled: analytics workers run for the
	// process lifetime, matching the single-binary bootstrap pattern (cmd/gateway/run.go).
	analyticsCtx, analyticsCancel := context.WithCancel(context.Background())
	_ = analyticsCancel
	if cfg.Analytics != nil && pgPool != nil {
		costRates := bootstrap.LoadCostRatesFromDB(context.Background(), cfg, pgPool, logger)
		costRegistry := newCostRateRegistry(cfg, costRates)
		logger.Info("cost rate registry created", slog.Int("rates", len(costRates)))

		pgUsageStore := analyticsrepo.NewPgUsageStore(pgPool)
		batchInterval, _ := time.ParseDuration(cfg.Analytics.BatchInterval)
		if batchInterval <= 0 {
			batchInterval = 5 * time.Second
		}
		asyncWorker := analyticsapp.NewAsyncWorker(pgUsageStore, 1000, batchInterval, logger)
		go asyncWorker.Run(analyticsCtx)
		logger.Info("analytics async worker started", slog.Duration("batch_interval", batchInterval))

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
		logger.Info("analytics cleanup worker started", slog.Duration("retention", retention))
	} else {
		logger.Debug("analytics pipeline disabled — no analytics config or no db pool")
	}

	// @sk-task conversation-logging#T2.3: Bootstrap conversation pipeline in combined binary (AC-008, DEC-002)
	// Context is intentionally never cancelled: workers run for the process lifetime,
	// matching the single-binary bootstrap pattern (cmd/gateway/run.go).
	conversationCtx, conversationCancel := context.WithCancel(context.Background())
	_ = conversationCancel
	if cfg.Conversations != nil && cfg.Conversations.Enabled && pgPool != nil {
		key := os.Getenv(crypto.KeyEnvVar)
		enc, err := crypto.New(key)
		if err != nil {
			logger.Error("conversation logging enabled but encryption key unavailable — failing closed",
				slog.String("env", crypto.KeyEnvVar), slog.String("error", err.Error()))
			os.Exit(1)
		}

		store := conversationrepo.NewPgConversationStore(pgPool)
		retention := time.Duration(cfg.Conversations.RetentionDays) * 24 * time.Hour
		if retention <= 0 {
			retention = 90 * 24 * time.Hour
		}

		asyncWorker := conversationapp.NewAsyncWorker(store, enc, 1024, 5*time.Second, logger)
		go asyncWorker.Run(conversationCtx)
		cleanupWorker := conversationapp.NewCleanupWorker(store, time.Minute, retention, logger)
		go cleanupWorker.Run(conversationCtx)

		convMw := middleware.NewConversationMiddleware(asyncWorker, logger)
		if mode, err := value.ParseRetentionMode(cfg.DefaultRetentionMode()); err == nil {
			convMw.SetDefaultRetentionMode(mode)
		}
		srv.RegisterConversationMiddleware(convMw.Handler())
		logger.Info("conversation logging pipeline started", slog.Int("retention_days", cfg.Conversations.RetentionDays))
	} else {
		logger.Debug("conversation logging disabled — no conversations config, disabled, or no db pool")
	}

	// @sk-task 301-budget-enforcement#T2.4: Wire budget middleware and aggregation worker in combined binary (AC-004, AC-005)
	budgetCtx, budgetCancel := context.WithCancel(context.Background())
	_ = budgetCancel
	if cfg.Budgets != nil && pgPool != nil && vkClient != nil {
		costRates := bootstrap.LoadCostRatesFromDB(context.Background(), cfg, pgPool, logger)
		budgetRepo := bootstrap.NewBudgetRepo(pgPool)
		counter := bootstrap.NewBudgetCounter(vkClient)
		vkRepo := bootstrap.NewVirtualKeyRepo(pgPool)
		notifier := budgetapp.NewWebhookNotifier(cfg.Budgets.AlertWebhookURL, logger)

		budgetMw := middleware.NewBudgetMiddleware(budgetRepo, counter,
			newCostRateRegistry(cfg, costRates), vkRepo, notifier, logger)
		srv.RegisterBudgetMiddleware(budgetMw.Handler())
		logger.Info("budget enforcement middleware registered")

		interval, _ := time.ParseDuration(bootstrap.BudgetAggregationInterval(cfg))
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go budgetapp.NewAggregationWorker(budgetRepo, interval, logger).Run(budgetCtx)
		logger.Info("budget aggregation worker started", slog.Duration("interval", interval))
	} else {
		logger.Debug("budget enforcement disabled — no budgets config, no db pool, or no valkey")
	}

	// @sk-task semantic-cache-masked#T2.2: Register cache middleware in combined binary (AC-001, AC-007)
	// @sk-task semantic-cache-masked#T3.2: Attach budget-aware write guard (AC-008)
	if cfg.Data != nil && cfg.Data.Cache != nil && cfg.Data.Cache.Enabled {
		svc := bootstrap.NewSemanticCacheService(cfg.Data.Cache, vkClient, logger)
		if svc != nil {
			guard := bootstrap.NewBudgetWriteGuard(bootstrap.NewBudgetRepo(pgPool), bootstrap.NewBudgetCounter(vkClient), cfg.Data.Cache.BudgetGuardPercent)
			svc.WithGuard(guard)
			srv.RegisterCacheMiddleware(middleware.NewSemanticCacheMiddleware(svc, cfg.Data.Cache, logger).Handler())
			logger.Info("semantic cache middleware registered")
		}
	}

	// @sk-task log-export#T2.5: wire the log export pipeline (AC-001)
	if cfg.LogExport != nil && cfg.LogExport.Enabled {
		sinks := bootstrap.BuildExportSinks(context.Background(), cfg.LogExport, logger)
		pipeline := logexportapp.NewPipeline(sinks, domainlogexport.NewResolver(cfg.LogExport.Tenants),
			cfg.LogExport.BatchSize, cfg.LogExport.QueueSize, cfg.LogExport.Timeout, logger)
		go pipeline.Run(context.Background())
		exportRates := newCostRateRegistry(cfg, bootstrap.LoadCostRatesFromDB(context.Background(), cfg, pgPool, logger))
		srv.RegisterExportMiddleware(middleware.ExportMiddleware(pipeline, exportRates, logger))
		logger.Info("log export pipeline started", slog.Int("sinks", len(sinks)))
	}

	// @sk-task embeddings-passthrough#T2.2: wire the embeddings input shield (AC-001)
	srv.RegisterEmbeddingsShield(middleware.EmbeddingsShieldMiddleware(shieldEngine, cfg.Shield, logger))
	srv.RegisterProxyRoute(middleware.ShieldMiddleware(shieldEngine, cfg.Shield, logger, sessionUseCase), routingHandler)
	logger.Info("gateway routes registered")

	return srv
}

// @sk-task usage-accounting-integrity#T2.3: build the registry with the configured fallback rate (AC-006)
func newCostRateRegistry(cfg *config.Config, rates []*analytics.CostRate) *analytics.CostRateRegistry {
	var fallback *analytics.CostRate
	if cfg != nil && cfg.Analytics != nil && cfg.Analytics.DefaultCostRate != nil {
		d := cfg.Analytics.DefaultCostRate
		model := d.Model
		if model == "" {
			model = "default"
		}
		if fb, err := analytics.NewCostRateWithCurrency(model, d.InputPricePer1K, d.OutputPricePer1K, analytics.DefaultCurrency); err == nil {
			fallback = fb
		}
	}
	return analytics.NewCostRateRegistryWithFallback(rates, fallback)
}

// @sk-task combined-binary: Init detectors with CompositeDetector
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
