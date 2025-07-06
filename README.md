# HPA Monitor

[![Go Version](https://img.shields.io/badge/go-1.24+-000000?style=flat-square&logo=go&logoColor=white)](go.mod)
[![GitHub release](https://img.shields.io/github/v/release/younsl/hpa-monitor?style=flat-square&color=black&logo=github&logoColor=white&label=release)](https://github.com/younsl/hpa-monitor/releases)
[![License](https://img.shields.io/github/license/younsl/hpa-monitor?style=flat-square&color=black&logo=github&logoColor=white)](LICENSE)

A Kubernetes HPA (Horizontal Pod Autoscaler) monitoring application with real-time dashboard and WebSocket updates.

![HPA Monitor Dashboard](docs/1.png)

The dashboard provides real-time monitoring of HPA resources with tolerance calculations, showing current vs desired replicas, scaling status, and recent events.

## Features

- Real-time HPA monitoring with tolerance calculations using native Kubernetes API
- Multi-metric support (CPU, memory, custom metrics)
- WebSocket-based live updates
- Kubernetes events tracking
- Web dashboard interface

## Quick Start

### Prerequisites

- Go 1.24+
- Docker
- Kubernetes cluster (or KWOK for testing)
- Helm 3.x

### Development

```bash
# Run locally
make run

# Build binary
make build

# Run tests
make test
```

### Deployment

#### Helm Installation

HPA Monitor provides a Helm chart for easy deployment to Kubernetes clusters with RBAC configuration, customizable values, and follows Kubernetes best practices.

```bash
# Install from a specific values file
helm upgrade --install hpa-monitor ./charts/hpa-monitor \
  --namespace hpa-monitor \
  --create-namespace \
  --values ./charts/hpa-monitor/values.yaml
```

#### Quick Deployment

```bash
# Deploy to KWOK cluster (recommended for testing)
make deploy-kwok

# Or deploy to existing cluster
make deploy
```

### Access

For local development, run `go run cmd/hpa-monitor/main.go` and access the dashboard at http://localhost:8080.

For Kubernetes deployments, use port-forwarding to access the dashboard:

```bash
# Port-forward to access the dashboard
kubectl port-forward svc/hpa-monitor 8080:8080 -n hpa-monitor
# Then access at http://localhost:8080
```

## Configuration

Environment variables:
- `PORT` - Server port (default: 8080)
- `WEBSOCKET_INTERVAL` - Update interval in seconds (default: 5)
- `TOLERANCE` - HPA tolerance percentage, 0.1 means 10% (default: 0.1) - [Kubernetes HPA Tolerance](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#tolerance)
- `LOG_LEVEL` - Log level: debug, info, warn, error, fatal, panic (default: info)

## Commands

```bash
make run              # Run locally
make build            # Build binary
make test             # Run tests
make docker-build     # Build container
make deploy           # Deploy to cluster
make check-hpa        # Check HPA status
make logs             # View logs
make clean            # Clean artifacts
```

## Architecture

HPA Monitor is built with a lightweight Go backend that directly interfaces with the Kubernetes API to monitor HPA resources and stream real-time data to a responsive web dashboard.

- **Backend**: Go with Gin framework for HTTP server and WebSocket-based real-time HPA data streaming
- **Frontend**: HTML/JavaScript with WebSocket
- **Deployment**: Helm chart with RBAC
- **Monitoring**: Kubernetes client-go with custom [tolerance logic](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#tolerance)
