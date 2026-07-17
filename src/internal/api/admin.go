package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/docs"
	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/admin"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/analytics"
	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task 100-admin-control-plane#T2.1: Admin server with static files, profile/incident handlers (AC-004, AC-007, AC-010)
type AdminServer struct {
	engine         *gin.Engine
	http           *http.Server
	cfg            *config.ServerConfig
	log            *zap.Logger
	serviceName    string
	metricsHandler gin.HandlerFunc
	healthHandler  *health.Handler
	authMw         gin.HandlerFunc
	adminSessionMw gin.HandlerFunc
}

// @sk-task 114-real-health-probes#T2.2: Accept healthSvc and replace static handlers (AC-001, AC-005, AC-008)
func NewAdminServer(cfg *config.ServerConfig, log *zap.Logger, serviceName string, healthSvc *health.HealthService) *AdminServer {
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

	h := health.NewHandler(healthSvc)
	engine.GET("/health", h.LivenessHandler())
	engine.GET("/ready", h.ReadinessHandler())
	engine.GET("/live", h.StartupHandler())

	return &AdminServer{
		engine:        engine,
		cfg:           cfg,
		log:           log,
		serviceName:   serviceName,
		healthHandler: h,
	}
}

func (s *AdminServer) RegisterAuth(mw gin.HandlerFunc) {
	s.authMw = mw
}

// @sk-task admin-ui-design#T2.3: Register admin session middleware for API protection (AC-001)
func (s *AdminServer) RegisterAdminSessionMiddleware(mw gin.HandlerFunc) {
	s.adminSessionMw = mw
}

// @sk-task admin-ui-design#T2.3: Register admin auth login/logout/verify routes (AC-001)
func (s *AdminServer) RegisterAdminAuthRoutes(h *admin.AdminAuthHandler) {
	group := s.engine.Group("/api/v1/admin")
	group.POST("/login", h.HandleLogin)
	group.POST("/logout", h.HandleLogout)
	verify := group.Group("")
	verify.Use(s.adminSessionMw)
	verify.GET("/verify", h.HandleVerify)
}

func (s *AdminServer) RegisterMetricsRoute(handler gin.HandlerFunc) {
	s.engine.GET("/metrics", handler)
}

// @sk-task remove-audit-incidents#T3.3: RegisterIncidentHandler removed from AdminServer (AC-009)
// @sk-task seed-tenant-fix#T1.1: Accept middleware parameter for combined auth (AC-001)
func (s *AdminServer) RegisterTenantHandler(h *admin.TenantHandler, mw gin.HandlerFunc) {
	group := s.engine.Group("/api/v1/tenants")
	if mw != nil {
		group.Use(mw)
	} else if s.adminSessionMw != nil {
		group.Use(s.adminSessionMw)
	}
	group.POST("", h.CreateTenant)
	group.GET("", h.ListTenants)
	group.GET("/:slug", h.GetTenant)
	group.PUT("/:slug", h.UpdateTenant)
	group.DELETE("/:slug", h.DeleteTenant)
	group.GET("/:slug/dictionaries", h.GetDictionaries)
	group.PUT("/:slug/dictionaries", h.UpdateDictionaries)
}

// @sk-task 118-api-consistency#T3.4: Register Swagger UI at /api/v1/docs (AC-008, RQ-010)
func (s *AdminServer) RegisterSwaggerUI() {
	yamlData, err := docs.DocsFiles.ReadFile("openapi.yaml")
	if err != nil {
		s.log.Fatal("failed to read embedded openapi.yaml", zap.Error(err))
	}
	s.engine.GET("/api/v1/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/x-yaml", yamlData)
	})

	swaggerFS := http.FS(docs.DocsFiles)
	s.engine.GET("/api/v1/docs", func(c *gin.Context) {
		c.Request.URL.Path = "/swagger-ui/index.html"
		http.FileServer(swaggerFS).ServeHTTP(c.Writer, c.Request)
	})
}

// @sk-task admin-ui-design#T2.3: Register session routes with admin session middleware (AC-001)
func (s *AdminServer) RegisterSessionHandler(h *SessionHandler) {
	group := s.engine.Group("/api/v1/sessions")
	if s.adminSessionMw != nil {
		group.Use(s.adminSessionMw)
	}
	group.POST("", h.HandleCreate)
	group.GET("", h.HandleList)
	group.GET("/:id", h.HandleGet)
	group.PATCH("/:id/extend", h.HandleExtend)
	group.DELETE("/:id", h.HandleClose)
}

// @sk-task admin-ui-design#T3.2: Register audit log route (AC-005)
func (s *AdminServer) RegisterAuditHandler(h *admin.AuditHandler) {
	group := s.engine.Group("/api/v1/audit")
	if s.adminSessionMw != nil {
		group.Use(s.adminSessionMw)
	}
	group.GET("", h.HandleList)
	group.GET("/", h.HandleList)
}

// @sk-task admin-ui-design#T3.1: Register routing data endpoint (AC-006)
func (s *AdminServer) RegisterRoutingHandler(h *admin.RoutingHandler) {
	group := s.engine.Group("/api/v1/routing")
	if s.adminSessionMw != nil {
		group.Use(s.adminSessionMw)
	}
	group.GET("", h.HandleRouting)
	group.GET("/", h.HandleRouting)
}

// @sk-task 118-api-consistency#T3.5: NoRoute checks Accept:text/html for SPA fallback (AC-009)
// @sk-task 132-analytics-api#T2.2: Register analytics routes (AC-001, AC-002, AC-003, AC-004)
func (s *AdminServer) RegisterAnalyticsHandler(h *analytics.AnalyticsHandler, debugCfg *config.DebugConfig) {
	group := s.engine.Group("/api/v1/analytics")
	if s.adminSessionMw != nil {
		group.Use(s.adminSessionMw)
	}
	group.GET("/tokens", h.HandleTokens)
	group.GET("/cost", h.HandleCost)
	group.GET("/traffic", h.HandleTraffic)
	summary := group.Group("/tenants/:slug/summary")
	summary.Use(middleware.AdminAuth(debugCfg))
	summary.GET("", h.HandleTenantSummary)
}

// @sk-task 118-api-consistency#T3.5: NoRoute checks Accept:text/html for SPA fallback (AC-009)
func (s *AdminServer) RegisterStaticFiles(fsys fs.FS) {
	sub, err := fs.Sub(fsys, "dist")
	if err != nil {
		s.log.Fatal("failed to create static sub-filesystem", zap.Error(err))
	}
	root := http.FS(sub)
	fileServer := http.FileServer(root)
	s.engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, dto.NewErrorResponse("NOT_FOUND", "route not found"))
			return
		}

		spaPath := strings.TrimPrefix(path, "/")
		f, err := root.Open(spaPath)
		if err != nil {
			c.Request.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

func (s *AdminServer) RegisterDebugRoutes(adminMw gin.HandlerFunc) {
	group := s.engine.Group("/debug/pprof", adminMw)
	group.GET("", gin.WrapH(http.HandlerFunc(pprof.Index)))
	group.GET("/", gin.WrapH(http.HandlerFunc(pprof.Index)))
	group.GET("/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
	group.GET("/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	group.GET("/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	group.GET("/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))
}

func (s *AdminServer) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.http = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	s.log.Info("admin server starting", zap.String("addr", addr))
	return s.http.ListenAndServe()
}

func (s *AdminServer) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down admin server")
	return s.http.Shutdown(ctx)
}
