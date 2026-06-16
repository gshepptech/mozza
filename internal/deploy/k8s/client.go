// Package k8s implements the deploy.Deployer interface using the Kubernetes
// client-go library with server-side apply.
package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// clientConfig resolves cluster credentials.
// Priority: explicit context flag > kubeconfig (KUBECONFIG env or ~/.kube/config).
func clientConfig(contextOverride string) (*rest.Config, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	if contextOverride != "" {
		overrides.CurrentContext = contextOverride
	}

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("clientConfig: load kubeconfig: %w", err)
	}

	currentContext := rawConfig.CurrentContext
	if contextOverride != "" {
		currentContext = contextOverride
	}

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("clientConfig: build rest config: %w", err)
	}

	return restConfig, currentContext, nil
}

// newClientset creates a Kubernetes clientset from the resolved config.
func newClientset(contextOverride string) (kubernetes.Interface, string, error) {
	config, ctx, err := clientConfig(contextOverride)
	if err != nil {
		return nil, "", err
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("newClientset: %w", err)
	}

	return cs, ctx, nil
}
