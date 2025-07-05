package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"hpa-monitor/pkg/config"
	"hpa-monitor/pkg/logger"
	"hpa-monitor/pkg/monitor"
)

// Server handles HTTP requests and WebSocket connections
type Server struct {
	hpaMonitor *monitor.HPAMonitor
	config     *config.Config
	upgrader   websocket.Upgrader
}

// NewServer creates a new server instance
func NewServer(hpaMonitor *monitor.HPAMonitor, cfg *config.Config) *Server {
	log := logger.GetLogger()
	
	server := &Server{
		hpaMonitor: hpaMonitor,
		config:     cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	
	log.WithField("port", cfg.Port).Info("Server instance created")
	return server
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes(r *gin.Engine) {
	// Serve static files
	r.Static("/static", "./web/static")
	r.LoadHTMLGlob("web/templates/*")

	// Routes
	r.GET("/", s.handleIndex)
	r.GET("/api/hpa", s.handleHTTP)
	r.GET("/api/config", s.handleConfig)
	r.GET("/ws", s.handleWebSocket)
	r.GET("/health", s.handleHealth)
}

// handleIndex serves the main dashboard page
func (s *Server) handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// handleHTTP handles HTTP API requests for HPA status
func (s *Server) handleHTTP(c *gin.Context) {
	log := logger.GetLogger()
	ctx := context.Background()
	
	hpaStatuses, err := s.hpaMonitor.GetHPAStatus(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get HPA status via HTTP API")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.WithField("hpa_count", len(hpaStatuses)).Debug("HTTP API request completed")
	c.JSON(http.StatusOK, hpaStatuses)
}

// handleConfig handles configuration API requests
func (s *Server) handleConfig(c *gin.Context) {
	configResponse := gin.H{
		"websocketInterval": s.config.WebSocketInterval,
		"tolerance":         s.config.Tolerance,
	}
	c.JSON(http.StatusOK, configResponse)
}

// handleWebSocket handles WebSocket connections for real-time updates
func (s *Server) handleWebSocket(c *gin.Context) {
	log := logger.GetLogger()
	clientIP := c.ClientIP()
	
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.WithFields(logger.Fields{
			"client_ip": clientIP,
		}).WithError(err).Error("Failed to upgrade websocket connection")
		return
	}
	defer conn.Close()

	log.WithField("client_ip", clientIP).Info("WebSocket connection established")

	ticker := time.NewTicker(time.Duration(s.config.WebSocketInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			hpaStatuses, err := s.hpaMonitor.GetHPAStatus(ctx)
			if err != nil {
				log.WithFields(logger.Fields{
					"client_ip": clientIP,
				}).WithError(err).Error("Error getting HPA status for websocket")
				continue
			}

			if err := conn.WriteJSON(hpaStatuses); err != nil {
				log.WithFields(logger.Fields{
					"client_ip": clientIP,
				}).WithError(err).Error("Error writing JSON to websocket")
				return
			}
			
			log.WithFields(logger.Fields{
				"client_ip": clientIP,
				"hpa_count": len(hpaStatuses),
			}).Debug("WebSocket data sent")
		}
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log := logger.GetLogger()
	
	// Setup Gin router
	r := gin.Default()
	s.SetupRoutes(r)
	
	log.WithFields(logger.Fields{
		"port":               s.config.Port,
		"websocket_interval": s.config.WebSocketInterval,
		"tolerance":          s.config.Tolerance,
	}).Info("Starting HPA Monitor server")
	
	return r.Run(":" + s.config.Port)
}