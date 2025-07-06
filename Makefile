.PHONY: build run test clean deploy undeploy test-resources

# Docker image name
IMAGE_NAME = hpa-monitor
IMAGE_TAG = latest

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -X 'hpa-monitor/pkg/server.Version=$(VERSION)'

# Build the Go application
build:
	go mod tidy
	go build -ldflags "$(LDFLAGS)" -o bin/hpa-monitor .

# Run the application locally
run:
	go run -ldflags "$(LDFLAGS)" .

# Test the application
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
	podman rmi $(IMAGE_NAME):$(IMAGE_TAG) || true

# Build Docker image
docker-build:
	podman build --build-arg VERSION=$(VERSION) -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Load Docker image to KWOK cluster
docker-load:
	kwokctl load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name hpa-test

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s/

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f k8s/ || true

# Create test resources
test-resources:
	kubectl apply -f test/test-app.yaml

# Remove test resources
clean-test-resources:
	kubectl delete -f test/test-app.yaml || true

# Full deployment to KWOK cluster
deploy-kwok: docker-build docker-load deploy test-resources
	@echo "Deployment completed!"
	@echo "Access the application at: http://localhost:30080"
	@echo "Use 'kubectl port-forward svc/hpa-monitor 8080:80 -n hpa-monitor' for port forwarding"

# Clean up KWOK cluster
clean-kwok: undeploy clean-test-resources
	@echo "Cleanup completed!"

# Check HPA status
check-hpa:
	kubectl get hpa --all-namespaces

# Show application logs
logs:
	kubectl logs -f deployment/hpa-monitor -n hpa-monitor

# Show application status
status:
	kubectl get pods,svc -n hpa-monitor
	kubectl get hpa --all-namespaces