# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HPA Monitor is a Kubernetes HPA (Horizontal Pod Autoscaler) monitoring application built with Go and deployed via Helm. It provides real-time monitoring of HPA resources with a web dashboard and WebSocket-based updates.

## Key Architecture

- **Backend**: Go with Gin framework (`pkg/server/server.go`)
- **HPA Monitoring**: Custom monitor package (`pkg/monitor/monitor.go`) using Kubernetes client-go
- **Frontend**: Standalone HTML file with separate CSS (`web/index.html`, `web/style.css`)
- **Deployment**: Helm chart (`charts/hpa-monitor/`) with RBAC and service configurations
- **Testing**: KWOK cluster support for local development

## Package Structure

- `cmd/hpa-monitor/main.go` - Application entry point
- `pkg/config/` - Configuration management
- `pkg/k8s/` - Kubernetes client setup
- `pkg/logger/` - Structured logging with logrus
- `pkg/monitor/` - Core HPA monitoring logic with tolerance calculations
- `pkg/server/` - HTTP server and WebSocket handler with version endpoint

## Common Commands

### Development
```bash
# Run locally
make run

# Build binary
make build

# Run tests
make test

# Clean build artifacts
make clean
```

### Docker & Deployment
```bash
# Build and deploy to KWOK cluster (full workflow)
make deploy-kwok

# Individual steps
make docker-build      # Build container image
make docker-load       # Load to KWOK cluster
make deploy            # Deploy to Kubernetes
make test-resources    # Create test HPA resources
```

### Monitoring & Debugging
```bash
# Check HPA status
make check-hpa

# View application logs
make logs

# Check deployment status
make status

# Clean up KWOK cluster
make clean-kwok
```

### Version Management
When releasing new versions:
1. Update `charts/hpa-monitor/Chart.yaml` (both `version` and `appVersion`)
2. Build process automatically injects version info via `-ldflags`
3. Version available at `/api/version` endpoint

## Key Features

- **Tolerance Calculation**: Applies 10% tolerance to min/max replicas (`ToleranceAdjustedMin`, `ToleranceAdjustedMax`)
- **Multi-metric Support**: Handles CPU, memory, and custom metrics
- **Real-time Updates**: WebSocket connections with configurable intervals
- **Event Tracking**: Fetches and displays Kubernetes events for HPAs
- **Stabilization Checks**: Tracks scale-up (3min) and scale-down (5min) stabilization

## Configuration

Configuration is handled through environment variables and defaults in `pkg/config/config.go`:
- `PORT` - Server port (default: 8080)
- `WEBSOCKET_INTERVAL` - WebSocket update interval in seconds (default: 5)
- `TOLERANCE` - HPA tolerance percentage (default: 0.1)
- `LOG_LEVEL` - Log level: debug, info, warn, error, fatal, panic (default: info)

## Deployment Notes

- Uses RBAC with minimal permissions (ClusterRole for HPA read access)
- Runs as non-root user in container
- Supports both NodePort (30080) and port-forward access
- Helm chart includes PodDisruptionBudget and configurable resource limits
- Designed for KWOK cluster testing but works with real Kubernetes clusters
- Build process supports version injection via Docker build args