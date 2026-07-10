package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
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
