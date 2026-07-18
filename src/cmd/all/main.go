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
	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	sessionrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/session"
	"github.com/bzdvdn/maskchain/src/internal/api"
	adminhandler "github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	analyticshandler "github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	analyticsapp "github.com/bzdvdn/maskchain/src/internal/app/analytics"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/app/worker"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/bzdvdn/maskchain/ui"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
)

// @sk-task combined-binary: Combined binary with admin + gateway in one process
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

	logger.Debug("config loaded", zap.Object("config", cfg))

	if cfg.DB != nil {
		logger.Info("database pool config",
			zap.Int("max_open_conns", cfg.DB.MaxConns),
			zap.Int("min_idle_conns", cfg.DB.MinConns),
			zap.Duration("conn_max_lifetime", cfg.DB.MaxConnLifetime),
		)
	}
	if cfg.Egress != nil {
		logger.Info("http pool config",
			zap.Int("max_idle_conns", cfg.Egress.MaxIdleConns),
			zap.Int("max_idle_conns_per_host", cfg.Egress.MaxIdleConnsPerHost),
			zap.Duration("idle_timeout", cfg.Egress.IdleTimeout),
			zap.Bool("disable_keep_alives", cfg.Egress.DisableKeepAlives),
		)
	}

	// --- gateway wiring ---

	registry, err := routingSvc.NewProviderRegistry(toDomainRoutingConfig(cfg.Routing))
	if err != nil {
		logger.Fatal("failed to create provider registry", zap.Error(err))
	}
	selector := routingSvc.NewRouteSelector(registry)
	clients := make(map[string]ports.ProviderClient)
	if cfg.Egress != nil && cfg.Routing != nil {
		for i := range cfg.Routing.Providers {
			pcfg := &cfg.Routing.Providers[i]
			client, err := provider.NewProviderClient(pcfg, cfg.Egress)
			if err != nil {
				logger.Fatal("failed to create provider client", zap.String("provider", pcfg.Name), zap.Error(err))
			}
			clients[pcfg.Name] = client
			logger.Info("provider client created",
				zap.String("provider", pcfg.Name),
				zap.String("api_type", pcfg.APIType),
				zap.String("base_url", pcfg.BaseURL),
			)
		}
	}
	fallbackHandler := routingSvc.NewFallbackHandler(clients)
	routingHandler := api.NewRoutingProxyHandler(selector, fallbackHandler)

	healthCtx, healthCancel := context.WithCancel(context.Background())
	defer healthCancel()
	if cfg.Routing != nil {
		healthChecker := routingSvc.NewHealthChecker(registry, nil)
		go healthChecker.Start(healthCtx, 30*time.Second)
		logger.Info("provider health checker started")
	}

	// --- shared init ---

	serviceName := "maskchain"
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		serviceName = cfg.OTel.ServiceName
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

	gwPromRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(gwPromRegistry)
	gwMetricsHandler := metrics.Handler(gwPromRegistry)

	adminPromRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(adminPromRegistry)
	adminMetricsHandler := metrics.Handler(adminPromRegistry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgPool, err := bootstrap.InitPG(ctx, cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to init postgres", zap.Error(err))
	}

	if pgPool != nil {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	vkClient, err := bootstrap.InitValkey(cfg.Valkey, logger)
	if err != nil {
		logger.Fatal("failed to init valkey", zap.Error(err))
	}

	// --- gateway server ---

	gwServer := buildGatewayServer(cfg, logger, serviceName, pgPool, vkClient, gwPromRegistry, gwMetricsHandler, registry, selector, clients, fallbackHandler, routingHandler, otelShutdown)

	// --- admin server ---

	adminServer := buildAdminServer(cfg, logger, serviceName, pgPool, vkClient, adminPromRegistry, adminMetricsHandler, otelShutdown)

	// --- start both ---

	errCh := make(chan error, 2)

	go func() {
		logger.Info("starting combined gateway server", zap.Int("port", cfg.Server.Port))
		if err := gwServer.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("gateway: %w", err)
		}
	}()

	go func() {
		logger.Info("starting combined admin server", zap.Int("port", cfg.Server.AdminPort))
		if err := adminServer.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("admin: %w", err)
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

	if err := gwServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("gateway shutdown error", zap.Error(err))
	}
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin shutdown error", zap.Error(err))
	}

	logger.Info("combined server stopped")
}

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
	// shared resources for gateway
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

// @sk-task combined-binary: Init detectors
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

	return registry
}

func toDomainRoutingConfig(cfg *config.RoutingConfig) *routingDomain.RoutingConfig {
	if cfg == nil {
		return nil
	}
	domainCfg := &routingDomain.RoutingConfig{
		Providers: make([]routingDomain.ProviderConfig, len(cfg.Providers)),
		Rules:     make([]routingDomain.RuleConfig, 0, len(cfg.Rules)),
	}
	for i, p := range cfg.Providers {
		domainCfg.Providers[i] = routingDomain.ProviderConfig{
			Name:           p.Name,
			BaseURL:        p.BaseURL,
			HealthEndpoint: p.HealthEndpoint,
			Timeout:        p.Timeout,
			Priority:       p.Priority,
		}
	}
	for _, r := range cfg.Rules {
		routes := make([]routingDomain.RouteConfig, len(r.Routes))
		for j, rt := range r.Routes {
			routes[j] = routingDomain.RouteConfig{
				Model:     rt.Model,
				Providers: rt.Providers,
			}
		}
		domainCfg.Rules = append(domainCfg.Rules, routingDomain.RuleConfig{
			Tenant: r.Tenant,
			Routes: routes,
		})
	}
	return domainCfg
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
