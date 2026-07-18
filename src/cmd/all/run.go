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
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/api"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task combined-binary: Combined binary run — starts gateway + admin servers
func run() {
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

	// @sk-task config-hot-reload#T3.1: Start config watcher for runtime hot-reload in combined binary (AC-001)
	if cfgDir := config.ConfigDirFromArgs(); cfgDir != "" {
		reloadCtx, reloadCancel := context.WithCancel(context.Background())
		defer reloadCancel()
		config.WatchConfigDir(reloadCtx, cfgDir, func(old, new *config.Config) {
			changed := config.DiffSections(old, new)
			if changed["routing"] {
				if updateErr := registry.UpdateConfig(toDomainRoutingConfig(new.Routing)); updateErr != nil {
					logger.Error("config reload: routing registry update failed", zap.Error(updateErr))
					return
				}
				newClients := make(map[string]ports.ProviderClient)
				if new.Egress != nil && new.Routing != nil {
					for i := range new.Routing.Providers {
						pcfg := &new.Routing.Providers[i]
						client, clientErr := provider.NewProviderClient(pcfg, new.Egress)
						if clientErr != nil {
							logger.Error("config reload: failed to create provider client", zap.String("provider", pcfg.Name), zap.Error(clientErr))
							continue
						}
						newClients[pcfg.Name] = client
					}
				}
				fallbackHandler.UpdateClients(newClients)
				logger.Info("config reloaded: routing changed")
			}
			// @sk-task config-hot-reload#T3.3: Apply shield config changes through shared pointer (AC-006)
			if changed["shield"] && cfg.Shield != nil && new.Shield != nil {
				*cfg.Shield = *new.Shield
				logger.Info("config reloaded: shield changed")
			}
			// @sk-task config-hot-reload#T3.4: Apply ratelimit config changes through shared pointer (AC-006)
			if changed["ratelimit"] && cfg.RateLimit != nil && new.RateLimit != nil {
				*cfg.RateLimit = *new.RateLimit
				logger.Info("config reloaded: ratelimit changed")
			}
			// @sk-task config-hot-reload#T3.4: Apply debug config changes through shared pointer (AC-006)
			if changed["debug"] && cfg.Debug != nil && new.Debug != nil {
				*cfg.Debug = *new.Debug
				logger.Info("config reloaded: debug changed")
			}
		})
	}

	healthCtx, healthCancel := context.WithCancel(context.Background())
	defer healthCancel()
	if cfg.Routing != nil {
		healthChecker := routingSvc.NewHealthChecker(registry, nil)
		go healthChecker.Start(healthCtx, 30*time.Second)
		logger.Info("provider health checker started")
	}

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

	gwServer := buildGatewayServer(cfg, logger, serviceName, pgPool, vkClient, gwPromRegistry, gwMetricsHandler, registry, selector, clients, fallbackHandler, routingHandler, otelShutdown)
	adminServer := buildAdminServer(cfg, logger, serviceName, pgPool, vkClient, adminPromRegistry, adminMetricsHandler, otelShutdown)

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
