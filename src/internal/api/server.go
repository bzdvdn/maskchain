package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/api/handler/profile"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 10-gateway-skeleton#T2.1: Implement Server struct with New/Start/Shutdown (AC-001, AC-002, AC-003, AC-005)
type Server struct {
	engine *gin.Engine
	http   *http.Server
	cfg    *config.ServerConfig
	log    *zap.Logger
}

func New(cfg *config.ServerConfig, log *zap.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(log))
	engine.Use(middleware.Recovery(log))
	engine.Use(middleware.CORS(cfg.CORSOrigins))
	engine.Use(middleware.ErrorHandler())

	engine.GET("/health", healthHandler("ok"))
	engine.GET("/ready", healthHandler("ok"))
	engine.GET("/live", healthHandler("alive"))

	return &Server{
		engine: engine,
		cfg:    cfg,
		log:    log,
	}
}

func healthHandler(status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": status})
	}
}

// @sk-task 22-shield-mask-storage#T4.2: Register mask routes (AC-002, AC-003)
func (s *Server) RegisterMaskHandler(h *MaskHandler) {
	s.engine.POST("/api/v1/shield/mask", h.HandleMask)
	s.engine.POST("/api/v1/shield/unmask", h.HandleUnmask)
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
