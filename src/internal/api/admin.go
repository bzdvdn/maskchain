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

	"github.com/bzdvdn/maskchain/src/internal/api/handler/incident"
	"github.com/bzdvdn/maskchain/src/internal/api/handler/profile"
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
}

func NewAdminServer(cfg *config.ServerConfig, log *zap.Logger, serviceName string) *AdminServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	engine.Use(otelgin.Middleware(serviceName, otelgin.WithFilter(func(req *http.Request) bool {
		return req.URL.Path != "/metrics"
	})))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(log))
	engine.Use(middleware.Recovery(log))
	engine.Use(middleware.CORS(cfg.CORSOrigins))
	engine.Use(middleware.ErrorHandler())
	engine.Use(metrics.Middleware())

	engine.GET("/health", healthHandler("ok"))
	engine.GET("/ready", healthHandler("ok"))
	engine.GET("/live", healthHandler("alive"))

	return &AdminServer{
		engine:      engine,
		cfg:         cfg,
		log:         log,
		serviceName: serviceName,
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

func (s *AdminServer) RegisterProfileHandler(h *profile.ProfileHandler) {
	group := s.engine.Group("/api/v1/profiles")
	group.POST("", h.CreateProfile)
	group.GET("", h.ListProfiles)
	group.GET("/:slug", h.GetProfile)
	group.PUT("/:slug", h.UpdateProfile)
	group.DELETE("/:slug", h.DeleteProfile)
	group.PATCH("/:slug/dictionary", h.PatchDictionary)
}

func (s *AdminServer) RegisterStaticFiles(fsys fs.FS) {
	sub, err := fs.Sub(fsys, "dist")
	if err != nil {
		s.log.Fatal("failed to create static sub-filesystem", zap.Error(err))
	}
	root := http.FS(sub)
	fileServer := http.FileServer(root)
	s.engine.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		f, err := root.Open(path)
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
