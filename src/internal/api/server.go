package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task 10-gateway-skeleton#T2.1: Implement Server struct with New/Start/Shutdown (AC-001, AC-002, AC-003, AC-005)
// @sk-task 61-observability#T2.1: Add OTel and metrics middleware (AC-001, AC-002, AC-003)
// @sk-task 117-critical-test-coverage#T2.1: Export http field for test access (AC-001)
type Server struct {
	engine            *gin.Engine
	HTTP              *http.Server
	cfg               *config.ServerConfig
	log               *zap.Logger
	serviceName       string
	metricsHandler    gin.HandlerFunc
	healthHandler     *health.Handler
	sessionMiddleware gin.HandlerFunc
	usageMiddleware   gin.HandlerFunc
}

// @sk-task 114-real-health-probes#T2.2: Accept healthSvc and replace static handlers (AC-001, AC-005, AC-008)
func New(cfg *config.ServerConfig, log *zap.Logger, serviceName string, healthSvc *health.HealthService) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	engine.Use(otelgin.Middleware(serviceName, otelgin.WithFilter(func(req *http.Request) bool {
		return req.URL.Path != "/metrics"
	})))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(log))
	engine.Use(middleware.Recovery(log))
	engine.Use(middleware.CORS(cfg.CORSOrigins))
	engine.Use(middleware.ResponseEnvelope())
	engine.Use(middleware.ErrorHandler())
	engine.Use(metrics.Middleware())

	engine.NoRoute(func(c *gin.Context) {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "route not found")
	})

	h := health.NewHandler(healthSvc)
	engine.GET("/health", h.LivenessHandler())
	engine.GET("/ready", h.ReadinessHandler())
	engine.GET("/live", h.StartupHandler())

	return &Server{
		engine:        engine,
		cfg:           cfg,
		log:           log,
		serviceName:   serviceName,
		healthHandler: h,
	}
}

// @sk-task 61-observability#T2.1: Register metrics route (AC-002)
func (s *Server) RegisterMetricsRoute(handler gin.HandlerFunc) {
	s.engine.GET("/metrics", handler)
}

// @sk-task 22-shield-mask-storage#T4.2: Register mask routes (AC-002, AC-003)
func (s *Server) RegisterMaskHandler(h *MaskHandler) {
	s.engine.POST("/api/v1/shield/mask", h.HandleMask)
	s.engine.POST("/api/v1/shield/unmask", h.HandleUnmask)
}

// @sk-task 80-tenant-isolation#T2.2: Register auth middleware (AC-002, AC-004)
func (s *Server) RegisterAuth(mw gin.HandlerFunc) {
	s.engine.Use(mw)
}

// @sk-task rate-limiting-budgets#T2.2: Register rate limit middleware (AC-001, AC-004)
func (s *Server) RegisterRateLimit(mw gin.HandlerFunc) {
	s.engine.Use(mw)
}

// @sk-task remove-audit-incidents#T3.3: RegisterIncidentHandler removed (AC-009)
// @sk-task 70-routing-engine#T3.2: Register proxy routes with routing handler (AC-003, AC-004)
// @sk-task 112-proxy-streaming-wiring#T2.2: Register WrapSSE middleware on streaming route (AC-002)
// @sk-task 118-api-consistency#T2.2: Add /api/v1/ prefix and 301 redirect from /v1/ (AC-001, AC-002)
func (s *Server) RegisterProxyRoute(shieldMiddleware gin.HandlerFunc, routingHandler *RoutingProxyHandler) {
	primary := s.engine.Group("/api/v1")
	if routingHandler != nil {
		chain := []gin.HandlerFunc{middleware.WrapSSE()}
		if s.sessionMiddleware != nil {
			chain = append(chain, s.sessionMiddleware)
		}
		chain = append(chain, shieldMiddleware)
		if s.usageMiddleware != nil {
			chain = append(chain, s.usageMiddleware)
		}
		chain = append(chain, routingHandler.HandleChatCompletion)
		primary.POST("/chat/completions", chain...)
	} else {
		chain := []gin.HandlerFunc{}
		if s.sessionMiddleware != nil {
			chain = append(chain, s.sessionMiddleware)
		}
		chain = append(chain, shieldMiddleware)
		if s.usageMiddleware != nil {
			chain = append(chain, s.usageMiddleware)
		}
		chain = append(chain, ProxyChatCompletionHandler)
		primary.POST("/chat/completions", chain...)
	}
	primary.POST("/completions", s.withSessionMiddleware(shieldMiddleware), ProxyCompletionHandler)

	// @sk-task 118-api-consistency#T2.2: Deprecated /v1/ paths with 301 redirect (AC-002)
	redirect := s.engine.Group("/v1")
	redirect.Any("/chat/completions", redirectPermanent("/api/v1/chat/completions"))
	redirect.Any("/completions", redirectPermanent("/api/v1/completions"))
}

func redirectPermanent(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, target)
	}
}

// @sk-task sessions#T4.1: RegisterSessionMiddleware on gateway Server (AC-010)
func (s *Server) RegisterSessionMiddleware(mw gin.HandlerFunc) {
	s.sessionMiddleware = mw
}

// @sk-task 131-analytics-pipeline#T3.3: RegisterUsageMiddleware on gateway Server (AC-006)
func (s *Server) RegisterUsageMiddleware(mw gin.HandlerFunc) {
	s.usageMiddleware = mw
}

func (s *Server) withSessionMiddleware(next gin.HandlerFunc) gin.HandlerFunc {
	if s.sessionMiddleware == nil {
		return next
	}
	return func(c *gin.Context) {
		s.sessionMiddleware(c)
		if !c.IsAborted() {
			next(c)
		}
	}
}

// @sk-task 101-gateway-diet#T1.1: Remove RegisterStaticFiles from Server (AC-001, AC-005)
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.HTTP = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	s.log.Info("server starting", zap.String("addr", addr))
	return s.HTTP.ListenAndServe()
}

// @sk-task 90-production-hardening#T2.2: Register pprof routes behind admin auth (<AC-001>)
func (s *Server) RegisterDebugRoutes(adminMw gin.HandlerFunc) {
	group := s.engine.Group("/debug/pprof", adminMw)
	group.GET("", gin.WrapH(http.HandlerFunc(pprof.Index)))
	group.GET("/", gin.WrapH(http.HandlerFunc(pprof.Index)))
	group.GET("/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
	group.GET("/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	group.GET("/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	group.GET("/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))
}

// @sk-task 90-production-hardening#T2.4: Graceful shutdown with config timeout (<SC-003>)
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down server")
	return s.HTTP.Shutdown(ctx)
}
