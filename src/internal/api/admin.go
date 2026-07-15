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
	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
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
	s.engine.Use(mw)
}

func (s *AdminServer) RegisterMetricsRoute(handler gin.HandlerFunc) {
	s.engine.GET("/metrics", handler)
}

func (s *AdminServer) RegisterIncidentHandler(h *incident.Handler) {
	group := s.engine.Group("/api/v1/incidents")
	group.GET("/export", h.ExportIncidents)
	group.GET("", h.ListIncidents)
	group.GET("/:id", h.GetIncident)
}

func (s *AdminServer) RegisterTenantHandler(h *admin.TenantHandler) {
	group := s.engine.Group("/api/v1/tenants")
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

// @sk-task 118-api-consistency#T3.5: NoRoute checks Accept:text/html for SPA fallback (AC-009)
// @sk-task 118-api-consistency#T3.5: NoRoute checks Accept:text/html for SPA fallback (AC-009)
func (s *AdminServer) RegisterStaticFiles(fsys fs.FS) {
	sub, err := fs.Sub(fsys, "dist")
	if err != nil {
		s.log.Fatal("failed to create static sub-filesystem", zap.Error(err))
	}
	root := http.FS(sub)
	fileServer := http.FileServer(root)
	s.engine.NoRoute(func(c *gin.Context) {
		accept := c.GetHeader("Accept")
		apiPath := strings.HasPrefix(c.Request.URL.Path, "/api/")
		if !apiPath && (strings.Contains(accept, "text/html") || accept == "") {
			spaPath := strings.TrimPrefix(c.Request.URL.Path, "/")
			f, err := root.Open(spaPath)
			if err != nil {
				c.Request.URL.Path = "/"
			} else {
				f.Close()
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.JSON(http.StatusNotFound, dto.NewErrorResponse("NOT_FOUND", "route not found"))
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
