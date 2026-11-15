package server

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// kubeClient returns a lazily-initialized Kubernetes clientset.
func (s *Server) kubeClient() (kubernetes.Interface, error) {
	if s.k8sClient != nil {
		return s.k8sClient, nil
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeClient: %w", err)
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("kubeClient: %w", err)
	}

	s.k8sClient = cs
	return cs, nil
}
