package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"hpa-monitor/pkg/logger"
)

// NewClient creates a new Kubernetes client
func NewClient() (kubernetes.Interface, error) {
	log := logger.GetLogger()
	var config *rest.Config
	var err error

	// Try in-cluster config first
	log.Debug("Attempting to use in-cluster Kubernetes configuration")
	config, err = rest.InClusterConfig()
	if err != nil {
		log.Debug("In-cluster config not available, falling back to kubeconfig")
		
		// Fall back to kubeconfig
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
			kubeconfig = kubeconfigPath
		}

		log.WithField("kubeconfig_path", kubeconfig).Debug("Using kubeconfig file")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.WithError(err).Error("Failed to build Kubernetes config")
			return nil, fmt.Errorf("failed to build config: %v", err)
		}
	} else {
		log.Info("Using in-cluster Kubernetes configuration")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Error("Failed to create Kubernetes client")
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	log.WithField("host", config.Host).Info("Kubernetes client created successfully")
	return client, nil
}