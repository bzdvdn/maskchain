package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
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

// @sk-task 10-gateway-skeleton#T2.1: Implement Server struct with New/Start/Shutdown (AC-001, AC-002, AC-003, AC-005)
// @sk-task 61-observability#T2.1: Add OTel and metrics middleware (AC-001, AC-002, AC-003)
type Server struct {
	engine         *gin.Engine
	http           *http.Server
	cfg            *config.ServerConfig
	log            *zap.Logger
	serviceName    string
	metricsHandler gin.HandlerFunc
}

// @sk-task 61-observability#T2.1: Add OTel and metrics middleware to server (AC-001, AC-002, AC-003)
func New(cfg *config.ServerConfig, log *zap.Logger, serviceName string) *Server {
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

	return &Server{
		engine:      engine,
		cfg:         cfg,
		log:         log,
		serviceName: serviceName,
	}
}

func healthHandler(status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": status})
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

// @sk-task 60-audit-incidents#T2.3: Register incident routes with /export before /:id (AC-001, AC-002, AC-003)
func (s *Server) RegisterIncidentHandler(h *incident.Handler) {
	group := s.engine.Group("/api/v1/incidents")
	group.GET("/export", h.ExportIncidents)
	group.GET("", h.ListIncidents)
	group.GET("/:id", h.GetIncident)
}

// @sk-task 40-profiles-api#T4.1: Register profile routes (AC-001..AC-011)
func (s *Server) RegisterProfileHandler(h *profile.ProfileHandler) {
	group := s.engine.Group("/api/v1/profiles")
	group.POST("", h.CreateProfile)
	group.GET("", h.ListProfiles)
	group.GET("/:slug", h.GetProfile)
	group.PUT("/:slug", h.UpdateProfile)
	group.DELETE("/:slug", h.DeleteProfile)
	group.PATCH("/:slug/dictionary", h.PatchDictionary)
}

// @sk-task 70-routing-engine#T3.2: Register proxy routes with routing handler (AC-003, AC-004)
func (s *Server) RegisterProxyRoute(shieldMiddleware gin.HandlerFunc, routingHandler *RoutingProxyHandler) {
	group := s.engine.Group("/v1")
	if routingHandler != nil {
		group.POST("/chat/completions", shieldMiddleware, routingHandler.HandleChatCompletion)
	} else {
		group.POST("/chat/completions", shieldMiddleware, ProxyChatCompletionHandler)
	}
	group.POST("/completions", shieldMiddleware, ProxyCompletionHandler)
}

// @sk-task 41-profiles-ui#T1.2: Register SPA static files handler (AC-001)
func (s *Server) RegisterStaticFiles(fsys fs.FS) {
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

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.http = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	s.log.Info("server starting", zap.String("addr", addr))
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down server")
	return s.http.Shutdown(ctx)
}
