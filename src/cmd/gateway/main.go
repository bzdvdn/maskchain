package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/api"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/reaction"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
	"github.com/bzdvdn/maskchain/src/internal/infra/logging"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
	"github.com/bzdvdn/maskchain/src/internal/infra/telemetry"
	"github.com/bzdvdn/maskchain/ui"
)

// @sk-task 30-shield-persistence#T2.4: Wire pool, migrations, and new repos in main
// @sk-task 61-observability#T2.2: Wire OTel telemetry, metrics, and logging (AC-001, AC-002, AC-003, AC-005, AC-006)
// @sk-task 71-egress-streaming#T5.1: Wire egress client in main for all configured providers (AC-001, AC-002, AC-004, AC-005)
func main() {
	cfg := config.MustLoadConfig()

	logger, err := buildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Debug("config loaded", zap.Any("config", cfg))

	registry, err := routingDomain.NewProviderRegistry(cfg.Routing)
	if err != nil {
		logger.Fatal("failed to create provider registry", zap.Error(err))
	}
	selector := routingDomain.NewRouteSelector(registry)
	clients := make(map[string]ports.ProviderClient)
	if cfg.Egress != nil {
		egClient := egress.NewClient(cfg.Egress)
		for _, p := range registry.List() {
			clients[p.Name] = egClient
		}
	}
	fallbackHandler := routingDomain.NewFallbackHandler(clients)
	routingHandler := api.NewRoutingProxyHandler(selector, fallbackHandler)

	slogLogger := logging.NewLogger(io.Discard, slog.LevelInfo)

	serviceName := "maskchain-gateway"
	if cfg.OTel != nil && cfg.OTel.ServiceName != "" {
		serviceName = cfg.OTel.ServiceName
	}

	otelShutdown := noopShutdown
	if cfg.OTel != nil {
		shutdown, err := telemetry.InitProvider(
			context.Background(),
			cfg.OTel.Endpoint,
			serviceName,
			cfg.OTel.Environment,
			cfg.OTel.SamplingRatio,
			slogLogger,
		)
		if err != nil {
			slogLogger.Warn("telemetry init", "error", err)
		}
		otelShutdown = shutdown
	}

	promRegistry := prometheus.NewRegistry()
	metrics.RegisterMetrics(promRegistry)
	metricsHandler := metrics.Handler(promRegistry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgPool, err := initPG(ctx, cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to init postgres", zap.Error(err))
	}

	if pgPool != nil {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	vkClient, err := initValkey(cfg.Valkey, logger)
	if err != nil {
		logger.Fatal("failed to init valkey", zap.Error(err))
	}

	detectorRegistry := initDetectors(logger)

	maskTTL := time.Duration(cfg.Mask.CacheTTLSec) * time.Second
	pgRepo := maskrepo.NewPostgresMaskRepo(pgPool)
	vkRepo := maskrepo.NewValkeyMaskRepo(vkClient, maskTTL)
	maskStorage := maskrepo.NewCachedMaskRepo(pgRepo, vkRepo)
	maskUseCase := domainMask.NewMaskUseCase(detectorRegistry, maskStorage)
	maskHandler := api.NewMaskHandler(maskUseCase, detectorRegistry)

	srv := api.New(cfg.Server, logger, serviceName)
	srv.RegisterMetricsRoute(metricsHandler)
	srv.RegisterMaskHandler(maskHandler)
	srv.RegisterStaticFiles(ui.DistFiles)

	if pgPool != nil {
		dictRepo := postgres.NewPostgresDictionaryRepo(pgPool)
		txMgr := postgres.NewPGXTransactionManager(pgPool)
		profileRepo := postgres.NewPostgresProfileRepo(pgPool, dictRepo, txMgr)

		pipelineFactory := appshield.NewScanPipelineFactory(detectorRegistry)
		policyEval := service.NewPolicyEvaluator()
		blockExec := reaction.NewBlockReaction()
		redactExec := reaction.NewRedactReaction()
		alertExec := reaction.NewAlertReaction(nil)
		reactionPipeline := reaction.NewDefaultReactionPipeline(blockExec, redactExec, alertExec)

		tenantID, err := value.NewTenantID("default")
		if err != nil {
			logger.Fatal("failed to create tenant ID", zap.Error(err))
		}

		scanUseCase := appshield.NewScanUseCase(profileRepo, pipelineFactory, policyEval, reactionPipeline, tenantID)
		shieldEngine := appshield.NewShieldEngine(scanUseCase)
		shieldMw := middleware.ShieldMiddleware(shieldEngine, profileRepo, cfg.Shield, logger)
		srv.RegisterProxyRoute(shieldMw, routingHandler)
		logger.Info("proxy routes with shield registered")

		// @sk-task 60-audit-incidents#T2.3: Wire incident handler (AC-001, AC-002)
		incidentRepo := postgres.NewPostgresIncidentRepo(pgPool)
		incidentHandler := incident.New(incidentRepo)
		srv.RegisterIncidentHandler(incidentHandler)
		logger.Info("incident handler registered")
	} else {
		logger.Warn("no database configured, shield middleware disabled")
		noopMw := func(c *gin.Context) { c.Next() }
		srv.RegisterProxyRoute(noopMw, routingHandler)
		logger.Info("proxy routes without shield registered")
	}

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := otelShutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("server stopped")
}

func noopShutdown(_ context.Context) error { return nil }

// @sk-task 22-shield-mask-storage#T5.2: Init PG with nil-safe DSN (AC-012)
// @sk-task 30-shield-persistence#T2.4: Use NewPool from postgres package (AC-005)
func initPG(ctx context.Context, dbCfg *config.DatabaseConfig, log *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("init pool: %w", err)
	}
	if pool == nil {
		log.Warn("no database configured, persistence disabled")
	}
	return pool, nil
}

// @sk-task 22-shield-mask-storage#T5.2: Init Valkey with nil-safe addr (AC-012)
func initValkey(vkCfg *config.ValkeyConfig, log *zap.Logger) (valkey.Client, error) {
	if vkCfg == nil || vkCfg.Addr == "" {
		log.Warn("no valkey configured, mask cache disabled")
		return nil, nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkCfg.Addr},
		Password:    vkCfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("valkey client: %w", err)
	}
	return client, nil
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

func buildLogger(level string) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return zapCfg.Build()
}
