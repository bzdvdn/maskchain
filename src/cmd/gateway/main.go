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

	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/resolver"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
)

// @sk-task 30-shield-persistence#T2.4: Wire pool, migrations, and new repos in main
// @sk-task 61-observability#T2.2: Wire OTel telemetry, metrics, and logging (AC-001, AC-002, AC-003, AC-005, AC-006)
// @sk-task 71-egress-streaming#T5.1: Wire egress client in main for all configured providers (AC-001, AC-002, AC-004, AC-005)
// @sk-task 90-production-hardening#T2.3: Log pool params at startup (<AC-002>)
// @sk-task 90-production-hardening#T2.2: Wire debug routes with admin auth (<AC-001>)
// @sk-task 100-admin-control-plane#T2.3: Remove ui import from gateway, move to admin (AC-001, AC-005)
// @sk-task 100-admin-control-plane#T3.2: Final cleanup — no ui references in gateway (AC-001)
// @sk-task tenant-profile-sync#T3.1: Remove profileRepo from ShieldMiddleware call (AC-006, AC-007)
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

	serviceName := "maskchain-gateway"
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

	promRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(promRegistry)
	metricsHandler := metrics.Handler(promRegistry)

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
		// @sk-task 90-production-hardening#T3.2: Register PG pool metrics collector (<AC-003>)
		metrics.RegisterPGPoolCollector(promRegistry, pgPool)
		logger.Info("PG pool metrics collector registered")
	}

	vkClient, err := bootstrap.InitValkey(cfg.Valkey, logger)
	if err != nil {
		logger.Fatal("failed to init valkey", zap.Error(err))
	}

	// @sk-task 115-rate-limit-wiring#T2.2: Initialize rate limit repos (AC-001, AC-002, AC-004, AC-005, AC-006, AC-007, AC-008)
	// @sk-task 115-rate-limit-wiring#T3.1: Warn when rate limit configured but Valkey unavailable (AC-006)
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

	detectorRegistry := initDetectors(logger)

	maskTTL := time.Duration(cfg.Mask.CacheTTLSec) * time.Second
	pgRepo := maskrepo.NewPostgresMaskRepo(pgPool)
	vkRepo := maskrepo.NewValkeyMaskRepo(vkClient, maskTTL)
	maskStorage := maskrepo.NewCachedMaskRepo(pgRepo, vkRepo)
	maskUseCase := domainMask.NewMaskUseCase(detectorRegistry, maskStorage)
	maskHandler := api.NewMaskHandler(maskUseCase, detectorRegistry)

	// @sk-task 114-real-health-probes#T2.3: Create health service and wire into server (AC-001, AC-005, AC-008)
	if cfg.Server.HealthCheck == nil {
		cfg.Server.HealthCheck = &config.HealthCheckConfig{CriticalDeps: []string{"database"}}
	}
	healthSvc := health.NewService(cfg.Server.HealthCheck.CriticalDeps)

	// @sk-task 114-real-health-probes#T3.2: Register PG, Valkey, and Egress probes for gateway (AC-002, AC-003, AC-004)
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

		authMw := middleware.Auth(dbTenants)
		srv.RegisterAuth(authMw)
		logger.Info("auth middleware registered", zap.Int("tenants", len(dbTenants)))
	} else {
		logger.Warn("no tenants configured, auth disabled")
	}

	// @sk-task 115-rate-limit-wiring#T2.2: Register rate limit middleware (AC-001, AC-002, AC-004, AC-005, AC-006, AC-007, AC-008)
	if cfg.RateLimit != nil && rlRepo != nil {
		rateLimitMw := middleware.RateLimit(rlRepo, cfg.RateLimit, tbRepo)
		srv.RegisterRateLimit(rateLimitMw)
		logger.Info("rate limit middleware registered")
	}

	adminMw := middleware.AdminAuth(cfg.Debug)
	srv.RegisterDebugRoutes(adminMw)

	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterMaskHandler(maskHandler)

	// @sk-task 60-audit-incidents#T2.3: Wire incident handler (AC-001, AC-002)
	if pgPool != nil {
		incidentRepo := postgres.NewPostgresIncidentRepo(pgPool)
		incidentHandler := incident.New(incidentRepo)
		srv.RegisterIncidentHandler(incidentHandler)
		logger.Info("incident handler registered")
	}

	// @sk-task 13-shield-middleware-wiring#T2.3: Simplified — no ProfileRepository DI (AC-005)
	pipelineFactory := appshield.NewScanPipelineFactory(detectorRegistry)
	scanUseCase := appshield.NewScanUseCase(pipelineFactory)
	shieldEngine := appshield.NewShieldEngine(scanUseCase)
	logger.Info("shield engine initialized")

	srv.RegisterProxyRoute(middleware.ShieldMiddleware(shieldEngine, cfg.Shield, logger), routingHandler)
	logger.Info("proxy routes registered")

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
	logger.Info("server stopped")
}

// @sk-task 22-shield-mask-storage#T5.2: Init detectors with CompositeDetector (AC-011)
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

	// @sk-task 24-shield-dictionaries#T5.1: Register dictionary detector type (AC-007)
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
