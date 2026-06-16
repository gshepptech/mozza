package k8s

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildNetworkPolicy generates a Kubernetes NetworkPolicy for the given slice.
// Returns nil if no NetworkPolicy is configured.
func BuildNetworkPolicy(appName string, s plan.Slice, namespace string) *networkingv1.NetworkPolicy {
	if s.NetworkPolicy == nil {
		return nil
	}

	lbls := labels(appName, s.Name)

	np := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: lbls,
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	// DenyAll with no rules means block everything.
	if s.NetworkPolicy.DenyAll && len(s.NetworkPolicy.AllowFrom) == 0 && len(s.NetworkPolicy.AllowNamespace) == 0 {
		np.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{}
		return np
	}

	var ingressRules []networkingv1.NetworkPolicyIngressRule

	// AllowFrom: pod selector matching allowed slice labels.
	for _, from := range s.NetworkPolicy.AllowFrom {
		rule := networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							labelComponent: from,
						},
					},
				},
			},
		}
		ingressRules = append(ingressRules, rule)
	}

	// AllowNamespace: namespace selector.
	for _, ns := range s.NetworkPolicy.AllowNamespace {
		rule := networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": ns,
						},
					},
				},
			},
		}
		ingressRules = append(ingressRules, rule)
	}

	np.Spec.Ingress = ingressRules
	return np
}
