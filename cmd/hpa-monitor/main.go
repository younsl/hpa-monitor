package main

import (
	"hpa-monitor/pkg/config"
	"hpa-monitor/pkg/k8s"
	"hpa-monitor/pkg/logger"
	"hpa-monitor/pkg/monitor"
	"hpa-monitor/pkg/server"
)

func main() {
	// Load configuration (this initializes the logger)
	cfg := config.NewConfig()
	log := logger.GetLogger()

	log.Info("HPA Monitor starting up")

	// Create Kubernetes client
	client, err := k8s.NewClient()
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kubernetes client")
	}
	log.Info("Kubernetes client created successfully")

	// Create HPA monitor
	hpaMonitor := monitor.NewHPAMonitor(client)
	hpaMonitor.SetTolerance(cfg.Tolerance)
	log.WithField("tolerance", cfg.Tolerance).Info("HPA monitor created")

	// Create and start server
	srv := server.NewServer(hpaMonitor, cfg)
	log.Info("Server components initialized")

	// Start server
	if err := srv.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}
}