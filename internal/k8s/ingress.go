package k8s

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildIngress generates a typed Kubernetes Ingress for a public slice.
func BuildIngress(s plan.Slice, namespace string, appName string) *networkingv1.Ingress {
	lbls := labels(appName, s.Name)

	path := "/" + s.Name
	var host string
	if s.Domain != "" {
		host = s.Domain
		path = "/"
	}

	pathType := networkingv1.PathTypePrefix

	return &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: s.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(s.Port),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
